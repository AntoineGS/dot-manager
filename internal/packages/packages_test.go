package packages

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/AntoineGS/tidydots/internal/config"
	"github.com/AntoineGS/tidydots/internal/platform"
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
				Config:       &Config{},
				Available:    tt.available,
				availableSet: toAvailableSet(tt.available),
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
				OS:           tt.osType,
				Available:    tt.available,
				availableSet: toAvailableSet(tt.available),
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
				ctx:          context.Background(),
				Config:       &Config{},
				OS:           tt.osType,
				Available:    tt.available,
				availableSet: toAvailableSet(tt.available),
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
				ctx:          context.Background(),
				Config:       &Config{},
				OS:           tt.osType,
				Available:    tt.available,
				availableSet: toAvailableSet(tt.available),
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
				ctx:          context.Background(),
				Config:       &Config{},
				OS:           tt.osType,
				DryRun:       true,
				Available:    tt.available,
				availableSet: toAvailableSet(tt.available),
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

// toAvailableSet builds a set map from a slice of PackageManagers for test setup.
func toAvailableSet(managers []PackageManager) map[PackageManager]bool {
	s := make(map[PackageManager]bool, len(managers))
	for _, m := range managers {
		s[m] = true
	}
	return s
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

func TestFromApplication(t *testing.T) {
	tests := []struct {
		name       string
		wantName   string
		wantDesc   string
		app        config.Application
		wantMgrLen int
		wantNil    bool
	}{
		{
			name: "app with package - manager only",
			app: config.Application{
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
			name: "app without package",
			app: config.Application{
				Name: "dotfiles",
			},
			wantNil: true,
		},
		{
			name: "app with package - custom command",
			app: config.Application{
				Name: "custom-tool",
				Package: &config.EntryPackage{
					Custom: map[string]string{"linux": "curl -sSL example.com | sh"},
				},
			},
			wantNil:  false,
			wantName: "custom-tool",
		},
		{
			name: "app with package - URL install",
			app: config.Application{
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
			name: "app with when expression",
			app: config.Application{
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
			name: "app with all package options",
			app: config.Application{
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
			got := FromApplication(tt.app)

			if tt.wantNil {
				if got != nil {
					t.Errorf("FromApplication() = %v, want nil", got)
				}

				return
			}

			if got == nil {
				t.Fatal("FromApplication() = nil, want non-nil")
			}

			if got.Name != tt.wantName {
				t.Errorf("FromApplication().Name = %q, want %q", got.Name, tt.wantName)
			}

			if got.Description != tt.wantDesc {
				t.Errorf("FromApplication().Description = %q, want %q", got.Description, tt.wantDesc)
			}

			if tt.wantMgrLen > 0 && len(got.Managers) != tt.wantMgrLen {
				t.Errorf("FromApplication().Managers has %d entries, want %d", len(got.Managers), tt.wantMgrLen)
			}
		})
	}
}

func TestFromApplications(t *testing.T) {
	tests := []struct {
		name string
		apps []config.Application
		want []string // expected package names
	}{
		{
			name: "mixed apps - only packages returned",
			apps: []config.Application{
				{
					Name: "dotfiles",
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
			name: "all package apps",
			apps: []config.Application{
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
			name: "no package apps",
			apps: []config.Application{
				{Name: "dotfiles"},
			},
			want: []string{},
		},
		{
			name: "empty apps",
			apps: []config.Application{},
			want: []string{},
		},
		{
			name: "nil apps",
			apps: nil,
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FromApplications(tt.apps)

			if len(got) != len(tt.want) {
				t.Errorf("FromApplications() returned %d packages, want %d", len(got), len(tt.want))
				return
			}

			for i, pkg := range got {
				if pkg.Name != tt.want[i] {
					t.Errorf("FromApplications()[%d].Name = %q, want %q", i, pkg.Name, tt.want[i])
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
				OS:           tt.osType,
				Available:    tt.available,
				availableSet: toAvailableSet(tt.available),
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
				ctx:          context.Background(),
				Config:       &Config{},
				OS:           tt.osType,
				DryRun:       true,
				Available:    tt.available,
				availableSet: toAvailableSet(tt.available),
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
				ctx:          context.Background(),
				Config:       &Config{},
				OS:           tt.osType,
				Available:    tt.available,
				availableSet: toAvailableSet(tt.available),
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
			name:      "installer without OS command retains CLI method for explicit error",
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
			want: "installer",
		},
		{
			name:      "installer takes priority as in CLI execution",
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
			want: "installer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Manager{
				ctx:          context.Background(),
				Config:       &Config{},
				OS:           tt.osType,
				Available:    tt.available,
				availableSet: toAvailableSet(tt.available),
			}

			got := m.GetInstallMethod(tt.pkg)
			if got != tt.want {
				t.Errorf("GetInstallMethod() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFromApplication_Installer(t *testing.T) {
	app := config.Application{
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

	got := FromApplication(app)
	if got == nil {
		t.Fatal("FromApplication() = nil, want non-nil")
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

func TestParseWingetListOutput(t *testing.T) {
	t.Parallel()

	t.Run("standard output", func(t *testing.T) {
		t.Parallel()
		output := `Name                                       Id                                         Version          Available Source
-----------------------------------------------------------------------------------------------------------------------
7-Zip 26.00 (x64)                          7zip.7zip                                  26.00                      winget
Git                                        Git.Git                                    2.53.0                     winget
Starship                                   Starship.Starship                          1.17.1                     winget
zoxide                                     ajeetdsouza.zoxide                         0.9.4                      winget
`
		ids := parseWingetListOutput(output)

		expected := []string{"7zip.7zip", "git.git", "starship.starship", "ajeetdsouza.zoxide"}
		for _, id := range expected {
			if !ids[id] {
				t.Errorf("expected %q in installed IDs", id)
			}
		}
	})

	t.Run("case insensitive lookup", func(t *testing.T) {
		t.Parallel()
		output := `Name       Id                    Version Source
----------------------------------------------------
Git        Git.Git               2.53.0  winget
`
		ids := parseWingetListOutput(output)

		if !ids["git.git"] {
			t.Error("expected case-insensitive ID lookup to work")
		}
	})

	t.Run("empty output", func(t *testing.T) {
		t.Parallel()
		ids := parseWingetListOutput("")

		if len(ids) != 0 {
			t.Errorf("expected 0 IDs, got %d", len(ids))
		}
	})

	t.Run("no header separator", func(t *testing.T) {
		t.Parallel()
		ids := parseWingetListOutput("Some random text\nwithout dashes")

		if len(ids) != 0 {
			t.Errorf("expected 0 IDs, got %d", len(ids))
		}
	})

	t.Run("with progress spinner lines", func(t *testing.T) {
		t.Parallel()
		output := `-    \
-    \    |    /
Name       Id                    Version Source
----------------------------------------------------
Git        Git.Git               2.53.0  winget
`
		ids := parseWingetListOutput(output)

		if !ids["git.git"] {
			t.Error("expected git.git to be found despite progress spinner lines")
		}
	})

	t.Run("with carriage return spinner from piped output", func(t *testing.T) {
		t.Parallel()
		// When winget writes to a pipe, progress spinner uses \r to overwrite.
		// Lines use \r\n endings (Windows). The spinner chars and header are on
		// the same line separated by \r, with \r\n at the end.
		output := "\r- \r\\ \r| \r/ \rName       Id                    Version Source\r\n" +
			"----------------------------------------------------\r\n" +
			"Git        Git.Git               2.53.0  winget\r\n" +
			"zoxide     ajeetdsouza.zoxide    0.9.4   winget\r\n"

		ids := parseWingetListOutput(output)

		if !ids["git.git"] {
			t.Error("expected git.git to be found with \\r spinner prefix")
		}
		if !ids["ajeetdsouza.zoxide"] {
			t.Error("expected ajeetdsouza.zoxide to be found with \\r spinner prefix")
		}
	})
}

func TestBuildCommand(t *testing.T) {
	tests := []struct {
		name     string
		pkg      Package
		method   string
		osType   string
		wantNil  bool
		wantArgs []string
	}{
		{
			name: "pacman",
			pkg: Package{
				Name:     "vim",
				Managers: map[PackageManager]ManagerValue{Pacman: {PackageName: "vim"}},
			},
			method:   "pacman",
			osType:   "linux",
			wantArgs: []string{"sudo", "pacman", "-S", "--noconfirm", "vim"},
		},
		{
			name: "yay",
			pkg: Package{
				Name:     "aur-pkg",
				Managers: map[PackageManager]ManagerValue{Yay: {PackageName: "aur-pkg"}},
			},
			method:   "yay",
			osType:   "linux",
			wantArgs: []string{"yay", "-S", "--noconfirm", "aur-pkg"},
		},
		{
			name: "apt",
			pkg: Package{
				Name:     "curl",
				Managers: map[PackageManager]ManagerValue{Apt: {PackageName: "curl"}},
			},
			method:   "apt",
			osType:   "linux",
			wantArgs: []string{"sudo", "apt-get", "install", "-y", "curl"},
		},
		{
			name: "winget",
			pkg: Package{
				Name:     "Publisher.App",
				Managers: map[PackageManager]ManagerValue{Winget: {PackageName: "Publisher.App"}},
			},
			method:   "winget",
			osType:   "windows",
			wantArgs: []string{"winget", "install", "--accept-package-agreements", "--accept-source-agreements", "Publisher.App"},
		},
		{
			name: "installer linux",
			pkg: Package{
				Name: "rustup",
				Managers: map[PackageManager]ManagerValue{
					Installer: {Installer: &InstallerConfig{
						Command: map[string]string{
							"linux": "curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh",
						},
						Binary: "rustup",
					}},
				},
			},
			method:   "installer",
			osType:   "linux",
			wantArgs: []string{"sh", "-c", "curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh"},
		},
		{
			name: "installer windows",
			pkg: Package{
				Name: "rustup",
				Managers: map[PackageManager]ManagerValue{
					Installer: {Installer: &InstallerConfig{
						Command: map[string]string{
							"windows": "iwr https://win.rustup.rs | iex",
						},
					}},
				},
			},
			method:   "installer",
			osType:   "windows",
			wantArgs: []string{"powershell", "-Command", "iwr https://win.rustup.rs | iex"},
		},
		{
			name: "installer no command for OS",
			pkg: Package{
				Name: "tool",
				Managers: map[PackageManager]ManagerValue{
					Installer: {Installer: &InstallerConfig{
						Command: map[string]string{"windows": "install.bat"},
					}},
				},
			},
			method:  "installer",
			osType:  "linux",
			wantNil: true,
		},
		{
			name: "git clone without sudo",
			pkg: Package{
				Name: "dotfiles",
				Managers: map[PackageManager]ManagerValue{
					Git: {Git: &GitConfig{
						URL:     "https://github.com/user/dotfiles.git",
						Branch:  "main",
						Targets: map[string]string{"linux": "/home/user/.dotfiles"},
					}},
				},
			},
			method:   "git",
			osType:   "linux",
			wantArgs: []string{"git", "clone", "-b", "main", "https://github.com/user/dotfiles.git", "/home/user/.dotfiles"},
		},
		{
			name: "git clone with sudo",
			pkg: Package{
				Name: "sys-repo",
				Managers: map[PackageManager]ManagerValue{
					Git: {Git: &GitConfig{
						URL:     "https://github.com/user/repo.git",
						Targets: map[string]string{"linux": "/opt/repo"},
						Sudo:    true,
					}},
				},
			},
			method:   "git",
			osType:   "linux",
			wantArgs: []string{"sudo", "git", "clone", "https://github.com/user/repo.git", "/opt/repo"},
		},
		{
			name: "git no target for OS",
			pkg: Package{
				Name: "repo",
				Managers: map[PackageManager]ManagerValue{
					Git: {Git: &GitConfig{
						URL:     "https://github.com/user/repo.git",
						Targets: map[string]string{"windows": "C:\\repo"},
					}},
				},
			},
			method:  "git",
			osType:  "linux",
			wantNil: true,
		},
		{
			name: "custom linux",
			pkg: Package{
				Name:   "custom-tool",
				Custom: map[string]string{"linux": "make install"},
			},
			method:   "custom",
			osType:   "linux",
			wantArgs: []string{"sh", "-c", "make install"},
		},
		{
			name: "custom windows",
			pkg: Package{
				Name:   "custom-tool",
				Custom: map[string]string{"windows": "msbuild /t:install"},
			},
			method:   "custom",
			osType:   "windows",
			wantArgs: []string{"powershell", "-Command", "msbuild /t:install"},
		},
		{
			name: "url linux",
			pkg: Package{
				Name: "url-tool",
				URL: map[string]URLInstall{
					"linux": {URL: "https://example.com/tool", Command: "chmod +x {file} && {file}"},
				},
			},
			method: "url",
			osType: "linux",
		},
		{
			name: "unknown method",
			pkg: Package{
				Name: "pkg",
			},
			method:  "unknown",
			osType:  "linux",
			wantNil: true,
		},
		{
			name:    "empty method",
			pkg:     Package{Name: "pkg"},
			method:  "",
			osType:  "linux",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := BuildCommand(context.Background(), tt.pkg, tt.method, tt.osType)

			if tt.wantNil {
				if cmd != nil {
					t.Errorf("BuildCommand() = %v, want nil", cmd.Args)
				}
				return
			}

			if cmd == nil {
				t.Fatal("BuildCommand() = nil, want non-nil")
			}

			// For url method, just verify it's a shell command (args vary with temp paths)
			if tt.method == "url" {
				if tt.osType == "windows" {
					if cmd.Args[0] != "powershell" {
						t.Errorf("expected powershell, got %s", cmd.Args[0])
					}
				} else {
					if cmd.Args[0] != "sh" {
						t.Errorf("expected sh, got %s", cmd.Args[0])
					}
				}
				return
			}

			if len(cmd.Args) != len(tt.wantArgs) {
				t.Errorf("BuildCommand() args length = %d, want %d\n  got:  %v\n  want: %v",
					len(cmd.Args), len(tt.wantArgs), cmd.Args, tt.wantArgs)
				return
			}

			for i, arg := range cmd.Args {
				if arg != tt.wantArgs[i] {
					t.Errorf("BuildCommand() arg[%d] = %q, want %q\n  got:  %v\n  want: %v",
						i, arg, tt.wantArgs[i], cmd.Args, tt.wantArgs)
				}
			}
		})
	}
}

func TestBuildCommand_GitExpandsTildeInTarget(t *testing.T) {
	t.Parallel()

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("could not get home directory: %v", err)
	}

	tests := []struct {
		name       string
		target     string
		wantTarget string
	}{
		{
			name:       "tilde-slash prefix is expanded to home",
			target:     "~/.local/share/catppuccin/lazygit",
			wantTarget: filepath.Join(home, ".local/share/catppuccin/lazygit"),
		},
		{
			name:       "bare tilde is expanded to home",
			target:     "~",
			wantTarget: home,
		},
		{
			name:       "absolute path is passed through unchanged",
			target:     "/opt/repo",
			wantTarget: "/opt/repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pkg := Package{
				Name: "git-pkg",
				Managers: map[PackageManager]ManagerValue{
					Git: {Git: &GitConfig{
						URL:     "https://github.com/example/repo.git",
						Targets: map[string]string{"linux": tt.target},
					}},
				},
			}

			cmd := BuildCommand(context.Background(), pkg, string(Git), "linux")
			if cmd == nil {
				t.Fatal("BuildCommand() returned nil")
			}

			// Last arg is always the target path for git clone
			gotTarget := cmd.Args[len(cmd.Args)-1]
			if gotTarget != tt.wantTarget {
				t.Errorf("git clone target = %q, want %q", gotTarget, tt.wantTarget)
			}
		})
	}
}

func TestInstall_DryRun_WithDeps(t *testing.T) {
	tests := []struct {
		name        string
		osType      string
		wantMethod  string
		pkg         Package
		available   []PackageManager
		wantSuccess bool
	}{
		{
			name:      "winget main with winget deps installs successfully",
			available: []PackageManager{Winget},
			osType:    "windows",
			pkg: Package{
				Name: "app",
				Managers: map[PackageManager]ManagerValue{
					Winget: {
						PackageName: "Publisher.App",
						Deps:        []string{"Publisher.DepA", "Publisher.DepB"},
					},
				},
			},
			wantSuccess: true,
			wantMethod:  "winget",
		},
		{
			name:      "deps-only with no main name plus installer",
			available: []PackageManager{Pacman},
			osType:    "linux",
			pkg: Package{
				Name: "tool-with-deps",
				Managers: map[PackageManager]ManagerValue{
					Pacman: {
						PackageName: "",
						Deps:        []string{"base-devel", "cmake"},
					},
					Installer: {Installer: &InstallerConfig{
						Command: map[string]string{
							"linux": "curl -fsSL https://example.com/install.sh | sh",
						},
					}},
				},
			},
			wantSuccess: true,
			wantMethod:  "installer",
		},
		{
			name:      "deps with unavailable manager are skipped",
			available: []PackageManager{Pacman},
			osType:    "linux",
			pkg: Package{
				Name: "cross-platform-deps",
				Managers: map[PackageManager]ManagerValue{
					Pacman: {PackageName: "main-pkg"},
					PackageManager("apt"): {
						Deps: []string{"libssl-dev"},
					},
				},
			},
			wantSuccess: true,
			wantMethod:  "pacman",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Manager{
				ctx:          context.Background(),
				Config:       &Config{},
				OS:           tt.osType,
				DryRun:       true,
				Available:    tt.available,
				availableSet: toAvailableSet(tt.available),
			}

			result := m.Install(tt.pkg)

			if result.Success != tt.wantSuccess {
				t.Errorf("Install().Success = %v, want %v (message: %s)", result.Success, tt.wantSuccess, result.Message)
			}

			if result.Method != tt.wantMethod {
				t.Errorf("Install().Method = %q, want %q", result.Method, tt.wantMethod)
			}

			if result.Package != tt.pkg.Name {
				t.Errorf("Install().Package = %q, want %q", result.Package, tt.pkg.Name)
			}

			if tt.wantSuccess && result.Message == "" {
				t.Error("Expected non-empty message for successful dry-run")
			}
		})
	}
}

func TestInstall_DryRun_DepFailure(t *testing.T) {
	tests := []struct {
		name       string
		osType     string
		wantMethod string
		pkg        Package
		available  []PackageManager
	}{
		{
			name:      "dep failure with unknown but available manager aborts main install",
			available: []PackageManager{Pacman, PackageManager("fakemgr")},
			osType:    "linux",
			pkg: Package{
				Name: "aborted-pkg",
				Managers: map[PackageManager]ManagerValue{
					Pacman: {PackageName: "main-pkg"},
					PackageManager("fakemgr"): {
						PackageName: "side-pkg",
						Deps:        []string{"missing-dep"},
					},
				},
			},
			wantMethod: "fakemgr",
		},
		{
			name:      "dep failure prevents custom fallback",
			available: []PackageManager{PackageManager("bogus")},
			osType:    "linux",
			pkg: Package{
				Name: "custom-fallback-pkg",
				Managers: map[PackageManager]ManagerValue{
					PackageManager("bogus"): {
						Deps: []string{"some-dep"},
					},
				},
				Custom: map[string]string{"linux": "make install"},
			},
			wantMethod: "bogus",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Manager{
				ctx:          context.Background(),
				Config:       &Config{},
				OS:           tt.osType,
				DryRun:       true,
				Available:    tt.available,
				availableSet: toAvailableSet(tt.available),
			}

			result := m.Install(tt.pkg)

			if result.Success {
				t.Errorf("Expected failure due to dep, but got success (message: %s)", result.Message)
			}

			if result.Method != tt.wantMethod {
				t.Errorf("Install().Method = %q, want %q", result.Method, tt.wantMethod)
			}

			if !strings.Contains(result.Message, "Dependency") {
				t.Errorf("Expected 'Dependency' in failure message, got: %s", result.Message)
			}
		})
	}
}

func TestIsGitInstalled(t *testing.T) {
	t.Parallel()

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("could not get home dir: %v", err)
	}

	// Create a real temp dir with a .git subdirectory
	clonedDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(clonedDir, ".git"), 0755); err != nil {
		t.Fatalf("could not create .git dir: %v", err)
	}

	// A temp dir that exists but has no .git
	bareDir := t.TempDir()

	// Create a temp dir under home so tilde expansion maps back to it.
	// On Windows CI, os.MkdirTemp may return 8.3 short paths (e.g., RUNNER~1)
	// that don't match os.UserHomeDir(), so we skip the tilde test there.
	var tildeCloned string
	hasTildeTest := runtime.GOOS != "windows"
	if hasTildeTest {
		tildeDir, tildeErr := os.MkdirTemp(home, ".tidydots-test-*")
		if tildeErr != nil {
			t.Fatalf("could not create tilde test dir: %v", tildeErr)
		}
		t.Cleanup(func() { os.RemoveAll(tildeDir) })
		if mkErr := os.Mkdir(filepath.Join(tildeDir, ".git"), 0755); mkErr != nil {
			t.Fatalf("could not create .git in tilde test dir: %v", mkErr)
		}

		// Build a tilde path that maps to the tilde dir (which is under home)
		tildeCloned = "~" + strings.TrimPrefix(tildeDir, home)
	}

	tests := []struct {
		name    string
		targets map[string]string
		osType  string
		want    bool
	}{
		{
			name:    "cloned repo with .git exists",
			targets: map[string]string{"linux": clonedDir},
			osType:  "linux",
			want:    true,
		},
		{
			name:    "dir exists but no .git",
			targets: map[string]string{"linux": bareDir},
			osType:  "linux",
			want:    false,
		},
		{
			name:    "target dir does not exist",
			targets: map[string]string{"linux": "/tmp/tidydots-does-not-exist-xyz"},
			osType:  "linux",
			want:    false,
		},
		{
			name:    "no target for this os",
			targets: map[string]string{"windows": clonedDir},
			osType:  "linux",
			want:    false,
		},
	}

	if hasTildeTest {
		tests = append(tests, struct {
			name    string
			targets map[string]string
			osType  string
			want    bool
		}{
			name:    "tilde path expansion - cloned",
			targets: map[string]string{"linux": tildeCloned},
			osType:  "linux",
			want:    true,
		})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := IsGitInstalled(tt.targets, tt.osType)
			if got != tt.want {
				t.Errorf("IsGitInstalled() = %v, want %v", got, tt.want)
			}
		})
	}
}
