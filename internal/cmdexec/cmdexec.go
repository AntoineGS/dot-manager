package cmdexec

import "context"

// Result holds the output of a command execution.
type Result struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
}

// RunOptions configures a single command execution.
type RunOptions struct {
	// Dir is the working directory for the command. Empty inherits the
	// current process working directory.
	Dir string
	// Sudo runs the command with elevated privileges.
	Sudo bool
	// Stdin is provided to the command as standard input.
	Stdin []byte
}

// Runner abstracts command execution.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) (Result, error)
	RunWithSudo(ctx context.Context, name string, args ...string) (Result, error)
	// RunIn executes a command with an explicit working directory and/or sudo.
	RunIn(ctx context.Context, opts RunOptions, name string, args ...string) (Result, error)
	LookPath(name string) (string, error)
}
