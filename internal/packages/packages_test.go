package packages

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AntoineGS/dot-manager/internal/config"
	"github.com/AntoineGS/dot-manager/internal/platform"
	"gopkg.in/yaml.v3"
)

func TestNewManager(t *testing.T) {
	tests := []struct {
		config        *Config
		name          string
		osType        string
		wantOS        string
		dryRun        bool
		verbose       bool
		wantDryRun    bool
		wantVerbose   bool
		wantConfigNil bool
	}{
		{
			name: "basic creation with empty config",
			config: &Config{
				Packages: []Package{},
			},
			osType:     "linux",
			dryRun:     false,
			verbose:    false,
			wantDryRun: false,
			wantOS:     "linux",
		},
		{
			name: "with dry-run enabled",
			config: &Config{
				Packages: []Package{},
			},
			osType:     "linux",
			dryRun:     true,
			verbose:    false,
			wantDryRun: true,
			wantOS:     "linux",
		},
		{
			name: "with verbose enabled",
			config: &Config{
				Packages: []Package{},
			},
			osType:      "windows",
			dryRun:      false,
			verbose:     true,
			wantVerbose: true,
			wantOS:      "windows",
		},
		{
			name: "with default manager set",
			config: &Config{
				Packages:       []Package{},
				DefaultManager: Pacman,
			},
			osType: "linux",
			wantOS: "linux",
		},
		{
			name: "with manager priority set",
			config: &Config{
				Packages:        []Package{},
				ManagerPriority: []PackageManager{Yay, Paru, Pacman},
			},
			osType: "linux",
			wantOS: "linux",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManager(tt.config, tt.osType, tt.dryRun, tt.verbose)

			if m == nil {
				t.Fatal("NewManager returned nil")
			}

			if m.Config != tt.config {
				t.Errorf("Config not set correctly")
			}

			if m.OS != tt.wantOS {
				t.Errorf("OS = %q, want %q", m.OS, tt.wantOS)
			}

			if m.DryRun != tt.wantDryRun {
				t.Errorf("DryRun = %v, want %v", m.DryRun, tt.wantDryRun)
			}

			if m.Verbose != tt.wantVerbose {
				t.Errorf("Verbose = %v, want %v", m.Verbose, tt.wantVerbose)
			}
		})
	}
}

func TestHasManager(t *testing.T) {
	tests := []struct {
		name      string
		check     PackageManager
		available []PackageManager
		want      bool
	}{
		{
			name:      "manager in list",
			available: []PackageManager{Pacman, Yay, Paru},
			check:     Pacman,
			want:      true,
		},
		{
			name:      "manager not in list",
			available: []PackageManager{Pacman, Yay, Paru},
			check:     Apt,
			want:      false,
		},
		{
			name:      "empty available list",
			available: []PackageManager{},
			check:     Pacman,
			want:      false,
		},
		{
			name:      "nil available list",
			available: nil,
			check:     Pacman,
			want:      false,
		},
		{
			name:      "check for yay in mixed list",
			available: []PackageManager{Apt, Dnf, Yay, Brew},
			check:     Yay,
			want:      true,
		},
		{
			name:      "windows managers",
			available: []PackageManager{Winget, Scoop, Choco},
			check:     Scoop,
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Manager{
				Config:    &Config{},
				Available: tt.available,
			}

			got := m.HasManager(tt.check)
			if got != tt.want {
				t.Errorf("HasManager(%q) = %v, want %v", tt.check, got, tt.want)
			}
		})
	}
}

func TestSelectPreferredManager(t *testing.T) {
	tests := []struct {
		name            string
		defaultManager  PackageManager
		osType          string
		wantPreferred   PackageManager
		available       []PackageManager
		managerPriority []PackageManager
	}{
		{
			name:            "priority takes precedence over default",
			available:       []PackageManager{Pacman, Yay, Paru},
			defaultManager:  Pacman,
			managerPriority: []PackageManager{Yay, Paru, Pacman},
			osType:          "linux",
			wantPreferred:   Yay,
		},
		{
			name:            "default used when no priority set",
			available:       []PackageManager{Pacman, Yay},
			defaultManager:  Pacman,
			managerPriority: nil,
			osType:          "linux",
			wantPreferred:   Pacman,
		},
		{
			name:            "auto-select linux (yay first)",
			available:       []PackageManager{Yay, Pacman},
			defaultManager:  "",
			managerPriority: nil,
			osType:          "linux",
			wantPreferred:   Yay,
		},
		{
			name:            "auto-select linux (paru when yay not available)",
			available:       []PackageManager{Paru, Pacman},
			defaultManager:  "",
			managerPriority: nil,
			osType:          "linux",
			wantPreferred:   Paru,
		},
		{
			name:            "auto-select linux (pacman when aur helpers not available)",
			available:       []PackageManager{Pacman, Apt},
			defaultManager:  "",
			managerPriority: nil,
			osType:          "linux",
			wantPreferred:   Pacman,
		},
		{
			name:            "auto-select linux (apt)",
			available:       []PackageManager{Apt, Dnf},
			defaultManager:  "",
			managerPriority: nil,
			osType:          "linux",
			wantPreferred:   Apt,
		},
		{
			name:            "auto-select linux (brew)",
			available:       []PackageManager{Brew},
			defaultManager:  "",
			managerPriority: nil,
			osType:          "linux",
			wantPreferred:   Brew,
		},
		{
			name:            "auto-select windows (winget first)",
			available:       []PackageManager{Winget, Scoop, Choco},
			defaultManager:  "",
			managerPriority: nil,
			osType:          "windows",
			wantPreferred:   Winget,
		},
		{
			name:            "auto-select windows (scoop when winget not available)",
			available:       []PackageManager{Scoop, Choco},
			defaultManager:  "",
			managerPriority: nil,
			osType:          "windows",
			wantPreferred:   Scoop,
		},
		{
			name:            "auto-select windows (choco only)",
			available:       []PackageManager{Choco},
			defaultManager:  "",
			managerPriority: nil,
			osType:          "windows",
			wantPreferred:   Choco,
		},
		{
			name:            "priority skips unavailable managers",
			available:       []PackageManager{Pacman},
			defaultManager:  "",
			managerPriority: []PackageManager{Yay, Paru, Pacman},
			osType:          "linux",
			wantPreferred:   Pacman,
		},
		{
			name:            "default skipped if not available",
			available:       []PackageManager{Apt},
			defaultManager:  Pacman,
			managerPriority: nil,
			osType:          "linux",
			wantPreferred:   Apt,
		},
		{
			name:            "no available managers",
			available:       []PackageManager{},
			defaultManager:  "",
			managerPriority: nil,
			osType:          "linux",
			wantPreferred:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Manager{
				Config: &Config{
					DefaultManager:  tt.defaultManager,
					ManagerPriority: tt.managerPriority,
				},
				OS:        tt.osType,
				Available: tt.available,
			}

			m.selectPreferredManager()

			if m.Preferred != tt.wantPreferred {
				t.Errorf("Preferred = %q, want %q", m.Preferred, tt.wantPreferred)
			}
		})
	}
}

