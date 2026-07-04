// Package prompt provides template rendering for prompts
package prompt

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Renderer handles prompt template rendering with variable substitution
type Renderer struct {
	varPattern *regexp.Regexp
}

// NewRenderer creates a new prompt renderer
func NewRenderer() *Renderer {
	// Pattern matches {{var}}, {{ var }}, {{var|default}}, {{ var | default }}
	return &Renderer{
		varPattern: regexp.MustCompile(`\{\{\s*(\w+)(?:\s*\|\s*(.+?))?\s*\}\}`),
	}
}

// Render renders a prompt template with the given context
func (r *Renderer) Render(content string, ctx *RenderContext) (string, error) {
	if ctx == nil {
		ctx = &RenderContext{Variables: make(map[string]interface{})}
	}
	if ctx.Variables == nil {
		ctx.Variables = make(map[string]interface{})
	}

	// Replace all variables
	result := r.varPattern.ReplaceAllStringFunc(content, func(match string) string {
		// Extract variable name and default value
		submatches := r.varPattern.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match // Invalid pattern, keep as-is
		}

		varName := submatches[1]
		defaultValue := ""
		if len(submatches) > 2 && submatches[2] != "" {
			defaultValue = strings.TrimSpace(submatches[2])
		}

		// Look up variable value
		if value, exists := ctx.Variables[varName]; exists {
			return formatValue(value)
		}

		// Use default value if provided
		if defaultValue != "" {
			return defaultValue
		}

		// Keep placeholder if not found and no default
		return match
	})

	return result, nil
}

// RenderWithValidation renders and validates all required variables are provided
func (r *Renderer) RenderWithValidation(content string, varSchema string, ctx *RenderContext) (string, error) {
	// Parse variable schema
	var varSet VariableSet
	if varSchema != "" {
		if err := json.Unmarshal([]byte(varSchema), &varSet); err != nil {
			return "", fmt.Errorf("parse variable schema: %w", err)
		}
	}

	// Validate required variables
	for _, v := range varSet.Variables {
		if v.Required {
			if ctx == nil || ctx.Variables == nil {
				return "", fmt.Errorf("required variable '%s' not provided", v.Name)
			}
			if _, exists := ctx.Variables[v.Name]; !exists {
				if v.Default == nil {
					return "", fmt.Errorf("required variable '%s' not provided", v.Name)
				}
			}
		}
	}

	// Apply defaults
	if ctx == nil {
		ctx = &RenderContext{Variables: make(map[string]interface{})}
	}
	if ctx.Variables == nil {
		ctx.Variables = make(map[string]interface{})
	}

	for _, v := range varSet.Variables {
		if _, exists := ctx.Variables[v.Name]; !exists && v.Default != nil {
			ctx.Variables[v.Name] = v.Default
		}
	}

	// Render template
	return r.Render(content, ctx)
}

// ExtractVariables extracts all variable names from a template
func (r *Renderer) ExtractVariables(content string) []string {
	matches := r.varPattern.FindAllStringSubmatch(content, -1)
	var vars []string
	seen := make(map[string]bool)

	for _, m := range matches {
		if len(m) >= 2 {
			varName := m[1]
			if !seen[varName] {
				seen[varName] = true
				vars = append(vars, varName)
			}
		}
	}

	sort.Strings(vars)
	return vars
}

