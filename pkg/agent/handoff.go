package agent

import (
	"fmt"
	"strings"
)

// HandoffPrefix is the prefix for handoff tool names
const HandoffPrefix = "transfer_to_"

// HandoffTool represents a handoff tool definition
type HandoffTool struct {
	TargetAgentID   string `json:"target_agent_id"`
	TargetAgentName string `json:"target_agent_name"`
	Description     string `json:"description"`
}

// NewHandoffTool creates a handoff tool for an agent
func NewHandoffTool(targetAgent *Agent) HandoffTool {
	return HandoffTool{
		TargetAgentID:   targetAgent.ID,
		TargetAgentName: targetAgent.Name,
		Description:     fmt.Sprintf("Transfer the conversation to %s. %s", targetAgent.Name, targetAgent.Description),
	}
}

// ToolName returns the tool name for this handoff
func (h HandoffTool) ToolName() string {
	return HandoffPrefix + h.TargetAgentID
}

// ToToolDefinition converts to LLM tool definition format
func (h HandoffTool) ToToolDefinition() map[string]any {
	return map[string]any{
		"type": "function",
		"function": map[string]any{
			"name":        h.ToolName(),
			"description": h.Description,
			"parameters": map[string]any{
				"type":       "object",
				"properties": map[string]any{},
				"required":   []string{},
			},
		},
	}
}

// IsHandoffTool checks if a tool name is a handoff tool
func IsHandoffTool(toolName string) bool {
	return strings.HasPrefix(toolName, HandoffPrefix)
}

// ParseHandoffTarget extracts the target agent ID from a handoff tool name
func ParseHandoffTarget(toolName string) string {
	if !IsHandoffTool(toolName) {
		return ""
	}
	return strings.TrimPrefix(toolName, HandoffPrefix)
}

// HandoffResult represents the result of a handoff operation
type HandoffResult struct {
	// Type indicates the result type
	Type string `json:"type"` // "handoff", "response", "tool_call"

	// TargetAgent is the agent being handed off to
	TargetAgent string `json:"target_agent,omitempty"`

	// Content is the response content
	Content string `json:"content,omitempty"`

	// Variables to update in context
	Variables map[string]any `json:"variables,omitempty"`

	// Reason for the handoff
	Reason string `json:"reason,omitempty"`
}

// NewHandoffResult creates a handoff result
func NewHandoffResult(targetAgentID string) *HandoffResult {
	return &HandoffResult{
		Type:        "handoff",
		TargetAgent: targetAgentID,
		Variables:   make(map[string]any),
	}
}

// NewResponseResult creates a response result
func NewResponseResult(content string) *HandoffResult {
	return &HandoffResult{
		Type:      "response",
		Content:   content,
		Variables: make(map[string]any),
	}
}

// WithReason sets the reason for handoff
func (r *HandoffResult) WithReason(reason string) *HandoffResult {
	r.Reason = reason
	return r
}

// WithVariable sets a variable
func (r *HandoffResult) WithVariable(key string, value any) *HandoffResult {
	if r.Variables == nil {
		r.Variables = make(map[string]any)
	}
	r.Variables[key] = value
	return r
}

// BuildHandoffTools builds handoff tools for an agent
func BuildHandoffTools(agent *Agent, registry *Registry) []HandoffTool {
	tools := make([]HandoffTool, 0, len(agent.Handoffs))

	for _, targetID := range agent.Handoffs {
		targetAgent := registry.Get(targetID)
		if targetAgent != nil {
			tools = append(tools, NewHandoffTool(targetAgent))
		}
	}

	return tools
}

// BuildAllTools builds all tools for an agent (regular tools + handoff tools)
func BuildAllTools(agent *Agent, registry *Registry, toolDefinitions map[string]any) []map[string]any {
	tools := make([]map[string]any, 0)

	// Add regular tools
	for _, toolName := range agent.Tools {
		if def, ok := toolDefinitions[toolName]; ok {
			tools = append(tools, def.(map[string]any))
		}
	}

	// Add handoff tools
	for _, targetID := range agent.Handoffs {
		targetAgent := registry.Get(targetID)
		if targetAgent != nil {
			ht := NewHandoffTool(targetAgent)
			tools = append(tools, ht.ToToolDefinition())
		}
	}

	return tools
}

// ValidateHandoff validates a handoff request
func ValidateHandoff(fromAgent *Agent, toAgentID string, registry *Registry) error {
	// Check if target exists
	if !registry.Exists(toAgentID) {
		return fmt.Errorf("target agent %s not found", toAgentID)
	}

	// Check if handoff is allowed
	if !fromAgent.CanHandoffTo(toAgentID) {
		return fmt.Errorf("agent %s cannot handoff to %s", fromAgent.ID, toAgentID)
	}

	return nil
}
