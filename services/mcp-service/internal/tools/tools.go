// Package tools provides tool implementations
package tools

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Executor is the interface for tool execution
type Executor interface {
	Execute(ctx context.Context, args map[string]interface{}) (string, error)
	ExecuteWithConfig(ctx context.Context, args map[string]interface{}, config map[string]interface{}) (string, error)
}

// ToolInfo contains tool metadata
type ToolInfo struct {
	Name        string
	Description string
	Parameters   map[string]interface{}
}

// GetInfo returns tool information for LLM
func (t *CalculatorTool) GetInfo() ToolInfo {
	return ToolInfo{
		Name:        "calculator",
		Description: "Perform mathematical calculations. Supports basic arithmetic operations (+, -, *, /) and parentheses.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"expression": map[string]interface{}{
					"type":        "string",
					"description": "Mathematical expression to evaluate (e.g., '2 + 3 * 4')",
				},
			},
			"required": []string{"expression"},
		},
	}
}

// CalculatorTool implements calculations
type CalculatorTool struct{}

// NewCalculatorTool creates a new calculator tool
func NewCalculatorTool() *CalculatorTool {
	return &CalculatorTool{}
}

// Execute executes the calculator tool
func (t *CalculatorTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	return t.ExecuteWithConfig(ctx, args, nil)
}

// ExecuteWithConfig executes the calculator tool with config (config is ignored for this tool)
func (t *CalculatorTool) ExecuteWithConfig(ctx context.Context, args map[string]interface{}, config map[string]interface{}) (string, error) {
	expression, _ := args["expression"].(string)
	if expression == "" {
		return "", fmt.Errorf("expression is required")
	}

	// Sanitize expression - only allow numbers and basic operators
	expression = strings.TrimSpace(expression)

	// Simple expression parser for basic math
	result, err := t.evaluateExpression(expression)
	if err != nil {
		return fmt.Sprintf("Error evaluating expression: %v", err), nil
	}

	return fmt.Sprintf("Expression: %s\nResult: %g", expression, result), nil
}

// evaluateExpression evaluates simple mathematical expressions
func (t *CalculatorTool) evaluateExpression(expr string) (float64, error) {
	// Remove spaces
	expr = strings.ReplaceAll(expr, " ", "")

	// Check for valid characters
	validPattern := regexp.MustCompile(`^[\d+\-*/().]+$`)
	if !validPattern.MatchString(expr) {
		return 0, fmt.Errorf("invalid characters in expression")
	}

	// Parse and evaluate
	// Simple implementation - handle basic operations
	numbers := []float64{}
	operators := []string{}

	// Extract numbers and operators
	numStr := ""
	for i, ch := range expr {
		if ch >= '0' && ch <= '9' || ch == '.' {
			numStr += string(ch)
		} else if ch == '+' || ch == '-' || ch == '*' || ch == '/' {
			if numStr != "" {
				num, err := strconv.ParseFloat(numStr, 64)
				if err != nil {
					return 0, err
				}
				numbers = append(numbers, num)
				numStr = ""
			}
			// Handle negative numbers at start or after operator
			if ch == '-' && (i == 0 || len(operators) > 0 && len(numbers) == len(operators)) {
				numStr = "-"
			} else {
				operators = append(operators, string(ch))
			}
		}
	}

	if numStr != "" {
		num, err := strconv.ParseFloat(numStr, 64)
		if err != nil {
			return 0, err
		}
		numbers = append(numbers, num)
	}

	if len(numbers) == 0 {
		return 0, fmt.Errorf("no numbers in expression")
	}

	if len(numbers) != len(operators)+1 {
		return 0, fmt.Errorf("invalid expression structure")
	}

	// First pass: handle * and /
	result := numbers[0]
	newNumbers := []float64{result}
	newOperators := []string{}

	for i, op := range operators {
		if op == "*" {
			newNumbers[len(newNumbers)-1] *= numbers[i+1]
		} else if op == "/" {
			if numbers[i+1] == 0 {
				return 0, fmt.Errorf("division by zero")
			}
			newNumbers[len(newNumbers)-1] /= numbers[i+1]
		} else {
			newNumbers = append(newNumbers, numbers[i+1])
			newOperators = append(newOperators, op)
		}
	}

	// Second pass: handle + and -
	result = newNumbers[0]
	for i, op := range newOperators {
		if op == "+" {
			result += newNumbers[i+1]
		} else if op == "-" {
			result -= newNumbers[i+1]
		}
	}

	// Round to reasonable precision
	return math.Round(result*1000000) / 1000000, nil
}

