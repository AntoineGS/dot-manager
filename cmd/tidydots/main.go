// Package main provides the CLI entry point for tidydots.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/pprof"
	"syscall"

	"github.com/AntoineGS/tidydots/internal/config"
	"github.com/AntoineGS/tidydots/internal/manager"
	"github.com/AntoineGS/tidydots/internal/packages"
	"github.com/AntoineGS/tidydots/internal/platform"
	"github.com/AntoineGS/tidydots/internal/preview"
	tmpl "github.com/AntoineGS/tidydots/internal/template"
	"github.com/AntoineGS/tidydots/internal/tui"
	"github.com/spf13/cobra"
)

var version = "dev"

var (
	configDir     string // Override from --dir flag
	osOverride    string
	dryRun        bool
	verbose       bool
	interactive   bool
	actions       bool
	statusActions bool
	statusJSON    bool
	noMerge       bool
	forceDelete   bool
	forceRender   bool
	cpuProfile    string
	logFile       *os.File
)

var interactiveIsTerminal = tui.IsTerminal

var runInteractiveTUI = func(
	cfg *config.Config,
	plat *platform.Platform,
	dryRun bool,
	configPath string,
	actionFilterEnabled bool,
) error {
	if actionFilterEnabled {
		return tui.RunWithActionFilter(cfg, plat, dryRun, configPath)
	}
	return tui.Run(cfg, plat, dryRun, configPath)
}

