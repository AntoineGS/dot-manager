package config

import "strings"

// EvaluateWhen evaluates a template-based when expression.
// Empty when returns true (always match). Nil renderer returns false.
// The template is rendered and the trimmed result is checked against "true".
// Any render error results in false (no match).
func EvaluateWhen(when string, renderer PathRenderer) bool {
	if strings.TrimSpace(when) == "" {
		return true
	}

	if renderer == nil {
		return false
	}

	result, err := renderer.RenderString("when", when)
	if err != nil {
		return false
	}

	return strings.TrimSpace(result) == "true"
}