// GetInfo returns tool information for LLM
func (t *TimeTool) GetInfo() ToolInfo {
	return ToolInfo{
		Name:        "time",
		Description: "Get current time information.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"format": map[string]interface{}{
					"type":        "string",
					"description": "Time format (optional): 'default', 'iso', 'unix'",
				},
			},
			"required": []string{},
		},
	}
}

// TimeTool implements time queries
type TimeTool struct{}

// NewTimeTool creates a new time tool
func NewTimeTool() *TimeTool {
	return &TimeTool{}
}

// Execute executes the time tool
func (t *TimeTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	return t.ExecuteWithConfig(ctx, args, nil)
}

// ExecuteWithConfig executes the time tool with config (config is ignored)
func (t *TimeTool) ExecuteWithConfig(ctx context.Context, args map[string]interface{}, config map[string]interface{}) (string, error) {
	format, _ := args["format"].(string)
	now := time.Now()

	var result string
	switch format {
	case "iso":
		result = now.Format(time.RFC3339)
	case "unix":
		result = fmt.Sprintf("%d", now.Unix())
	default:
		result = fmt.Sprintf(`Current Time Information:

UTC: %s
Local Time: %s
Unix Timestamp: %d

Format requested: %s`, now.UTC().Format(time.RFC3339), now.Format("2006-01-02 15:04:05"), now.Unix(), format)
	}

	return result, nil
}

// GetInfo returns tool information for LLM
func (t *CodeExecTool) GetInfo() ToolInfo {
	return ToolInfo{
		Name:        "code_execute",
		Description: "Execute code in a sandboxed environment. Supports Python for calculations and data processing.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"code": map[string]interface{}{
					"type":        "string",
					"description": "The code to execute",
				},
				"language": map[string]interface{}{
					"type":        "string",
					"description": "Programming language (default: python)",
					"enum":        []string{"python"},
				},
			},
			"required": []string{"code"},
		},
	}
}

// CodeExecTool implements code execution
type CodeExecTool struct{}

// NewCodeExecTool creates a new code execution tool
func NewCodeExecTool() *CodeExecTool {
	return &CodeExecTool{}
}

// Execute executes code (mock implementation)
func (t *CodeExecTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	return t.ExecuteWithConfig(ctx, args, nil)
}

// ExecuteWithConfig executes code with config (config is ignored)
func (t *CodeExecTool) ExecuteWithConfig(ctx context.Context, args map[string]interface{}, config map[string]interface{}) (string, error) {
	code, _ := args["code"].(string)
	language, _ := args["language"].(string)
	if language == "" {
		language = "python"
	}

	if code == "" {
		return "", fmt.Errorf("code is required")
	}

	// TODO: Implement safe code execution with sandbox
	// For security, this is a mock that returns the code without executing
	return fmt.Sprintf(`Code Execution Request:
Language: %s
Code:
%s

Note: Code execution is disabled for security.
Implement a sandboxed executor for actual functionality.`, language, code), nil
}

// GetInfo returns tool information for LLM
func (t *FileReadTool) GetInfo() ToolInfo {
	return ToolInfo{
		Name:        "file_read",
		Description: "Read the contents of a file from the allowed directory.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Path to the file to read",
				},
			},
			"required": []string{"path"},
		},
	}
}

// FileReadTool reads file contents
type FileReadTool struct {
	AllowedDir string
}

// NewFileReadTool creates a new file read tool
func NewFileReadTool(allowedDir string) *FileReadTool {
	return &FileReadTool{AllowedDir: allowedDir}
}

// Execute executes the file read tool
func (t *FileReadTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	return t.ExecuteWithConfig(ctx, args, nil)
}