func TestCanInstall(t *testing.T) {
	tests := []struct {
		name      string
		available []PackageManager
		osType    string
		pkg       Package
		want      bool
	}{
		{
			name:      "can install via available manager",
			available: []PackageManager{Pacman},
			osType:    "linux",
			pkg: Package{
				Name:     "vim",
				Managers: map[PackageManager]ManagerValue{Pacman: {PackageName: "vim"}},
			},
			want: true,
		},
		{
			name:      "cannot install - manager not available",
			available: []PackageManager{Apt},
			osType:    "linux",
			pkg: Package{
				Name:     "vim",
				Managers: map[PackageManager]ManagerValue{Pacman: {PackageName: "vim"}},
			},
			want: false,
		},
		{
			name:      "can install via custom command",
			available: []PackageManager{},
			osType:    "linux",
			pkg: Package{
				Name:   "custom-tool",
				Custom: map[string]string{"linux": "curl -sSL example.com | sh"},
			},
			want: true,
		},
		{
			name:      "cannot install - custom for different OS",
			available: []PackageManager{},
			osType:    "windows",
			pkg: Package{
				Name:   "custom-tool",
				Custom: map[string]string{"linux": "curl -sSL example.com | sh"},
			},
			want: false,
		},
		{
			name:      "can install via URL",
			available: []PackageManager{},
			osType:    "linux",
			pkg: Package{
				Name: "url-tool",
				URL: map[string]URLInstall{
					"linux": {URL: "https://example.com/tool", Command: "{file}"},
				},
			},
			want: true,
		},
		{
			name:      "cannot install - URL for different OS",
			available: []PackageManager{},
			osType:    "linux",
			pkg: Package{
				Name: "url-tool",
				URL: map[string]URLInstall{
					"windows": {URL: "https://example.com/tool.exe", Command: "{file}"},
				},
			},
			want: false,
		},
		{
			name:      "can install - multiple options available",
			available: []PackageManager{Pacman, Yay},
			osType:    "linux",
			pkg: Package{
				Name: "multi-tool",
				Managers: map[PackageManager]ManagerValue{
					Apt:    {PackageName: "multi-tool"},
					Pacman: {PackageName: "multi-tool"},
				},
				Custom: map[string]string{"linux": "make install"},
			},
			want: true,
		},
		{
			name:      "cannot install - no methods available",
			available: []PackageManager{},
			osType:    "linux",
			pkg: Package{
				Name: "unavailable",
			},
			want: false,
		},
		{
			name:      "empty package",
			available: []PackageManager{Pacman},
			osType:    "linux",
			pkg:       Package{Name: "empty"},
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Manager{
				ctx:       context.Background(),
				Config:    &Config{},
				OS:        tt.osType,
				Available: tt.available,
			}

			got := m.CanInstall(tt.pkg)
			if got != tt.want {
				t.Errorf("CanInstall() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetInstallMethod(t *testing.T) {
	tests := []struct {
		name      string
		osType    string
		want      string
		pkg       Package
		available []PackageManager
	}{
		{
			name:      "returns manager name",
			available: []PackageManager{Pacman},
			osType:    "linux",
			pkg: Package{
				Name:     "vim",
				Managers: map[PackageManager]ManagerValue{Pacman: {PackageName: "vim"}},
			},
			want: "pacman",
		},
		{
			name:      "returns first available manager",
			available: []PackageManager{Yay, Pacman},
			osType:    "linux",
			pkg: Package{
				Name: "vim",
				Managers: map[PackageManager]ManagerValue{
					Pacman: {PackageName: "vim"},
					Yay:    {PackageName: "vim"},
				},
			},
			want: "yay",
		},
		{
			name:      "returns custom",
			available: []PackageManager{},
			osType:    "linux",
			pkg: Package{
				Name:   "custom-tool",
				Custom: map[string]string{"linux": "make install"},
			},
			want: "custom",
		},
		{
			name:      "returns url",
			available: []PackageManager{},
			osType:    "linux",
			pkg: Package{
				Name: "url-tool",
				URL: map[string]URLInstall{
					"linux": {URL: "https://example.com/tool", Command: "{file}"},
				},
			},
			want: "url",
		},
		{
			name:      "returns none when no method available",
			available: []PackageManager{},
			osType:    "linux",
			pkg: Package{
				Name: "unavailable",
			},
			want: "none",
		},
		{
			name:      "manager takes priority over custom",
			available: []PackageManager{Pacman},
			osType:    "linux",
			pkg: Package{
				Name:     "tool",
				Managers: map[PackageManager]ManagerValue{Pacman: {PackageName: "tool"}},
				Custom:   map[string]string{"linux": "make install"},
			},
			want: "pacman",
		},
		{
			name:      "custom takes priority over url",
			available: []PackageManager{},
			osType:    "linux",
			pkg: Package{
				Name:   "tool",
				Custom: map[string]string{"linux": "make install"},
				URL: map[string]URLInstall{
					"linux": {URL: "https://example.com/tool", Command: "{file}"},
				},
			},
			want: "custom",
		},
		{
			name:      "windows manager",
			available: []PackageManager{Winget},
			osType:    "windows",
			pkg: Package{
				Name:     "app",
				Managers: map[PackageManager]ManagerValue{Winget: {PackageName: "Publisher.App"}},
			},
			want: "winget",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Manager{
				ctx:       context.Background(),
				Config:    &Config{},
				OS:        tt.osType,
				Available: tt.available,
			}

			got := m.GetInstallMethod(tt.pkg)
			if got != tt.want {
				t.Errorf("GetInstallMethod() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInstall_DryRun(t *testing.T) {
	tests := []struct {
		name        string
		osType      string
		wantMethod  string
		pkg         Package
		available   []PackageManager
		wantSuccess bool
	}{
		{
			name:      "dry-run manager install",
			available: []PackageManager{Pacman},
			osType:    "linux",
			pkg: Package{
				Name:     "vim",
				Managers: map[PackageManager]ManagerValue{Pacman: {PackageName: "vim"}},
			},
			wantSuccess: true,
			wantMethod:  "pacman",
		},
		{
			name:      "dry-run custom install",
			available: []PackageManager{},
			osType:    "linux",
			pkg: Package{
				Name:   "custom-tool",
				Custom: map[string]string{"linux": "curl -sSL example.com | sh"},
			},
			wantSuccess: true,
			wantMethod:  "custom",
		},
		{
			name:      "dry-run url install",
			available: []PackageManager{},
			osType:    "linux",
			pkg: Package{
				Name: "url-tool",
				URL: map[string]URLInstall{
					"linux": {URL: "https://example.com/tool", Command: "chmod +x {file} && {file}"},
				},
			},
			wantSuccess: true,
			wantMethod:  "url",
		},
		{
			name:      "dry-run no method available",
			available: []PackageManager{},
			osType:    "linux",
			pkg: Package{
				Name: "unavailable",
			},
			wantSuccess: false,
			wantMethod:  "",
		},
		{
			name:      "dry-run with yay",
			available: []PackageManager{Yay},
			osType:    "linux",
			pkg: Package{
				Name:     "aur-package",
				Managers: map[PackageManager]ManagerValue{Yay: {PackageName: "aur-package"}},
			},
			wantSuccess: true,
			wantMethod:  "yay",
		},
		{
			name:      "dry-run windows winget",
			available: []PackageManager{Winget},
			osType:    "windows",
			pkg: Package{
				Name:     "app",
				Managers: map[PackageManager]ManagerValue{Winget: {PackageName: "Publisher.App"}},
			},
			wantSuccess: true,
			wantMethod:  "winget",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Manager{
				ctx:       context.Background(),
				Config:    &Config{},
				OS:        tt.osType,
				DryRun:    true,
				Available: tt.available,
			}

			result := m.Install(tt.pkg)

			if result.Success != tt.wantSuccess {
				t.Errorf("Install().Success = %v, want %v", result.Success, tt.wantSuccess)
			}

			if result.Method != tt.wantMethod {
				t.Errorf("Install().Method = %q, want %q", result.Method, tt.wantMethod)
			}

			if result.Package != tt.pkg.Name {
				t.Errorf("Install().Package = %q, want %q", result.Package, tt.pkg.Name)
			}
			// Verify dry-run message indicates it would run (not actually executed)
			if tt.wantSuccess && result.Message == "" {
				t.Error("Expected non-empty message for successful dry-run")
			}
		})
	}
}

// mockRenderer implements config.PathRenderer for testing.
type mockRenderer struct {
	result string
	err    error
}

func (m *mockRenderer) RenderString(_, _ string) (string, error) {
	if m.err != nil {
		return "", m.err
	}

	return m.result, nil
}

func TestFilterPackages(t *testing.T) {
	tests := []struct {
		name     string
		packages []Package
		renderer config.PathRenderer
		want     []string // expected package names
	}{
		{
			name: "no when - all packages returned",
			packages: []Package{
				{Name: "pkg1"},
				{Name: "pkg2"},
			},
			renderer: &mockRenderer{result: "true"},
			want:     []string{"pkg1", "pkg2"},
		},
		{
			name: "with when - true renderer matches all",
			packages: []Package{
				{Name: "linux-pkg", When: `{{ eq .OS "linux" }}`},
				{Name: "windows-pkg", When: `{{ eq .OS "windows" }}`},
			},
			renderer: &mockRenderer{result: "true"},
			want:     []string{"linux-pkg", "windows-pkg"},
		},
		{
			name: "with when - false renderer excludes when-bearing packages",
			packages: []Package{
				{Name: "linux-pkg", When: `{{ eq .OS "linux" }}`},
				{Name: "no-when-pkg"},
			},
			renderer: &mockRenderer{result: "false"},
			want:     []string{"no-when-pkg"},
		},
		{
			name: "nil renderer - only no-when packages match",
			packages: []Package{
				{Name: "filtered-pkg", When: `{{ eq .OS "linux" }}`},
				{Name: "unfiltered-pkg"},
			},
			renderer: nil,
			want:     []string{"unfiltered-pkg"},
		},
		{
			name: "render error - when-bearing packages excluded",
			packages: []Package{
				{Name: "error-pkg", When: `{{ invalid }}`},
				{Name: "ok-pkg"},
			},
			renderer: &mockRenderer{err: fmt.Errorf("render error")},
			want:     []string{"ok-pkg"},
		},
		{
			name:     "empty packages list",
			packages: []Package{},
			renderer: &mockRenderer{result: "true"},
			want:     []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterPackages(tt.packages, tt.renderer)

			if len(got) != len(tt.want) {
				t.Errorf("FilterPackages() returned %d packages, want %d", len(got), len(tt.want))
				return
			}

			for i, pkg := range got {
				if pkg.Name != tt.want[i] {
					t.Errorf("FilterPackages()[%d].Name = %q, want %q", i, pkg.Name, tt.want[i])
				}
			}
		})
	}
}

func TestFromEntry(t *testing.T) {
	tests := []struct {
		name       string
		wantName   string
		wantDesc   string
		entry      config.Entry
		wantMgrLen int
		wantNil    bool
	}{
		{
			name: "entry with package - manager only",
			entry: config.Entry{
				Name:        "vim",
				Description: "Text editor",
				Package: &config.EntryPackage{
					Managers: map[string]config.ManagerValue{"pacman": {PackageName: "vim"}, "apt": {PackageName: "vim"}},
				},
			},
			wantNil:    false,
			wantName:   "vim",
			wantDesc:   "Text editor",
			wantMgrLen: 2,
		},
		{
			name: "entry without package",
			entry: config.Entry{
				Name:   "dotfiles",
				Backup: "./dotfiles",
			},
			wantNil: true,
		},
		{
			name: "entry with package - custom command",
			entry: config.Entry{
				Name: "custom-tool",
				Package: &config.EntryPackage{
					Custom: map[string]string{"linux": "curl -sSL example.com | sh"},
				},
			},
			wantNil:  false,
			wantName: "custom-tool",
		},
		{
			name: "entry with package - URL install",
			entry: config.Entry{
				Name: "url-tool",
				Package: &config.EntryPackage{
					URL: map[string]config.URLInstallSpec{
						"linux": {URL: "https://example.com/tool", Command: "{file}"},
					},
				},
			},
			wantNil:  false,
			wantName: "url-tool",
		},
		{
			name: "entry with when expression",
			entry: config.Entry{
				Name: "filtered-pkg",
				When: `{{ eq .OS "linux" }}`,
				Package: &config.EntryPackage{
					Managers: map[string]config.ManagerValue{"pacman": {PackageName: "filtered-pkg"}},
				},
			},
			wantNil:  false,
			wantName: "filtered-pkg",
		},
		{
			name: "entry with all package options",
			entry: config.Entry{
				Name:        "full-pkg",
				Description: "Full package example",
				Package: &config.EntryPackage{
					Managers: map[string]config.ManagerValue{"pacman": {PackageName: "full-pkg"}, "apt": {PackageName: "full-pkg"}},
					Custom:   map[string]string{"darwin": "brew install full-pkg"},
					URL: map[string]config.URLInstallSpec{
						"windows": {URL: "https://example.com/full-pkg.exe", Command: "{file} /install"},
					},
				},
			},
			wantNil:    false,
			wantName:   "full-pkg",
			wantDesc:   "Full package example",
			wantMgrLen: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FromEntry(tt.entry)

			if tt.wantNil {
				if got != nil {
					t.Errorf("FromEntry() = %v, want nil", got)
				}

				return
			}

			if got == nil {
				t.Fatal("FromEntry() = nil, want non-nil")
			}

			if got.Name != tt.wantName {
				t.Errorf("FromEntry().Name = %q, want %q", got.Name, tt.wantName)
			}

			if got.Description != tt.wantDesc {
				t.Errorf("FromEntry().Description = %q, want %q", got.Description, tt.wantDesc)
			}

			if tt.wantMgrLen > 0 && len(got.Managers) != tt.wantMgrLen {
				t.Errorf("FromEntry().Managers has %d entries, want %d", len(got.Managers), tt.wantMgrLen)
			}
		})
	}
}

func TestFromEntries(t *testing.T) {
	tests := []struct {
		name    string
		entries []config.Entry
		want    []string // expected package names
	}{
		{
			name: "mixed entries - only packages returned",
			entries: []config.Entry{
				{
					Name:   "dotfiles",
					Backup: "./dotfiles",
				},
				{
					Name: "vim",
					Package: &config.EntryPackage{
						Managers: map[string]config.ManagerValue{"pacman": {PackageName: "vim"}},
					},
				},
				{
					Name: "tmux",
					Package: &config.EntryPackage{
						Managers: map[string]config.ManagerValue{"pacman": {PackageName: "tmux"}},
					},
				},
			},
			want: []string{"vim", "tmux"},
		},
		{
			name: "all package entries",
			entries: []config.Entry{
				{
					Name: "pkg1",
					Package: &config.EntryPackage{
						Managers: map[string]config.ManagerValue{"pacman": {PackageName: "pkg1"}},
					},
				},
				{
					Name: "pkg2",
					Package: &config.EntryPackage{
						Custom: map[string]string{"linux": "install.sh"},
					},
				},
			},
			want: []string{"pkg1", "pkg2"},
		},
		{
			name: "no package entries",
			entries: []config.Entry{
				{Name: "dotfiles", Backup: "./dotfiles"},
			},
			want: []string{},
		},
		{
			name:    "empty entries",
			entries: []config.Entry{},
			want:    []string{},
		},
		{
			name:    "nil entries",
			entries: nil,
			want:    []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FromEntries(tt.entries)

			if len(got) != len(tt.want) {
				t.Errorf("FromEntries() returned %d packages, want %d", len(got), len(tt.want))
				return
			}

			for i, pkg := range got {
				if pkg.Name != tt.want[i] {
					t.Errorf("FromEntries()[%d].Name = %q, want %q", i, pkg.Name, tt.want[i])
				}
			}
		})
	}
}

func TestGetInstallablePackages(t *testing.T) {
	tests := []struct {
		name      string
		available []PackageManager
		osType    string
		packages  []Package
		want      []string // expected package names
	}{
		{
			name:      "filter by available manager",
			available: []PackageManager{Pacman},
			osType:    "linux",
			packages: []Package{
				{Name: "pacman-pkg", Managers: map[PackageManager]ManagerValue{Pacman: {PackageName: "pacman-pkg"}}},
				{Name: "apt-pkg", Managers: map[PackageManager]ManagerValue{Apt: {PackageName: "apt-pkg"}}},
			},
			want: []string{"pacman-pkg"},
		},
		{
			name:      "include packages with custom for OS",
			available: []PackageManager{},
			osType:    "linux",
			packages: []Package{
				{Name: "custom-linux", Custom: map[string]string{"linux": "install.sh"}},
				{Name: "custom-windows", Custom: map[string]string{"windows": "install.bat"}},
			},
			want: []string{"custom-linux"},
		},
		{
			name:      "include packages with URL for OS",
			available: []PackageManager{},
			osType:    "linux",
			packages: []Package{
				{Name: "url-linux", URL: map[string]URLInstall{"linux": {URL: "http://example.com"}}},
				{Name: "url-windows", URL: map[string]URLInstall{"windows": {URL: "http://example.com"}}},
			},
			want: []string{"url-linux"},
		},
		{
			name:      "mixed install methods",
			available: []PackageManager{Pacman, Yay},
			osType:    "linux",
			packages: []Package{
				{Name: "pkg1", Managers: map[PackageManager]ManagerValue{Pacman: {PackageName: "pkg1"}}},
				{Name: "pkg2", Managers: map[PackageManager]ManagerValue{Apt: {PackageName: "pkg2"}}},
				{Name: "pkg3", Custom: map[string]string{"linux": "install.sh"}},
				{Name: "pkg4", URL: map[string]URLInstall{"darwin": {URL: "http://example.com"}}},
			},
			want: []string{"pkg1", "pkg3"},
		},
		{
			name:      "no installable packages",
			available: []PackageManager{},
			osType:    "linux",
			packages: []Package{
				{Name: "pkg1", Managers: map[PackageManager]ManagerValue{Winget: {PackageName: "pkg1"}}},
				{Name: "pkg2", Custom: map[string]string{"windows": "install.bat"}},
			},
			want: []string{},
		},
		{
			name:      "empty packages list",
			available: []PackageManager{Pacman},
			osType:    "linux",
			packages:  []Package{},
			want:      []string{},
		},
		{
			name:      "multiple managers available for same package",
			available: []PackageManager{Yay, Pacman},
			osType:    "linux",
			packages: []Package{
				{Name: "pkg1", Managers: map[PackageManager]ManagerValue{Pacman: {PackageName: "pkg1"}, Yay: {PackageName: "pkg1"}}},
			},
			want: []string{"pkg1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Manager{
				Config: &Config{
					Packages: tt.packages,
				},
				OS:        tt.osType,
				Available: tt.available,
			}

			got := m.GetInstallablePackages()

			if len(got) != len(tt.want) {
				t.Errorf("GetInstallablePackages() returned %d packages, want %d", len(got), len(tt.want))
				return
			}

			for i, pkg := range got {
				if pkg.Name != tt.want[i] {
					t.Errorf("GetInstallablePackages()[%d].Name = %q, want %q", i, pkg.Name, tt.want[i])
				}
			}
		})
	}
}

func TestInstallAll_DryRun(t *testing.T) {
	tests := []struct {
		name        string
		available   []PackageManager
		osType      string
		packages    []Package
		wantResults int
		wantSuccess int
	}{
		{
			name:      "install multiple packages",
			available: []PackageManager{Pacman},
			osType:    "linux",
			packages: []Package{
				{Name: "pkg1", Managers: map[PackageManager]ManagerValue{Pacman: {PackageName: "pkg1"}}},
				{Name: "pkg2", Managers: map[PackageManager]ManagerValue{Pacman: {PackageName: "pkg2"}}},
				{Name: "pkg3", Managers: map[PackageManager]ManagerValue{Pacman: {PackageName: "pkg3"}}},
			},
			wantResults: 3,
			wantSuccess: 3,
		},
		{
			name:      "mixed success and failure",
			available: []PackageManager{Pacman},
			osType:    "linux",
			packages: []Package{
				{Name: "pkg1", Managers: map[PackageManager]ManagerValue{Pacman: {PackageName: "pkg1"}}},
				{Name: "pkg2"}, // No install method
				{Name: "pkg3", Managers: map[PackageManager]ManagerValue{Pacman: {PackageName: "pkg3"}}},
			},
			wantResults: 3,
			wantSuccess: 2,
		},
		{
			name:        "empty packages list",
			available:   []PackageManager{Pacman},
			osType:      "linux",
			packages:    []Package{},
			wantResults: 0,
			wantSuccess: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Manager{
				ctx:       context.Background(),
				Config:    &Config{},
				OS:        tt.osType,
				DryRun:    true,
				Available: tt.available,
			}

			results := m.InstallAll(tt.packages)

			if len(results) != tt.wantResults {
				t.Errorf("InstallAll() returned %d results, want %d", len(results), tt.wantResults)
			}

			successCount := 0

			for _, r := range results {
				if r.Success {
					successCount++
				}
			}

			if successCount != tt.wantSuccess {
				t.Errorf("InstallAll() had %d successes, want %d", successCount, tt.wantSuccess)
			}
		})
	}
}

func TestPackage_GitConfigInManagers(t *testing.T) {
	pkg := Package{
		Name:        "my-dotfiles",
		Description: "My dotfiles repo",
		Managers: map[PackageManager]ManagerValue{
			Pacman: {PackageName: "neovim"},
			Git: {Git: &GitConfig{
				URL:    "https://github.com/user/dotfiles.git",
				Branch: "main",
				Targets: map[string]string{
					"linux":   "~/.dotfiles",
					"windows": "~/dotfiles",
				},
				Sudo: false,
			}},
		},
	}

	// Check traditional manager (string)
	if pkg.Managers[Pacman].PackageName != "neovim" { //nolint:goconst // test data
		t.Errorf("Expected pacman package name, got %v", pkg.Managers[Pacman].PackageName)
	}

	// Check git manager (GitConfig)
	gitValue := pkg.Managers[Git]
	if !gitValue.IsGit() {
		t.Fatal("Expected git manager to be GitConfig")
	}

	gitCfg := gitValue.Git
	if gitCfg.URL != "https://github.com/user/dotfiles.git" {
		t.Errorf("Expected git repo URL, got %s", gitCfg.URL)
	}

	if gitCfg.Branch != "main" {
		t.Errorf("Expected branch 'main', got %s", gitCfg.Branch)
	}

	if gitCfg.Targets["linux"] != "~/.dotfiles" {
		t.Errorf("Expected linux target, got %s", gitCfg.Targets["linux"])
	}
}

func TestManager_InstallGitPackage_Clone(t *testing.T) {
	if !platform.IsCommandAvailable("git") {
		t.Skip("git not available for testing")
	}

	// Create bare repo for testing
	tmpDir := t.TempDir()
	bareRepo := filepath.Join(tmpDir, "test-repo.git")
	cloneDest := filepath.Join(tmpDir, "cloned")

	// Initialize bare repo
	cmd := exec.CommandContext(context.Background(), "git", "init", "--bare", bareRepo) //nolint:gosec // test command
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create bare repo: %v", err)
	}

	// Create package manager
	cfg := &Config{Packages: []Package{}}
	mgr := NewManager(cfg, platform.OSLinux, false, false)

	// Create git package
	pkg := Package{
		Name: "test-repo",
		Managers: map[PackageManager]ManagerValue{
			Git: {Git: &GitConfig{
				URL: bareRepo,
				Targets: map[string]string{
					platform.OSLinux: cloneDest,
				},
				Sudo: false,
			}},
		},
	}

	// Install
	result := mgr.Install(pkg)

	if !result.Success {
		t.Errorf("Expected success, got: %s", result.Message)
	}

	if result.Method != "git" {
		t.Errorf("Expected method 'git', got: %s", result.Method)
	}

	// Verify clone exists
	if _, err := os.Stat(filepath.Join(cloneDest, ".git")); err != nil {
		t.Errorf("Expected .git directory to exist: %v", err)
	}
}

func TestManager_InstallGitPackage_Pull(t *testing.T) {
	if !platform.IsCommandAvailable("git") {
		t.Skip("git not available for testing")
	}

	// Create working repo, bare repo, and clone dest
	tmpDir := t.TempDir()
	workingRepo := filepath.Join(tmpDir, "working")
	bareRepo := filepath.Join(tmpDir, "test-repo.git")
	cloneDest := filepath.Join(tmpDir, "cloned")

	// Initialize working repo with a commit
	cmd := exec.CommandContext(context.Background(), "git", "init", workingRepo) //nolint:gosec // test command
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create working repo: %v", err)
	}

	// Configure git user for commit
	cmd = exec.CommandContext(context.Background(), "git", "-C", workingRepo, "config", "user.email", "test@example.com") //nolint:gosec // test command
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to set git email: %v", err)
	}
	cmd = exec.CommandContext(context.Background(), "git", "-C", workingRepo, "config", "user.name", "Test User") //nolint:gosec // test command
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to set git name: %v", err)
	}

	// Create initial commit
	testFile := filepath.Join(workingRepo, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	cmd = exec.CommandContext(context.Background(), "git", "-C", workingRepo, "add", "test.txt") //nolint:gosec // test command
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}
	cmd = exec.CommandContext(context.Background(), "git", "-C", workingRepo, "commit", "-m", "Initial commit") //nolint:gosec // test command
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Clone to bare repo
	cmd = exec.CommandContext(context.Background(), "git", "clone", "--bare", workingRepo, bareRepo) //nolint:gosec // test command
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create bare repo: %v", err)
	}

	// Clone from bare repo to destination
	cmd = exec.CommandContext(context.Background(), "git", "clone", bareRepo, cloneDest) //nolint:gosec // test command
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to clone repo: %v", err)
	}

	// Create package manager
	cfg := &Config{Packages: []Package{}}
	mgr := NewManager(cfg, platform.OSLinux, false, false)

	// Create git package
	pkg := Package{
		Name: "test-repo",
		Managers: map[PackageManager]ManagerValue{
			Git: {Git: &GitConfig{
				URL: bareRepo,
				Targets: map[string]string{
					platform.OSLinux: cloneDest,
				},
				Sudo: false,
			}},
		},
	}

	// Install (should pull)
	result := mgr.Install(pkg)

	if !result.Success {
		t.Errorf("Expected success, got: %s", result.Message)
	}

	if !strings.Contains(result.Message, "updated") {
		t.Errorf("Expected 'updated' in message, got: %s", result.Message)
	}
}

