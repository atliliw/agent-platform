// Package reflection provides self-reflection capabilities for agents
package reflection

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"agent-platform/pkg/llm"
)

// ============================================================
// Reflection Types
// ============================================================

// ReflectionPhase defines phases of reflection
type ReflectionPhase string

const (
	PhasePreAction  ReflectionPhase = "pre_action"  // Before taking action
	PhasePostAction ReflectionPhase = "post_action" // After taking action
	PhaseComplete   ReflectionPhase = "complete"   // After task completion
	PhaseError      ReflectionPhase = "error"      // After error
)

// ReflectionType defines the type of reflection
type ReflectionType string

const (
	ReflectionSelf    ReflectionType = "self"    // Self-evaluation
	ReflectionCritique ReflectionType = "critique" // Critical analysis
	ReflectionImprove ReflectionType = "improve" // Improvement suggestions
	ReflectionLearn   ReflectionType = "learn"   // Learning extraction
)

// ============================================================
// Reflection Result
// ============================================================

// ReflectionResult represents the result of a reflection
type ReflectionResult struct {
	ID              string                 `json:"id"`
	SessionID       string                 `json:"session_id"`
	Phase           ReflectionPhase        `json:"phase"`
	Type            ReflectionType         `json:"type"`
	Score           float64                `json:"score"` // 0-1 self-evaluation score
	Strengths       []string               `json:"strengths"`
	Weaknesses      []string               `json:"weaknesses"`
	Suggestions     []string               `json:"suggestions"`
	LessonsLearned  []string               `json:"lessons_learned"`
	AlternativeActions []string            `json:"alternative_actions"`
	Confidence      float64                `json:"confidence"` // Confidence in reflection
	Analysis        string                 `json:"analysis"`
	Timestamp       time.Time              `json:"timestamp"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// ErrorAnalysis represents analysis of an error
type ErrorAnalysis struct {
	ID              string                 `json:"id"`
	SessionID       string                 `json:"session_id"`
	StepNum         int                    `json:"step_num"`
	ErrorMessage    string                 `json:"error_message"`
	ErrorType       string                 `json:"error_type"` // timeout, tool_error, llm_error, etc.
	RootCause       string                 `json:"root_cause"`
	ContributingFactors []string           `json:"contributing_factors"`
	RecoveryOptions []RecoveryOption       `json:"recovery_options"`
	PreventionTips  []string               `json:"prevention_tips"`
	Severity        string                 `json:"severity"` // low, medium, high, critical
	Timestamp       time.Time              `json:"timestamp"`
}

// RecoveryOption represents a recovery option
type RecoveryOption struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Probability string `json:"probability"` // high, medium, low
	Effort      string `json:"effort"`      // low, medium, high
	Recommended bool   `json:"recommended"`
}

// StrategyAdjustment represents a strategy adjustment
type StrategyAdjustment struct {
	ID              string                 `json:"id"`
	SessionID       string                 `json:"session_id"`
	Trigger         string                 `json:"trigger"` // What triggered the adjustment
	CurrentStrategy string                 `json:"current_strategy"`
	NewStrategy     string                 `json:"new_strategy"`
	Reason          string                 `json:"reason"`
	ExpectedImpact  string                 `json:"expected_impact"`
	Applied         bool                   `json:"applied"`
	AppliedAt       time.Time              `json:"applied_at,omitempty"`
	Result          string                 `json:"result,omitempty"`
	Timestamp       time.Time              `json:"timestamp"`
}

// ============================================================
// Reflection Loop
// ============================================================

// ReflectionLoop manages the reflection process
type ReflectionLoop struct {
	llmClient  llm.Client
	llmModel   string
	results    map[string]*ReflectionResult // id -> result
	bySession  map[string][]string          // sessionID -> result IDs
	adjustments map[string]*StrategyAdjustment
	mu         sync.RWMutex
	enabled    bool
}

// NewReflectionLoop creates a new reflection loop
func NewReflectionLoop() *ReflectionLoop {
	return &ReflectionLoop{
		results:     make(map[string]*ReflectionResult),
		bySession:   make(map[string][]string),
		adjustments: make(map[string]*StrategyAdjustment),
		enabled:     true,
	}
}

// NewReflectionLoopWithLLM creates a reflection loop with LLM
func NewReflectionLoopWithLLM(client llm.Client, model string) *ReflectionLoop {
	loop := NewReflectionLoop()
	loop.llmClient = client
	loop.llmModel = model
	return loop
}

// SetEnabled enables or disables reflection
func (l *ReflectionLoop) SetEnabled(enabled bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.enabled = enabled
}

// Reflect performs a reflection
func (l *ReflectionLoop) Reflect(ctx context.Context, sessionID string, phase ReflectionPhase, context_ ReflectionContext) (*ReflectionResult, error) {
	l.mu.RLock()
	enabled := l.enabled
	l.mu.RUnlock()

	if !enabled {
		return nil, nil
	}

	// Generate reflection
	result := &ReflectionResult{
		ID:        generateReflectionID(),
		SessionID: sessionID,
		Phase:     phase,
		Timestamp: time.Now(),
	}

	// Use LLM if available
	if l.llmClient != nil {
		llmResult, err := l.reflectWithLLM(ctx, phase, context_)
		if err == nil {
			result = llmResult
			result.ID = generateReflectionID()
			result.SessionID = sessionID
			result.Timestamp = time.Now()
		}
	} else {
		// Use rule-based reflection
		result = l.reflectWithRules(phase, context_)
	}

	// Store result
	l.mu.Lock()
	l.results[result.ID] = result
	l.bySession[sessionID] = append(l.bySession[sessionID], result.ID)
	l.mu.Unlock()

	return result, nil
}

// ReflectionContext provides context for reflection
type ReflectionContext struct {
	Task            string                 `json:"task"`
	Goal            string                 `json:"goal"`
	ActionsTaken    []ActionRecord         `json:"actions_taken"`
	CurrentState    string                 `json:"current_state"`
	ExpectedState   string                 `json:"expected_state"`
	Errors          []ErrorRecord          `json:"errors"`
	TokenUsage      int                    `json:"token_usage"`
	ElapsedTime     int64                  `json:"elapsed_time_ms"`
	Success         bool                   `json:"success"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// ActionRecord records an action
type ActionRecord struct {
	StepNum   int                    `json:"step_num"`
	Action    string                 `json:"action"`
	Input     map[string]interface{} `json:"input"`
	Output    interface{}            `json:"output"`
	Success   bool                   `json:"success"`
	Duration  int64                  `json:"duration_ms"`
}

// ErrorRecord records an error
type ErrorRecord struct {
	StepNum     int    `json:"step_num"`
	Error       string `json:"error"`
	ErrorType   string `json:"error_type"`
	RecoveryAttempt string `json:"recovery_attempt,omitempty"`
}

// reflectWithLLM uses LLM for reflection
func (l *ReflectionLoop) reflectWithLLM(ctx context.Context, phase ReflectionPhase, context_ ReflectionContext) (*ReflectionResult, error) {
	var prompt strings.Builder

	prompt.WriteString("你是一个 AI Agent 自我反思专家。请对以下执行过程进行深入反思和分析。\n\n")

	// Add context based on phase
	switch phase {
	case PhasePreAction:
		prompt.WriteString("## 反思阶段：行动前反思\n\n")
		prompt.WriteString("请分析即将采取的行动是否合理，是否有更好的替代方案。\n\n")
	case PhasePostAction:
		prompt.WriteString("## 反思阶段：行动后反思\n\n")
		prompt.WriteString("请分析刚才采取的行动效果如何，是否达到了预期。\n\n")
	case PhaseComplete:
		prompt.WriteString("## 反思阶段：任务完成反思\n\n")
		prompt.WriteString("请总结整个任务执行过程中的经验教训。\n\n")
	case PhaseError:
		prompt.WriteString("## 反思阶段：错误反思\n\n")
		prompt.WriteString("请分析错误原因，并提出改进建议。\n\n")
	}

	// Add task context
	prompt.WriteString("## 任务信息\n")
	prompt.WriteString(fmt.Sprintf("- 任务: %s\n", context_.Task))
	prompt.WriteString(fmt.Sprintf("- 目标: %s\n", context_.Goal))
	prompt.WriteString(fmt.Sprintf("- 成功: %v\n", context_.Success))

	// Add actions
	if len(context_.ActionsTaken) > 0 {
		prompt.WriteString("\n## 执行步骤\n")
		for _, action := range context_.ActionsTaken {
			prompt.WriteString(fmt.Sprintf("- 步骤%d: %s (成功: %v, 耗时: %dms)\n",
				action.StepNum, action.Action, action.Success, action.Duration))
		}
	}

	// Add errors
	if len(context_.Errors) > 0 {
		prompt.WriteString("\n## 错误记录\n")
		for _, err := range context_.Errors {
			prompt.WriteString(fmt.Sprintf("- 步骤%d: %s (类型: %s)\n", err.StepNum, err.Error, err.ErrorType))
		}
	}

	// Add metrics
	prompt.WriteString(fmt.Sprintf("\n## 执行指标\n"))
	prompt.WriteString(fmt.Sprintf("- Token使用: %d\n", context_.TokenUsage))
	prompt.WriteString(fmt.Sprintf("- 执行时间: %dms\n", context_.ElapsedTime))

	// Request structured output
	prompt.WriteString(`
请以 JSON 格式输出反思结果：
{
  "score": 0.85,
  "strengths": ["优点1", "优点2"],
  "weaknesses": ["缺点1", "缺点2"],
  "suggestions": ["建议1", "建议2"],
  "lessons_learned": ["经验1", "经验2"],
  "alternative_actions": ["替代方案1", "替代方案2"],
  "confidence": 0.9,
  "analysis": "详细分析..."
}
`)

	resp, err := l.llmClient.Chat(ctx, &llm.ChatRequest{
		Model:    l.llmModel,
		Messages: []llm.Message{{Role: "user", Content: prompt.String()}},
	})
	if err != nil {
		return nil, fmt.Errorf("LLM reflection failed: %w", err)
	}

	// Parse response
	result := &ReflectionResult{
		Phase:     phase,
		Timestamp: time.Now(),
	}

	jsonStr := extractJSON(resp.Content)
	if jsonStr != "" {
		var llmResult struct {
			Score            float64  `json:"score"`
			Strengths        []string `json:"strengths"`
			Weaknesses       []string `json:"weaknesses"`
			Suggestions      []string `json:"suggestions"`
			LessonsLearned   []string `json:"lessons_learned"`
			AlternativeActions []string `json:"alternative_actions"`
			Confidence       float64  `json:"confidence"`
			Analysis         string   `json:"analysis"`
		}

		if err := parseJSON(jsonStr, &llmResult); err == nil {
			result.Score = llmResult.Score
			result.Strengths = llmResult.Strengths
			result.Weaknesses = llmResult.Weaknesses
			result.Suggestions = llmResult.Suggestions
			result.LessonsLearned = llmResult.LessonsLearned
			result.AlternativeActions = llmResult.AlternativeActions
			result.Confidence = llmResult.Confidence
			result.Analysis = llmResult.Analysis
		}
	}

	return result, nil
}

// reflectWithRules uses rules for reflection
func (l *ReflectionLoop) reflectWithRules(phase ReflectionPhase, context_ ReflectionContext) *ReflectionResult {
	result := &ReflectionResult{
		Phase:     phase,
		Timestamp: time.Now(),
	}

	// Basic rule-based analysis
	successRate := calculateSuccessRate(context_.ActionsTaken)
	result.Score = successRate

	// Identify strengths
	if successRate >= 0.8 {
		result.Strengths = append(result.Strengths, "执行效率高")
	}
	if len(context_.Errors) == 0 {
		result.Strengths = append(result.Strengths, "无错误执行")
	}

	// Identify weaknesses
	if successRate < 0.5 {
		result.Weaknesses = append(result.Weaknesses, "执行成功率低")
	}
	if len(context_.Errors) > 0 {
		result.Weaknesses = append(result.Weaknesses, "存在执行错误")
	}

	// Generate suggestions
	if successRate < 0.8 {
		result.Suggestions = append(result.Suggestions, "考虑优化执行策略")
	}
	if context_.TokenUsage > 10000 {
		result.Suggestions = append(result.Suggestions, "考虑减少Token使用")
	}

	result.Confidence = 0.7
	result.Analysis = fmt.Sprintf("基于规则的反思分析。成功率: %.2f%%", successRate*100)

	return result
}

// ============================================================
// Error Analyzer
// ============================================================

// ErrorAnalyzer analyzes errors
type ErrorAnalyzer struct {
	llmClient   llm.Client
	llmModel    string
	analyses    map[string]*ErrorAnalysis
	errorTypes  map[string]int // error type -> count
	mu          sync.RWMutex
}

// NewErrorAnalyzer creates a new error analyzer
func NewErrorAnalyzer() *ErrorAnalyzer {
	return &ErrorAnalyzer{
		analyses:   make(map[string]*ErrorAnalysis),
		errorTypes: make(map[string]int),
	}
}

// NewErrorAnalyzerWithLLM creates an error analyzer with LLM
func NewErrorAnalyzerWithLLM(client llm.Client, model string) *ErrorAnalyzer {
	a := NewErrorAnalyzer()
	a.llmClient = client
	a.llmModel = model
	return a
}

// Analyze analyzes an error
func (a *ErrorAnalyzer) Analyze(ctx context.Context, sessionID string, stepNum int, errorMessage, errorType string) (*ErrorAnalysis, error) {
	analysis := &ErrorAnalysis{
		ID:           generateAnalysisID(),
		SessionID:    sessionID,
		StepNum:      stepNum,
		ErrorMessage: errorMessage,
		ErrorType:    errorType,
		Timestamp:    time.Now(),
	}

	// Determine severity
	analysis.Severity = a.determineSeverity(errorType, errorMessage)

	// Use LLM if available
	if a.llmClient != nil {
		llmAnalysis, err := a.analyzeWithLLM(ctx, errorMessage, errorType)
		if err == nil {
			analysis.RootCause = llmAnalysis.RootCause
			analysis.ContributingFactors = llmAnalysis.ContributingFactors
			analysis.RecoveryOptions = llmAnalysis.RecoveryOptions
			analysis.PreventionTips = llmAnalysis.PreventionTips
		}
	} else {
		// Use rule-based analysis
		analysis.RootCause = a.inferRootCause(errorType, errorMessage)
		analysis.RecoveryOptions = a.suggestRecoveryOptions(errorType)
		analysis.PreventionTips = a.suggestPreventionTips(errorType)
	}

	// Store analysis
	a.mu.Lock()
	a.analyses[analysis.ID] = analysis
	a.errorTypes[errorType]++
	a.mu.Unlock()

	return analysis, nil
}

// analyzeWithLLM uses LLM for error analysis
func (a *ErrorAnalyzer) analyzeWithLLM(ctx context.Context, errorMessage, errorType string) (*ErrorAnalysis, error) {
	prompt := fmt.Sprintf(`你是一个错误分析专家。请分析以下错误：

## 错误信息
%s

## 错误类型
%s

请分析：
1. 根本原因
2. 贡献因素
3. 恢复选项
4. 预防建议

以 JSON 格式输出：
{
  "root_cause": "根本原因分析",
  "contributing_factors": ["因素1", "因素2"],
  "recovery_options": [
    {"description": "选项1", "probability": "high", "effort": "low", "recommended": true}
  ],
  "prevention_tips": ["建议1", "建议2"]
}`, errorMessage, errorType)

	resp, err := a.llmClient.Chat(ctx, &llm.ChatRequest{
		Model:    a.llmModel,
		Messages: []llm.Message{{Role: "user", Content: prompt}},
	})
	if err != nil {
		return nil, err
	}

	analysis := &ErrorAnalysis{}
	jsonStr := extractJSON(resp.Content)
	if jsonStr != "" {
		parseJSON(jsonStr, analysis)
	}

	return analysis, nil
}

// determineSeverity determines error severity
func (a *ErrorAnalyzer) determineSeverity(errorType, errorMessage string) string {
	criticalKeywords := []string{"fatal", "critical", "crash", "data loss"}
	highKeywords := []string{"timeout", "connection refused", "unauthorized"}
	mediumKeywords := []string{"retry", "temporary", "rate limit"}

	errorLower := strings.ToLower(errorMessage)

	for _, kw := range criticalKeywords {
		if strings.Contains(errorLower, kw) {
			return "critical"
		}
	}
	for _, kw := range highKeywords {
		if strings.Contains(errorLower, kw) {
			return "high"
		}
	}
	for _, kw := range mediumKeywords {
		if strings.Contains(errorLower, kw) {
			return "medium"
		}
	}

	return "low"
}

// inferRootCause infers root cause from error type
func (a *ErrorAnalyzer) inferRootCause(errorType, errorMessage string) string {
	causes := map[string]string{
		"timeout":         "请求超时，可能是网络延迟或服务响应慢",
		"connection":      "网络连接问题，可能是服务不可用",
		"auth":            "认证失败，可能是凭证过期或无效",
		"rate_limit":      "请求频率超限，需要降低请求速率",
		"tool_error":      "工具执行错误，可能是参数不正确或环境问题",
		"llm_error":       "LLM 调用失败，可能是模型服务问题",
	}

	if cause, ok := causes[errorType]; ok {
		return cause
	}
	return "未知原因，需要进一步分析"
}

// suggestRecoveryOptions suggests recovery options
func (a *ErrorAnalyzer) suggestRecoveryOptions(errorType string) []RecoveryOption {
	options := map[string][]RecoveryOption{
		"timeout": {
			{Description: "重试请求", Probability: "medium", Effort: "low", Recommended: true},
			{Description: "增加超时时间", Probability: "high", Effort: "low", Recommended: true},
		},
		"connection": {
			{Description: "检查网络连接", Probability: "high", Effort: "low", Recommended: true},
			{Description: "使用备用端点", Probability: "medium", Effort: "medium", Recommended: false},
		},
		"rate_limit": {
			{Description: "等待后重试", Probability: "high", Effort: "low", Recommended: true},
			{Description: "降低请求频率", Probability: "high", Effort: "medium", Recommended: true},
		},
	}

	return options[errorType]
}

// suggestPreventionTips suggests prevention tips
func (a *ErrorAnalyzer) suggestPreventionTips(errorType string) []string {
	tips := map[string][]string{
		"timeout": {
			"设置合理的超时时间",
			"实现超时重试机制",
		},
		"connection": {
			"实现连接池",
			"添加健康检查",
		},
		"rate_limit": {
			"实现请求速率限制",
			"使用指数退避重试",
		},
	}

	return tips[errorType]
}

// GetErrorStatistics returns error statistics
func (a *ErrorAnalyzer) GetErrorStatistics() map[string]interface{} {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return map[string]interface{}{
		"total_analyses": len(a.analyses),
		"by_type":        a.errorTypes,
	}
}

// ============================================================
// Strategy Adjuster
// ============================================================

// StrategyAdjuster adjusts agent strategies based on reflection
type StrategyAdjuster struct {
	adjustments map[string]*StrategyAdjustment
	rules       []AdjustmentRule
	llmClient   llm.Client
	llmModel    string
	mu          sync.RWMutex
}

// AdjustmentRule defines a strategy adjustment rule
type AdjustmentRule struct {
	ID              string `json:"id"`
	Trigger         string `json:"trigger"` // Condition to trigger adjustment
	CurrentStrategy string `json:"current_strategy"`
	NewStrategy     string `json:"new_strategy"`
	Priority        int    `json:"priority"`
	Enabled         bool   `json:"enabled"`
}

// NewStrategyAdjuster creates a new strategy adjuster
func NewStrategyAdjuster() *StrategyAdjuster {
	return &StrategyAdjuster{
		adjustments: make(map[string]*StrategyAdjustment),
		rules:       make([]AdjustmentRule, 0),
	}
}

// AddRule adds an adjustment rule
func (s *StrategyAdjuster) AddRule(rule AdjustmentRule) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if rule.ID == "" {
		rule.ID = generateRuleID()
	}
	rule.Enabled = true
	s.rules = append(s.rules, rule)
}

