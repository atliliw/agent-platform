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
	ID      string
	Name    string
	Type    string
	Command string
	URL     string
	Env     map[string]string
	Status  string
}