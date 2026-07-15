package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

// ── MCP Protocol Types ──────────────────────────────────────────

const protocolVersion = "2025-06-18"

type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *jsonRPCError) Error() string {
	return fmt.Sprintf("JSON-RPC error %d: %s", e.Code, e.Message)
}

// ── MCP Initialize Types ────────────────────────────────────────

type serverCapabilities struct {
	Tools *toolsCapability `json:"tools,omitempty"`
}

type toolsCapability struct {
	ListChanged bool `json:"listChanged"`
}

type serverInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type initializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    serverCapabilities `json:"capabilities"`
	ServerInfo      serverInfo         `json:"serverInfo"`
	Instructions    string             `json:"instructions,omitempty"`
}

// ── MCP Tool Types ──────────────────────────────────────────────

type toolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

type toolsListResult struct {
	Tools []toolDefinition `json:"tools"`
}

type toolCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

type toolCallResult struct {
	Content []toolContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

type toolContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// ── Tool Definitions ────────────────────────────────────────────

var demoTools = []toolDefinition{
	{
		Name:        "echo",
		Description: "Echoes back the input message. Useful for testing connectivity.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"message": {
					"type": "string",
					"description": "The message to echo back"
				}
			},
			"required": ["message"]
		}`),
	},
	{
		Name:        "add",
		Description: "Adds two numbers and returns the result.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"a": {
					"type": "number",
					"description": "First number"
				},
				"b": {
					"type": "number",
					"description": "Second number"
				}
			},
			"required": ["a", "b"]
		}`),
	},
	{
		Name:        "get_time",
		Description: "Returns the current server time in various formats.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"format": {
					"type": "string",
					"description": "Time format: 'iso' (default), 'unix', or 'rfc3339'"
				}
			}
		}`),
	},
	{
		Name:        "random_number",
		Description: "Generates a random number within a specified range.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"min": {
					"type": "integer",
					"description": "Minimum value (default: 1)"
				},
				"max": {
					"type": "integer",
					"description": "Maximum value (default: 100)"
				}
			}
		}`),
	},
	{
		Name:        "string_reverse",
		Description: "Reverses the input string.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"text": {
					"type": "string",
					"description": "The string to reverse"
				}
			},
			"required": ["text"]
		}`),
	},
}

// ── Server ──────────────────────────────────────────────────────

type mcpServer struct {
	mu       sync.RWMutex
	sessions map[string]bool // active session IDs
	rng      *rand.Rand
	rngMu    sync.Mutex
}

func newMCPServer() *mcpServer {
	return &mcpServer{
		sessions: make(map[string]bool),
		rng:      rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (s *mcpServer) generateSessionID() string {
	return fmt.Sprintf("mcp-session-%d", time.Now().UnixNano())
}

func (s *mcpServer) nextRandom(min, max int) int {
	s.rngMu.Lock()
	defer s.rngMu.Unlock()
	return s.rng.Intn(max-min+1) + min
}

// handleMCP is the main HTTP handler for the MCP endpoint.
// It accepts POST requests with JSON-RPC messages and routes them
// to the appropriate handler based on the method.
func (s *mcpServer) handleMCP(w http.ResponseWriter, r *http.Request) {
	// Only accept POST
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the JSON-RPC request
	var req jsonRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, nil, -32700, "Parse error")
		return
	}

	// Validate JSON-RPC version
	if req.JSONRPC != "2.0" {
		s.writeError(w, req.ID, -32600, "Invalid Request: jsonrpc must be 2.0")
		return
	}

	// Check if this is a notification (no ID)
	isNotification := req.ID == nil || string(req.ID) == "null"

	// Route to handler
	var result json.RawMessage
	var rpcErr *jsonRPCError
	var newSessionID string // set only by initialize

	switch req.Method {
	case "initialize":
		result, rpcErr, newSessionID = s.handleInitialize(req.Params)

	case "notifications/initialized":
		// Client confirmed initialization — just acknowledge
		if isNotification {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		// Should be a notification, but handle gracefully
		result = json.RawMessage(`{}`)

	case "ping":
		result = json.RawMessage(`{}`)

	case "tools/list":
		result, rpcErr = s.handleToolsList()

	case "tools/call":
		result, rpcErr = s.handleToolsCall(req.Params)

	default:
		rpcErr = &jsonRPCError{Code: -32601, Message: fmt.Sprintf("Method not found: %s", req.Method)}
	}

	// Notifications get 202 Accepted
	if isNotification {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	// Write response
	if rpcErr != nil {
		s.writeError(w, req.ID, rpcErr.Code, rpcErr.Message)
		return
	}

	s.writeResult(w, req.ID, result, newSessionID)
}

// handleInitialize handles the MCP initialize request.
func (s *mcpServer) handleInitialize(_ json.RawMessage) (json.RawMessage, *jsonRPCError, string) {
	// Generate a session ID
	sessionID := s.generateSessionID()
	s.mu.Lock()
	s.sessions[sessionID] = true
	s.mu.Unlock()

	result := initializeResult{
		ProtocolVersion: protocolVersion,
		Capabilities: serverCapabilities{
			Tools: &toolsCapability{ListChanged: false},
		},
		ServerInfo: serverInfo{
			Name:    "mcp-demo-server",
			Version: "1.0.0",
		},
		Instructions: "This is a demo MCP server with sample tools: echo, add, get_time, random_number, string_reverse. Use them to test MCP client connectivity.",
	}

	data, err := json.Marshal(result)
	if err != nil {
		return nil, &jsonRPCError{Code: -32603, Message: "Internal error"}, ""
	}

	return data, nil, sessionID
}

// handleToolsList returns the list of available tools.
func (s *mcpServer) handleToolsList() (json.RawMessage, *jsonRPCError) {
	result := toolsListResult{Tools: demoTools}
	data, err := json.Marshal(result)
	if err != nil {
		return nil, &jsonRPCError{Code: -32603, Message: "Internal error"}
	}
	return data, nil
}

// handleToolsCall executes a tool call.
func (s *mcpServer) handleToolsCall(params json.RawMessage) (json.RawMessage, *jsonRPCError) {
	var callParams toolCallParams
	if err := json.Unmarshal(params, &callParams); err != nil {
		return nil, &jsonRPCError{Code: -32602, Message: fmt.Sprintf("Invalid params: %v", err)}
	}

	var callResult toolCallResult

	switch callParams.Name {
	case "echo":
		msg, _ := callParams.Arguments["message"].(string)
		if msg == "" {
			callResult = toolCallResult{
				IsError: true,
				Content: []toolContent{{Type: "text", Text: "Missing required parameter: message"}},
			}
		} else {
			callResult = toolCallResult{
				Content: []toolContent{{Type: "text", Text: msg}},
			}
		}

	case "add":
		a, okA := toFloat64(callParams.Arguments["a"])
		b, okB := toFloat64(callParams.Arguments["b"])
		if !okA || !okB {
			callResult = toolCallResult{
				IsError: true,
				Content: []toolContent{{Type: "text", Text: "Missing or invalid parameters: a and b must be numbers"}},
			}
		} else {
			callResult = toolCallResult{
				Content: []toolContent{{Type: "text", Text: fmt.Sprintf("%.6g", a+b)}},
			}
		}

	case "get_time":
		format := "iso"
		if f, ok := callParams.Arguments["format"].(string); ok && f != "" {
			format = f
		}
		now := time.Now()
		var timeStr string
		switch format {
		case "unix":
			timeStr = fmt.Sprintf("%d", now.Unix())
		case "rfc3339":
			timeStr = now.Format(time.RFC3339)
		default: // iso
			timeStr = now.Format(time.RFC3339)
		}
		callResult = toolCallResult{
			Content: []toolContent{{Type: "text", Text: timeStr}},
		}

	case "random_number":
		min := 1
		max := 100
		if m, ok := toInt(callParams.Arguments["min"]); ok {
			min = m
		}
		if m, ok := toInt(callParams.Arguments["max"]); ok {
			max = m
		}
		if min > max {
			min, max = max, min
		}
		n := s.nextRandom(min, max)
		callResult = toolCallResult{
			Content: []toolContent{{Type: "text", Text: fmt.Sprintf("%d", n)}},
		}

	case "string_reverse":
		text, _ := callParams.Arguments["text"].(string)
		if text == "" {
			callResult = toolCallResult{
				IsError: true,
				Content: []toolContent{{Type: "text", Text: "Missing required parameter: text"}},
			}
		} else {
			runes := []rune(text)
			for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
				runes[i], runes[j] = runes[j], runes[i]
			}
			callResult = toolCallResult{
				Content: []toolContent{{Type: "text", Text: string(runes)}},
			}
		}

	default:
		callResult = toolCallResult{
			IsError: true,
			Content: []toolContent{{Type: "text", Text: fmt.Sprintf("Unknown tool: %s", callParams.Name)}},
		}
	}

	data, err := json.Marshal(callResult)
	if err != nil {
		return nil, &jsonRPCError{Code: -32603, Message: "Internal error"}
	}
	return data, nil
}

// ── HTTP Response Helpers ───────────────────────────────────────

func (s *mcpServer) writeResult(w http.ResponseWriter, id json.RawMessage, result json.RawMessage, sessionID string) {
	resp := jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("MCP-Protocol-Version", protocolVersion)

	// Set session ID on initialize response
	if sessionID != "" {
		w.Header().Set("Mcp-Session-Id", sessionID)
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

func (s *mcpServer) writeError(w http.ResponseWriter, id json.RawMessage, code int, message string) {
	resp := jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &jsonRPCError{Code: code, Message: message},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK) // JSON-RPC errors still use 200
	json.NewEncoder(w).Encode(resp)
}

// ── Utility ─────────────────────────────────────────────────────

func toFloat64(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

func toInt(v interface{}) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case float32:
		return int(n), true
	case int:
		return n, true
	case int64:
		return int(n), true
	case json.Number:
		i, err := n.Int64()
		return int(i), err == nil
	default:
		return 0, false
	}
}

// ── Main ────────────────────────────────────────────────────────

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "50009"
	}

	server := newMCPServer()

	// MCP endpoint
	http.HandleFunc("/mcp", server.handleMCP)

	// Health check
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":   "healthy",
			"server":   "mcp-demo-server",
			"version":  "1.0.0",
			"protocol": protocolVersion,
			"tools":    len(demoTools),
		})
	})

	log.Printf("MCP Demo Server starting on :%s", port)
	log.Printf("  MCP endpoint: http://localhost:%s/mcp", port)
	log.Printf("  Health check: http://localhost:%s/health", port)
	log.Printf("  Tools: %s", toolNames())

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := http.ListenAndServe(":"+port, nil); err != nil {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	<-quit
	log.Println("Shutting down MCP Demo Server...")
}

func toolNames() string {
	names := make([]string, len(demoTools))
	for i, t := range demoTools {
		names[i] = t.Name
	}
	return strings.Join(names, ", ")
}
