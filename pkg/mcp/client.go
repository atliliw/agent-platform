package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

// ClientCapabilities declares what the MCP client supports.
type ClientCapabilities struct {
	Roots      *RootsCapability      `json:"roots,omitempty"`
	Sampling   *SamplingCapability   `json:"sampling,omitempty"`
	Elicitation *ElicitationCapability `json:"elicitation,omitempty"`
}

type RootsCapability struct {
	ListChanged bool `json:"listChanged"`
}

type SamplingCapability struct{}

type ElicitationCapability struct{}

// ServerCapabilities describes what the MCP server supports.
type ServerCapabilities struct {
	Tools      *ToolsCapability      `json:"tools,omitempty"`
	Resources  *ResourcesCapability  `json:"resources,omitempty"`
	Prompts    *PromptsCapability    `json:"prompts,omitempty"`
	Logging    *LoggingCapability    `json:"logging,omitempty"`
	Completions *CompletionsCapability `json:"completions,omitempty"`
}

type ToolsCapability struct {
	ListChanged bool `json:"listChanged"`
}

type ResourcesCapability struct {
	Subscribe   bool `json:"subscribe"`
	ListChanged bool `json:"listChanged"`
}

type PromptsCapability struct {
	ListChanged bool `json:"listChanged"`
}

type LoggingCapability struct{}

type CompletionsCapability struct{}

// InitializeParams is the params for the initialize request.
type InitializeParams struct {
	ProtocolVersion string            `json:"protocolVersion"`
	Capabilities    ClientCapabilities `json:"capabilities"`
	ClientInfo      ClientInfo        `json:"clientInfo"`
}

type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// InitializeResult is the result of the initialize request.
type InitializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      ServerInfo         `json:"serverInfo"`
	Instructions    string             `json:"instructions,omitempty"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// RemoteTool represents a tool discovered from a remote MCP server.
type RemoteTool struct {
	Name        string          `json:"name"`
	Title       string          `json:"title,omitempty"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// ToolsListResult is the result of tools/list.
type ToolsListResult struct {
	Tools      []RemoteTool `json:"tools"`
	NextCursor string       `json:"nextCursor,omitempty"`
}

// ToolCallParams is the params for tools/call.
type ToolCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// ToolCallResult is the result of tools/call.
type ToolCallResult struct {
	Content []ToolContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// ToolContent represents a content block in a tool call result.
type ToolContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// ResourcesListResult is the result of resources/list.
type ResourcesListResult struct {
	Resources []RemoteResource `json:"resources"`
}

// RemoteResource represents a resource from a remote MCP server.
type RemoteResource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// ResourceReadResult is the result of resources/read.
type ResourceReadResult struct {
	Contents []ResourceContent `json:"contents"`
}

// ResourceContent represents content from a resource read.
type ResourceContent struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
	Blob     string `json:"blob,omitempty"`
}

// PromptsListResult is the result of prompts/list.
type PromptsListResult struct {
	Prompts []RemotePrompt `json:"prompts"`
}

// RemotePrompt represents a prompt from a remote MCP server.
type RemotePrompt struct {
	Name        string             `json:"name"`
	Description string             `json:"description,omitempty"`
	Arguments   []PromptArgDef     `json:"arguments,omitempty"`
}

// PromptArgDef defines a prompt argument.
type PromptArgDef struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required"`
}

// PromptGetResult is the result of prompts/get.
type PromptGetResult struct {
	Description string          `json:"description,omitempty"`
	Messages    []PromptMessage `json:"messages"`
}

// PromptMessage is a message in a prompt result.
type PromptMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

// Client is an MCP protocol client that connects to an MCP server.
type Client struct {
	transport Transport
	idGen     nextRequestID
	mu        sync.Mutex // protects serverInfo / serverCaps / tools
	callMu    sync.Mutex // serializes sendRequest send+receive cycles (transport is single-reader)

	// Server info (populated after Initialize)
	serverInfo      ServerInfo
	serverCaps      ServerCapabilities
	serverInstr     string
	protocolVersion string

	// Cached tools from the remote server
	tools []RemoteTool

	// Background goroutine for receiving messages
	recvDone chan struct{}
}

// NewClient creates a new MCP client with the given transport.
func NewClient(transport Transport) *Client {
	return &Client{
		transport: transport,
		recvDone:  make(chan struct{}),
	}
}

