// Package browseragent provides browser automation agent
package browseragent

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
	ErrMaxStepsReached = errors.New("maximum steps reached")
	ErrTaskFailed      = errors.New("task failed")
)

// Result represents the execution result
type Result struct {
	Success     bool
	Answer      string
	Steps       int
	Duration    time.Duration
	Error       error
	StepHistory []StepRecord
}

// StepRecord records each execution step
type StepRecord struct {
	Step     int
	Action   *Action
	Result   string
	Error    error
	Duration time.Duration
}

// LLMClient interface for LLM calls
type LLMClient interface {
	Chat(ctx context.Context, systemPrompt string, userPrompt string) (string, error)
}

// BrowserClient interface for browser operations
type BrowserClient interface {
	Start(ctx context.Context) error
	Close()
	GetState(ctx context.Context) (*PageState, error)
	ExecuteAction(ctx context.Context, action *Action) (string, error)
}

// PageState represents current page state
type PageState struct {
	URL      string
	Title    string
	Elements []Element
	Text     string
}

// Element represents an interactive element
type Element struct {
	Index    int
	Tag      string
	Text     string
	Selector string
}

// ActionType represents the type of browser action
type ActionType string

const (
	ActionNavigate ActionType = "navigate"
	ActionClick    ActionType = "click"
	ActionInput    ActionType = "type"
	ActionScroll   ActionType = "scroll"
	ActionWait     ActionType = "wait"
	ActionDone     ActionType = "done"
)

// Action represents a browser action
type Action struct {
	Type      ActionType `json:"action"`
	Element   int        `json:"element,omitempty"`
	Text      string     `json:"text,omitempty"`
	URL       string     `json:"url,omitempty"`
	Direction string     `json:"direction,omitempty"`
	Seconds   int        `json:"seconds,omitempty"`
	Result    string     `json:"result,omitempty"`
}

// Agent is the browser automation agent
type Agent struct {
	browser        BrowserClient
	llm            LLMClient
	options        *Options
	cookies        []Cookie        // 预注入的 Cookie
	cookieStorage  CookieStorage   // Cookie 持久化存储
	userID         string          // 用户 ID (用于 Cookie 存储)
	tenantID       string          // 租户 ID (用于 Cookie 存储)
	autoSaveCookie bool            // 是否自动保存登录后的 Cookie
}

// Cookie represents a browser cookie
type Cookie struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Domain   string `json:"domain,omitempty"`
	Path     string `json:"path,omitempty"`
	Expires  int64  `json:"expires,omitempty"`
	HTTPOnly bool   `json:"httpOnly,omitempty"`
	Secure   bool   `json:"secure,omitempty"`
}

// Options holds agent configuration
type Options struct {
	MaxSteps   int
	MaxRetries int
	Debug      bool
}

// DefaultOptions returns default options
func DefaultOptions() *Options {
	return &Options{
		MaxSteps:   8,  // 减少最大步数，防止死循环
		MaxRetries: 2,  // 减少重试次数
		Debug:      false,
	}
}

// New creates a new browser agent
func New(llmClient LLMClient, browserClient BrowserClient, opts ...func(*Agent)) *Agent {
	options := DefaultOptions()

	agent := &Agent{
		llm:     llmClient,
		browser: browserClient,
		options: options,
	}

	for _, opt := range opts {
		opt(agent)
	}

	return agent
}

// WithCookies sets cookies to be injected before navigation
func WithCookies(cookies []Cookie) func(*Agent) {
	return func(a *Agent) {
		a.cookies = cookies
	}
}

// WithCookieStorage sets cookie storage for persistence
func WithCookieStorage(storage CookieStorage) func(*Agent) {
	return func(a *Agent) {
		a.cookieStorage = storage
	}
}

// WithUserContext sets user and tenant context for cookie storage
func WithUserContext(userID, tenantID string) func(*Agent) {
	return func(a *Agent) {
		a.userID = userID
		a.tenantID = tenantID
	}
}

// WithAutoSaveCookie enables auto-saving cookies after login
func WithAutoSaveCookie(enable bool) func(*Agent) {
	return func(a *Agent) {
		a.autoSaveCookie = enable
	}
}

