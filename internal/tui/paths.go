package tui

import "github.com/AntoineGS/tidydots/internal/manager"

// pathManager also supports NewModel callers before the operational manager is
// attached by the app entry point. Both use the same platform template context.
func (m Model) pathManager() *manager.Manager {
	if m.Manager != nil {
		return m.Manager
	}
	return manager.New(m.Config, m.Platform)
}

func resolveSubEntryPaths(item SubEntryItem, mgr *manager.Manager) (target, backup string, err error) {
	// Prefer raw configuration so already-rendered template output is not
	// interpreted as a second template on status refresh or restore.
	target = item.SubEntry.GetTarget(mgr.Platform.OS)
	if target == "" {
		target = item.Target
	}
	target, err = mgr.ExpandTarget(target)
	if err != nil {
		return "", "", err
	}
	backup, err = mgr.ResolvePath(item.SubEntry.Backup)
	return target, backup, err
}