func main() {
	rootCmd := newRootCommand()

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:     "tidydots",
		Version: version,
		Short:   "Manage dotfiles and configurations across platforms",
		Args:    cobra.NoArgs,
		Long: `tidydots is a cross-platform tool for managing dotfiles and configurations.
It supports backup and restore operations using symlinks, with support for
both Windows and Linux systems.

Configuration is stored in two places:
  ~/.config/tidydots/config.yaml  - Points to your configurations repo
  <repo>/tidydots.yaml            - Defines paths to manage

Run 'tidydots init <path>' to set up the app configuration.
Run without arguments to start the interactive TUI.`,
		RunE: runInteractive,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if actions && cmd.Name() != "tidydots" {
				return fmt.Errorf("--actions cannot be used with %s; use it with tidydots or status --actions", cmd.Name())
			}
			if verbose {
				logWriter := os.Stderr
				// When running interactively (TUI), write logs to a file to avoid corrupting the display
				if tui.IsTerminal() {
					logPath := filepath.Join(os.TempDir(), "tidydots.log")
					f, err := os.Create(logPath)
					if err == nil {
						logFile = f
						logWriter = f
						fmt.Fprintf(os.Stderr, "Verbose logs: %s\n", logPath)
					}
				}
				slog.SetDefault(slog.New(slog.NewTextHandler(logWriter, &slog.HandlerOptions{
					Level: slog.LevelDebug,
				})))
			}
			if cpuProfile != "" {
				f, err := os.Create(filepath.Clean(cpuProfile))
				if err != nil {
					return fmt.Errorf("creating CPU profile: %w", err)
				}
				if err := pprof.StartCPUProfile(f); err != nil {
					_ = f.Close()
					return fmt.Errorf("starting CPU profile: %w", err)
				}
			}
			return nil
		},
		PersistentPostRun: func(_ *cobra.Command, _ []string) {
			if logFile != nil {
				_ = logFile.Close()
			}
			if cpuProfile != "" {
				pprof.StopCPUProfile()
			}
		},
	}

	rootCmd.PersistentFlags().StringVarP(&configDir, "dir", "d", "", "Override repository directory, including TUI hostname choices")
	rootCmd.PersistentFlags().StringVarP(&osOverride, "os", "o", "", "Override OS detection (linux or windows)")
	rootCmd.PersistentFlags().BoolVarP(&dryRun, "dry-run", "n", false, "Show what would be done without making changes")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.Flags().BoolVar(&actions, "actions", false, "Start the interactive TUI with action filtering enabled")
	rootCmd.PersistentFlags().StringVar(&cpuProfile, "cpuprofile", "", "Write CPU profile to file (e.g. cpu.prof)")
	_ = rootCmd.PersistentFlags().MarkHidden("cpuprofile")

	initCmd := &cobra.Command{
		Use:   "init <path>",
		Short: "Initialize app configuration",
		Long: `Initialize the app configuration by setting the path to your configurations repository.

This creates ~/.config/tidydots/config.yaml with the path to your repo.
The repo should contain a tidydots.yaml file with your path definitions.`,
		Args: cobra.ExactArgs(1),
		RunE: runInit,
	}

	restoreCmd := &cobra.Command{
		Use:   "restore [app [entry]]",
		Short: "Restore configurations by creating symlinks",
		Long:  `Restore configurations by creating symlinks from target locations to backup sources. Optionally target one application or one entry.`,
		Args:  validateTargetArgs,
		RunE:  runRestore,
	}
	restoreCmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Run in interactive mode")
	restoreCmd.Flags().BoolVar(&noMerge, "no-merge", false, "Disable merge mode, return error if target exists")
	restoreCmd.Flags().BoolVar(&forceDelete, "force", false, "When combined with --no-merge, delete existing files without prompting")
	restoreCmd.Flags().BoolVar(&forceRender, "force-render", false, "Force re-render of templates, skipping 3-way merge")

	backupCmd := &cobra.Command{
		Use:   "backup [app [entry]]",
		Short: "Backup configurations from target locations",
		Long:  `Copy configuration files from target locations to backup directory. Optionally target one application or one config entry.`,
		Args:  validateTargetArgs,
		RunE:  runBackup,
	}
	backupCmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Run in interactive mode")

	listCmd := &cobra.Command{
		Use:   "list [app [entry]]",
		Short: "List configured paths",
		Long:  `Display configured paths and their targets for the current OS. Optionally target one application or one config entry.`,
		Args:  validateTargetArgs,
		RunE:  runList,
	}

	installCmd := &cobra.Command{
		Use:   "install [package-names...]",
		Short: "Install packages using configured package managers",
		Long: `Install packages from your configuration using the appropriate package manager.
If no package names are provided, all matching packages will be installed.
Packages are filtered based on their filters (os, hostname, user).`,
		RunE: runInstall,
	}
	installCmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Run in interactive mode")

	listPkgsCmd := &cobra.Command{
		Use:   "list-packages",
		Short: "List all configured packages",
		Long:  `Display all configured packages and their installation methods for the current OS.`,
		RunE:  runListPackages,
	}

	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show resolved configuration and package status",
		Long:  "Resolve all package and entry checks and report their current status.",
		Args:  cobra.NoArgs,
		RunE:  runStatus,
	}
	statusCmd.Flags().BoolVar(&statusActions, "actions", false, "Show only applications and entries needing action")
	statusCmd.Flags().BoolVar(&statusJSON, "json", false, "Output status as JSON")

	previewCmd := &cobra.Command{
		Use:   "preview <path>",
		Short: "Live preview template rendering",
		Long: `Watch .tmpl files for changes and render them to .tmpl.rendered in real time.

Open the .tmpl file and its .tmpl.rendered file side by side in your editor
for a live preview experience. The rendered file updates on each save.

The path can be a single .tmpl file or a directory containing .tmpl files.`,
		Args: cobra.ExactArgs(1),
		RunE: runPreview,
	}

	rootCmd.AddCommand(initCmd, restoreCmd, backupCmd, listCmd, installCmd, listPkgsCmd, statusCmd, previewCmd)

	return rootCmd
}