func TestManager_InstallGitPackage_DryRun(t *testing.T) {
	tmpDir := t.TempDir()
	cloneDest := filepath.Join(tmpDir, "cloned")

	cfg := &Config{Packages: []Package{}}
	mgr := NewManager(cfg, platform.OSLinux, true, false) // dry-run = true

	pkg := Package{
		Name: "test-repo",
		Managers: map[PackageManager]ManagerValue{
			Git: {Git: &GitConfig{
				URL:    "https://github.com/test/repo.git",
				Branch: "main",
				Targets: map[string]string{
					platform.OSLinux: cloneDest,
				},
				Sudo: false,
			}},
		},
	}

	result := mgr.Install(pkg)

	if !result.Success {
		t.Errorf("Expected success in dry-run, got: %s", result.Message)
	}

	if !strings.Contains(result.Message, "Would run") {
		t.Errorf("Expected 'Would run' in dry-run message, got: %s", result.Message)
	}

	// Verify nothing was actually cloned
	if _, err := os.Stat(cloneDest); err == nil {
		t.Error("Expected no clone in dry-run mode, but directory exists")
	}
}

func TestPackage_UnmarshalYAML(t *testing.T) {
	yamlData := `
name: "test-pkg"
managers:
  pacman: "neovim"
  git:
    url: "https://github.com/user/repo.git"
    branch: "main"
    targets:
      linux: "~/.dotfiles"
    sudo: true
`

	var pkg Package
	err := yaml.Unmarshal([]byte(yamlData), &pkg)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Verify pacman manager (string)
	pacmanValue := pkg.Managers[Pacman]
	if pacmanValue.PackageName != "neovim" {
		t.Errorf("Expected 'neovim', got %s", pacmanValue.PackageName)
	}

	// Verify git manager (GitConfig)
	gitValue := pkg.Managers[Git]
	if !gitValue.IsGit() {
		t.Fatal("Expected git to be GitConfig")
	}

	if gitValue.Git.URL != "https://github.com/user/repo.git" {
		t.Errorf("Expected URL, got %s", gitValue.Git.URL)
	}

	if !gitValue.Git.Sudo {
		t.Error("Expected sudo to be true")
	}
}

