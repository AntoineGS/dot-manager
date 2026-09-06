package template

// RenderMergeResult holds the content to deploy and any conflict recovery artifact.
type RenderMergeResult struct {
	Content  []byte
	Conflict []byte
}

// MergeRender applies the render merge policy to a rendered template.
func MergeRender(base, current, rendered []byte, hasHistory, force bool) RenderMergeResult {
	result := RenderMergeResult{Content: rendered}
	if !hasHistory || force {
		return result
	}
	merged := ThreeWayMerge(string(base), string(current), string(rendered))
	if merged.HasConflict {
		result.Conflict = []byte(merged.Content)
	} else {
		result.Content = []byte(merged.Content)
	}
	return result
}