// ExecuteWithConfig executes the file read tool with config
func (t *FileReadTool) ExecuteWithConfig(ctx context.Context, args map[string]interface{}, config map[string]interface{}) (string, error) {
	path, _ := args["path"].(string)
	if path == "" {
		return "", fmt.Errorf("path is required")
	}

	// Security check - only allow reading from allowed directory
	// TODO: Implement actual file reading with proper security
	return fmt.Sprintf(`File Read Request:
Path: %s

Note: File reading is restricted for security.
Configure allowed directories to enable this feature.`, path), nil
}

// GetInfo returns tool information for LLM
func (t *FileWriteTool) GetInfo() ToolInfo {
	return ToolInfo{
		Name:        "file_write",
		Description: "Write content to a file in the allowed directory.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Path to the file to write",
				},
				"content": map[string]interface{}{
					"type":        "string",
					"description": "Content to write to the file",
				},
			},
			"required": []string{"path", "content"},
		},
	}
}

// FileWriteTool writes to files
type FileWriteTool struct {
	AllowedDir string
}

// NewFileWriteTool creates a new file write tool
func NewFileWriteTool(allowedDir string) *FileWriteTool {
	return &FileWriteTool{AllowedDir: allowedDir}
}

// Execute executes the file write tool
func (t *FileWriteTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	return t.ExecuteWithConfig(ctx, args, nil)
}

// ExecuteWithConfig executes the file write tool with config
func (t *FileWriteTool) ExecuteWithConfig(ctx context.Context, args map[string]interface{}, config map[string]interface{}) (string, error) {
	path, _ := args["path"].(string)
	content, _ := args["content"].(string)

	if path == "" {
		return "", fmt.Errorf("path is required")
	}

	// Security check - only allow writing to allowed directory
	// TODO: Implement actual file writing with proper security
	return fmt.Sprintf(`File Write Request:
Path: %s
Content Length: %d characters

Note: File writing is restricted for security.
Configure allowed directories to enable this feature.`, path, len(content)), nil
}

// GetInfo returns tool information for LLM
func (t *DataAnalysisTool) GetInfo() ToolInfo {
	return ToolInfo{
		Name:        "data_analysis",
		Description: "Perform statistical analysis on data. Calculates mean, median, standard deviation, and other statistics.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"data": map[string]interface{}{
					"type":        "array",
					"description": "Array of numerical values to analyze",
					"items": map[string]interface{}{
						"type": "number",
					},
				},
				"operations": map[string]interface{}{
					"type":        "array",
					"description": "Operations to perform (e.g., ['mean', 'median', 'std'])",
					"items": map[string]interface{}{
						"type": "string",
						"enum": []string{"mean", "median", "mode", "std", "var", "min", "max", "sum", "count"},
					},
				},
			},
			"required": []string{"data"},
		},
	}
}

// DataAnalysisTool performs statistical analysis
type DataAnalysisTool struct{}

// NewDataAnalysisTool creates a new data analysis tool
func NewDataAnalysisTool() *DataAnalysisTool {
	return &DataAnalysisTool{}
}

// Execute executes the data analysis tool
func (t *DataAnalysisTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	return t.ExecuteWithConfig(ctx, args, nil)
}

