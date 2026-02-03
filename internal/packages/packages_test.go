package packages

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AntoineGS/dot-manager/internal/config"
	"github.com/AntoineGS/dot-manager/internal/platform"
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
				Managers: map[PackageManager]string{Pacman: "vim"},
			},
			want: true,
		},
		{
			name:      "cannot install - manager not available",
			available: []PackageManager{Apt},
			osType:    "linux",
			pkg: Package{
				Name:     "vim",
				Managers: map[PackageManager]string{Pacman: "vim"},
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
				Managers: map[PackageManager]string{
					Apt:    "multi-tool",
					Pacman: "multi-tool",
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
				Managers: map[PackageManager]string{Pacman: "vim"},
			},
			want: "pacman",
		},
		{
			name:      "returns first available manager",
			available: []PackageManager{Yay, Pacman},
			osType:    "linux",
			pkg: Package{
				Name: "vim",
				Managers: map[PackageManager]string{
					Pacman: "vim",
					Yay:    "vim",
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
				Managers: map[PackageManager]string{Pacman: "tool"},
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
				Managers: map[PackageManager]string{Winget: "Publisher.App"},
			},
			want: "winget",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Manager{
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
				Managers: map[PackageManager]string{Pacman: "vim"},
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
				Managers: map[PackageManager]string{Yay: "aur-package"},
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
				Managers: map[PackageManager]string{Winget: "Publisher.App"},
			},
			wantSuccess: true,
			wantMethod:  "winget",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Manager{
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

func TestFilterPackages(t *testing.T) {
	tests := []struct {
		name     string
		packages []Package
		ctx      *config.FilterContext
		want     []string // expected package names
	}{
		{
			name: "no filters - all packages returned",
			packages: []Package{
				{Name: "pkg1"},
				{Name: "pkg2"},
			},
			ctx:  &config.FilterContext{OS: "linux"},
			want: []string{"pkg1", "pkg2"},
		},
		{
			name: "filter by OS - include linux",
			packages: []Package{
				{
					Name: "linux-pkg",
					Filters: []config.Filter{
						{Include: map[string]string{"os": "linux"}},
					},
				},
				{
					Name: "windows-pkg",
					Filters: []config.Filter{
						{Include: map[string]string{"os": "windows"}},
					},
				},
			},
			ctx:  &config.FilterContext{OS: "linux"},
			want: []string{"linux-pkg"},
		},
		{
			name: "filter by distro",
			packages: []Package{
				{
					Name: "arch-pkg",
					Filters: []config.Filter{
						{Include: map[string]string{"distro": "arch"}},
					},
				},
				{
					Name: "ubuntu-pkg",
					Filters: []config.Filter{
						{Include: map[string]string{"distro": "ubuntu"}},
					},
				},
			},
			ctx:  &config.FilterContext{OS: "linux", Distro: "arch"},
			want: []string{"arch-pkg"},
		},
		{
			name: "filter by hostname",
			packages: []Package{
				{
					Name: "work-pkg",
					Filters: []config.Filter{
						{Include: map[string]string{"hostname": "work-laptop"}},
					},
				},
				{
					Name: "home-pkg",
					Filters: []config.Filter{
						{Include: map[string]string{"hostname": "home-desktop"}},
					},
				},
			},
			ctx:  &config.FilterContext{Hostname: "work-laptop"},
			want: []string{"work-pkg"},
		},
		{
			name: "filter by user",
			packages: []Package{
				{
					Name: "admin-pkg",
					Filters: []config.Filter{
						{Include: map[string]string{"user": "admin"}},
					},
				},
				{
					Name: "user-pkg",
					Filters: []config.Filter{
						{Include: map[string]string{"user": "user"}},
					},
				},
			},
			ctx:  &config.FilterContext{User: "admin"},
			want: []string{"admin-pkg"},
		},
		{
			name: "exclude filter",
			packages: []Package{
				{
					Name: "general-pkg",
					Filters: []config.Filter{
						{Exclude: map[string]string{"distro": "ubuntu"}},
					},
				},
			},
			ctx:  &config.FilterContext{OS: "linux", Distro: "arch"},
			want: []string{"general-pkg"},
		},
		{
			name: "exclude filter - matches exclusion",
			packages: []Package{
				{
					Name: "general-pkg",
					Filters: []config.Filter{
						{Exclude: map[string]string{"distro": "ubuntu"}},
					},
				},
			},
			ctx:  &config.FilterContext{OS: "linux", Distro: "ubuntu"},
			want: []string{},
		},
		{
			name: "regex filter",
			packages: []Package{
				{
					Name: "debian-family-pkg",
					Filters: []config.Filter{
						{Include: map[string]string{"distro": "ubuntu|debian|mint"}},
					},
				},
			},
			ctx:  &config.FilterContext{OS: "linux", Distro: "debian"},
			want: []string{"debian-family-pkg"},
		},
		{
			name: "multiple conditions AND logic",
			packages: []Package{
				{
					Name: "specific-pkg",
					Filters: []config.Filter{
						{Include: map[string]string{"os": "linux", "distro": "arch"}},
					},
				},
			},
			ctx:  &config.FilterContext{OS: "linux", Distro: "arch"},
			want: []string{"specific-pkg"},
		},
		{
			name: "multiple conditions AND logic - partial match fails",
			packages: []Package{
				{
					Name: "specific-pkg",
					Filters: []config.Filter{
						{Include: map[string]string{"os": "linux", "distro": "arch"}},
					},
				},
			},
			ctx:  &config.FilterContext{OS: "linux", Distro: "ubuntu"},
			want: []string{},
		},
		{
			name: "multiple filters OR logic",
			packages: []Package{
				{
					Name: "multi-os-pkg",
					Filters: []config.Filter{
						{Include: map[string]string{"os": "linux"}},
						{Include: map[string]string{"os": "darwin"}},
					},
				},
			},
			ctx:  &config.FilterContext{OS: "darwin"},
			want: []string{"multi-os-pkg"},
		},
		{
			name:     "empty packages list",
			packages: []Package{},
			ctx:      &config.FilterContext{OS: "linux"},
			want:     []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterPackages(tt.packages, tt.ctx)

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
					Managers: map[string]string{"pacman": "vim", "apt": "vim"},
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
			name: "entry with filters",
			entry: config.Entry{
				Name: "filtered-pkg",
				Filters: []config.Filter{
					{Include: map[string]string{"os": "linux"}},
				},
				Package: &config.EntryPackage{
					Managers: map[string]string{"pacman": "filtered-pkg"},
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
					Managers: map[string]string{"pacman": "full-pkg", "apt": "full-pkg"},
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
						Managers: map[string]string{"pacman": "vim"},
					},
				},
				{
					Name: "tmux",
					Package: &config.EntryPackage{
						Managers: map[string]string{"pacman": "tmux"},
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
						Managers: map[string]string{"pacman": "pkg1"},
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
				{Name: "pacman-pkg", Managers: map[PackageManager]string{Pacman: "pacman-pkg"}},
				{Name: "apt-pkg", Managers: map[PackageManager]string{Apt: "apt-pkg"}},
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
				{Name: "pkg1", Managers: map[PackageManager]string{Pacman: "pkg1"}},
				{Name: "pkg2", Managers: map[PackageManager]string{Apt: "pkg2"}},
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
				{Name: "pkg1", Managers: map[PackageManager]string{Winget: "pkg1"}},
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
				{Name: "pkg1", Managers: map[PackageManager]string{Pacman: "pkg1", Yay: "pkg1"}},
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
				{Name: "pkg1", Managers: map[PackageManager]string{Pacman: "pkg1"}},
				{Name: "pkg2", Managers: map[PackageManager]string{Pacman: "pkg2"}},
				{Name: "pkg3", Managers: map[PackageManager]string{Pacman: "pkg3"}},
			},
			wantResults: 3,
			wantSuccess: 3,
		},
		{
			name:      "mixed success and failure",
			available: []PackageManager{Pacman},
			osType:    "linux",
			packages: []Package{
				{Name: "pkg1", Managers: map[PackageManager]string{Pacman: "pkg1"}},
				{Name: "pkg2"}, // No install method
				{Name: "pkg3", Managers: map[PackageManager]string{Pacman: "pkg3"}},
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

func TestPackage_GitFields(t *testing.T) {
	pkg := Package{
		Name:        "my-dotfiles",
		Description: "My dotfiles repo",
		Managers: map[PackageManager]string{
			Git: "https://github.com/user/dotfiles.git",
		},
		GitBranch: "main",
		GitTargets: map[string]string{
			"linux":   "~/.dotfiles",
			"windows": "~/dotfiles",
		},
	}

	if pkg.Managers[Git] != "https://github.com/user/dotfiles.git" {
		t.Errorf("Expected git repo URL, got %s", pkg.Managers[Git])
	}

	if pkg.GitBranch != "main" {
		t.Errorf("Expected branch 'main', got %s", pkg.GitBranch)
	}

	if pkg.GitTargets["linux"] != "~/.dotfiles" {
		t.Errorf("Expected linux target, got %s", pkg.GitTargets["linux"])
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
	cmd := exec.Command("git", "init", "--bare", bareRepo)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create bare repo: %v", err)
	}

	// Create package manager
	cfg := &Config{Packages: []Package{}}
	mgr := NewManager(cfg, platform.OSLinux, false, false)

	// Create git package
	pkg := Package{
		Name: "test-repo",
		Managers: map[PackageManager]string{
			Git: bareRepo,
		},
		GitTargets: map[string]string{
			platform.OSLinux: cloneDest,
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
	cmd := exec.Command("git", "init", workingRepo)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create working repo: %v", err)
	}

	// Configure git user for commit
	cmd = exec.Command("git", "-C", workingRepo, "config", "user.email", "test@example.com")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to set git email: %v", err)
	}
	cmd = exec.Command("git", "-C", workingRepo, "config", "user.name", "Test User")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to set git name: %v", err)
	}

	// Create initial commit
	testFile := filepath.Join(workingRepo, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	cmd = exec.Command("git", "-C", workingRepo, "add", "test.txt")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}
	cmd = exec.Command("git", "-C", workingRepo, "commit", "-m", "Initial commit")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Clone to bare repo
	cmd = exec.Command("git", "clone", "--bare", workingRepo, bareRepo)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create bare repo: %v", err)
	}

	// Clone from bare repo to destination
	cmd = exec.Command("git", "clone", bareRepo, cloneDest)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to clone repo: %v", err)
	}

	// Create package manager
	cfg := &Config{Packages: []Package{}}
	mgr := NewManager(cfg, platform.OSLinux, false, false)

	// Create git package
	pkg := Package{
		Name: "test-repo",
		Managers: map[PackageManager]string{
			Git: bareRepo,
		},
		GitTargets: map[string]string{
			platform.OSLinux: cloneDest,
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
		Managers: map[PackageManager]string{
			Git: "https://github.com/test/repo.git",
		},
		GitBranch: "main",
		GitTargets: map[string]string{
			platform.OSLinux: cloneDest,
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