func TestPackage_InstallerConfigInManagers(t *testing.T) {
	pkg := Package{
		Name:        "my-tool",
		Description: "Custom tool installed via script",
		Managers: map[PackageManager]ManagerValue{
			Pacman: {PackageName: "mytool"},
			Installer: {Installer: &InstallerConfig{
				Command: map[string]string{
					"linux":   "curl -fsSL https://example.com/install.sh | sh",
					"windows": "iwr https://example.com/install.ps1 | iex",
				},
				Binary: "mytool",
			}},
		},
	}

	// Check traditional manager (string)
	if pkg.Managers[Pacman].PackageName != "mytool" { //nolint:goconst // test data
		t.Errorf("Expected pacman package name 'mytool', got %v", pkg.Managers[Pacman].PackageName)
	}

	// Check installer manager (InstallerConfig)
	installerValue := pkg.Managers[Installer]
	if !installerValue.IsInstaller() {
		t.Fatal("Expected installer manager to be InstallerConfig")
	}

	installerCfg := installerValue.Installer
	if installerCfg.Command["linux"] != "curl -fsSL https://example.com/install.sh | sh" { //nolint:goconst // test data
		t.Errorf("Expected linux command, got %s", installerCfg.Command["linux"])
	}

	if installerCfg.Command["windows"] != "iwr https://example.com/install.ps1 | iex" {
		t.Errorf("Expected windows command, got %s", installerCfg.Command["windows"])
	}

	if installerCfg.Binary != "mytool" {
		t.Errorf("Expected binary 'mytool', got %s", installerCfg.Binary)
	}
}

