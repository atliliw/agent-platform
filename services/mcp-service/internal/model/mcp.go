// Package model provides data models for MCP service
package model

// Tool represents an MCP tool
type Tool struct {
	Name        string
	Description string
	InputSchema string
}

// Resource represents an MCP resource
type Resource struct {
	URI         string
	Name        string
	Description string
	MimeType    string
}

// Prompt represents an MCP prompt
type Prompt struct {
	Name        string
	Description string
	Arguments   []PromptArgument
}

// PromptArgument represents a prompt argument
type PromptArgument struct {
	Name        string
	Description string
	Required    bool
}

// Connection represents an MCP connection
type Connection struct {
	ID            string
	Name          string
	Type          string            // "stdio", "streamable-http"
	Command       string            // for stdio
	URL           string            // for streamable-http
	Env           map[string]string
	Status        string            // "connecting" | "connected" | "disconnected" | "error"
	ServerName    string            // from initialize handshake
	ServerVersion string            // from initialize handshake
	ToolCount     int               // number of remote tools discovered
	ErrorMsg      string            // error message if status is "error"
}