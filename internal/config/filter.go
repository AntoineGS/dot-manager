package config

import (
	"regexp"
	"sync"
)

// Filter represents a single filter with include/exclude conditions.
// Include conditions are AND'd together - all must match.
// Exclude conditions are AND'd together - none must match.
type Filter struct {
	Include map[string]string `yaml:"include,omitempty"`
	Exclude map[string]string `yaml:"exclude,omitempty"`
}

// FilterContext contains the current environment attributes for matching.
type FilterContext struct {
	OS       string
	Distro   string // Linux distribution ID (e.g., "arch", "ubuntu", "fedora")
	Hostname string
	User     string
}

// Matches returns true if this filter matches the given context.
// Include: ALL conditions must match (AND logic).
// Exclude: NONE of the conditions must match (AND logic for exclusions).
func (f *Filter) Matches(ctx *FilterContext) bool {
	// Check all include conditions (AND logic)
	for attr, pattern := range f.Include {
		value := ctx.getAttribute(attr)
		if !matchesPattern(pattern, value) {
			return false
		}
	}

	// Check all exclude conditions (none should match)
	for attr, pattern := range f.Exclude {
		value := ctx.getAttribute(attr)
		if matchesPattern(pattern, value) {
			return false
		}
	}

	return true
}

func (ctx *FilterContext) getAttribute(attr string) string {
	switch attr {
	case "os":
		return ctx.OS
	case "distro":
		return ctx.Distro
	case "hostname":
		return ctx.Hostname
	case "user":
		return ctx.User
	default:
		return ""
	}
}

var regexCache sync.Map // pattern string -> *regexp.Regexp

func matchesPattern(pattern, value string) bool {
	fullPattern := "^(" + pattern + ")$"

	if cached, ok := regexCache.Load(fullPattern); ok {
		return cached.(*regexp.Regexp).MatchString(value)
	}

	re, err := regexp.Compile(fullPattern)
	if err != nil {
		return pattern == value // Fallback to exact match
	}

	regexCache.Store(fullPattern, re)

	return re.MatchString(value)
}

// MatchesFilters returns true if any filter matches (OR logic between filters).
// Empty/nil filters slice means always match (backward compatible).
func MatchesFilters(filters []Filter, ctx *FilterContext) bool {
	if len(filters) == 0 {
		return true
	}

	for _, f := range filters {
		if f.Matches(ctx) {
			return true
		}
	}

	return false
}