func TestManager_InstallInstallerPackage_DryRun(t *testing.T) {
	cfg := &Config{Packages: []Package{}}
	mgr := NewManager(cfg, platform.OSLinux, true, false) // dry-run = true

	pkg := Package{
		Name: "installer-tool",
		Managers: map[PackageManager]ManagerValue{
			Installer: {Installer: &InstallerConfig{
				Command: map[string]string{
					"linux": "curl -fsSL https://example.com/install.sh | sh",
				},
				Binary: "mytool",
			}},
		},
	}

	result := mgr.Install(pkg)

	if !result.Success {
		t.Errorf("Expected success in dry-run, got: %s", result.Message)
	}

	if result.Method != "installer" {
		t.Errorf("Expected method 'installer', got: %s", result.Method)
	}

	if !strings.Contains(result.Message, "Would run") {
		t.Errorf("Expected 'Would run' in dry-run message, got: %s", result.Message)
	}
}

func TestManager_InstallInstallerPackage_NoCommandForOS(t *testing.T) {
	cfg := &Config{Packages: []Package{}}
	mgr := NewManager(cfg, platform.OSLinux, false, false)

	pkg := Package{
		Name: "windows-only-tool",
		Managers: map[PackageManager]ManagerValue{
			Installer: {Installer: &InstallerConfig{
				Command: map[string]string{
					"windows": "iwr https://example.com/install.ps1 | iex",
				},
				Binary: "mytool",
			}},
		},
	}

	result := mgr.Install(pkg)

	if result.Success {
		t.Error("Expected failure when no command for current OS")
	}

	if !strings.Contains(result.Message, "No installer command defined for OS") {
		t.Errorf("Expected 'No installer command defined for OS' message, got: %s", result.Message)
	}
}