// WithMaxSteps sets max steps (backward compatible)
func WithMaxSteps(steps int) func(*Agent) {
	return func(a *Agent) {
		a.options.MaxSteps = steps
	}
}

// WithMaxRetries sets max retries (backward compatible)
func WithMaxRetries(retries int) func(*Agent) {
	return func(a *Agent) {
		a.options.MaxRetries = retries
	}
}

// WithDebug sets debug mode (backward compatible)
func WithDebug(debug bool) func(*Agent) {
	return func(a *Agent) {
		a.options.Debug = debug
	}
}

// SetCookies sets cookies to be injected before navigation
func (a *Agent) SetCookies(cookies []Cookie) {
	a.cookies = cookies
	// 如果 browser 已经创建，也设置给 browser
	if br, ok := a.browser.(*Browser); ok {
		br.SetCookies(cookies)
	}
}

// SetCookieStorage sets cookie storage
func (a *Agent) SetCookieStorage(storage CookieStorage) {
	a.cookieStorage = storage
}

// SetUserContext sets user and tenant context
func (a *Agent) SetUserContext(userID, tenantID string) {
	a.userID = userID
	a.tenantID = tenantID
}

// LoadCookiesFromStorage loads cookies from storage for a domain
func (a *Agent) LoadCookiesFromStorage(ctx context.Context, domain string) error {
	if a.cookieStorage == nil || a.userID == "" || a.tenantID == "" {
		return nil
	}

	cookies, err := a.cookieStorage.Get(ctx, a.userID, a.tenantID, domain)
	if err != nil {
		return fmt.Errorf("load cookies: %w", err)
	}

	if len(cookies) > 0 {
		a.SetCookies(cookies)
	}

	return nil
}

// SaveCookiesToStorage saves current browser cookies to storage
func (a *Agent) SaveCookiesToStorage(ctx context.Context) error {
	if a.cookieStorage == nil || a.userID == "" || a.tenantID == "" {
		return nil
	}

	br, ok := a.browser.(*Browser)
	if !ok {
		return nil
	}

	cookies, err := br.ExtractCookies(ctx)
	if err != nil {
		return fmt.Errorf("extract cookies: %w", err)
	}

	if len(cookies) > 0 {
		// Group by domain
		domainCookies := make(map[string][]Cookie)
		for _, c := range cookies {
			domainCookies[c.Domain] = append(domainCookies[c.Domain], c)
		}

		// Save each domain's cookies
		for domain, domainCks := range domainCookies {
			if err := a.cookieStorage.Save(ctx, a.userID, a.tenantID, domainCks); err != nil {
				return fmt.Errorf("save cookies for %s: %w", domain, err)
			}
		}
	}

	return nil
}

// Run executes a task
func (a *Agent) Run(ctx context.Context, task string) (*Result, error) {
	return a.RunWithDomain(ctx, task, "")
}

