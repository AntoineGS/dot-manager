package packages

import (
	"fmt"
	"strings"

	"github.com/AntoineGS/tidydots/internal/config"
)

// validateSelectedMain rejects invalid configuration before dependencies can
// change the system. It performs no filesystem inspection or command execution.
// An empty message means validation passed; failures preserve domain wording.
func validateSelectedMain(pkg Package, method, osType string) string {
	switch method {
	case string(Git):
		_, msg := validatedGitTarget(*pkg.Managers[Git].Git, osType)
		return msg
	case string(Installer):
		if _, ok := pkg.Managers[Installer].Installer.Command[osType]; !ok {
			return fmt.Sprintf("No installer command defined for OS: %s", osType)
		}
	case MethodURL:
		if err := validateURLScheme(pkg.URL[osType].URL); err != nil {
			return fmt.Sprintf("URL rejected: %v", err)
		}
	case MethodCustom, MethodNone:
		// Selection already resolved these methods; custom commands are trusted
		// user configuration and are intentionally not interpreted here.
	default:
		if _, ok := managerCmds[PackageManager(method)]; !ok {
			return fmt.Sprintf("Unknown package manager: %s", method)
		}
	}
	return ""
}

// validatedGitTarget validates git configuration and expands its target without
// touching the filesystem. Keep legitimate spaces in paths; only reject targets
// that are entirely empty/whitespace after expansion, which could select cwd.
// Returns the expanded target and an empty message, or a domain failure message.
func validatedGitTarget(gitCfg GitConfig, osType string) (string, string) {
	if err := validateURLScheme(gitCfg.URL); err != nil {
		return "", fmt.Sprintf("Git URL rejected: %v", err)
	}
	target, ok := gitCfg.Targets[osType]
	if !ok {
		return "", fmt.Sprintf("No git target path defined for OS: %s", osType)
	}
	target = config.ExpandPath(target, nil)
	if strings.TrimSpace(target) == "" {
		return "", fmt.Sprintf("No git target path defined for OS: %s", osType)
	}
	if err := ValidateGitBranch(gitCfg.Branch); err != nil {
		return "", fmt.Sprintf("Invalid git branch: %v", err)
	}
	return target, ""
}
