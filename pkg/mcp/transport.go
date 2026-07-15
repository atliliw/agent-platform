package mcp

import "context"

// Transport is the interface for MCP message transport.
// Implementations handle the underlying communication channel (stdio, HTTP, etc.)
// and provide raw JSON-RPC message exchange.
type Transport interface {
	// Start initializes the transport connection.
	Start(ctx context.Context) error

	// Send sends a JSON-RPC message (request or notification).
	Send(ctx context.Context, msg []byte) error

	// Receive returns the next JSON-RPC message from the transport.
	// Returns the raw bytes of the message. Blocks until a message is available.
	// Returns an error if the transport is closed or the context is cancelled.
	Receive(ctx context.Context) ([]byte, error)

	// Close shuts down the transport, releasing all resources.
	Close() error

	// IsAlive returns whether the transport is still connected.
	IsAlive() bool
}
