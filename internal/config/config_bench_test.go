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

	renderer := &mockWhenRenderer{result: "true"}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = cfg.GetAllConfigSubEntries(renderer)
	}
}
