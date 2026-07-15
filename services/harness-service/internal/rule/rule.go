// Package rule provides rule engine and guardrail functionality
package rule

import (
	"fmt"
	"regexp"
	"strings"
)

// AgentSpec defines agent constraints
type AgentSpec struct {
	Type             string   `json:"type"`
	Name             string   `json:"name"`
	Capabilities     []string `json:"capabilities"`
	MaxRounds        int      `json:"max_rounds"`
	NeedsLLM         bool     `json:"needs_llm"`
	SystemPrompt     string   `json:"system_prompt"`
	MaxTokensPerCall int      `json:"max_tokens_per_call"`
	AllowedTools     []string `json:"allowed_tools"`
	BudgetLimitUSD   float64  `json:"budget_limit_usd"`
}

// Engine is the rule engine
type Engine struct {
	specs map[string]*AgentSpec
}

// NewEngine creates a new rule engine
func NewEngine() *Engine {
	return &Engine{
		specs: make(map[string]*AgentSpec),
	}
}

// RegisterSpec registers an agent spec
func (e *Engine) RegisterSpec(spec *AgentSpec) {
	if spec == nil || spec.Type == "" {
		return
	}
	e.specs[spec.Type] = spec
}

// GetSpec gets an agent spec
func (e *Engine) GetSpec(agentType string) *AgentSpec {
	return e.specs[agentType]
}

// CheckResult is the result of rule check
type CheckResult struct {
	Passed     bool
	Violations []string
}

// Check checks rules for an agent
func (e *Engine) Check(agentID, content string) *CheckResult {
	// Basic rule check
	return &CheckResult{
		Passed:     true,
		Violations: nil,
	}
}

// CheckTokens checks token limit
func (e *Engine) CheckTokens(agentType string, tokenCount int) error {
	spec := e.specs[agentType]
	if spec == nil {
		return nil
	}
	if spec.MaxTokensPerCall > 0 && tokenCount > spec.MaxTokensPerCall {
		return fmt.Errorf("token limit exceeded: %d > %d", tokenCount, spec.MaxTokensPerCall)
	}
	return nil
}

// CheckTool checks if tool is allowed
func (e *Engine) CheckTool(agentType, toolName string) error {
	spec := e.specs[agentType]
	if spec == nil {
		return nil
	}
	if len(spec.AllowedTools) == 0 {
		return nil
	}
	for _, t := range spec.AllowedTools {
		if t == toolName || t == "*" {
			return nil
		}
	}
	return fmt.Errorf("tool %s is not allowed for agent %s", toolName, agentType)
}

// CheckBudget checks budget limit
func (e *Engine) CheckBudget(agentType string, costUSD float64) error {
	spec := e.specs[agentType]
	if spec == nil {
		return nil
	}
	if spec.BudgetLimitUSD > 0 && costUSD > spec.BudgetLimitUSD {
		return fmt.Errorf("budget limit exceeded: $%.4f > $%.4f", costUSD, spec.BudgetLimitUSD)
	}
	return nil
}

// CheckAll checks all rules
func (e *Engine) CheckAll(agentType string, tokenCount int, toolName string, costUSD float64) error {
	if err := e.CheckTokens(agentType, tokenCount); err != nil {
		return err
	}
	if err := e.CheckTool(agentType, toolName); err != nil {
		return err
	}
	if err := e.CheckBudget(agentType, costUSD); err != nil {
		return err
	}
	return nil
}

// Prompt injection patterns
var promptInjections = []*regexp.Regexp{
	regexp.MustCompile(`(?i)ignore\s+(all\s+)?(previous|above|prior)\s+(instructions|directions|prompts?|commands?)`),
	regexp.MustCompile(`(?i)forget\s+(all\s+)?(previous|above|prior)\s+instructions`),
	regexp.MustCompile(`(?i)you\s+are\s+(not\s+)?(an?\s+)?(ai|assistant|chatbot)`),
	regexp.MustCompile(`(?i)system\s*prompt`),
	regexp.MustCompile(`(?i)do\s+what\s+i\s+say`),
	regexp.MustCompile(`(?i)ignore\s+above`),
	regexp.MustCompile(`(?i)disregard\s+(all\s+)?(previous|above)`),
	regexp.MustCompile(`(?i)override\s+(previous|default|system)`),
	regexp.MustCompile(`(?i)jailbreak`),
	regexp.MustCompile(`(?i)DAN\s+mode`),
}

// Sensitive information patterns
var sensitivePatterns = []*regexp.Regexp{
	regexp.MustCompile(`sk-[a-zA-Z0-9]{20,}`),
	regexp.MustCompile(`[aA][pP][iI]_?[kK][eE][yY]\s*[:=]\s*['"][a-zA-Z0-9_\-]{16,}['"]`),
	regexp.MustCompile(`(?i)password\s*[:=]\s*\S{8,}`),
	regexp.MustCompile(`-----BEGIN\s+(RSA\s+)?PRIVATE\s+KEY-----`),
	regexp.MustCompile(`(?i)secret[_-]?key\s*[:=]\s*['"][a-zA-Z0-9_\-]{16,}['"]`),
	regexp.MustCompile(`(?i)access[_-]?token\s*[:=]\s*['"][a-zA-Z0-9_\-]{16,}['"]`),
}