// Evaluate evaluates if adjustment is needed
func (s *StrategyAdjuster) Evaluate(ctx context.Context, sessionID string, reflection *ReflectionResult) (*StrategyAdjustment, error) {
	s.mu.RLock()
	rules := s.rules
	s.mu.RUnlock()

	// Check rules
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		if s.matchesTrigger(rule.Trigger, reflection) {
			adjustment := &StrategyAdjustment{
				ID:              generateAdjustmentID(),
				SessionID:       sessionID,
				Trigger:         rule.Trigger,
				CurrentStrategy: rule.CurrentStrategy,
				NewStrategy:     rule.NewStrategy,
				Reason:          fmt.Sprintf("Reflection score: %.2f", reflection.Score),
				ExpectedImpact:  "Improve execution quality",
				Timestamp:       time.Now(),
			}

			s.mu.Lock()
			s.adjustments[adjustment.ID] = adjustment
			s.mu.Unlock()

			return adjustment, nil
		}
	}

	return nil, nil
}

// matchesTrigger checks if reflection matches a trigger
func (s *StrategyAdjuster) matchesTrigger(trigger string, reflection *ReflectionResult) bool {
	// Simple trigger matching
	switch trigger {
	case "low_score":
		return reflection.Score < 0.5
	case "high_errors":
		return len(reflection.Weaknesses) > 2
	case "needs_improvement":
		return len(reflection.Suggestions) > 0
	default:
		return false
	}
}