// ValidateVariables validates that provided variables match the schema
func (r *Renderer) ValidateVariables(varSchema string, vars map[string]interface{}) ([]string, error) {
	if varSchema == "" {
		return nil, nil
	}

	var varSet VariableSet
	if err := json.Unmarshal([]byte(varSchema), &varSet); err != nil {
		return nil, fmt.Errorf("parse variable schema: %w", err)
	}

	var warnings []string

	for _, v := range varSet.Variables {
		value, exists := vars[v.Name]

		// Check required
		if v.Required && !exists && v.Default == nil {
			return nil, fmt.Errorf("required variable '%s' missing", v.Name)
		}

		if !exists {
			continue
		}

		// Validate type
		if err := validateType(v, value); err != nil {
			return nil, fmt.Errorf("variable '%s': %w", v.Name, err)
		}

		// Validate enum
		if len(v.Enum) > 0 {
			strVal := fmt.Sprintf("%v", value)
			found := false
			for _, e := range v.Enum {
				if e == strVal {
					found = true
					break
				}
			}
			if !found {
				warnings = append(warnings, fmt.Sprintf("variable '%s' value '%s' not in enum %v", v.Name, strVal, v.Enum))
			}
		}

		// Validate min/max for numbers
		if v.Type == "number" {
			numVal, ok := toFloat64(value)
			if ok {
				if v.Min != nil && numVal < *v.Min {
					warnings = append(warnings, fmt.Sprintf("variable '%s' value %.2f below min %.2f", v.Name, numVal, *v.Min))
				}
				if v.Max != nil && numVal > *v.Max {
					warnings = append(warnings, fmt.Sprintf("variable '%s' value %.2f above max %.2f", v.Name, numVal, *v.Max))
				}
			}
		}
	}

	return warnings, nil
}

// InferSchema infers a variable schema from a template
func (r *Renderer) InferSchema(content string) *VariableSet {
	vars := r.ExtractVariables(content)
	var varSet VariableSet

	for _, v := range vars {
		varSet.Variables = append(varSet.Variables, Variable{
			Name:     v,
			Type:     "string", // Default to string
			Required: true,     // Default to required
		})
	}

	return &varSet
}

// MergeVariables merges user-provided variables with defaults from schema
func (r *Renderer) MergeVariables(varSchema string, userVars map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	// Apply defaults from schema
	if varSchema != "" {
		var varSet VariableSet
		if err := json.Unmarshal([]byte(varSchema), &varSet); err == nil {
			for _, v := range varSet.Variables {
				if v.Default != nil {
					result[v.Name] = v.Default
				}
			}
		}
	}

	// Override with user-provided values
	for k, v := range userVars {
		result[k] = v
	}

	return result
}

// BuildContext builds a RenderContext from various sources
func (r *Renderer) BuildContext(userID, sessionID, agentID, model string, vars map[string]interface{}, metadata map[string]string) *RenderContext {
	ctx := &RenderContext{
		Variables: vars,
		UserID:    userID,
		SessionID: sessionID,
		AgentID:   agentID,
		Model:     model,
		Metadata:  metadata,
	}

	// Add context variables
	if ctx.Variables == nil {
		ctx.Variables = make(map[string]interface{})
	}

	// Add special context variables
	ctx.Variables["_user_id"] = userID
	ctx.Variables["_session_id"] = sessionID
	ctx.Variables["_agent_id"] = agentID
	ctx.Variables["_model"] = model
	ctx.Variables["_timestamp"] = time.Now().Format(time.RFC3339)

	return ctx
}

// Helper functions

func formatValue(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case float64, float32, int, int64, int32:
		return fmt.Sprintf("%v", v)
	case bool:
		return fmt.Sprintf("%v", v)
	case time.Time:
		return v.Format(time.RFC3339)
	case []interface{}:
		// Format as newline-separated list
		var strs []string
		for _, item := range v {
			strs = append(strs, formatValue(item))
		}
		return strings.Join(strs, "\n")
	case map[string]interface{}:
		// Format as JSON
		jsonBytes, _ := json.Marshal(v)
		return string(jsonBytes)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func validateType(v Variable, value interface{}) error {
	switch v.Type {
	case "string":
		// Accept anything, will be formatted as string
		return nil
	case "number":
		if _, ok := toFloat64(value); !ok {
			return fmt.Errorf("expected number, got %T", value)
		}
		return nil
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("expected boolean, got %T", value)
		}
		return nil
	case "array":
		if _, ok := value.([]interface{}); !ok {
			// Also accept string as comma-separated array
			if _, ok := value.(string); !ok {
				return fmt.Errorf("expected array, got %T", value)
			}
		}
		return nil
	case "object":
		if _, ok := value.(map[string]interface{}); !ok {
			return fmt.Errorf("expected object, got %T", value)
		}
		return nil
	default:
		return nil
	}
}

func toFloat64(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case int32:
		return float64(v), true
	case string:
		var f float64
		if _, err := fmt.Sscanf(v, "%f", &f); err == nil {
			return f, true
		}
		return 0, false
	default:
		return 0, false
	}
}