// RunWithDomain executes a task with automatic cookie loading for a domain
func (a *Agent) RunWithDomain(ctx context.Context, task string, domain string) (*Result, error) {
	start := time.Now()

	if err := a.browser.Start(ctx); err != nil {
		return nil, fmt.Errorf("start browser: %w", err)
	}
	defer a.browser.Close()

	// 如果有 Cookie 存储，尝试从存储加载 Cookie
	if a.cookieStorage != nil && domain != "" && a.userID != "" && a.tenantID != "" {
		if err := a.LoadCookiesFromStorage(ctx, domain); err != nil {
			fmt.Printf("Warning: failed to load cookies: %v\n", err)
		}
	}

	// 如果有预设置的 Cookie，注入到浏览器
	if len(a.cookies) > 0 {
		if br, ok := a.browser.(*Browser); ok {
			br.SetCookies(a.cookies)
		}
	}

	var lastResult string
	var history []StepRecord

	for step := 0; step < a.options.MaxSteps; step++ {
		stepStart := time.Now()

		// Get page state
		state, err := a.browser.GetState(ctx)
		if err != nil {
			return nil, fmt.Errorf("get state: %w", err)
		}

		// Build prompt
		userPrompt := buildUserPrompt(task, state, lastResult)
		response, err := a.llm.Chat(ctx, SystemPrompt, userPrompt)
		if err != nil {
			return nil, fmt.Errorf("llm chat: %w", err)
		}

		// Parse action
		action := parseAction(response)
		if action == nil {
			lastResult = "无法解析动作"
			continue
		}

		// Check if done
		if action.Type == ActionDone {
			// 自动保存 Cookie
			if a.autoSaveCookie && a.cookieStorage != nil {
				if err := a.SaveCookiesToStorage(ctx); err != nil {
					fmt.Printf("Warning: failed to save cookies: %v\n", err)
				}
			}

			return &Result{
				Success:     true,
				Answer:      action.Result,
				Steps:       step + 1,
				Duration:    time.Since(start),
				StepHistory: history,
			}, nil
		}

		// Execute action
		result, err := a.browser.ExecuteAction(ctx, action)
		if err != nil {
			lastResult = fmt.Sprintf("执行失败: %v", err)
		} else {
			lastResult = result
		}

		// Record step
		history = append(history, StepRecord{
			Step:     step + 1,
			Action:   action,
			Result:   lastResult,
			Duration: time.Since(stepStart),
		})

		if a.options.Debug {
			fmt.Printf("[Step %d] %s -> %s\n", step+1, action.Type, lastResult)
		}
	}

	// 即使达到最大步数，也尝试保存 Cookie
	if a.autoSaveCookie && a.cookieStorage != nil {
		if err := a.SaveCookiesToStorage(ctx); err != nil {
			fmt.Printf("Warning: failed to save cookies: %v\n", err)
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

// SystemPrompt is the system prompt for LLM
const SystemPrompt = `你是一个浏览器自动化助手。你的任务是操控浏览器完成用户请求。

## 你可以执行的动作

1. navigate(url) - 导航到指定 URL
   示例: {"action": "navigate", "url": "https://www.baidu.com"}

2. click(element_index) - 点击元素
   示例: {"action": "click", "element": 5}

3. type(element_index, text) - 输入文本
   示例: {"action": "type", "element": 3, "text": "Golang 教程"}

4. scroll(direction) - 滚动页面 (up/down)
   示例: {"action": "scroll", "direction": "down"}

5. wait(seconds) - 等待（最多3秒）
   示例: {"action": "wait", "seconds": 2}

6. done(result) - 任务完成，必须调用！
   示例: {"action": "done", "result": "搜索完成"}

## 重要规则

1. 每次只执行一个动作，以 JSON 格式回复
2. 任务完成后，**立即调用 done**，不要继续操作
3. 如果找不到元素，**立即 done 返回当前状态**
4. 如果遇到错误，**立即 done 返回错误信息**
5. 不要重复执行相同操作
6. 最多执行 8 步，超过后必须返回结果`

func buildUserPrompt(task string, state *PageState, lastResult string) string {
	var sb strings.Builder

	sb.WriteString("## 用户任务\n\n")
	sb.WriteString(task)
	sb.WriteString("\n\n")

	sb.WriteString("## 当前页面\n\n")
	sb.WriteString(fmt.Sprintf("- URL: %s\n", state.URL))
	sb.WriteString(fmt.Sprintf("- 标题: %s\n\n", state.Title))

	sb.WriteString("## 可交互元素\n\n")
	if len(state.Elements) == 0 {
		sb.WriteString("无\n")
	} else {
		for _, el := range state.Elements {
			text := el.Text
			if len(text) > 50 {
				text = text[:50] + "..."
			}
			sb.WriteString(fmt.Sprintf("[%d] %s: %s\n", el.Index, el.Tag, text))
		}
	}

	if lastResult != "" {
		sb.WriteString("\n## 上一步结果\n\n")
		sb.WriteString(lastResult)
		sb.WriteString("\n")
	}

	sb.WriteString("\n请决定下一步动作，以 JSON 格式回复。\n")

	return sb.String()
}

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

	return &action
}