func runInit(_ *cobra.Command, args []string) error {
	path := args[0]

	// Expand ~ if present
	path = config.ExpandPath(path, nil)

	// Make absolute
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolving path: %w", err)
	}

	// Check directory exists
	info, err := os.Stat(absPath)
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("directory does not exist %s: %w", absPath, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("not a directory: %s", absPath)
	}

	// Check for tidydots.yaml in the repo
	repoConfig := filepath.Join(absPath, "tidydots.yaml")
	if _, err := os.Stat(repoConfig); errors.Is(err, os.ErrNotExist) {
		fmt.Printf("Warning: %s not found in %s\n", "tidydots.yaml", absPath)
		fmt.Println("You'll need to create it before using tidydots.")
	}

	// Save app config
	appCfg, err := config.LoadAppConfigMetadata()
	if err != nil {
		appCfg = &config.AppConfig{}
	}
	appCfg.ConfigDir = absPath

	if err := config.SaveAppConfig(appCfg); err != nil {
		return fmt.Errorf("saving app config: %w", err)
	}

	fmt.Printf("App configuration saved to %s\n", config.AppConfigPath())
	fmt.Printf("Configurations directory: %s\n", absPath)

	return nil
}

func getConfigDir() (string, error) {
	// 1. Use --dir flag if provided
	if configDir != "" {
		absPath, err := filepath.Abs(configDir)
		if err != nil {
			return "", fmt.Errorf("invalid config directory: %w", err)
		}
		return absPath, nil
	}

	// 2. Load from app config
	appCfg, err := config.LoadAppConfig()
	if err != nil {
		return "", err
	}

	return appCfg.ConfigDir, nil
}

func loadConfig() (*config.Config, *platform.Platform, string, error) {
	cfgDir, err := getConfigDir()
	if err != nil {
		return nil, nil, "", err
	}

	configFile := filepath.Join(cfgDir, "tidydots.yaml")
	cfg, err := config.Load(configFile)
	if err != nil {
		return nil, nil, "", fmt.Errorf("loading config from %s: %w", configFile, err)
	}

	cfg.BackupRoot = cfgDir

	plat := platform.Detect()

	if osOverride != "" {
		if osOverride != platform.OSLinux && osOverride != platform.OSWindows {
			return nil, nil, "", fmt.Errorf("invalid OS override: %s (must be 'linux' or 'windows')", osOverride)
		}
		plat = plat.WithOS(osOverride)
	}

	// Paths are kept with ~ in the config for portability
	// They will be expanded when needed for file operations

	return cfg, plat, configFile, nil
}

func createManager(args []string, allowSetup bool) (*manager.Manager, error) {
	cfg, plat, _, err := loadConfig()
	if err != nil {
		return nil, err
	}

	tmplCtx := tmpl.NewContextFromPlatform(plat)
	engine := tmpl.NewEngine(tmplCtx)
	cfg, err = selectConfigTarget(cfg, engine, args, allowSetup)
	if err != nil {
		return nil, err
	}

	fmt.Printf("Detected OS: %s\n", plat.OS)
	fmt.Printf("Config directory: %s\n", cfg.BackupRoot)

	mgr := manager.New(cfg, plat)
	mgr.DryRun = dryRun
	mgr.Verbose = verbose
	mgr.NoMerge = noMerge
	mgr.ForceDelete = forceDelete
	mgr.ForceRender = forceRender

	if err := mgr.InitStateStore(); err != nil {
		fmt.Printf("Warning: could not initialize template state store: %v\n", err)
	}

	return mgr, nil
}

var createCommandManager = createManager

func runInteractive(_ *cobra.Command, _ []string) error {
	cfg, plat, configPath, err := loadConfig()
	if err != nil {
		return err
	}

	// Check if we're in a terminal
	if !interactiveIsTerminal() {
		return fmt.Errorf("interactive mode requires a terminal; use subcommands (restore, backup, list) for non-interactive use")
	}

	return runInteractiveTUI(cfg, plat, dryRun, configPath, actions)
}

