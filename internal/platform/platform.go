package platform

import (
	"os"
	"os/exec"
	"os/user"
	"runtime"
	"strings"
)

const (
	OSLinux   = "linux"
	OSWindows = "windows"
)

type Platform struct {
	OS       string
	Distro   string // Linux distribution ID (e.g., "arch", "ubuntu", "fedora")
	Hostname string
	User     string
	IsRoot   bool
	IsArch   bool
	EnvVars  map[string]string
}

func Detect() *Platform {
	p := &Platform{
		OS:       detectOS(),
		Hostname: detectHostname(),
		User:     detectUser(),
		EnvVars:  make(map[string]string),
	}

	if p.OS == OSLinux {
		p.Distro = detectDistro()
		p.IsRoot = detectRoot()
		p.IsArch = p.Distro == "arch"
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
	hostname, _ := os.Hostname()
	return hostname
}

func detectUser() string {
	u, err := user.Current()
	if err != nil {
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

func detectRoot() bool {
	u, err := user.Current()
	if err != nil {
		return false
	}
	return u.Uid == "0"
}


func (p *Platform) detectPowerShellProfile() {
	cmd := exec.Command("pwsh", "-NoProfile", "-Command", "echo $PROFILE")
	output, err := cmd.Output()
	if err != nil {
		return
	}

	profile := strings.TrimSpace(string(output))
	if profile != "" {
		p.EnvVars["PWSH_PROFILE"] = profile
		p.EnvVars["PWSH_PROFILE_FILE"] = getBasename(profile)
		p.EnvVars["PWSH_PROFILE_PATH"] = getDirname(profile)
	}
}

func getBasename(path string) string {
	// Handle both Unix and Windows separators
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[i+1:]
		}
	}
	return path
}

func getDirname(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[:i]
		}
	}
	return "."
}

func (p *Platform) WithOS(osType string) *Platform {
	newP := *p
	newP.OS = osType
	return &newP
}

func (p *Platform) WithHostname(hostname string) *Platform {
	newP := *p
	newP.Hostname = hostname
	return &newP
}

func (p *Platform) WithUser(username string) *Platform {
	newP := *p
	newP.User = username
	return &newP
}

func (p *Platform) WithDistro(distro string) *Platform {
	newP := *p
	newP.Distro = distro
	return &newP
}

// IsCommandAvailable checks if a command is available in PATH
func IsCommandAvailable(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

// KnownPackageManagers returns the list of supported package managers
var KnownPackageManagers = []string{
	"yay", "paru", "pacman", // Arch Linux
	"apt", "dnf", "brew", // Debian/Fedora/macOS
	"winget", "scoop", "choco", // Windows
}

// DetectAvailableManagers returns a list of package managers available on the system
func DetectAvailableManagers() []string {
	var available []string
	for _, mgr := range KnownPackageManagers {
		if IsCommandAvailable(mgr) {
			available = append(available, mgr)
		}
	}
	return available
}
