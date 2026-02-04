package tui

import (
	"testing"

	"github.com/AntoineGS/dot-manager/internal/config"
	"github.com/AntoineGS/dot-manager/internal/platform"
)

func TestNewModel_InitializesSelectionState(t *testing.T) {
	cfg := &config.Config{
		Applications: []config.Application{},
	}
	plat := &platform.Platform{
		OS:      "linux",
		EnvVars: map[string]string{"HOME": "/home/test"},
	}

	m := NewModel(cfg, plat, false)

	if m.selectedApps == nil {
		t.Error("selectedApps map should be initialized")
	}
	if m.selectedSubEntries == nil {
		t.Error("selectedSubEntries map should be initialized")
	}
	if m.multiSelectActive {
		t.Error("multiSelectActive should be false initially")
	}
}