func runStatus(cmd *cobra.Command, _ []string) error {
	cfg, plat, _, err := loadConfig()
	if err != nil {
		return err
	}

	logLevel := slog.LevelInfo
	if verbose {
		logLevel = slog.LevelDebug
	}
	mgr := manager.New(cfg, plat).WithLogger(slog.New(slog.NewTextHandler(
		cmd.ErrOrStderr(),
		&slog.HandlerOptions{Level: logLevel},
	)))
	mgr.Verbose = verbose
	if err := mgr.InitStateStore(); err != nil {
		return fmt.Errorf("initializing status checks: %w", err)
	}
	defer mgr.Close() //nolint:errcheck // best-effort cleanup

	report, err := tui.ComputeStatus(cfg, plat, mgr, statusActions)
	if err != nil {
		return err
	}

	if statusJSON {
		encoder := json.NewEncoder(cmd.OutOrStdout())
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(report); err != nil {
			return fmt.Errorf("writing status JSON: %w", err)
		}
		return nil
	}

	return writeHumanStatus(cmd.OutOrStdout(), report)
}

func writeHumanStatus(out io.Writer, report tui.StatusReport) error {
	if _, err := fmt.Fprintf(out, "Actionable: %t\n", report.Actionable); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "Applications: %d (%d actionable)\n", report.Counts.Applications, report.Counts.ActionableApplications); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "Entries: %d (%d actionable)\n", report.Counts.Entries, report.Counts.ActionableEntries); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "Packages: %d (%d actionable)\n", report.Counts.Packages, report.Counts.ActionablePackages); err != nil {
		return err
	}

	for _, app := range report.Applications {
		if _, err := fmt.Fprintf(out, "\n%s: %s", app.Name, app.Status); err != nil {
			return err
		}
		if app.Actionable {
			if _, err := fmt.Fprint(out, " [actionable]"); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(out); err != nil {
			return err
		}
		for _, entry := range app.Entries {
			detail := entry.State
			if entry.Error != "" {
				detail += ": " + entry.Error
			}
			if _, err := fmt.Fprintf(out, "  %s: %s\n", entry.Name, detail); err != nil {
				return err
			}
		}
	}

	return nil
}

func runRestore(cmd *cobra.Command, args []string) error {
	if err := validateInteractiveTarget(interactive, args); err != nil {
		return err
	}
	if interactive {
		return runInteractive(cmd, args)
	}

	mgr, err := createCommandManager(args, true)
	if err != nil {
		return err
	}
	defer mgr.Close() //nolint:errcheck // best-effort cleanup

	if dryRun {
		fmt.Println("=== DRY RUN MODE ===")
	}

	return runRestoreWithManager(mgr)
}

func runRestoreWithManager(m manager.Restorer) error {
	return runWithCancellation(m.RestoreWithContext)
}

func runBackup(cmd *cobra.Command, args []string) error {
	if err := validateInteractiveTarget(interactive, args); err != nil {
		return err
	}
	if interactive {
		return runInteractive(cmd, args)
	}

	mgr, err := createCommandManager(args, false)
	if err != nil {
		return err
	}
	defer mgr.Close() //nolint:errcheck // best-effort cleanup

	if dryRun {
		fmt.Println("=== DRY RUN MODE ===")
	}

	return runBackupWithManager(mgr)
}

func runBackupWithManager(m manager.Backuper) error {
	return runWithCancellation(m.BackupWithContext)
}

// runWithCancellation runs a context-aware function with signal-based cancellation.
// It sets up SIGINT/SIGTERM handling and cancels the context when a signal is received.
func runWithCancellation(fn func(ctx context.Context) error) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	go func() {
		<-sigChan
		fmt.Println("\nOperation canceled by user")
		cancel()
	}()

	return fn(ctx)
}

func runList(_ *cobra.Command, args []string) error {
	mgr, err := createCommandManager(args, false)
	if err != nil {
		return err
	}
	defer mgr.Close() //nolint:errcheck // best-effort cleanup

	return runListWithManager(mgr)
}

func runListWithManager(m manager.Lister) error {
	return m.List()
}