func TestPackage_UnmarshalYAML_Installer(t *testing.T) {
	yamlData := `
name: "test-installer-pkg"
managers:
  pacman: "neovim"
  installer:
    command:
      linux: "curl -fsSL https://example.com/install.sh | sh"
      windows: "iwr https://example.com/install.ps1 | iex"
    binary: "mytool"
`

	var pkg Package
	err := yaml.Unmarshal([]byte(yamlData), &pkg)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Verify pacman manager (string)
	pacmanValue := pkg.Managers[Pacman]
	if pacmanValue.PackageName != "neovim" {
		t.Errorf("Expected 'neovim', got %s", pacmanValue.PackageName)
	}

	// Verify installer manager (InstallerConfig)
	installerValue := pkg.Managers[Installer]
	if !installerValue.IsInstaller() {
		t.Fatal("Expected installer to be InstallerConfig")
	}

	if installerValue.Installer.Command["linux"] != "curl -fsSL https://example.com/install.sh | sh" {
		t.Errorf("Expected linux command, got %s", installerValue.Installer.Command["linux"])
	}

	if installerValue.Installer.Command["windows"] != "iwr https://example.com/install.ps1 | iex" {
		t.Errorf("Expected windows command, got %s", installerValue.Installer.Command["windows"])
	}

	if installerValue.Installer.Binary != "mytool" {
		t.Errorf("Expected binary 'mytool', got %s", installerValue.Installer.Binary)
	}
}

