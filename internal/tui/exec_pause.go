package tui

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
)

// pauseOnFailExec wraps an *exec.Cmd and pauses for user input on failure,
// giving the user time to read error output before the TUI resumes.
type pauseOnFailExec struct {
	cmd    *exec.Cmd
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

func (p *pauseOnFailExec) SetStdin(r io.Reader) {
	p.stdin = r
	p.cmd.Stdin = r
}

func (p *pauseOnFailExec) SetStdout(w io.Writer) {
	p.stdout = w
	p.cmd.Stdout = w
}

func (p *pauseOnFailExec) SetStderr(w io.Writer) {
	p.stderr = w
	p.cmd.Stderr = w
}

func (p *pauseOnFailExec) Run() error {
	err := p.cmd.Run()
	if err != nil {
		_, _ = fmt.Fprintf(p.stderr, "\nPress Enter to continue...")
		reader := bufio.NewReader(p.stdin)
		_, _ = reader.ReadBytes('\n')
	}

	return err
}
