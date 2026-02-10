// Package platform provides OS and distribution detection.
package platform

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// Supported operating system identifiers.
const (
	// OSLinux represents Linux operating systems
	OSLinux = "linux"
	// OSWindows represents Windows operating systems
	OSWindows = "windows"
)

// Platform holds detected platform information including the operating system,
// Linux distribution, hostname, current user, and privilege status.
type Platform struct {
	EnvVars    map[string]string
	OS         string
	Distro     string
	Hostname   string
	User       string
	HasDisplay bool
}

// Detect detects the current platform characteristics including OS type,
// Linux distribution (if applicable), hostname, current user, and root status.
func Detect() *Platform {
	p := &Platform{
		OS:       detectOS(),
		Hostname: detectHostname(),
		User:     detectUser(),
		EnvVars:  make(map[string]string),
	}

	p.HasDisplay = detectDisplay(p.OS)

	if p.OS == OSLinux {
		p.Distro = detectDistro()
	}

	if p.OS == OSWindows {
		p.detectPowerShellProfile()
	}

	return p
}

// detectDistro returns the Linux distribution ID from /etc/os-release
// Returns values like "arch", "ubuntu", "fedora", "debian", etc.
func detectDistro() string {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		slog.Debug("unable to detect linux distribution",
			slog.String("file", "/etc/os-release"),
			slog.String("error", err.Error()),
			slog.String("fallback", "empty"))
		return ""
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "ID=") {
			id := strings.TrimPrefix(line, "ID=")
			id = strings.Trim(id, "\"")

			return id
		}
	}

	return ""
}

func detectHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		slog.Debug("unable to detect hostname",
			slog.String("error", err.Error()),
			slog.String("fallback", "empty"))
		return ""
	}

	return hostname
}

func detectUser() string {
	u, err := user.Current()
	if err != nil {
		slog.Debug("unable to detect current user",
			slog.String("error", err.Error()),
			slog.String("fallback", "empty"))
		return ""
	}

	return u.Username
}

func detectOS() string {
	if runtime.GOOS == "windows" {
		return OSWindows
	}

	// Also check OS environment variable (for cross-platform scripts)
	osEnv := os.Getenv("OS")
	if strings.Contains(strings.ToLower(osEnv), "windows") {
		return OSWindows
	}

	return OSLinux
}

// detectDisplay checks whether a display server is available.
// On Linux, it checks for DISPLAY (X11) or WAYLAND_DISPLAY (Wayland).
// On Windows, it always returns true.
func detectDisplay(osType string) bool {
	if osType == OSWindows {
		return true
	}

	if os.Getenv("DISPLAY") != "" {
		return true
	}

	if os.Getenv("WAYLAND_DISPLAY") != "" {
		return true
	}

	return false
}

func (p *Platform) detectPowerShellProfile() {
	cmd := exec.CommandContext(context.Background(), "pwsh", "-NoProfile", "-Command", "echo $PROFILE")

	output, err := cmd.Output()
	if err != nil {
		return
	}

	profile := strings.TrimSpace(string(output))
	if profile != "" {
		p.EnvVars["PWSH_PROFILE"] = profile
		p.EnvVars["PWSH_PROFILE_FILE"] = filepath.Base(profile)
		p.EnvVars["PWSH_PROFILE_PATH"] = filepath.Dir(profile)
	}
}

// IsArchLinux returns true if the detected distribution is Arch Linux.
func (p *Platform) IsArchLinux() bool {
	return p.Distro == "arch"
}

// copyMap returns a shallow copy of a string map.
func copyMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}

	cp := make(map[string]string, len(m))
	for k, v := range m {
		cp[k] = v
	}

	return cp
}

// WithOS returns a copy of the Platform with the OS field overridden.
func (p *Platform) WithOS(osType string) *Platform {
	newP := *p
	newP.OS = osType
	newP.EnvVars = copyMap(p.EnvVars)

	return &newP
}

// WithHostname returns a copy of the Platform with the Hostname field overridden.
func (p *Platform) WithHostname(hostname string) *Platform {
	newP := *p
	newP.Hostname = hostname
	newP.EnvVars = copyMap(p.EnvVars)

	return &newP
}

// WithUser returns a copy of the Platform with the User field overridden.
func (p *Platform) WithUser(username string) *Platform {
	newP := *p
	newP.User = username
	newP.EnvVars = copyMap(p.EnvVars)

	return &newP
}

// WithDistro returns a copy of the Platform with the Distro field overridden.
func (p *Platform) WithDistro(distro string) *Platform {
	newP := *p
	newP.Distro = distro
	newP.EnvVars = copyMap(p.EnvVars)

	return &newP
}

// IsCommandAvailable checks if a command is available in PATH
func IsCommandAvailable(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

// KnownPackageManagers is the list of supported package managers across all platforms.
// Includes Arch Linux (yay, paru, pacman), Debian/Fedora/macOS (apt, dnf, brew),
// Windows (winget, scoop, choco) package managers, and git for repository cloning.
var KnownPackageManagers = []string{
	"yay", "paru", "pacman", // Arch Linux
	"apt", "dnf", "brew", // Debian/Fedora/macOS
	"winget", "scoop", "choco", // Windows
	"git", // Git for repository cloning
}

var (
	availableManagersOnce   sync.Once
	availableManagersCached []string
)

// DetectAvailableManagers returns a list of package managers available on the system
// by checking which managers from KnownPackageManagers are present in the PATH.
// Results are cached after the first call since PATH rarely changes during execution.
func DetectAvailableManagers() []string {
	availableManagersOnce.Do(func() {
		available := make([]string, 0, len(KnownPackageManagers))

		for _, mgr := range KnownPackageManagers {
			if IsCommandAvailable(mgr) {
				available = append(available, mgr)
			}
		}

		availableManagersCached = available
	})

	return availableManagersCached
}

// ResetAvailableManagersCache clears the cached manager detection results,
// causing the next call to DetectAvailableManagers to re-scan PATH.
func ResetAvailableManagersCache() {
	availableManagersOnce = sync.Once{}
	availableManagersCached = nil
}