// ExecuteWithConfig executes the data analysis tool with config
func (t *DataAnalysisTool) ExecuteWithConfig(ctx context.Context, args map[string]interface{}, config map[string]interface{}) (string, error) {
	// Extract data array
	dataRaw, ok := args["data"].([]interface{})
	if !ok || len(dataRaw) == 0 {
		return "", fmt.Errorf("data array is required and must not be empty")
	}

	// Convert to float64 array
	data := make([]float64, 0, len(dataRaw))
	for _, v := range dataRaw {
		switch val := v.(type) {
		case float64:
			data = append(data, val)
		case int:
			data = append(data, float64(val))
		case int64:
			data = append(data, float64(val))
		default:
			return "", fmt.Errorf("invalid data type in array")
		}
	}

	// Calculate statistics
	n := len(data)

	// Sort for median
	sorted := make([]float64, n)
	copy(sorted, data)
	for i := 0; i < n-1; i++ {
		for j := i + 1; j < n; j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	// Mean
	var sum float64
	for _, v := range data {
		sum += v
	}
	mean := sum / float64(n)

	// Median
	var median float64
	if n%2 == 0 {
		median = (sorted[n/2-1] + sorted[n/2]) / 2
	} else {
		median = sorted[n/2]
	}

	// Standard deviation
	var variance float64
	for _, v := range data {
		variance += (v - mean) * (v - mean)
	}
	variance /= float64(n)
	std := math.Sqrt(variance)

	// Min, Max
	minVal := sorted[0]
	maxVal := sorted[n-1]

	result := fmt.Sprintf(`Data Analysis Results:

Summary Statistics:
- Count: %d
- Sum: %.4f
- Mean: %.4f
- Median: %.4f
- Min: %.4f
- Max: %.4f
- Range: %.4f
- Standard Deviation: %.4f
- Variance: %.4f

Data: %v`, n, sum, mean, median, minVal, maxVal, maxVal-minVal, std, variance, data)

	return result, nil
}

// GetInfo returns tool information for LLM
func (t *VisualizationTool) GetInfo() ToolInfo {
	return ToolInfo{
		Name:        "visualization",
		Description: "Generate visualization specifications for data. Returns chart configuration in JSON format.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"type": map[string]interface{}{
					"type":        "string",
					"description": "Type of chart: 'bar', 'line', 'pie', 'scatter'",
					"enum":        []string{"bar", "line", "pie", "scatter"},
				},
				"title": map[string]interface{}{
					"type":        "string",
					"description": "Chart title",
				},
				"labels": map[string]interface{}{
					"type":        "array",
					"description": "Labels for data points",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
				"data": map[string]interface{}{
					"type":        "array",
					"description": "Data values",
					"items": map[string]interface{}{
						"type": "number",
					},
				},
			},
			"required": []string{"type", "data"},
		},
	}
}

// VisualizationTool generates visualizations
type VisualizationTool struct{}

// NewVisualizationTool creates a new visualization tool
func NewVisualizationTool() *VisualizationTool {
	return &VisualizationTool{}
}

// Execute executes the visualization tool
func (t *VisualizationTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	return t.ExecuteWithConfig(ctx, args, nil)
}

// ExecuteWithConfig executes the visualization tool with config
func (t *VisualizationTool) ExecuteWithConfig(ctx context.Context, args map[string]interface{}, config map[string]interface{}) (string, error) {
	chartType, _ := args["type"].(string)
	title, _ := args["title"].(string)

	if chartType == "" {
		chartType = "bar"
	}

	dataRaw, ok := args["data"].([]interface{})
	if !ok || len(dataRaw) == 0 {
		return "", fmt.Errorf("data array is required")
	}

	// Convert data
	data := make([]float64, 0, len(dataRaw))
	for _, v := range dataRaw {
		switch val := v.(type) {
		case float64:
			data = append(data, val)
		case int:
			data = append(data, float64(val))
		}
	}

	// Extract labels
	labelsRaw, _ := args["labels"].([]interface{})
	labels := make([]string, 0, len(labelsRaw))
	for _, l := range labelsRaw {
		if s, ok := l.(string); ok {
			labels = append(labels, s)
		}
	}

	// Generate labels if not provided
	if len(labels) == 0 {
		for i := range data {
			labels = append(labels, fmt.Sprintf("Item %d", i+1))
		}
	}

	if title == "" {
		title = "Data Visualization"
	}

	// Return visualization specification
	result := fmt.Sprintf(`Visualization Specification:

{
  "type": "%s",
  "title": "%s",
  "data": {
    "labels": %v,
    "datasets": [{
      "data": %v,
      "backgroundColor": ["#FF6384", "#36A2EB", "#FFCE56", "#4BC0C0", "#9966FF"]
    }]
  },
  "options": {
    "responsive": true,
    "plugins": {
      "legend": {
        "position": "top"
      },
      "title": {
        "display": true,
        "text": "%s"
      }
    }
  }
}

Note: This is a chart.js compatible specification.
Use a frontend library to render this visualization.`, chartType, title, labels, data, title)

	return result, nil
}
