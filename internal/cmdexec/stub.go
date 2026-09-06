package cmdexec

import (
	"bytes"
	"context"
	"os/exec"
)

// Call records a single invocation of Run, RunWithSudo, or RunIn.
type Call struct {
	Name  string
	Args  []string
	Dir   string
	Sudo  bool
	Stdin []byte
}

// StubRunner is a test fake that records calls and returns pre-configured results.
type StubRunner struct {
	// Calls is the ordered list of all recorded invocations.
	Calls   []Call
	results map[string][]Result
	paths   map[string]string
}

// NewStubRunner creates an empty StubRunner.
func NewStubRunner() *StubRunner {
	return &StubRunner{
		results: make(map[string][]Result),
		paths:   make(map[string]string),
	}
}

// AddResult queues a Result to be returned the next time Run or RunWithSudo is
// called with the given command name. Results are consumed in FIFO order.
func (s *StubRunner) AddResult(name string, r Result) {
	s.results[name] = append(s.results[name], r)
}

// AddPath registers a path to be returned by LookPath for the given name.
func (s *StubRunner) AddPath(name, path string) {
	s.paths[name] = path
}

// Run records the call and returns the next queued Result for name.
// If no result is queued, a zero Result is returned with no error.
func (s *StubRunner) Run(_ context.Context, name string, args ...string) (Result, error) {
	s.Calls = append(s.Calls, Call{Name: name, Args: args, Sudo: false})

	return s.popResult(name), nil
}

// RunWithSudo records the call with Sudo=true and returns the next queued Result.
func (s *StubRunner) RunWithSudo(_ context.Context, name string, args ...string) (Result, error) {
	s.Calls = append(s.Calls, Call{Name: name, Args: args, Sudo: true})

	return s.popResult(name), nil
}

// RunIn records the call with its options and returns the next queued Result.
func (s *StubRunner) RunIn(_ context.Context, opts RunOptions, name string, args ...string) (Result, error) {
	s.Calls = append(s.Calls, Call{
		Name:  name,
		Args:  args,
		Dir:   opts.Dir,
		Sudo:  opts.Sudo,
		Stdin: bytes.Clone(opts.Stdin),
	})

	return s.popResult(name), nil
}

// LookPath returns the registered path for name, or exec.ErrNotFound if none.
func (s *StubRunner) LookPath(name string) (string, error) {
	if p, ok := s.paths[name]; ok {
		return p, nil
	}

	return "", exec.ErrNotFound
}

// popResult removes and returns the first queued Result for name.
// Returns a zero Result if the queue is empty.
func (s *StubRunner) popResult(name string) Result {
	queue := s.results[name]
	if len(queue) == 0 {
		return Result{}
	}

	r := queue[0]
	s.results[name] = queue[1:]

	return r
}
