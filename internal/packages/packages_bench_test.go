package packages

import (
	"testing"

	"github.com/AntoineGS/dot-manager/internal/config"
)

func BenchmarkFilterPackages(b *testing.B) {
	packages := make([]Package, 100)
	for i := range packages {
		packages[i] = Package{
			Name: "test-package",
			Managers: map[PackageManager]interface{}{
				Pacman: "test",
			},
		}
	}

	ctx := &config.FilterContext{
		OS:       "linux",
		Distro:   "arch",
		Hostname: "localhost",
		User:     "user",
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = FilterPackages(packages, ctx)
	}
}

func BenchmarkFromEntries(b *testing.B) {
	entries := make([]config.Entry, 100)
	for i := range entries {
		entries[i] = config.Entry{
			Name: "test",
			Package: &config.EntryPackage{
				Managers: map[string]string{
					"pacman": "test",
				},
			},
		}
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = FromEntries(entries)
	}
}
