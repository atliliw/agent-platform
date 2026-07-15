package mcp

import (
	"encoding/json"
	"fmt"
	"sync/atomic"
)

// JSON-RPC 2.0 message types per the MCP specification.

// ProtocolVersion is the latest MCP protocol version we support.
const ProtocolVersion = "2025-06-18"

// JSONRPCRequest represents a JSON-RPC 2.0 request.
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response.
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCError represents a JSON-RPC 2.0 error.
type JSONRPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *JSONRPCError) Error() string {
	return fmt.Sprintf("JSON-RPC error %d: %s", e.Code, e.Message)
}

// JSONRPCNotification represents a JSON-RPC 2.0 notification (no ID, no response expected).
type JSONRPCNotification struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Standard JSON-RPC error codes.
const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
	CodeResourceNotFound = -32002
)

// nextRequestID generates sequential integer IDs for JSON-RPC requests.
// The counter is accessed atomically so it is safe under concurrency
// (call cycles are also serialized by Client.callMu).
type nextRequestID struct {
	counter atomic.Int64
}

func (n *nextRequestID) next() json.RawMessage {
	id := n.counter.Add(1)
	b, _ := json.Marshal(id)
	return b
}

// marshalParams marshals params to JSON, returning nil if v is nil.
func marshalParams(v interface{}) json.RawMessage {
	if v == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		// Should never happen with well-typed params
		return nil
	}
	return b
}

// unmarshalResult unmarshals a JSON-RPC result into the target.
func unmarshalResult(data json.RawMessage, target interface{}) error {
	if data == nil {
		return fmt.Errorf("empty result")
	}
	return json.Unmarshal(data, target)
}

// isJSONRPCResponse checks if a raw JSON message is a response (has "result" or "error").
func isJSONRPCResponse(data []byte) bool {
	return len(data) > 0 && (containsKey(data, "result") || containsKey(data, "error"))
}

// isJSONRPCNotification checks if a raw JSON message is a notification (has "method" but no "id").
func isJSONRPCNotification(data []byte) bool {
	return len(data) > 0 && containsKey(data, "method") && !containsKey(data, "id")
}

// containsKey does a cheap check for a top-level JSON key.
func containsKey(data []byte, key string) bool {
	needle := []byte(`"` + key + `"`)
	for i := 0; i < len(data)-len(needle); i++ {
		if data[i] == '"' {
			match := true
			for j := 0; j < len(needle) && i+j < len(data); j++ {
				if data[i+j] != needle[j] {
					match = false
					break
				}
			}
			if match {
				return true
			}
		}
	}
	return false
}
