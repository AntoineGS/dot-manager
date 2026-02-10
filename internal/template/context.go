// Package template provides template rendering with platform-aware context and 3-way merge support.
package template

import (
	"os"
	"strings"

	"github.com/AntoineGS/tidydots/internal/platform"
)

// Context holds platform-aware data available to all templates.
type Context struct {
	OS         string
	Distro     string
	Hostname   string
	User       string
	HasDisplay bool
	Env        map[string]string
}

// NewContextFromPlatform creates a Context from platform detection results,
// merging platform EnvVars with the process environment.
func NewContextFromPlatform(p *platform.Platform) *Context {
	env := make(map[string]string)

	// Populate from os.Environ()
	for _, e := range os.Environ() {
		if k, v, ok := strings.Cut(e, "="); ok {
			env[k] = v
		}
	}

	// Merge platform-specific env vars (override process env)
	for k, v := range p.EnvVars {
		env[k] = v
	}

	return &Context{
		OS:         p.OS,
		Distro:     p.Distro,
		Hostname:   p.Hostname,
		User:       p.User,
		HasDisplay: p.HasDisplay,
		Env:        env,
	}
}