func runInstall(cmd *cobra.Command, args []string) error {
	if interactive {
		return runInteractive(cmd, args)
	}

	cfg, plat, _, err := loadConfig()
	if err != nil {
		return err
	}

	fmt.Printf("Detected OS: %s\n", plat.OS)
	fmt.Printf("Config directory: %s\n", cfg.BackupRoot)

	// Create template engine for when expression evaluation
	tmplCtx := tmpl.NewContextFromPlatform(plat)
	engine := tmpl.NewEngine(tmplCtx)

	// Get filtered package entries
	packageEntries := cfg.GetFilteredPackages(engine)
	if len(packageEntries) == 0 {
		return fmt.Errorf("no matching packages configured in tidydots.yaml")
	}

	// Create package manager
	pkgMgr := packages.NewManager(&packages.Config{
		Packages:        packages.FromApplications(packageEntries),
		DefaultManager:  packages.PackageManager(cfg.DefaultManager),
		ManagerPriority: convertToPackageManagers(cfg.ManagerPriority),
	}, plat.OS, dryRun, verbose)

	fmt.Printf("Available package managers: %v\n", pkgMgr.Available)
	if pkgMgr.Preferred != "" {
		fmt.Printf("Preferred package manager: %s\n", pkgMgr.Preferred)
	}

	if dryRun {
		fmt.Println("=== DRY RUN MODE ===")
	}

	// Get installable packages
	packagesToInstall := pkgMgr.GetInstallablePackages()

	// Filter by name if args provided
	if len(args) > 0 {
		var filtered []packages.Package
		for _, pkg := range packagesToInstall {
			for _, name := range args {
				if pkg.Name == name {
					filtered = append(filtered, pkg)
					break
				}
			}
		}
		packagesToInstall = filtered
	}

	results := pkgMgr.InstallAll(packagesToInstall)

	// Print results
	successCount := 0
	failCount := 0
	for _, r := range results {
		if r.Success {
			fmt.Printf("[ok] %s: %s\n", r.Package, r.Message)
			successCount++
		} else {
			fmt.Printf("[error] %s: %s\n", r.Package, r.Message)
			failCount++
		}
	}

	fmt.Printf("\nInstallation complete: %d successful, %d failed\n", successCount, failCount)

	if failCount > 0 {
		return fmt.Errorf("%d packages failed to install", failCount)
	}
	return nil
}

func runListPackages(_ *cobra.Command, _ []string) error {
	cfg, plat, _, err := loadConfig()
	if err != nil {
		return err
	}

	// Create template engine for when expression evaluation
	tmplCtx := tmpl.NewContextFromPlatform(plat)
	engine := tmpl.NewEngine(tmplCtx)

	// Get filtered package entries
	packageEntries := cfg.GetFilteredPackages(engine)
	if len(packageEntries) == 0 {
		fmt.Println("No matching packages configured in tidydots.yaml")
		return nil
	}

	// Create package manager to determine install methods
	pkgMgr := packages.NewManager(&packages.Config{
		Packages:        packages.FromApplications(packageEntries),
		DefaultManager:  packages.PackageManager(cfg.DefaultManager),
		ManagerPriority: convertToPackageManagers(cfg.ManagerPriority),
	}, plat.OS, false, verbose)

	fmt.Printf("Available package managers: %v\n\n", pkgMgr.Available)

	for _, pkg := range pkgMgr.Config.Packages {
		method := pkgMgr.GetInstallMethod(pkg)
		canInstall := pkgMgr.CanInstall(pkg)

		status := "✓"
		if !canInstall {
			status = "✗"
			method = "unavailable"
		}

		fmt.Printf("%s %s (%s)\n", status, pkg.Name, method)
		if pkg.Description != "" {
			fmt.Printf("    %s\n", pkg.Description)
		}
	}

	return nil
}

func convertToPackageManagers(strs []string) []packages.PackageManager {
	result := make([]packages.PackageManager, 0, len(strs))
	for _, s := range strs {
		result = append(result, packages.PackageManager(s))
	}
	return result
}

func runPreview(_ *cobra.Command, args []string) error {
	_, plat, _, err := loadConfig()
	if err != nil {
		return err
	}

	tmplCtx := tmpl.NewContextFromPlatform(plat)
	engine := tmpl.NewEngine(tmplCtx)

	logger := slog.Default()
	if verbose {
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	}

	w := preview.New(engine, logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	return w.WatchWithStdin(ctx, args[0], os.Stdin)
}
