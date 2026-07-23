package tools

import (
	"context"
	"fmt"
	"time"

	"agent-platform/pkg/computeruse"
)

// ComputerUseTool runs a Computer Use task: a vision LLM observes desktop
// screenshots and drives the mouse/keyboard to complete a natural-language
// task. It is the desktop-level counterpart of browser_execute.
type ComputerUseTool struct {
	apiKey  string
	baseURL string
	model   string
	// Overridable for tests. Defaults create a real OpenAI-compatible VLM and
	// acquire a desktop from the global pool.
	vlmFactory     func(apiKey, baseURL, model string) computeruse.VLMClient
	desktopFactory func() (computeruse.Desktop, error)
}

// NewComputerUseTool creates a tool with empty VLM config (resolved at exec
// time from per-call config).
func NewComputerUseTool() *ComputerUseTool {
	return NewComputerUseToolWithConfig("", "", "")
}

// NewComputerUseToolWithConfig creates a tool with default VLM config. Pass
// model="" to let the VLM client default to qwen-vl-max.
func NewComputerUseToolWithConfig(apiKey, baseURL, model string) *ComputerUseTool {
	return &ComputerUseTool{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
		vlmFactory:     func(a, b, m string) computeruse.VLMClient { return computeruse.NewOpenAIVLMClient(a, b, m) },
		desktopFactory: func() (computeruse.Desktop, error) { return computeruse.AcquireDesktop() },
	}
}

// GetInfo returns tool metadata for the LLM.
func (t *ComputerUseTool) GetInfo() ToolInfo {
	return ToolInfo{
		Name:        "computer_use",
		Description: "控制整个桌面完成 GUI 任务。视觉模型观察屏幕截图，自动操作鼠标和键盘，适合操作桌面应用（计算器、文件管理器、编辑器等）。传入自然语言任务描述。",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"task": map[string]interface{}{
					"type":        "string",
					"description": "要完成的桌面任务，如：打开计算器计算 1+1",
				},
				"max_steps": map[string]interface{}{
					"type":        "integer",
					"description": "最大执行步数（默认 12）",
				},
			},
			"required": []string{"task"},
		},
	}
}

// Execute runs the tool with empty config.
func (t *ComputerUseTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	return t.ExecuteWithConfig(ctx, args, nil)
}

// ExecuteWithConfig runs the Computer Use agent loop.
func (t *ComputerUseTool) ExecuteWithConfig(ctx context.Context, args map[string]interface{}, config map[string]interface{}) (string, error) {
	task, _ := args["task"].(string)
	if task == "" {
		return "", fmt.Errorf("task is required")
	}

	maxSteps := 12
	if ms, ok := args["max_steps"].(float64); ok && ms > 0 {
		maxSteps = int(ms)
	}

	// Resolve VLM config: struct defaults overridden by per-call config.
	apiKey := t.apiKey
	baseURL := t.baseURL
	model := t.model
	if config != nil {
		if v, ok := config["api_key"].(string); ok && v != "" {
			apiKey = v
		}
		if v, ok := config["base_url"].(string); ok && v != "" {
			baseURL = v
		}
		if v, ok := config["model"].(string); ok && v != "" {
			model = v
		}
	}
	if apiKey == "" {
		return "", fmt.Errorf("LLM API Key not configured for computer_use tool")
	}

	vlm := t.vlmFactory(apiKey, baseURL, model)

	desk, err := t.desktopFactory()
	if err != nil {
		return "", fmt.Errorf("acquire desktop: %w", err)
	}
	defer desk.Close()

	agent := computeruse.New(vlm, desk, computeruse.WithMaxSteps(maxSteps))

	// Computer Use can be long-running; detach from the caller's deadline.
	execCtx, cancel := context.WithTimeout(context.Background(), 600*time.Second)
	defer cancel()

	result, err := agent.Run(execCtx, task)
	if result == nil {
		return "", err // hard error (screenshot/vlm failure)
	}
	if err != nil {
		// Step budget exhausted without "done" - report partial progress, not an error.
		return fmt.Sprintf("任务未在步数上限内完成（已执行 %d 步）。\n任务: %s\n耗时: %v", result.Steps, task, result.Duration), nil
	}
	return fmt.Sprintf("任务: %s\n\n结果: %s\n\n执行步数: %d\n耗时: %v", task, result.Answer, result.Steps, result.Duration), nil
}
