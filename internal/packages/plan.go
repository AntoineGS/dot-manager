package packages

// DependencyStep is one package-manager dependency in execution order.
type DependencyStep struct {
	Manager PackageManager
	Name    string
}

// InstallationPlan resolves selection without executing commands or doing discovery.
type InstallationPlan struct {
	Method       string
	Dependencies []DependencyStep
}

// PlanInstallation uses configured priority, then default, then discovery order.
// Git and installer retain their domain installation precedence. Dependency-only
// manager values never become main installation candidates.
func PlanInstallation(pkg Package, cfg *Config, osType string, available []PackageManager) InstallationPlan {
	plan := InstallationPlan{Method: MethodNone}
	order := installationManagerOrder(cfg, available)
	for _, mgr := range order {
		if mgr == Git || mgr == Installer {
			continue
		}
		val := pkg.Managers[mgr]
		for _, dep := range val.Deps {
			plan.Dependencies = append(plan.Dependencies, DependencyStep{Manager: mgr, Name: dep})
		}
		if plan.Method == MethodNone && val.PackageName != "" {
			plan.Method = string(mgr)
		}
	}
	plan.Method = installationMainMethod(pkg, osType, plan.Method)
	return plan
}

func installationManagerOrder(cfg *Config, available []PackageManager) []PackageManager {
	seen := make(map[PackageManager]bool)
	present := make(map[PackageManager]bool)
	for _, mgr := range available {
		present[mgr] = true
	}
	var order []PackageManager
	add := func(mgr PackageManager) {
		if present[mgr] && !seen[mgr] {
			order = append(order, mgr)
			seen[mgr] = true
		}
	}
	if cfg != nil {
		for _, mgr := range cfg.ManagerPriority {
			add(mgr)
		}
		add(cfg.DefaultManager)
	}
	for _, mgr := range available {
		add(mgr)
	}
	return order
}

func installationMainMethod(pkg Package, osType, managerMethod string) string {
	if val, ok := pkg.Managers[Git]; ok && val.IsGit() {
		return string(Git)
	}
	if val, ok := pkg.Managers[Installer]; ok && val.IsInstaller() {
		return string(Installer)
	}
	if managerMethod != MethodNone {
		return managerMethod
	}
	if _, ok := pkg.Custom[osType]; ok {
		return MethodCustom
	}
	if _, ok := pkg.URL[osType]; ok {
		return MethodURL
	}
	return MethodNone
}