func TestPackage_UnmarshalYAML_InstallerWithoutBinary(t *testing.T) {
	yamlData := `
name: "no-binary-pkg"
managers:
  installer:
    command:
      linux: "make install"
`

	var pkg Package
	err := yaml.Unmarshal([]byte(yamlData), &pkg)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	installerValue := pkg.Managers[Installer]
	if !installerValue.IsInstaller() {
		t.Fatal("Expected installer to be InstallerConfig")
	}

	if installerValue.Installer.Binary != "" {
		t.Errorf("Expected empty binary, got %q", installerValue.Installer.Binary)
	}

	if installerValue.Installer.Command["linux"] != "make install" {
		t.Errorf("Expected linux command 'make install', got %s", installerValue.Installer.Command["linux"])
	}
}

func TestIsInstallerInstalled(t *testing.T) {
	tests := []struct {
		name   string
		binary string
		want   bool
	}{
		{
			name:   "empty binary returns false",
			binary: "",
			want:   false,
		},
		{
			name:   "nonexistent binary returns false",
			binary: "definitely-not-a-real-binary-xyz123",
			want:   false,
		},
		{
			name:   "existing binary returns true",
			binary: "sh",
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsInstallerInstalled(tt.binary)
			if got != tt.want {
				t.Errorf("IsInstallerInstalled(%q) = %v, want %v", tt.binary, got, tt.want)
			}
		})
	}
}

