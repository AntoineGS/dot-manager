package config

import (
	"testing"
)

func BenchmarkGetFilteredConfigEntries(b *testing.B) {
	cfg := &Config{
		Applications: make([]Application, 100),
	}

	// Populate with test data
	for i := range cfg.Applications {
		cfg.Applications[i] = Application{
			Name: "test",
			Entries: []SubEntry{
				{
					Name:   "test-config",
					Backup: "./test",
				},
			},
		}
	}

	ctx := &FilterContext{
		OS:       "linux",
		Distro:   "arch",
		Hostname: "localhost",
		User:     "user",
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = cfg.GetAllConfigSubEntries(ctx)
	}
}
