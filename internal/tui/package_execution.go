package tui

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os/exec"

	"github.com/AntoineGS/tidydots/internal/cmdexec"
	"github.com/AntoineGS/tidydots/internal/packages"
)

// packageExec keeps all installation semantics in the domain manager while
// Bubble Tea releases the terminal for dependencies, installers and sudo alike.
type packageExec struct {
	manager  *packages.Manager
	pkg      packages.Package
	result   packages.InstallResult
	terminal packageTerminalRunner
}

func (m Model) newPackageExec(item PackageItem) *packageExec {
	pkg := packages.FromPackageSpec(item.Name, item.Package)
	if pkg == nil {
		return nil
	}
	return &packageExec{manager: packages.NewManager(m.packageConfig(), m.Platform.OS, m.DryRun, false), pkg: *pkg}
}

func (p *packageExec) SetStdin(r io.Reader)  { p.terminal.stdin = r }
func (p *packageExec) SetStdout(w io.Writer) { p.terminal.stdout = w }
func (p *packageExec) SetStderr(w io.Writer) { p.terminal.stderr = w }

func (p *packageExec) Run() error {
	p.result = p.manager.WithRunner(p.terminal).Install(p.pkg)
	if !p.result.Success {
		return errors.New(p.result.Message)
	}
	return nil
}

// packageTerminalRunner preserves interactive stdio instead of capturing input
// in OsRunner. Explicit RunOptions input still takes precedence when supplied.
type packageTerminalRunner struct {
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

func (r packageTerminalRunner) RunIn(ctx context.Context, opts cmdexec.RunOptions, name string, args ...string) (cmdexec.Result, error) {
	if opts.Sudo {
		args = append([]string{name}, args...)
		name = "sudo"
	}
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // domain manager validates arguments and intentional user commands
	cmd.Dir, cmd.Stdin, cmd.Stdout, cmd.Stderr = opts.Dir, r.stdin, r.stdout, r.stderr
	if opts.Stdin != nil {
		cmd.Stdin = bytes.NewReader(opts.Stdin)
	}
	var err error
	if r.stdin != nil && r.stderr != nil {
		paused := &pauseOnFailExec{cmd: cmd}
		paused.SetStdin(cmd.Stdin)
		paused.SetStdout(r.stdout)
		paused.SetStderr(r.stderr)
		err = paused.Run()
	} else {
		err = cmd.Run()
	}
	result := cmdexec.Result{}
	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	}
	return result, err
}

func (r packageTerminalRunner) Run(ctx context.Context, name string, args ...string) (cmdexec.Result, error) {
	return r.RunIn(ctx, cmdexec.RunOptions{}, name, args...)
}

func (r packageTerminalRunner) RunWithSudo(ctx context.Context, name string, args ...string) (cmdexec.Result, error) {
	return r.RunIn(ctx, cmdexec.RunOptions{Sudo: true}, name, args...)
}

func (r packageTerminalRunner) LookPath(name string) (string, error) { return exec.LookPath(name) }
