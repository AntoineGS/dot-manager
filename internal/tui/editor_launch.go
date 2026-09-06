package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/AntoineGS/tidydots/internal/manager"
	"github.com/sergi/go-diff/diffmatchpatch"
)

// shellEscape wraps a string in single quotes for safe interpolation into shell
// commands. Any embedded single quotes are escaped using the '\” idiom, which
// ends the current single-quoted segment, adds a literal single quote via
// backslash, and starts a new single-quoted segment.
func shellEscape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// Editor mode constants returned by detectEditorMode.
const (
	editorModeNvim     = "nvim"
	editorModeTmux     = "tmux"
	editorModeFallback = "fallback"
)

// editorVim is the vim executable, the first entry in the fallback editor chain.
const editorVim = "vim"

// editorLaunchCompleteMsg is sent when the editor exits and the TUI should resume.
type editorLaunchCompleteMsg struct {
	err error
}

// generateUnifiedDiff creates a unified diff string between the pure render (from DB)
// and the current on-disk content, using sergi/go-diff.
func generateUnifiedDiff(mt manager.ModifiedTemplate) string {
	dmp := diffmatchpatch.New()

	a, b, lineArray := dmp.DiffLinesToChars(string(mt.PureRender), string(mt.CurrentOnDisk))
	diffs := dmp.DiffMain(a, b, false)
	diffs = dmp.DiffCharsToLines(diffs, lineArray)

	patches := dmp.PatchMake(string(mt.PureRender), diffs)

	if len(patches) == 0 {
		return "No differences found.\n"
	}

	// Build a readable unified diff output
	var sb strings.Builder
	sb.WriteString("--- pure render (from DB)\n")
	fmt.Fprintf(&sb, "+++ edited file (%s)\n", mt.CurrentPath)
	sb.WriteString("\n")

	for _, diff := range diffs {
		lines := strings.Split(diff.Text, "\n")
		// Remove trailing empty string from split
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		for _, line := range lines {
			switch diff.Type {
			case diffmatchpatch.DiffDelete:
				sb.WriteString("- " + line + "\n")
			case diffmatchpatch.DiffInsert:
				sb.WriteString("+ " + line + "\n")
			case diffmatchpatch.DiffEqual:
				sb.WriteString("  " + line + "\n")
			}
		}
	}

	return sb.String()
}

// writeTempDiff writes the unified diff to a temp file and returns the path.
// The caller is responsible for cleaning up the file.
func writeTempDiff(diff string) (string, error) {
	f, err := os.CreateTemp("", "tidydots-diff-*.diff")
	if err != nil {
		return "", fmt.Errorf("creating temp diff file: %w", err)
	}

	if _, err := f.WriteString(diff); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name()) //nolint:gosec // temp file created above
		return "", fmt.Errorf("writing diff to temp file: %w", err)
	}

	if err := f.Close(); err != nil {
		_ = os.Remove(f.Name()) //nolint:gosec // temp file created above
		return "", fmt.Errorf("closing temp diff file: %w", err)
	}

	return f.Name(), nil
}

// detectEditorMode determines how to launch the editor based on the environment.
// Returns editorModeNvim if neovim is available, editorModeTmux if inside tmux
// with $EDITOR set, or editorModeFallback otherwise.
func detectEditorMode() string {
	// Priority 1: neovim available
	if _, err := exec.LookPath(editorModeNvim); err == nil {
		return editorModeNvim
	}

	// Priority 2: inside tmux with editor set
	if os.Getenv("TMUX") != "" && getEditor() != "" {
		return editorModeTmux
	}

	return editorModeFallback
}

// getEditor returns the user's preferred editor from environment.
func getEditor() string {
	if editor := os.Getenv("EDITOR"); editor != "" {
		return editor
	}
	if editor := os.Getenv("VISUAL"); editor != "" {
		return editor
	}
	// Fallback chain
	for _, e := range []string{editorVim, "vi", "nano"} {
		if _, err := exec.LookPath(e); err == nil {
			return e
		}
	}
	return ""
}

// buildEditorCmd creates the exec.Cmd to launch the editor with the diff and template files.
func buildEditorCmd(diffPath, templatePath string) *exec.Cmd {
	mode := detectEditorMode()

	switch mode {
	case editorModeNvim:
		// Open diff (read-only) and template in vertical splits
		// -c command: move to first window and make it read-only with diff syntax highlighting
		return exec.CommandContext(context.Background(), editorModeNvim, //nolint:gosec // intentional editor launch
			"-O", diffPath, templatePath,
			"-c", "1wincmd w | setlocal readonly nomodifiable buftype=nofile filetype=diff",
		)

	case editorModeTmux:
		// Inside tmux: open template for editing in a new split pane, view diff read-only in current pane.
		// The current pane shows the diff; when the user closes the split pane editor, they close the diff too.
		editor := getEditor()
		safeEditor := shellEscape(editor)
		safeTemplate := shellEscape(templatePath)
		safeDiff := shellEscape(diffPath)
		script := fmt.Sprintf(
			`tmux split-window -h %s %s && %s -R %s`,
			safeEditor, safeTemplate,
			safeEditor, safeDiff,
		)
		return exec.CommandContext(context.Background(), "sh", "-c", script) //nolint:gosec // intentional editor launch

	default:
		// Fallback: just open the template in the editor
		editor := getEditor()
		if editor == "" {
			return nil
		}
		return exec.CommandContext(context.Background(), editor, templatePath) //nolint:gosec // intentional editor launch
	}
}

// launchDiffEditor generates a diff, writes it to a temp file, and returns a tea.Cmd
// that suspends the TUI and launches the editor. The temp file is cleaned up after the editor exits.
func launchDiffEditor(mt manager.ModifiedTemplate) tea.Cmd {
	diff := generateUnifiedDiff(mt)

	diffPath, err := writeTempDiff(diff)
	if err != nil {
		return func() tea.Msg {
			return editorLaunchCompleteMsg{err: err}
		}
	}

	cmd := buildEditorCmd(diffPath, mt.TemplatePath)
	if cmd == nil {
		_ = os.Remove(diffPath)
		return func() tea.Msg {
			return editorLaunchCompleteMsg{err: fmt.Errorf("no editor found")}
		}
	}

	// Use tea.ExecProcess to suspend the TUI and give terminal control to the editor
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		// Clean up temp file after editor exits
		_ = os.Remove(diffPath)
		return editorLaunchCompleteMsg{err: err}
	})
}
