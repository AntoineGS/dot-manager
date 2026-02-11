package tui

import "github.com/charmbracelet/bubbles/key"

// RenderHelpFromBindings generates help text from key.Binding values.
// It extracts help pairs from each enabled binding and feeds them into
// the existing RenderHelpWithWidth, preserving the single-char highlight behavior.
func RenderHelpFromBindings(width int, bindings ...key.Binding) string {
	var pairs []string
	for _, b := range bindings {
		if !b.Enabled() {
			continue
		}
		h := b.Help()
		pairs = append(pairs, h.Key, h.Desc)
	}
	return RenderHelpWithWidth(width, pairs...)
}