// Sensitive tool prefixes that need extra caution
var sensitiveToolPrefixes = []string{"delete", "remove", "drop", "truncate", "exec", "shutdown", "rm", "kill", "format"}

// Guardrail checks content for violations
type Guardrail struct{}

// NewGuardrail creates a new guardrail
func NewGuardrail() *Guardrail {
	return &Guardrail{}
}

// CheckInput checks input content for violations
func (g *Guardrail) CheckInput(content string) (bool, string) {
	for _, pattern := range promptInjections {
		if pattern.MatchString(content) {
			return false, "input contains prompt injection pattern: " + pattern.String()
		}
	}
	return true, ""
}

// CheckOutput checks output content for sensitive information
func (g *Guardrail) CheckOutput(content string) (bool, string) {
	for _, pattern := range sensitivePatterns {
		if pattern.MatchString(content) {
			return false, "output contains sensitive information pattern: " + pattern.String()
		}
	}
	return true, ""
}

// Check checks content for violations (legacy method)
func (g *Guardrail) Check(content, checkType string) []string {
	var violations []string

	if checkType == "input" {
		for _, pattern := range promptInjections {
			if pattern.MatchString(content) {
				violations = append(violations, "prompt_injection_detected: "+pattern.String())
				break
			}
		}
	}

	// Always check for sensitive info
	for _, pattern := range sensitivePatterns {
		if pattern.MatchString(content) {
			violations = append(violations, "sensitive_info_detected: "+pattern.String())
			break
		}
	}

	return violations
}

// IsSensitiveTool checks if a tool is potentially dangerous
func (g *Guardrail) IsSensitiveTool(toolName string) bool {
	name := strings.ToLower(toolName)
	for _, prefix := range sensitiveToolPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// PermissionRule defines a permission rule
type PermissionRule struct {
	ToolPattern      string
	Effect           string // "allow" or "deny"
	MaxCalls         int
	RequiresApproval bool
}

// PermissionMatrix manages tool permissions
type PermissionMatrix struct {
	rules map[string][]PermissionRule
}

// NewPermissionMatrix creates a new permission matrix
func NewPermissionMatrix() *PermissionMatrix {
	m := &PermissionMatrix{
		rules: make(map[string][]PermissionRule),
	}
	m.setDefaults()
	return m
}

func (m *PermissionMatrix) setDefaults() {
	// Code agent - restricted destructive operations
	m.rules["code"] = []PermissionRule{
		{ToolPattern: "delete_*", Effect: "deny", MaxCalls: 0},
		{ToolPattern: "exec_*", Effect: "deny", MaxCalls: 0},
		{ToolPattern: "shutdown", Effect: "deny", MaxCalls: 0},
		{ToolPattern: "drop_*", Effect: "deny", MaxCalls: 0},
	}

	// Search agent - no exec or delete
	m.rules["search"] = []PermissionRule{
		{ToolPattern: "exec_*", Effect: "deny", MaxCalls: 0},
		{ToolPattern: "delete_*", Effect: "deny", MaxCalls: 0},
		{ToolPattern: "shutdown", Effect: "deny", MaxCalls: 0},
	}

	// General agent - only block shutdown
	m.rules["general"] = []PermissionRule{
		{ToolPattern: "shutdown", Effect: "deny", MaxCalls: 0},
	}

	// Analysis agent - no delete or shutdown
	m.rules["analysis"] = []PermissionRule{
		{ToolPattern: "delete_*", Effect: "deny", MaxCalls: 0},
		{ToolPattern: "shutdown", Effect: "deny", MaxCalls: 0},
	}
}

// Check checks if an agent can use a tool
func (m *PermissionMatrix) Check(agentType, toolName string, callCount int) error {
	rules, ok := m.rules[agentType]
	if !ok {
		return nil
	}

	for _, r := range rules {
		matched, err := regexp.MatchString(r.ToolPattern, toolName)
		if err != nil {
			// Try simple wildcard matching
			matched = simpleMatch(r.ToolPattern, toolName)
		}
		if !matched {
			continue
		}
		if r.Effect == "deny" {
			return fmt.Errorf("agent %s denied tool %s (matches pattern %s)", agentType, toolName, r.ToolPattern)
		}
		if r.MaxCalls > 0 && callCount >= r.MaxCalls {
			return fmt.Errorf("agent %s exceeded max calls (%d) for tool %s", agentType, r.MaxCalls, toolName)
		}
	}

	return nil
}

// AddRule adds a permission rule
func (m *PermissionMatrix) AddRule(agentType string, rule PermissionRule) {
	m.rules[agentType] = append(m.rules[agentType], rule)
}

// SetRules sets all rules for an agent type
func (m *PermissionMatrix) SetRules(agentType string, rules []PermissionRule) {
	m.rules[agentType] = rules
}

// simpleMatch does simple wildcard matching
func simpleMatch(pattern, str string) bool {
	if pattern == "*" {
		return true
	}
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(str, prefix)
	}
	if strings.HasPrefix(pattern, "*") {
		suffix := strings.TrimPrefix(pattern, "*")
		return strings.HasSuffix(str, suffix)
	}
	return pattern == str
}
