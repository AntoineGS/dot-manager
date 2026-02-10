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
	IsWSL      bool
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

	p.IsWSL = detectWSL()
	p.HasDisplay = detectDisplay(p.OS)

	if p.OS == OSLinux {
		p.Distro = detectDistro()
	}

	if p.OS == OSWindows {
		p.detectPowerShellProfile()
	}

	// Provide OS/WSL hints so DetectAvailableManagers can skip slow Windows drive mounts
	SetDetectionHints(p.OS, p.IsWSL)

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

// detectWSL checks if running inside Windows Subsystem for Linux.
func detectWSL() bool {
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}

	lower := strings.ToLower(string(data))
	return strings.Contains(lower, "microsoft") || strings.Contains(lower, "wsl")
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

// detectWindowsDriveMounts reads /proc/mounts and returns the mount points of
// Windows drive filesystems (9p/drvfs). Returns nil on error or if none found.
func detectWindowsDriveMounts() []string {
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return nil
	}

	var mounts []string

	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		fsType := fields[2]
		options := fields[3]

		if fsType == "9p" && strings.Contains(options, "drvfs") {
			mounts = append(mounts, fields[1])
		}
	}

	return mounts
}

// isUnderWindowsDrive reports whether dir falls under one of the given
// Windows drive mount points (e.g. /mnt/c, /mnt/d).
func isUnderWindowsDrive(dir string, mounts []string) bool {
	for _, mount := range mounts {
		if dir == mount || strings.HasPrefix(dir, mount+"/") {
			return true
		}
	}

	return false
}

// lookPathSkipWindowsDrives is like exec.LookPath but skips PATH directories
// under WSL Windows drive mounts (identified via /proc/mounts as 9p/drvfs).
// These are slow NTFS mounts and irrelevant for Linux package managers.
func lookPathSkipWindowsDrives(file string, mounts []string) bool {
	pathEnv := os.Getenv("PATH")
	if pathEnv == "" {
		return false
	}

	for _, dir := range strings.Split(pathEnv, ":") {
		if isUnderWindowsDrive(dir, mounts) {
			continue
		}

		path := dir + "/" + file
		if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
			return true
		}
	}

	return false
}

var (
	availableManagersOnce   sync.Once
	availableManagersCached []string
	detectedWSL             bool
	windowsDriveMounts      []string
)

// SetDetectionHints provides WSL information so that DetectAvailableManagers
// can skip slow Windows drive mounts. On WSL, it reads /proc/mounts to
// identify 9p/drvfs mount points. Call this before DetectAvailableManagers.
func SetDetectionHints(_ string, isWSL bool) {
	detectedWSL = isWSL
	if isWSL {
		windowsDriveMounts = detectWindowsDriveMounts()
	}
}

// DetectAvailableManagers returns a list of package managers available on the system
// by checking which managers from KnownPackageManagers are present in the PATH.
// On WSL, it skips slow Windows drive mount PATH entries (e.g. /mnt/c/).
// Results are cached after the first call since PATH rarely changes during execution.
func DetectAvailableManagers() []string {
	availableManagersOnce.Do(func() {
		available := make([]string, 0, len(KnownPackageManagers))

		for _, mgr := range KnownPackageManagers {
			if detectedWSL && len(windowsDriveMounts) > 0 {
				if lookPathSkipWindowsDrives(mgr, windowsDriveMounts) {
					available = append(available, mgr)
				}
			} else {
				if IsCommandAvailable(mgr) {
					available = append(available, mgr)
				}
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