// Apply applies a strategy adjustment
func (s *StrategyAdjuster) Apply(adjustmentID string, result string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	adjustment, ok := s.adjustments[adjustmentID]
	if !ok {
		return fmt.Errorf("adjustment not found: %s", adjustmentID)
	}

	adjustment.Applied = true
	adjustment.AppliedAt = time.Now()
	adjustment.Result = result

	return nil
}

// GetAdjustments gets all adjustments for a session
func (s *StrategyAdjuster) GetAdjustments(sessionID string) []*StrategyAdjustment {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*StrategyAdjustment
	for _, adj := range s.adjustments {
		if adj.SessionID == sessionID {
			result = append(result, adj)
		}
	}
	return result
}

// ============================================================
// Helper Functions
// ============================================================

func generateReflectionID() string {
	return fmt.Sprintf("reflect-%d", time.Now().UnixNano())
}

func generateAnalysisID() string {
	return fmt.Sprintf("analysis-%d", time.Now().UnixNano())
}

func generateAdjustmentID() string {
	return fmt.Sprintf("adjust-%d", time.Now().UnixNano())
}

func generateRuleID() string {
	return fmt.Sprintf("rule-%d", time.Now().UnixNano())
}

func calculateSuccessRate(actions []ActionRecord) float64 {
	if len(actions) == 0 {
		return 1.0
	}
	success := 0
	for _, a := range actions {
		if a.Success {
			success++
		}
	}
	return float64(success) / float64(len(actions))
}

func extractJSON(content string) string {
	re := regexp.MustCompile(`\{[\s\S]*?\}`)
	return re.FindString(content)
}

func parseJSON(jsonStr string, target interface{}) error {
	return json.Unmarshal([]byte(jsonStr), target)
}