package template

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/go-sprout/sprout"
	"github.com/go-sprout/sprout/registry/conversion"
	"github.com/go-sprout/sprout/registry/maps"
	"github.com/go-sprout/sprout/registry/numeric"
	"github.com/go-sprout/sprout/registry/regexp"
	"github.com/go-sprout/sprout/registry/slices"
	"github.com/go-sprout/sprout/registry/std"
	sproutstrings "github.com/go-sprout/sprout/registry/strings"
)

const (
	tmplSuffix     = ".tmpl"
	renderedSuffix = ".tmpl.rendered"
	conflictSuffix = ".tmpl.conflict"
)

// Engine renders Go templates with platform-aware context and sprout functions.
type Engine struct {
	ctx     *Context
	funcMap template.FuncMap
}

// NewEngine creates a template engine with sprout functions and the given context.
func NewEngine(ctx *Context) *Engine {
	handler := sprout.New(
		sprout.WithRegistries(
			std.NewRegistry(),
			sproutstrings.NewRegistry(),
			numeric.NewRegistry(),
			conversion.NewRegistry(),
			maps.NewRegistry(),
			slices.NewRegistry(),
			regexp.NewRegistry(),
		),
	)

	return &Engine{
		ctx:     ctx,
		funcMap: handler.Build(),
	}
}

// RenderString renders a template string. Returns input unchanged if no {{ delimiters are present.
func (e *Engine) RenderString(name, tmplStr string) (string, error) {
	if !strings.Contains(tmplStr, "{{") {
		return tmplStr, nil
	}

	tmpl, err := template.New(name).Funcs(e.funcMap).Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("parsing template %q: %w", name, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, e.ctx); err != nil {
		return "", fmt.Errorf("executing template %q: %w", name, err)
	}

	return buf.String(), nil
}

// RenderBytes renders a template from byte content.
func (e *Engine) RenderBytes(name string, content []byte) ([]byte, error) {
	tmpl, err := template.New(name).Funcs(e.funcMap).Parse(string(content))
	if err != nil {
		return nil, fmt.Errorf("parsing template %q: %w", name, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, e.ctx); err != nil {
		return nil, fmt.Errorf("executing template %q: %w", name, err)
	}

	return buf.Bytes(), nil
}

// IsTemplateFile returns true if the filename has a .tmpl suffix.
func IsTemplateFile(filename string) bool {
	return strings.HasSuffix(filename, tmplSuffix) &&
		!strings.HasSuffix(filename, renderedSuffix) &&
		!strings.HasSuffix(filename, conflictSuffix)
}

// RenderedPath returns the rendered output path for a template file (appends .rendered).
func RenderedPath(tmplPath string) string {
	return tmplPath + ".rendered"
}

// TargetName strips the .tmpl suffix to get the final target filename.
func TargetName(tmplFilename string) string {
	return strings.TrimSuffix(filepath.Base(tmplFilename), tmplSuffix)
}

// ConflictPath returns the conflict file path for a template.
func ConflictPath(tmplPath string) string {
	return tmplPath + ".conflict"
}

// IsRenderedFile returns true if the filename is a .tmpl.rendered file.
func IsRenderedFile(filename string) bool {
	return strings.HasSuffix(filename, renderedSuffix)
}

// IsConflictFile returns true if the filename is a .tmpl.conflict file.
func IsConflictFile(filename string) bool {
	return strings.HasSuffix(filename, conflictSuffix)
}
