package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

// StreamableHTTPTransport implements Transport over the MCP Streamable HTTP protocol.
// Client sends JSON-RPC via POST; server responds with JSON or SSE stream.
// Server-initiated messages arrive via an optional GET SSE channel.
type StreamableHTTPTransport struct {
	url     string
	headers map[string]string

	client      *http.Client
	sessionID   string // Mcp-Session-Id from server
	protocolVer string

	mu       sync.Mutex
	closed   bool
	alive    bool
	pending  chan []byte // buffered channel for received messages
	sseCh    chan []byte // messages from the SSE listener
	done     chan struct{} // signals goroutines to stop
}

// NewStreamableHTTPTransport creates an HTTP transport targeting the given URL.
func NewStreamableHTTPTransport(url string, headers map[string]string) *StreamableHTTPTransport {
	return &StreamableHTTPTransport{
		url:        url,
		headers:    headers,
		client:     &http.Client{},
		protocolVer: ProtocolVersion,
		pending:    make(chan []byte, 64),
		sseCh:      make(chan []byte, 64),
		done:       make(chan struct{}),
	}
}

// Start opens the optional SSE listener for server-initiated messages.
func (t *StreamableHTTPTransport) Start(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.alive {
		return fmt.Errorf("transport already started")
	}

	t.alive = true

	// Start background SSE listener for server-initiated messages
	go t.sseListener(ctx)

	return nil
}

// Send posts a JSON-RPC message to the server endpoint.
func (t *StreamableHTTPTransport) Send(ctx context.Context, msg []byte) error {
	t.mu.Lock()
	if !t.alive {
		t.mu.Unlock()
		return fmt.Errorf("transport not alive")
	}
	t.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.url, bytes.NewReader(msg))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", t.protocolVer)

	// Set session ID if we have one
	if t.sessionID != "" {
		req.Header.Set("Mcp-Session-Id", t.sessionID)
	}

	// Set custom headers
	for k, v := range t.headers {
		req.Header.Set(k, v)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Capture session ID from response
	if sid := resp.Header.Get("Mcp-Session-Id"); sid != "" {
		t.mu.Lock()
		t.sessionID = sid
		t.mu.Unlock()
	}

	if resp.StatusCode == http.StatusAccepted {
		// Notification acknowledged, no body
		return nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	contentType := resp.Header.Get("Content-Type")

	if strings.Contains(contentType, "text/event-stream") {
		// SSE response: parse events and feed to pending channel
		go t.parseSSEResponse(resp.Body)
		return nil
	}

	// Single JSON response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Feed to pending channel for Receive to pick up
	select {
	case t.pending <- body:
	default:
		return fmt.Errorf("pending channel full, dropping response")
	}

	return nil
}

// Receive returns the next message from either the pending channel or SSE channel.
func (t *StreamableHTTPTransport) Receive(ctx context.Context) ([]byte, error) {
	t.mu.Lock()
	alive := t.alive
	t.mu.Unlock()

	if !alive {
		return nil, fmt.Errorf("transport not alive")
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-t.done:
		return nil, fmt.Errorf("transport closed")
	case msg := <-t.pending:
		return msg, nil
	case msg := <-t.sseCh:
		return msg, nil
	}
}

// Close shuts down the transport.
func (t *StreamableHTTPTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return nil
	}
	t.closed = true
	t.alive = false

	// Signal goroutines to stop
	close(t.done)

	// Send DELETE to close session if we have a session ID
	if t.sessionID != "" {
		go func() {
			req, _ := http.NewRequest(http.MethodDelete, t.url, nil)
			req.Header.Set("Mcp-Session-Id", t.sessionID)
			t.client.Do(req)
		}()
	}

	return nil
}

// IsAlive returns whether the transport is still connected.
func (t *StreamableHTTPTransport) IsAlive() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.alive
}

// parseSSEResponse reads an SSE stream from an HTTP response body
// and feeds parsed data messages to the pending channel.
func (t *StreamableHTTPTransport) parseSSEResponse(body io.ReadCloser) {
	defer body.Close()

	scanner := bufio.NewScanner(body)
	var dataBuf strings.Builder

	for scanner.Scan() {
		select {
		case <-t.done:
			return
		default:
		}

		line := scanner.Text()

		if line == "" {
			// End of event
			if dataBuf.Len() > 0 {
				data := dataBuf.String()
				dataBuf.Reset()

				// Try to parse as JSON-RPC
				var raw json.RawMessage
				if err := json.Unmarshal([]byte(data), &raw); err == nil {
					select {
					case t.pending <- []byte(data):
					default:
						// Channel full, drop
					}
				}
			}
			continue
		}

		if strings.HasPrefix(line, "data:") {
			data := strings.TrimPrefix(line, "data:")
			data = strings.TrimSpace(data)
			if dataBuf.Len() > 0 {
				dataBuf.WriteByte('\n')
			}
			dataBuf.WriteString(data)
		}
		// Ignore event:, id:, retry: lines
	}
}

// sseListener opens a GET SSE connection for server-initiated messages.
func (t *StreamableHTTPTransport) sseListener(ctx context.Context) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.url, nil)
	if err != nil {
		return
	}

	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("MCP-Protocol-Version", t.protocolVer)

	t.mu.Lock()
	if t.sessionID != "" {
		req.Header.Set("Mcp-Session-Id", t.sessionID)
	}
	t.mu.Unlock()

	for k, v := range t.headers {
		req.Header.Set(k, v)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return
	}

	scanner := bufio.NewScanner(resp.Body)
	var sseDataBuf strings.Builder

	for scanner.Scan() {
		select {
		case <-t.done:
			return
		default:
		}

		line := scanner.Text()

		if line == "" {
			if sseDataBuf.Len() > 0 {
				data := sseDataBuf.String()
				sseDataBuf.Reset()
				select {
				case t.sseCh <- []byte(data):
				default:
				}
			}
			continue
		}

		if strings.HasPrefix(line, "data:") {
			data := strings.TrimPrefix(line, "data:")
			data = strings.TrimSpace(data)
			if sseDataBuf.Len() > 0 {
				sseDataBuf.WriteByte('\n')
			}
			sseDataBuf.WriteString(data)
		}
	}
}
