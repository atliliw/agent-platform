package mcp

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
)

// StdioTransport implements Transport over stdin/stdout with a subprocess.
// Messages are newline-delimited JSON-RPC.
type StdioTransport struct {
	command string
	args    []string
	env     map[string]string

	cmd    *exec.Cmd
	stdinPipe io.WriteCloser // underlying pipe from StdinPipe
	stdin  *bufio.Writer
	stdout *bufio.Reader
	stderr io.ReadCloser

	mu     sync.Mutex
	closed bool
	alive  bool
}

// NewStdioTransport creates a stdio transport that will launch the given command.
func NewStdioTransport(command string, args []string, env map[string]string) *StdioTransport {
	return &StdioTransport{
		command: command,
		args:    args,
		env:     env,
	}
}

// Start launches the subprocess and sets up stdin/stdout pipes.
func (t *StdioTransport) Start(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.alive {
		return fmt.Errorf("transport already started")
	}

	t.cmd = exec.Command(t.command, t.args...)
	// NOTE: deliberately NOT using exec.CommandContext(ctx, ...). The Start ctx
	// here is the caller's handshake timeout context (from MCPService.Connect),
	// which is cancelled as soon as Connect returns. CommandContext would kill
	// the subprocess when that context expires - destroying the connection right
	// after a successful handshake. The subprocess lifetime is managed by Close()
	// (Process.Kill), independent of any RPC context.

	// Set environment variables
	if len(t.env) > 0 {
		t.cmd.Env = makeEnv(t.env)
	}

	// Set up pipes
	pipeStdin, err := t.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}
	t.stdinPipe = pipeStdin
	t.stdin = bufio.NewWriter(pipeStdin)

	stdout, err := t.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	t.stdout = bufio.NewReader(stdout)

	stderr, err := t.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}
	t.stderr = stderr

	// Drain stderr in background (log it)
	go drainStderr(stderr)

	if err := t.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command %q: %w", t.command, err)
	}

	t.alive = true
	return nil
}

// Send writes a JSON-RPC message to the subprocess stdin.
func (t *StdioTransport) Send(_ context.Context, msg []byte) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.alive || t.stdin == nil {
		return fmt.Errorf("transport not alive")
	}

	// Write message + newline
	if _, err := t.stdin.Write(msg); err != nil {
		return fmt.Errorf("failed to write to stdin: %w", err)
	}
	if _, err := t.stdin.Write([]byte("\n")); err != nil {
		return fmt.Errorf("failed to write newline: %w", err)
	}
	return t.stdin.Flush()
}

// Receive reads the next newline-delimited JSON-RPC message from stdout.
func (t *StdioTransport) Receive(ctx context.Context) ([]byte, error) {
	t.mu.Lock()
	reader := t.stdout
	t.mu.Unlock()

	if reader == nil {
		return nil, fmt.Errorf("transport not started")
	}

	// Use a goroutine to make this cancellable
	type result struct {
		line []byte
		err  error
	}
	ch := make(chan result, 1)

	go func() {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			ch <- result{nil, err}
			return
		}
		// Trim the trailing newline
		if len(line) > 0 && line[len(line)-1] == '\n' {
			line = line[:len(line)-1]
		}
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}
		ch <- result{line, nil}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-ch:
		if r.err != nil {
			t.mu.Lock()
			t.alive = false
			t.mu.Unlock()
			return nil, fmt.Errorf("failed to read from stdout: %w", r.err)
		}
		return r.line, nil
	}
}

// Close shuts down the subprocess.
func (t *StdioTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return nil
	}
	t.closed = true
	t.alive = false

	var errs []error

	if t.stdin != nil {
		// Flush any buffered data before closing the pipe
		_ = t.stdin.Flush()
	}

	if t.stdinPipe != nil {
		if err := t.stdinPipe.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close stdin: %w", err))
		}
	}

	if t.cmd != nil && t.cmd.Process != nil {
		if err := t.cmd.Process.Kill(); err != nil {
			errs = append(errs, fmt.Errorf("kill process: %w", err))
		}
		// Wait to reap the process
		_ = t.cmd.Wait()
	}

	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}
	return nil
}

// IsAlive returns whether the subprocess is still running.
func (t *StdioTransport) IsAlive() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.alive
}

// makeEnv builds an environment variable list from the provided map,
// inheriting the current process environment and overriding with the provided values.
func makeEnv(env map[string]string) []string {
	result := os.Environ()
	for k, v := range env {
		result = append(result, k+"="+v)
	}
	return result
}

// drainStderr reads from stderr and discards it.
func drainStderr(r io.ReadCloser) {
	buf := make([]byte, 4096)
	for {
		_, err := r.Read(buf)
		if err != nil {
			return
		}
	}
}
