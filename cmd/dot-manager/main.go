// Package main provides the CLI entry point for dot-manager.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/AntoineGS/dot-manager/internal/config"
	"github.com/AntoineGS/dot-manager/internal/manager"
	"github.com/AntoineGS/dot-manager/internal/packages"
	"github.com/AntoineGS/dot-manager/internal/platform"
	"github.com/AntoineGS/dot-manager/internal/tui"
	"github.com/spf13/cobra"
)

var (
	configDir   string // Override from --dir flag
	osOverride  string
	dryRun      bool
	verbose     bool
	interactive bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "dot-manager",
		Short: "Manage dotfiles and configurations across platforms",
		Long: `dot-manager is a cross-platform tool for managing dotfiles and configurations.
It supports backup and restore operations using symlinks, with support for
both Windows and Linux systems.

Configuration is stored in two places:
  ~/.config/dot-manager/config.yaml  - Points to your configurations repo
  <repo>/dot-manager.yaml            - Defines paths to manage

Run 'dot-manager init <path>' to set up the app configuration.
Run without arguments to start the interactive TUI.`,
		RunE: runInteractive,
	}

	rootCmd.PersistentFlags().StringVarP(&configDir, "dir", "d", "", "Override configurations directory (ignores app config)")
	rootCmd.PersistentFlags().StringVarP(&osOverride, "os", "o", "", "Override OS detection (linux or windows)")
	rootCmd.PersistentFlags().BoolVarP(&dryRun, "dry-run", "n", false, "Show what would be done without making changes")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")

	initCmd := &cobra.Command{
		Use:   "init <path>",
		Short: "Initialize app configuration",
		Long: `Initialize the app configuration by setting the path to your configurations repository.

This creates ~/.config/dot-manager/config.yaml with the path to your repo.
The repo should contain a dot-manager.yaml file with your path definitions.`,
		Args: cobra.ExactArgs(1),
		RunE: runInit,
	}

	restoreCmd := &cobra.Command{
		Use:   "restore",
		Short: "Restore configurations by creating symlinks",
		Long:  `Restore configurations by creating symlinks from target locations to backup sources.`,
		RunE:  runRestore,
	}
	restoreCmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Run in interactive mode")

	backupCmd := &cobra.Command{
		Use:   "backup",
		Short: "Backup configurations from target locations",
		Long:  `Copy configuration files from target locations to backup directory.`,
		RunE:  runBackup,
	}
	backupCmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Run in interactive mode")

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all configured paths",
		Long:  `Display all configured paths and their targets for the current OS.`,
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

	rootCmd.AddCommand(initCmd, restoreCmd, backupCmd, listCmd, installCmd, listPkgsCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runInit(_ *cobra.Command, args []string) error {
	path := args[0]

	// Expand ~ if present
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("getting home directory: %w", err)
		}
		path = filepath.Join(home, path[1:])
	}

	// Make absolute
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolving path: %w", err)
	}

	// Check directory exists
	info, err := os.Stat(absPath)
	if os.IsNotExist(err) {
		return fmt.Errorf("directory does not exist: %s", absPath)
	}
	if !info.IsDir() {
		return fmt.Errorf("not a directory: %s", absPath)
	}

	// Check for dot-manager.yaml in the repo
	repoConfig := filepath.Join(absPath, "dot-manager.yaml")
	if _, err := os.Stat(repoConfig); os.IsNotExist(err) {
		fmt.Printf("Warning: %s not found in %s\n", "dot-manager.yaml", absPath)
		fmt.Println("You'll need to create it before using dot-manager.")
	}

	// Save app config
	appCfg := &config.AppConfig{
		ConfigDir: absPath,
	}

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

	configFile := filepath.Join(cfgDir, "dot-manager.yaml")
	cfg, err := config.Load(configFile)
	if err != nil {
		return nil, nil, "", fmt.Errorf("loading config from %s: %w", configFile, err)
	}

	// If backup_root is empty or ".", use the config directory
	if cfg.BackupRoot == "" || cfg.BackupRoot == "." {
		cfg.BackupRoot = cfgDir
	} else if !filepath.IsAbs(cfg.BackupRoot) && !strings.HasPrefix(cfg.BackupRoot, "~") {
		// Make relative paths relative to the config directory
		// Note: paths starting with ~ are kept as-is (they'll be expanded during operations)
		cfg.BackupRoot = filepath.Join(cfgDir, cfg.BackupRoot)
	}

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

func createManager() (*manager.Manager, error) {
	cfg, plat, _, err := loadConfig()
	if err != nil {
		return nil, err
	}

	fmt.Printf("Detected OS: %s\n", plat.OS)
	fmt.Printf("Config directory: %s\n", cfg.BackupRoot)

	mgr := manager.New(cfg, plat)
	mgr.DryRun = dryRun
	mgr.Verbose = verbose

	return mgr, nil
}

func runInteractive(_ *cobra.Command, _ []string) error {
	cfg, plat, configPath, err := loadConfig()
	if err != nil {
		return err
	}

	// Check if we're in a terminal
	if !tui.IsTerminal() {
		return fmt.Errorf("interactive mode requires a terminal; use subcommands (restore, backup, list) for non-interactive use")
	}

	return tui.Run(cfg, plat, dryRun, configPath)
}

func runRestore(cmd *cobra.Command, args []string) error {
	if interactive {
		return runInteractive(cmd, args)
	}

	mgr, err := createManager()
	if err != nil {
		return err
	}

	if dryRun {
		fmt.Println("=== DRY RUN MODE ===")
	}

	return runRestoreWithManager(mgr)
}

func runRestoreWithManager(m manager.Restorer) error {
	// Create context with cancellation support
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nOperation canceled by user")
		cancel()
	}()

	return m.RestoreWithContext(ctx)
}

func runBackup(cmd *cobra.Command, args []string) error {
	if interactive {
		return runInteractive(cmd, args)
	}

	mgr, err := createManager()
	if err != nil {
		return err
	}

	if dryRun {
		fmt.Println("=== DRY RUN MODE ===")
	}

	return runBackupWithManager(mgr)
}

func runBackupWithManager(m manager.Backuper) error {
	// Create context with cancellation support
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nOperation canceled by user")
		cancel()
	}()

	return m.BackupWithContext(ctx)
}

func runList(_ *cobra.Command, _ []string) error {
	mgr, err := createManager()
	if err != nil {
		return err
	}

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

	// Create filter context from platform
	filterCtx := &config.FilterContext{
		OS:       plat.OS,
		Distro:   plat.Distro,
		Hostname: plat.Hostname,
		User:     plat.User,
	}

	// Get filtered package entries
	packageEntries := cfg.GetFilteredPackages(filterCtx)
	if len(packageEntries) == 0 {
		return fmt.Errorf("no matching packages configured in dot-manager.yaml")
	}

	// Create package manager
	pkgMgr := packages.NewManager(&packages.Config{
		Packages:        packages.FromEntries(packageEntries),
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

	// Create filter context from platform
	filterCtx := &config.FilterContext{
		OS:       plat.OS,
		Distro:   plat.Distro,
		Hostname: plat.Hostname,
		User:     plat.User,
	}

	// Get filtered package entries
	packageEntries := cfg.GetFilteredPackages(filterCtx)
	if len(packageEntries) == 0 {
		fmt.Println("No matching packages configured in dot-manager.yaml")
		return nil
	}

	// Create package manager to determine install methods
	pkgMgr := packages.NewManager(&packages.Config{
		Packages:        packages.FromEntries(packageEntries),
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
