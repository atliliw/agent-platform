package computeruse

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Errors
var (
	// ErrMaxStepsReached is returned when the agent exhausts its step budget
	// without the VLM signalling "done".
	ErrMaxStepsReached = errors.New("computer-use: maximum steps reached")
)

// Result represents the outcome of a Computer Use task.
type Result struct {
	Success     bool
	Answer      string
	Steps       int
	Duration    time.Duration
	Error       error
	StepHistory []StepRecord
}

// StepRecord records one step of the loop.
type StepRecord struct {
	Step     int
	Action   *Action
	Result   string
	Error    error
	Duration time.Duration
}

// Options holds agent configuration.
type Options struct {
	MaxSteps int
	Debug    bool
}

// DefaultOptions returns the default options.
func DefaultOptions() *Options {
	return &Options{
		MaxSteps: 12,
		Debug:    false,
	}
}

// ActionGate optionally approves or vetoes an action before it is executed. This
// is the hook for human-in-the-loop approval of sensitive Computer Use actions
// (e.g. launching apps). A nil gate auto-approves everything. The gate receives
// the action and returns whether it is approved and a short reason; the
// implementation decides which actions are sensitive (it should auto-approve
// benign ones like click/type so the loop is not stalled on every step).
type ActionGate interface {
	Approve(ctx context.Context, action *Action) (approved bool, reason string, err error)
}

// Agent runs the Computer Use decision loop.
type Agent struct {
	desk    Desktop
	vlm     VLMClient
	gate    ActionGate
	options *Options
}