// Initialize performs the MCP 3-step handshake: initialize request → response → initialized notification.
// After this, the client is ready to use tools/resources/prompts.
func (c *Client) Initialize(ctx context.Context) error {
	// Start the transport
	if err := c.transport.Start(ctx); err != nil {
		return fmt.Errorf("failed to start transport: %w", err)
	}

	// Start background receiver
	go c.receiveLoop()

	// Step 1: Send initialize request
	params := InitializeParams{
		ProtocolVersion: ProtocolVersion,
		Capabilities: ClientCapabilities{
			Roots: &RootsCapability{ListChanged: true},
		},
		ClientInfo: ClientInfo{
			Name:    "agent-platform-mcp-client",
			Version: "1.0.0",
		},
	}

	result, err := c.sendRequest(ctx, "initialize", params)
	if err != nil {
		return fmt.Errorf("initialize request failed: %w", err)
	}

	var initResult InitializeResult
	if err := unmarshalResult(result, &initResult); err != nil {
		return fmt.Errorf("failed to unmarshal initialize result: %w", err)
	}

	c.mu.Lock()
	c.serverInfo = initResult.ServerInfo
	c.serverCaps = initResult.Capabilities
	c.serverInstr = initResult.Instructions
	c.protocolVersion = initResult.ProtocolVersion
	c.mu.Unlock()

	// Step 2: Send initialized notification
	if err := c.sendNotification(ctx, "notifications/initialized", nil); err != nil {
		return fmt.Errorf("failed to send initialized notification: %w", err)
	}

	// Step 3: Discover tools if server supports them
	if initResult.Capabilities.Tools != nil {
		if err := c.refreshTools(ctx); err != nil {
			log.Printf("[MCP] Warning: failed to discover tools from %s: %v", initResult.ServerInfo.Name, err)
			// Non-fatal: we can still use the connection, just no tools
		}
	}

	return nil
}

// ListTools returns the cached list of tools from the remote server.
func (c *Client) ListTools() []RemoteTool {
	c.mu.Lock()
	defer c.mu.Unlock()
	tools := make([]RemoteTool, len(c.tools))
	copy(tools, c.tools)
	return tools
}

// RefreshTools re-discovers tools from the remote server.
func (c *Client) RefreshTools(ctx context.Context) error {
	return c.refreshTools(ctx)
}

// CallTool invokes a tool on the remote MCP server.
func (c *Client) CallTool(ctx context.Context, name string, arguments map[string]interface{}) (*ToolCallResult, error) {
	params := ToolCallParams{
		Name:      name,
		Arguments: arguments,
	}

	result, err := c.sendRequest(ctx, "tools/call", params)
	if err != nil {
		return nil, fmt.Errorf("tools/call failed: %w", err)
	}

	var callResult ToolCallResult
	if err := unmarshalResult(result, &callResult); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tools/call result: %w", err)
	}

	return &callResult, nil
}

// ListResources discovers resources from the remote server.
func (c *Client) ListResources(ctx context.Context) (*ResourcesListResult, error) {
	result, err := c.sendRequest(ctx, "resources/list", nil)
	if err != nil {
		return nil, fmt.Errorf("resources/list failed: %w", err)
	}

	var listResult ResourcesListResult
	if err := unmarshalResult(result, &listResult); err != nil {
		return nil, fmt.Errorf("failed to unmarshal resources/list result: %w", err)
	}

	return &listResult, nil
}

// ReadResource reads a resource from the remote server.
func (c *Client) ReadResource(ctx context.Context, uri string) (*ResourceReadResult, error) {
	result, err := c.sendRequest(ctx, "resources/read", map[string]string{"uri": uri})
	if err != nil {
		return nil, fmt.Errorf("resources/read failed: %w", err)
	}

	var readResult ResourceReadResult
	if err := unmarshalResult(result, &readResult); err != nil {
		return nil, fmt.Errorf("failed to unmarshal resources/read result: %w", err)
	}

	return &readResult, nil
}

// ListPrompts discovers prompts from the remote server.
func (c *Client) ListPrompts(ctx context.Context) (*PromptsListResult, error) {
	result, err := c.sendRequest(ctx, "prompts/list", nil)
	if err != nil {
		return nil, fmt.Errorf("prompts/list failed: %w", err)
	}

	var listResult PromptsListResult
	if err := unmarshalResult(result, &listResult); err != nil {
		return nil, fmt.Errorf("failed to unmarshal prompts/list result: %w", err)
	}

	return &listResult, nil
}

