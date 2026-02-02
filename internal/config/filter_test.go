package config

import "testing"

func TestFilterMatches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		filter Filter
		ctx    *FilterContext
		want   bool
	}{
		{
			name:   "empty filter matches everything",
			filter: Filter{},
			ctx:    &FilterContext{OS: "linux", Hostname: "desktop", User: "john"},
			want:   true,
		},
		{
			name:   "include os matches",
			filter: Filter{Include: map[string]string{"os": "linux"}},
			ctx:    &FilterContext{OS: "linux", Hostname: "desktop", User: "john"},
			want:   true,
		},
		{
			name:   "include os does not match",
			filter: Filter{Include: map[string]string{"os": "windows"}},
			ctx:    &FilterContext{OS: "linux", Hostname: "desktop", User: "john"},
			want:   false,
		},
		{
			name:   "include hostname matches",
			filter: Filter{Include: map[string]string{"hostname": "desktop"}},
			ctx:    &FilterContext{OS: "linux", Hostname: "desktop", User: "john"},
			want:   true,
		},
		{
			name:   "include user matches",
			filter: Filter{Include: map[string]string{"user": "john"}},
			ctx:    &FilterContext{OS: "linux", Hostname: "desktop", User: "john"},
			want:   true,
		},
		{
			name:   "include multiple conditions AND - all match",
			filter: Filter{Include: map[string]string{"os": "linux", "user": "john"}},
			ctx:    &FilterContext{OS: "linux", Hostname: "desktop", User: "john"},
			want:   true,
		},
		{
			name:   "include multiple conditions AND - one fails",
			filter: Filter{Include: map[string]string{"os": "linux", "user": "jane"}},
			ctx:    &FilterContext{OS: "linux", Hostname: "desktop", User: "john"},
			want:   false,
		},
		{
			name:   "exclude user matches - filter fails",
			filter: Filter{Exclude: map[string]string{"user": "root"}},
			ctx:    &FilterContext{OS: "linux", Hostname: "desktop", User: "root"},
			want:   false,
		},
		{
			name:   "exclude user does not match - filter passes",
			filter: Filter{Exclude: map[string]string{"user": "root"}},
			ctx:    &FilterContext{OS: "linux", Hostname: "desktop", User: "john"},
			want:   true,
		},
		{
			name:   "include and exclude - both pass",
			filter: Filter{Include: map[string]string{"os": "linux"}, Exclude: map[string]string{"user": "root"}},
			ctx:    &FilterContext{OS: "linux", Hostname: "desktop", User: "john"},
			want:   true,
		},
		{
			name:   "include passes but exclude fails",
			filter: Filter{Include: map[string]string{"os": "linux"}, Exclude: map[string]string{"user": "root"}},
			ctx:    &FilterContext{OS: "linux", Hostname: "desktop", User: "root"},
			want:   false,
		},
		{
			name:   "unknown attribute returns empty string",
			filter: Filter{Include: map[string]string{"unknown": "value"}},
			ctx:    &FilterContext{OS: "linux", Hostname: "desktop", User: "john"},
			want:   false,
		},
		{
			name:   "include distro matches",
			filter: Filter{Include: map[string]string{"distro": "arch"}},
			ctx:    &FilterContext{OS: "linux", Distro: "arch", Hostname: "desktop", User: "john"},
			want:   true,
		},
		{
			name:   "include distro does not match",
			filter: Filter{Include: map[string]string{"distro": "ubuntu"}},
			ctx:    &FilterContext{OS: "linux", Distro: "arch", Hostname: "desktop", User: "john"},
			want:   false,
		},
		{
			name:   "include os and distro both match",
			filter: Filter{Include: map[string]string{"os": "linux", "distro": "arch"}},
			ctx:    &FilterContext{OS: "linux", Distro: "arch", Hostname: "desktop", User: "john"},
			want:   true,
		},
		{
			name:   "exclude distro matches - filter fails",
			filter: Filter{Exclude: map[string]string{"distro": "ubuntu"}},
			ctx:    &FilterContext{OS: "linux", Distro: "ubuntu", Hostname: "desktop", User: "john"},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.filter.Matches(tt.ctx)
			if got != tt.want {
				t.Errorf("Matches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterMatchesRegex(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		filter Filter
		ctx    *FilterContext
		want   bool
	}{
		{
			name:   "regex hostname pattern matches",
			filter: Filter{Include: map[string]string{"hostname": "work-.*"}},
			ctx:    &FilterContext{OS: "linux", Hostname: "work-laptop", User: "john"},
			want:   true,
		},
		{
			name:   "regex hostname pattern does not match",
			filter: Filter{Include: map[string]string{"hostname": "work-.*"}},
			ctx:    &FilterContext{OS: "linux", Hostname: "home-desktop", User: "john"},
			want:   false,
		},
		{
			name:   "regex OR pattern in hostname",
			filter: Filter{Include: map[string]string{"hostname": "desktop|laptop"}},
			ctx:    &FilterContext{OS: "linux", Hostname: "laptop", User: "john"},
			want:   true,
		},
		{
			name:   "regex OR pattern in os",
			filter: Filter{Include: map[string]string{"os": "linux|darwin"}},
			ctx:    &FilterContext{OS: "darwin", Hostname: "macbook", User: "john"},
			want:   true,
		},
		{
			name:   "regex exclude pattern matches",
			filter: Filter{Exclude: map[string]string{"hostname": "test-.*"}},
			ctx:    &FilterContext{OS: "linux", Hostname: "test-server", User: "john"},
			want:   false,
		},
		{
			name:   "regex exclude pattern does not match",
			filter: Filter{Exclude: map[string]string{"hostname": "test-.*"}},
			ctx:    &FilterContext{OS: "linux", Hostname: "prod-server", User: "john"},
			want:   true,
		},
		{
			name:   "invalid regex falls back to exact match - matches",
			filter: Filter{Include: map[string]string{"os": "[linux"}}, // invalid regex
			ctx:    &FilterContext{OS: "[linux", Hostname: "desktop", User: "john"},
			want:   true,
		},
		{
			name:   "invalid regex falls back to exact match - no match",
			filter: Filter{Include: map[string]string{"os": "[linux"}}, // invalid regex
			ctx:    &FilterContext{OS: "linux", Hostname: "desktop", User: "john"},
			want:   false,
		},
		{
			name:   "regex OR pattern in distro",
			filter: Filter{Include: map[string]string{"distro": "arch|manjaro|endeavouros"}},
			ctx:    &FilterContext{OS: "linux", Distro: "manjaro", Hostname: "desktop", User: "john"},
			want:   true,
		},
		{
			name:   "regex OR pattern in distro - ubuntu family",
			filter: Filter{Include: map[string]string{"distro": "ubuntu|debian|pop"}},
			ctx:    &FilterContext{OS: "linux", Distro: "ubuntu", Hostname: "desktop", User: "john"},
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.filter.Matches(tt.ctx)
			if got != tt.want {
				t.Errorf("Matches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatchesFilters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		filters []Filter
		ctx     *FilterContext
		want    bool
	}{
		{
			name:    "nil filters always match",
			filters: nil,
			ctx:     &FilterContext{OS: "linux", Hostname: "desktop", User: "john"},
			want:    true,
		},
		{
			name:    "empty filters always match",
			filters: []Filter{},
			ctx:     &FilterContext{OS: "linux", Hostname: "desktop", User: "john"},
			want:    true,
		},
		{
			name: "single filter matches",
			filters: []Filter{
				{Include: map[string]string{"os": "linux"}},
			},
			ctx:  &FilterContext{OS: "linux", Hostname: "desktop", User: "john"},
			want: true,
		},
		{
			name: "single filter does not match",
			filters: []Filter{
				{Include: map[string]string{"os": "windows"}},
			},
			ctx:  &FilterContext{OS: "linux", Hostname: "desktop", User: "john"},
			want: false,
		},
		{
			name: "multiple filters OR logic - first matches",
			filters: []Filter{
				{Include: map[string]string{"os": "linux"}},
				{Include: map[string]string{"os": "darwin"}},
			},
			ctx:  &FilterContext{OS: "linux", Hostname: "desktop", User: "john"},
			want: true,
		},
		{
			name: "multiple filters OR logic - second matches",
			filters: []Filter{
				{Include: map[string]string{"os": "windows"}},
				{Include: map[string]string{"os": "linux"}},
			},
			ctx:  &FilterContext{OS: "linux", Hostname: "desktop", User: "john"},
			want: true,
		},
		{
			name: "multiple filters OR logic - none match",
			filters: []Filter{
				{Include: map[string]string{"os": "windows"}},
				{Include: map[string]string{"os": "darwin"}},
			},
			ctx:  &FilterContext{OS: "linux", Hostname: "desktop", User: "john"},
			want: false,
		},
		{
			name: "complex filters - linux non-root or darwin",
			filters: []Filter{
				{Include: map[string]string{"os": "linux"}, Exclude: map[string]string{"user": "root"}},
				{Include: map[string]string{"os": "darwin"}},
			},
			ctx:  &FilterContext{OS: "linux", Hostname: "desktop", User: "john"},
			want: true,
		},
		{
			name: "complex filters - linux as root fails first, second doesn't match",
			filters: []Filter{
				{Include: map[string]string{"os": "linux"}, Exclude: map[string]string{"user": "root"}},
				{Include: map[string]string{"os": "darwin"}},
			},
			ctx:  &FilterContext{OS: "linux", Hostname: "desktop", User: "root"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := MatchesFilters(tt.filters, tt.ctx)
			if got != tt.want {
				t.Errorf("MatchesFilters() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterContextGetAttribute(t *testing.T) {
	t.Parallel()

	ctx := &FilterContext{
		OS:       "linux",
		Distro:   "arch",
		Hostname: "desktop",
		User:     "john",
	}

	tests := []struct {
		attr string
		want string
	}{
		{"os", "linux"},
		{"distro", "arch"},
		{"hostname", "desktop"},
		{"user", "john"},
		{"unknown", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.attr, func(t *testing.T) {
			t.Parallel()
			got := ctx.getAttribute(tt.attr)
			if got != tt.want {
				t.Errorf("getAttribute(%q) = %q, want %q", tt.attr, got, tt.want)
			}
		})
	}
}

func TestMatchesPattern(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		pattern string
		value   string
		want    bool
	}{
		{"exact match", "linux", "linux", true},
		{"exact no match", "linux", "windows", false},
		{"regex alternation", "linux|darwin", "linux", true},
		{"regex alternation second", "linux|darwin", "darwin", true},
		{"regex alternation no match", "linux|darwin", "windows", false},
		{"regex wildcard", "work-.*", "work-laptop", true},
		{"regex wildcard no match", "work-.*", "home-desktop", false},
		{"regex anchored - partial no match", "lin", "linux", false},
		{"empty pattern matches empty value", "", "", true},
		{"empty pattern no match non-empty", "", "linux", false},
		{"empty value no match non-empty pattern", "linux", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := matchesPattern(tt.pattern, tt.value)
			if got != tt.want {
				t.Errorf("matchesPattern(%q, %q) = %v, want %v", tt.pattern, tt.value, got, tt.want)
			}
		})
	}
}