// New creates a Computer Use agent.
func New(vlm VLMClient, desk Desktop, opts ...func(*Agent)) *Agent {
	a := &Agent{
		vlm:     vlm,
		desk:    desk,
		options: DefaultOptions(),
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// WithMaxSteps sets the step budget.
func WithMaxSteps(steps int) func(*Agent) {
	return func(a *Agent) {
		if steps > 0 {
			a.options.MaxSteps = steps
		}
	}
}

// WithDebug enables per-step logging.
func WithDebug(debug bool) func(*Agent) {
	return func(a *Agent) {
		a.options.Debug = debug
	}
}

// WithGate installs an approval gate consulted before each action. Pass nil (or
// omit) to auto-approve all actions.
func WithGate(gate ActionGate) func(*Agent) {
	return func(a *Agent) {
		a.gate = gate
	}
}

// Run executes a task: screenshot -> VLM -> action -> repeat until "done" or
// MaxSteps.
func (a *Agent) Run(ctx context.Context, task string) (*Result, error) {
	start := time.Now()
	var lastResult string
	var history []StepRecord

	// Best-effort screen size; the VLM still sees the image if this fails.
	w, h, sizeErr := a.desk.ScreenSize(ctx)
	if sizeErr != nil && a.options.Debug {
		fmt.Printf("[cu] screen size unavailable: %v (continuing)\n", sizeErr)
	}

	for step := 0; step < a.options.MaxSteps; step++ {
		stepStart := time.Now()

		shot, err := a.desk.Screenshot(ctx)
		if err != nil {
			return nil, fmt.Errorf("screenshot: %w", err)
		}

		userPrompt := buildUserPrompt(task, w, h, lastResult)
		response, err := a.vlm.Chat(ctx, SystemPrompt, userPrompt, shot)
		if err != nil {
			return nil, fmt.Errorf("vlm chat: %w", err)
		}

		action := parseAction(response)
		if action == nil {
			lastResult = "无法解析动作，请只输出一个合法 JSON 动作"
			history = append(history, StepRecord{
				Step:     step + 1,
				Result:   lastResult,
				Duration: time.Since(stepStart),
			})
			if a.options.Debug {
				fmt.Printf("[cu step %d] unparseable response: %s\n", step+1, response)
			}
			continue
		}

		if action.Type == ActionDone {
			return &Result{
				Success:     true,
				Answer:      action.Result,
				Steps:       step + 1,
				Duration:    time.Since(start),
				StepHistory: history,
			}, nil
		}

		// Consult the approval gate before executing. A rejected action is
		// skipped and its reason fed back so the VLM can choose differently.
		if a.gate != nil {
			approved, reason, gerr := a.gate.Approve(ctx, action)
			if gerr != nil {
				return nil, fmt.Errorf("approval gate: %w", gerr)
			}
			if !approved {
				lastResult = fmt.Sprintf("动作被拒绝: %s", reason)
				history = append(history, StepRecord{
					Step:     step + 1,
					Action:   action,
					Result:   lastResult,
					Duration: time.Since(stepStart),
				})
				if a.options.Debug {
					fmt.Printf("[cu step %d] %s rejected: %s\n", step+1, action.Type, reason)
				}
				continue
			}
		}

		res, err := executeAction(ctx, a.desk, action)
		if err != nil {
			lastResult = fmt.Sprintf("执行失败: %v", err)
		} else {
			lastResult = res
		}

		history = append(history, StepRecord{
			Step:     step + 1,
			Action:   action,
			Result:   lastResult,
			Error:    err,
			Duration: time.Since(stepStart),
		})

		if a.options.Debug {
			fmt.Printf("[cu step %d] %s -> %s\n", step+1, action.Type, lastResult)
		}
	}

	return &Result{
		Success:     false,
		Error:       ErrMaxStepsReached,
		Steps:       a.options.MaxSteps,
		Duration:    time.Since(start),
		StepHistory: history,
	}, ErrMaxStepsReached
}

// SystemPrompt instructs the VLM how to act and the JSON schema to emit.
const SystemPrompt = `你是一个桌面控制助手。你的任务是观察屏幕截图，操控鼠标和键盘完成用户请求。

## 你可以执行的动作（每次只输出一个，以 JSON 格式回复）

1. click - 左键点击坐标 {"action": "click", "coordinate": [x, y]}
2. double_click - 双击坐标 {"action": "double_click", "coordinate": [x, y]}
3. right_click - 右键点击坐标 {"action": "right_click", "coordinate": [x, y]}
4. mouse_move - 移动鼠标到坐标 {"action": "mouse_move", "coordinate": [x, y]}
5. type - 在当前焦点输入文本 {"action": "type", "text": "要输入的内容"}
6. key - 按键组合（xdotool 语法） {"action": "key", "key": "Return"} / {"action": "key", "key": "ctrl+c"}
7. scroll - 滚动 {"action": "scroll", "coordinate": [x, y], "direction": "down", "clicks": 3}
8. launch_app - 启动应用 {"action": "launch_app", "app": "firefox"}
9. done - 任务完成，返回结果 {"action": "done", "result": "完成描述"}

## 重要规则

1. coordinate 用屏幕像素坐标，左上角为 [0,0]，不要超出屏幕尺寸
2. 每次只输出一个动作，只输出 JSON，不要多余文字
3. 先点击目标元素使其获得焦点，再 type 输入文本
4. 不要重复执行相同动作；如果上一步没有效果，换一种方式
5. 任务完成后立即调用 done 返回结果`

func buildUserPrompt(task string, screenW, screenH int, lastResult string) string {
	var sb strings.Builder
	sb.WriteString("## 用户任务\n\n")
	sb.WriteString(task)
	sb.WriteString("\n\n")

	if screenW > 0 && screenH > 0 {
		sb.WriteString("## 屏幕尺寸\n\n")
		sb.WriteString(fmt.Sprintf("%d x %d 像素，坐标原点 [0,0] 在左上角\n\n", screenW, screenH))
	}

	if lastResult != "" {
		sb.WriteString("## 上一步结果\n\n")
		sb.WriteString(lastResult)
		sb.WriteString("\n\n")
	}

	sb.WriteString("请观察截图，决定下一步动作，只输出一个 JSON。\n")
	return sb.String()
}

// parseAction extracts the first JSON object in the response and decodes it.
func parseAction(response string) *Action {
	start := strings.Index(response, "{")
	end := strings.LastIndex(response, "}")
	if start == -1 || end == -1 || end <= start {
		return nil
	}
	jsonStr := response[start : end+1]
	var action Action
	if err := json.Unmarshal([]byte(jsonStr), &action); err != nil {
		return nil
	}
	if action.Type == "" {
		return nil
	}
	return &action
}
