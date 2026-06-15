// Package guardrail provides safety guardrail functionality
package guardrail

import (
	"regexp"
	"strings"
)

// GuardrailType represents the type of guardrail check
type GuardrailType string

const (
	GuardrailInput  GuardrailType = "input"
	GuardrailOutput GuardrailType = "output"
)

// Violation represents a guardrail violation
type Violation struct {
	Type        string
	Description string
	Severity    string
}

// Engine is the guardrail engine
type Engine struct {
	patterns map[string]*regexp.Regexp
}

// NewGuardrail creates a new guardrail engine
func NewGuardrail() *Engine {
	e := &Engine{
		patterns: make(map[string]*regexp.Regexp),
	}
	e.loadPatterns()
	return e
}

// loadPatterns loads default patterns
func (e *Engine) loadPatterns() {
	// Prompt injection patterns
	e.patterns["prompt_injection"] = regexp.MustCompile(`(?i)(ignore\s+previous|ignore\s+instructions|override\s+system|you\s+are\s+now|act\s+as|pretend\s+to\s+be|system\s+prompt)`)

	// PII patterns
	e.patterns["phone"] = regexp.MustCompile(`1[3-9]\d{9}`)
	e.patterns["id_card"] = regexp.MustCompile(`\d{17}[\dXx]`)
	e.patterns["bank_card"] = regexp.MustCompile(`\d{16,19}`)

	// Sensitive patterns
	e.patterns["api_key"] = regexp.MustCompile(`(?i)(api[_-]?key|secret[_-]?key|access[_-]?token|auth[_-]?token)[\s=:]+[a-zA-Z0-9_-]{20,}`)
	e.patterns["password"] = regexp.MustCompile(`(?i)(password|passwd|pwd)[\s=:]+[^\s]{8,}`)
}

// Check checks content for violations
func (e *Engine) Check(content string, checkType GuardrailType) []string {
	var violations []string

	for name, pattern := range e.patterns {
		if pattern.MatchString(content) {
			violations = append(violations, name)
		}
	}

	return violations
}

// Sanitize sanitizes content by masking sensitive information
func (e *Engine) Sanitize(content string) string {
	result := content

	// Mask phone numbers
	if e.patterns["phone"] != nil {
		result = e.patterns["phone"].ReplaceAllString(result, "[PHONE_MASKED]")
	}

	// Mask ID cards
	if e.patterns["id_card"] != nil {
		result = e.patterns["id_card"].ReplaceAllString(result, "[ID_MASKED]")
	}

	// Mask bank cards
	if e.patterns["bank_card"] != nil {
		result = e.patterns["bank_card"].ReplaceAllString(result, "[CARD_MASKED]")
	}

	// Mask API keys
	if e.patterns["api_key"] != nil {
		result = e.patterns["api_key"].ReplaceAllString(result, "[API_KEY_MASKED]")
	}

	// Mask passwords
	if e.patterns["password"] != nil {
		result = e.patterns["password"].ReplaceAllString(result, "[PASSWORD_MASKED]")
	}

	return result
}

// ContainsPII checks if content contains PII
func (e *Engine) ContainsPII(content string) bool {
	piiPatterns := []string{"phone", "id_card", "bank_card"}
	for _, name := range piiPatterns {
		if pattern, exists := e.patterns[name]; exists && pattern.MatchString(content) {
			return true
		}
	}
	return false
}

// ContainsPromptInjection checks if content contains prompt injection attempts
func (e *Engine) ContainsPromptInjection(content string) bool {
	if pattern, exists := e.patterns["prompt_injection"]; exists {
		return pattern.MatchString(content)
	}
	return false
}

// GetPattern retrieves a pattern by name
func (e *Engine) GetPattern(name string) *regexp.Regexp {
	return e.patterns[name]
}

// AddPattern adds a custom pattern
func (e *Engine) AddPattern(name, pattern string) error {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}
	e.patterns[name] = re
	return nil
}

// RemovePattern removes a pattern
func (e *Engine) RemovePattern(name string) {
	delete(e.patterns, name)
}

// ListPatterns lists all pattern names
func (e *Engine) ListPatterns() []string {
	var names []string
	for name := range e.patterns {
		names = append(names, name)
	}
	return names
}

// CheckInput performs input guardrail check
func (e *Engine) CheckInput(content string) []string {
	return e.Check(content, GuardrailInput)
}

// CheckOutput performs output guardrail check
func (e *Engine) CheckOutput(content string) []string {
	return e.Check(content, GuardrailOutput)
}

// IsSafe checks if content is safe (no violations)
func (e *Engine) IsSafe(content string) bool {
	return len(e.Check(content, GuardrailInput)) == 0
}

// Validate validates content and returns detailed result
func (e *Engine) Validate(content string) (bool, []Violation) {
	var violations []Violation
	violationNames := e.Check(content, GuardrailInput)

	for _, name := range violationNames {
		severity := "medium"
		if strings.Contains(name, "injection") || strings.Contains(name, "password") || strings.Contains(name, "api_key") {
			severity = "high"
		}

		violations = append(violations, Violation{
			Type:        name,
			Description: getViolationDescription(name),
			Severity:    severity,
		})
	}

	return len(violations) == 0, violations
}

func getViolationDescription(name string) string {
	descriptions := map[string]string{
		"prompt_injection": "Potential prompt injection detected",
		"phone":            "Phone number detected",
		"id_card":          "ID card number detected",
		"bank_card":        "Bank card number detected",
		"api_key":          "API key detected",
		"password":         "Password detected",
	}

	if desc, exists := descriptions[name]; exists {
		return desc
	}
	return "Sensitive content detected"
}