// GetPrompt gets a prompt from the remote server.
func (c *Client) GetPrompt(ctx context.Context, name string, arguments map[string]string) (*PromptGetResult, error) {
	params := map[string]interface{}{
		"name": name,
	}
	if arguments != nil {
		params["arguments"] = arguments
	}

	result, err := c.sendRequest(ctx, "prompts/get", params)
	if err != nil {
		return nil, fmt.Errorf("prompts/get failed: %w", err)
	}

	var getResult PromptGetResult
	if err := unmarshalResult(result, &getResult); err != nil {
		return nil, fmt.Errorf("failed to unmarshal prompts/get result: %w", err)
	}

	return &getResult, nil
}

// Ping sends a health check to the server.
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.sendRequest(ctx, "ping", nil)
	return err
}

// Close shuts down the client and its transport.
func (c *Client) Close() error {
	// Try to send a clean shutdown notification (best effort)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = c.sendNotification(ctx, "notifications/cancelled", nil)

	return c.transport.Close()
}

// ServerInfo returns the server info from the initialization handshake.
func (c *Client) ServerInfo() ServerInfo {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.serverInfo
}

// ServerCapabilities returns the server capabilities.
func (c *Client) ServerCapabilities() ServerCapabilities {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.serverCaps
}

// IsAlive returns whether the client's transport is still connected.
func (c *Client) IsAlive() bool {
	return c.transport.IsAlive()
}

// --- Internal methods ---

func (c *Client) refreshTools(ctx context.Context) error {
	result, err := c.sendRequest(ctx, "tools/list", nil)
	if err != nil {
		return err
	}

	var listResult ToolsListResult
	if err := unmarshalResult(result, &listResult); err != nil {
		return fmt.Errorf("failed to unmarshal tools/list result: %w", err)
	}

	c.mu.Lock()
	c.tools = listResult.Tools
	c.mu.Unlock()

	return nil
}

// sendRequest sends a JSON-RPC request and waits for the matching response.
func (c *Client) sendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	// Serialize the send+receive cycle. The transport is a single reader
	// (stdio stdout / HTTP pending channel), so concurrent waitForResponse
	// calls would race on reads; serializing here also keeps idGen safe.
	c.callMu.Lock()
	defer c.callMu.Unlock()

	id := c.idGen.next()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  marshalParams(params),
	}

	msg, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	if err := c.transport.Send(ctx, msg); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Wait for response with matching ID
	resp, err := c.waitForResponse(ctx, id)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, resp.Error
	}

	return resp.Result, nil
}

// sendNotification sends a JSON-RPC notification (no ID, no response expected).
func (c *Client) sendNotification(ctx context.Context, method string, params interface{}) error {
	notif := JSONRPCNotification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  marshalParams(params),
	}

	msg, err := json.Marshal(notif)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	return c.transport.Send(ctx, msg)
}

// defaultResponseTimeout caps how long waitForResponse waits when the caller's
// context has no deadline. When the caller already provides a deadline (the gRPC
// CallTool deadline, or the 30s handshake timeout), it is respected as-is so
// long-running tool calls are not truncated.
const defaultResponseTimeout = 2 * time.Minute

// waitForResponse reads messages from the transport until a response with the matching ID arrives.
func (c *Client) waitForResponse(ctx context.Context, id json.RawMessage) (*JSONRPCResponse, error) {
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultResponseTimeout)
		defer cancel()
	}

	for {
		raw, err := c.transport.Receive(ctx)
		if err != nil {
			return nil, fmt.Errorf("transport receive error: %w", err)
		}

		// Try to parse as response
		var resp JSONRPCResponse
		if err := json.Unmarshal(raw, &resp); err == nil && resp.ID != nil {
			// Check if this is our response
			if bytesEqual(resp.ID, id) {
				return &resp, nil
			}
			// Not our response, skip (could be from a concurrent request)
			continue
		}

		// Could be a notification, log and continue
		var notif JSONRPCNotification
		if err := json.Unmarshal(raw, &notif); err == nil {
			log.Printf("[MCP] Received notification: %s", notif.Method)
			continue
		}

		// Unknown message, skip
		log.Printf("[MCP] Received unparseable message: %s", string(raw))
	}
}

// receiveLoop is a background goroutine that keeps the transport alive.
// For stdio, it's not strictly needed since we read synchronously in waitForResponse.
// For HTTP, it helps with SSE messages. We keep it as a no-op for now since
// our sendRequest pattern handles reading inline.
func (c *Client) receiveLoop() {
	defer close(c.recvDone)
	// Currently a no-op: responses are read inline in waitForResponse.
	// Future: handle server-initiated requests (sampling, elicitation) here.
}

// bytesEqual compares two json.RawMessage values.
func bytesEqual(a, b json.RawMessage) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return string(a) == string(b)
}
