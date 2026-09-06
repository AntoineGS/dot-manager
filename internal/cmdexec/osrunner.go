package cmdexec

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
)

// OsRunner is the real implementation of Runner using os/exec.
type OsRunner struct{}

// RunIn executes a command with the given options, capturing stdout and stderr.
// If the command exits with a non-zero status, the error is returned along with
// whatever output was captured and the exit code from ProcessState.
func (r OsRunner) RunIn(ctx context.Context, opts RunOptions, name string, args ...string) (Result, error) {
	if opts.Sudo {
		sudoArgs := make([]string, 0, 1+len(args))
		sudoArgs = append(sudoArgs, name)
		sudoArgs = append(sudoArgs, args...)
		name, args = "sudo", sudoArgs
	}

	var stdoutBuf, stderrBuf bytes.Buffer

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = opts.Dir
	cmd.Stdin = bytes.NewReader(opts.Stdin)
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	runErr := cmd.Run()

	result := Result{
		Stdout: stdoutBuf.Bytes(),
		Stderr: stderrBuf.Bytes(),
	}

	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	}

	if runErr != nil {
		var exitErr *exec.ExitError
		if !errors.As(runErr, &exitErr) {
			// Not an exit error (e.g. command not found); propagate as-is.
			return result, runErr
		}
	}

	return result, runErr
}

// Run executes a command and captures its stdout and stderr.
func (r OsRunner) Run(ctx context.Context, name string, args ...string) (Result, error) {
	return r.RunIn(ctx, RunOptions{}, name, args...)
}

// RunWithSudo prepends "sudo" to the command and delegates to RunIn.
func (r OsRunner) RunWithSudo(ctx context.Context, name string, args ...string) (Result, error) {
	return r.RunIn(ctx, RunOptions{Sudo: true}, name, args...)
}

// LookPath delegates to exec.LookPath.
func (r OsRunner) LookPath(name string) (string, error) {
	return exec.LookPath(name)
}
