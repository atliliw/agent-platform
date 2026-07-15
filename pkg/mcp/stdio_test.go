package mcp

import (
	"context"
	"io"
	"os"
	"testing"
	"time"
)

// TestMain enables a stub subprocess mode used by the stdio transport tests.
// When MCP_STDIO_STUB=1 the binary does not run any tests; it just copies stdin
// to stdout (a minimal long-lived "server" that echoes), so the parent test can
// drive the transport over real pipes.
func TestMain(m *testing.M) {
	if os.Getenv("MCP_STDIO_STUB") == "1" {
		_, _ = io.Copy(os.Stdout, os.Stdin)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// TestStdioTransport_SurvivesStartContextCancel is a regression test for a bug
// where StdioTransport.Start used exec.CommandContext(handshakeCtx, ...). The
// handshake context passed in from MCPService.Connect is cancelled as soon as
// Connect returns, which killed the subprocess - so every stdio connection died
// immediately after a successful handshake and all later tool calls failed.
//
// With the fix (exec.Command; lifecycle owned by Close), the subprocess must
// stay alive and keep responding after the Start context is cancelled.
func TestStdioTransport_SurvivesStartContextCancel(t *testing.T) {
	// Re-exec this test binary as a stub "server" that echoes stdin -> stdout.
	tr := NewStdioTransport(os.Args[0], nil, map[string]string{"MCP_STDIO_STUB": "1"})

	ctx, cancel := context.WithCancel(context.Background())
	if err := tr.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer tr.Close()

	// Simulate the handshake context being cancelled right after Connect
	// returns. The subprocess must NOT be killed by this cancellation.
	cancel()
	time.Sleep(200 * time.Millisecond)

	// Round-trip a message to prove the subprocess is still running. With the
	// old CommandContext code the subprocess would be dead here, so Send/Receive
	// would error (broken pipe / EOF).
	sendCtx, sendCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer sendCancel()
	if err := tr.Send(sendCtx, []byte("ping\n")); err != nil {
		t.Fatalf("Send after Start-context cancel failed (subprocess likely killed): %v", err)
	}

	recvCtx, recvCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer recvCancel()
	msg, err := tr.Receive(recvCtx)
	if err != nil {
		t.Fatalf("Receive after Start-context cancel failed (subprocess likely killed): %v", err)
	}
	if string(msg) != "ping" {
		t.Fatalf("expected echo %q, got %q", "ping", string(msg))
	}
}
