package config

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestSetupCheckModeRoundTrip(t *testing.T) {
	for _, mode := range []string{"", "exit-code", "status"} {
		t.Run(mode, func(t *testing.T) {
			entry := SubEntry{Name: "binary", CheckMode: mode,
				Check: map[string]string{"linux": "check"},
				Run:   map[string]string{"linux": "apply"}}
			if errs := validateSetupEntry("tool", entry); len(errs) != 0 {
				t.Fatalf("validation: %v", errs)
			}
			data, err := yaml.Marshal(entry)
			if err != nil {
				t.Fatal(err)
			}
			var got SubEntry
			if err := yaml.Unmarshal(data, &got); err != nil {
				t.Fatal(err)
			}
			if got.CheckMode != mode {
				t.Fatalf("mode = %q, want %q", got.CheckMode, mode)
			}
			if mode == "" && strings.Contains(string(data), "check_mode:") {
				t.Fatal("default mode must stay omitted")
			}
		})
	}
}

func TestSetupCheckModeValidation(t *testing.T) {
	entries := []SubEntry{
		{Name: "bad", CheckMode: "unknown", Check: map[string]string{"linux": "check"}, Run: map[string]string{"linux": "run"}},
		{Name: "config", CheckMode: "status", Backup: "./config", Targets: map[string]string{"linux": "~/.config/tool"}},
		{Name: "config", CheckMode: "exit-code", Backup: "./config", Targets: map[string]string{"linux": "~/.config/tool"}},
		{Name: "orphan", CheckMode: "status"},
	}
	for _, entry := range entries {
		if errs := validateSetupEntry("tool", entry); len(errs) == 0 {
			t.Errorf("accepted invalid entry: %+v", entry)
		}
	}
}