func TestCanInstall_Installer(t *testing.T) {
	tests := []struct {
		name      string
		available []PackageManager
		osType    string
		pkg       Package
		want      bool
	}{
		{
			name:      "installer with command for current OS",
			available: []PackageManager{},
			osType:    "linux",
			pkg: Package{
				Name: "installer-tool",
				Managers: map[PackageManager]ManagerValue{
					Installer: {Installer: &InstallerConfig{
						Command: map[string]string{"linux": "make install"},
						Binary:  "mytool",
					}},
				},
			},
			want: true,
		},
		{
			name:      "installer without command for current OS",
			available: []PackageManager{},
			osType:    "linux",
			pkg: Package{
				Name: "installer-tool",
				Managers: map[PackageManager]ManagerValue{
					Installer: {Installer: &InstallerConfig{
						Command: map[string]string{"windows": "install.bat"},
						Binary:  "mytool",
					}},
				},
			},
			want: false,
		},
		{
			name:      "installer with no binary still installable",
			available: []PackageManager{},
			osType:    "linux",
			pkg: Package{
				Name: "installer-tool",
				Managers: map[PackageManager]ManagerValue{
					Installer: {Installer: &InstallerConfig{
						Command: map[string]string{"linux": "make install"},
					}},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Manager{
				ctx:       context.Background(),
				Config:    &Config{},
				OS:        tt.osType,
				Available: tt.available,
			}

			got := m.CanInstall(tt.pkg)
			if got != tt.want {
				t.Errorf("CanInstall() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetInstallMethod_Installer(t *testing.T) {
	tests := []struct {
		name      string
		osType    string
		want      string
		pkg       Package
		available []PackageManager
	}{
		{
			name:      "installer with command for current OS",
			available: []PackageManager{},
			osType:    "linux",
			pkg: Package{
				Name: "installer-tool",
				Managers: map[PackageManager]ManagerValue{
					Installer: {Installer: &InstallerConfig{
						Command: map[string]string{"linux": "make install"},
					}},
				},
			},
			want: "installer",
		},
		{
			name:      "installer without command for current OS falls through",
			available: []PackageManager{},
			osType:    "linux",
			pkg: Package{
				Name: "installer-tool",
				Managers: map[PackageManager]ManagerValue{
					Installer: {Installer: &InstallerConfig{
						Command: map[string]string{"windows": "install.bat"},
					}},
				},
			},
			want: "none",
		},
		{
			name:      "regular manager takes priority over installer",
			available: []PackageManager{Pacman},
			osType:    "linux",
			pkg: Package{
				Name: "tool",
				Managers: map[PackageManager]ManagerValue{
					Pacman: {PackageName: "tool"},
					Installer: {Installer: &InstallerConfig{
						Command: map[string]string{"linux": "make install"},
					}},
				},
			},
			want: "pacman",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Manager{
				ctx:       context.Background(),
				Config:    &Config{},
				OS:        tt.osType,
				Available: tt.available,
			}

			got := m.GetInstallMethod(tt.pkg)
			if got != tt.want {
				t.Errorf("GetInstallMethod() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFromEntry_Installer(t *testing.T) {
	entry := config.Entry{
		Name:        "installer-tool",
		Description: "A tool installed via script",
		Package: &config.EntryPackage{
			Managers: map[string]config.ManagerValue{
				"pacman": {PackageName: "mytool"},
				"installer": {Installer: &config.InstallerPackage{
					Command: map[string]string{
						"linux":   "curl -fsSL https://example.com/install.sh | sh",
						"windows": "iwr https://example.com/install.ps1 | iex",
					},
					Binary: "mytool",
				}},
			},
		},
	}

	got := FromEntry(entry)
	if got == nil {
		t.Fatal("FromEntry() = nil, want non-nil")
	}

	if got.Name != "installer-tool" {
		t.Errorf("Name = %q, want %q", got.Name, "installer-tool")
	}

	if len(got.Managers) != 2 {
		t.Errorf("len(Managers) = %d, want 2", len(got.Managers))
	}

	// Check pacman
	if got.Managers[Pacman].PackageName != "mytool" {
		t.Errorf("Managers[pacman] = %q, want %q", got.Managers[Pacman].PackageName, "mytool")
	}

	// Check installer
	installerVal := got.Managers[Installer]
	if !installerVal.IsInstaller() {
		t.Fatal("Expected installer manager")
	}

	if installerVal.Installer.Command["linux"] != "curl -fsSL https://example.com/install.sh | sh" {
		t.Errorf("Installer.Command[linux] = %q", installerVal.Installer.Command["linux"])
	}

	if installerVal.Installer.Binary != "mytool" {
		t.Errorf("Installer.Binary = %q, want %q", installerVal.Installer.Binary, "mytool")
	}
}

func TestManager_InstallGitPackage_WithSudo(t *testing.T) {
	tmpDir := t.TempDir()
	cloneDest := filepath.Join(tmpDir, "cloned")

	cfg := &Config{Packages: []Package{}}
	mgr := NewManager(cfg, platform.OSLinux, true, false) // dry-run to avoid actual sudo

	pkg := Package{
		Name: "test-repo",
		Managers: map[PackageManager]ManagerValue{
			Git: {Git: &GitConfig{
				URL: "https://github.com/test/repo.git",
				Targets: map[string]string{
					platform.OSLinux: cloneDest,
				},
				Sudo: true,
			}},
		},
	}

	result := mgr.Install(pkg)

	if !result.Success {
		t.Errorf("Expected success, got: %s", result.Message)
	}

	// Verify sudo is in the command
	if !strings.Contains(result.Message, "sudo") {
		t.Errorf("Expected 'sudo' in command, got: %s", result.Message)
	}
}
