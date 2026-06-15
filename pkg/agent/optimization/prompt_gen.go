// Package optimization provides prompt optimization and evolution capabilities
package optimization

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
// Prompt Types
// ============================================================

// PromptType defines the type of prompt
type PromptType string

const (
	PromptTypeSystem    PromptType = "system"
	PromptTypeTask      PromptType = "task"
	PromptTypeTool      PromptType = "tool"
	PromptTypeReasoning PromptType = "reasoning"
	PromptTypeOutput    PromptType = "output"
)

// PromptVariant represents a variant of a prompt
type PromptVariant struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Type         PromptType             `json:"type"`
	Template     string                 `json:"template"`
	Variables    []string               `json:"variables"`    // Variables in template
	Version      string                 `json:"version"`
	ParentID     string                 `json:"parent_id"`    // Parent variant ID
	CreatedAt    time.Time              `json:"created_at"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// PromptExecution represents an execution of a prompt
type PromptExecution struct {
	ID           string                 `json:"id"`
	VariantID    string                 `json:"variant_id"`
	SessionID    string                 `json:"session_id"`
	Input        map[string]interface{} `json:"input"`        // Variable values
	Rendered     string                 `json:"rendered"`     // Rendered prompt
	Output       string                 `json:"output"`       // Model output
	Success      bool                   `json:"success"`
	Score        float64                `json:"score"`        // Quality score
	TokensUsed   int                    `json:"tokens_used"`
	LatencyMs    int64                  `json:"latency_ms"`
	CreatedAt    time.Time              `json:"created_at"`
}

// PromptScore represents scoring for a prompt variant
type PromptScore struct {
	VariantID      string  `json:"variant_id"`
	TotalExecutions int    `json:"total_executions"`
	SuccessRate    float64 `json:"success_rate"`
	AvgScore       float64 `json:"avg_score"`
	AvgLatency     float64 `json:"avg_latency_ms"`
	AvgTokens      float64 `json:"avg_tokens"`
	LastExecuted   time.Time `json:"last_executed"`
}

// ============================================================
// Prompt Manager
// ============================================================

// PromptManager manages prompt variants and optimization
type PromptManager struct {
	variants    map[string]*PromptVariant    // ID -> Variant
	byType      map[PromptType][]string      // Type -> Variant IDs
	executions  map[string][]string          // VariantID -> Execution IDs
	scores      map[string]*PromptScore      // VariantID -> Score
	llmClient   llm.Client
	llmModel    string
	mu          sync.RWMutex
}

// NewPromptManager creates a new prompt manager
func NewPromptManager() *PromptManager {
	return &PromptManager{
		variants:   make(map[string]*PromptVariant),
		byType:     make(map[PromptType][]string),
		executions: make(map[string][]string),
		scores:     make(map[string]*PromptScore),
	}
}

// SetLLM sets LLM client
func (m *PromptManager) SetLLM(client llm.Client, model string) {
	m.llmClient = client
	m.llmModel = model
}

// RegisterVariant registers a prompt variant
func (m *PromptManager) RegisterVariant(variant *PromptVariant) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if variant.ID == "" {
		variant.ID = generateVariantID()
	}
	if variant.CreatedAt.IsZero() {
		variant.CreatedAt = time.Now()
	}

	m.variants[variant.ID] = variant
	m.byType[variant.Type] = append(m.byType[variant.Type], variant.ID)
	m.scores[variant.ID] = &PromptScore{VariantID: variant.ID}

	return nil
}

// GetVariant retrieves a variant by ID
func (m *PromptManager) GetVariant(id string) (*PromptVariant, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	variant, ok := m.variants[id]
	if !ok {
		return nil, fmt.Errorf("variant not found: %s", id)
	}
	return variant, nil
}

// GetVariantsByType retrieves variants by type
func (m *PromptManager) GetVariantsByType(promptType PromptType) []*PromptVariant {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var variants []*PromptVariant
	for _, id := range m.byType[promptType] {
		if v, ok := m.variants[id]; ok {
			variants = append(variants, v)
		}
	}
	return variants
}

// Render renders a prompt template with variables
func (m *PromptManager) Render(variantID string, variables map[string]interface{}) (string, error) {
	m.mu.RLock()
	variant, ok := m.variants[variantID]
	m.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("variant not found: %s", variantID)
	}

	return renderTemplate(variant.Template, variables)
}

// RecordExecution records an execution
func (m *PromptManager) RecordExecution(execution *PromptExecution) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if execution.ID == "" {
		execution.ID = generateExecutionID()
	}
	execution.CreatedAt = time.Now()

	m.executions[execution.VariantID] = append(m.executions[execution.VariantID], execution.ID)

	// Update score
	score := m.scores[execution.VariantID]
	if score != nil {
		score.TotalExecutions++
		if execution.Success {
			newSuccessRate := (score.SuccessRate*float64(score.TotalExecutions-1) + 1.0) / float64(score.TotalExecutions)
			score.SuccessRate = newSuccessRate
		} else {
			newSuccessRate := (score.SuccessRate * float64(score.TotalExecutions-1)) / float64(score.TotalExecutions)
			score.SuccessRate = newSuccessRate
		}

		// Update averages
		oldAvg := score.AvgScore
		score.AvgScore = (oldAvg*float64(score.TotalExecutions-1) + execution.Score) / float64(score.TotalExecutions)
		score.AvgLatency = (score.AvgLatency*float64(score.TotalExecutions-1) + float64(execution.LatencyMs)) / float64(score.TotalExecutions)
		score.AvgTokens = (score.AvgTokens*float64(score.TotalExecutions-1) + float64(execution.TokensUsed)) / float64(score.TotalExecutions)
		score.LastExecuted = execution.CreatedAt
	}

	return nil
}

// GetScore retrieves score for a variant
func (m *PromptManager) GetScore(variantID string) (*PromptScore, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	score, ok := m.scores[variantID]
	if !ok {
		return nil, fmt.Errorf("score not found for variant: %s", variantID)
	}
	return score, nil
}

// GetTopVariants retrieves top performing variants
func (m *PromptManager) GetTopVariants(promptType PromptType, limit int) []*PromptScore {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var scores []*PromptScore
	for _, id := range m.byType[promptType] {
		if score, ok := m.scores[id]; ok && score.TotalExecutions > 0 {
			scores = append(scores, score)
		}
	}

	// Sort by avg score
	for i := 0; i < len(scores)-1; i++ {
		for j := i + 1; j < len(scores); j++ {
			if scores[j].AvgScore > scores[i].AvgScore {
				scores[i], scores[j] = scores[j], scores[i]
			}
		}
	}

	if limit > 0 && len(scores) > limit {
		scores = scores[:limit]
	}

	return scores
}

// ============================================================
// DSPy-style Prompt Optimization
// ============================================================

// OptimizationConfig configures prompt optimization
type OptimizationConfig struct {
	MaxIterations   int     `json:"max_iterations"`
	TargetScore     float64 `json:"target_score"`
	MinExecutions   int     `json:"min_executions"`   // Minimum executions before optimizing
	Temperature     float64 `json:"temperature"`     // LLM temperature
	Model           string  `json:"model"`
}

// DefaultOptimizationConfig returns default config
func DefaultOptimizationConfig() *OptimizationConfig {
	return &OptimizationConfig{
		MaxIterations:  5,
		TargetScore:    0.9,
		MinExecutions:  10,
		Temperature:    0.7,
	}
}

// PromptOptimizer optimizes prompts using DSPy-style approach
type PromptOptimizer struct {
	manager *PromptManager
	config  *OptimizationConfig
	history map[string][]OptimizationResult // VariantID -> Results
	mu      sync.RWMutex
}

// OptimizationResult represents an optimization result
type OptimizationResult struct {
	ID             string    `json:"id"`
	ParentID       string    `json:"parent_id"`
	NewVariantID   string    `json:"new_variant_id"`
	Iteration      int       `json:"iteration"`
	ParentScore    float64   `json:"parent_score"`
	NewScore       float64   `json:"new_score"`
	Improvement    float64   `json:"improvement"`
	Changes        []string  `json:"changes"`
	Suggestions    []string  `json:"suggestions"`
	Timestamp      time.Time `json:"timestamp"`
}

// NewPromptOptimizer creates a new prompt optimizer
func NewPromptOptimizer(manager *PromptManager, config *OptimizationConfig) *PromptOptimizer {
	if config == nil {
		config = DefaultOptimizationConfig()
	}
	return &PromptOptimizer{
		manager: manager,
		config:  config,
		history: make(map[string][]OptimizationResult),
	}
}

// Optimize optimizes a prompt variant
func (o *PromptOptimizer) Optimize(ctx context.Context, variantID string, feedback string) (*PromptVariant, error) {
	// Get current variant
	variant, err := o.manager.GetVariant(variantID)
	if err != nil {
		return nil, err
	}

	// Get current score
	score, err := o.manager.GetScore(variantID)
	if err != nil {
		return nil, err
	}

	// Check if optimization needed
	if score.TotalExecutions < o.config.MinExecutions {
		return nil, fmt.Errorf("not enough executions: %d < %d", score.TotalExecutions, o.config.MinExecutions)
	}

	if score.AvgScore >= o.config.TargetScore {
		return nil, fmt.Errorf("target score already achieved: %.2f >= %.2f", score.AvgScore, o.config.TargetScore)
	}

	// Generate optimized prompt
	optimized, err := o.generateOptimizedPrompt(ctx, variant, score, feedback)
	if err != nil {
		return nil, fmt.Errorf("generate optimized prompt: %w", err)
	}

	// Register new variant
	newVariant := &PromptVariant{
		ID:        "",
		Name:      variant.Name + "_optimized",
		Type:      variant.Type,
		Template:  optimized,
		Variables: variant.Variables,
		Version:   incrementVersion(variant.Version),
		ParentID:  variantID,
	}

	if err := o.manager.RegisterVariant(newVariant); err != nil {
		return nil, err
	}

	// Record optimization result
	result := OptimizationResult{
		ID:           fmt.Sprintf("opt-%d", time.Now().UnixNano()),
		ParentID:     variantID,
		NewVariantID: newVariant.ID,
		ParentScore:  score.AvgScore,
		Timestamp:    time.Now(),
	}

	o.mu.Lock()
	o.history[variantID] = append(o.history[variantID], result)
	o.mu.Unlock()

	return newVariant, nil
}

// generateOptimizedPrompt generates an optimized prompt
func (o *PromptOptimizer) generateOptimizedPrompt(ctx context.Context, variant *PromptVariant, score *PromptScore, feedback string) (string, error) {
	if o.manager.llmClient == nil {
		return o.optimizeWithRules(variant, score), nil
	}

	// Use LLM for optimization
	prompt := fmt.Sprintf(`你是一个 Prompt 优化专家。请优化以下 Prompt。

## 当前 Prompt
%s

## 性能指标
- 执行次数: %d
- 成功率: %.2f%%
- 平均得分: %.2f
- 平均延迟: %.0fms

## 用户反馈
%s

## 优化建议
1. 分析当前 Prompt 的不足之处
2. 提供优化后的 Prompt
3. 说明优化的原因

请直接输出优化后的 Prompt，不要解释。`, variant.Template, score.TotalExecutions, score.SuccessRate*100, score.AvgScore, score.AvgLatency, feedback)

	resp, err := o.manager.llmClient.Chat(ctx, &llm.ChatRequest{
		Model:    o.manager.llmModel,
		Messages: []llm.Message{{Role: "user", Content: prompt}},
	})
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(resp.Content), nil
}

// optimizeWithRules applies rule-based optimization
func (o *PromptOptimizer) optimizeWithRules(variant *PromptVariant, score *PromptScore) string {
	template := variant.Template

	// Add clarity improvements
	if strings.Count(template, "\n") < 3 {
		template = addStructure(template)
	}

	// Add explicit instructions if missing
	if !strings.Contains(strings.ToLower(template), "步骤") && !strings.Contains(strings.ToLower(template), "step") {
		template = addStepGuidance(template)
	}

	// Add output format guidance if missing
	if !strings.Contains(strings.ToLower(template), "输出") && !strings.Contains(strings.ToLower(template), "output") {
		template = addOutputGuidance(template)
	}

	return template
}

// ============================================================
// A/B Testing for Prompts
// ============================================================

// ABTest represents an A/B test for prompts
type ABTest struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	VariantAID   string    `json:"variant_a_id"`
	VariantBID   string    `json:"variant_b_id"`
	TrafficSplit float64   `json:"traffic_split"` // 0.0-1.0, proportion for variant B
	Status       string    `json:"status"`        // "running", "completed", "paused"
	CreatedAt    time.Time `json:"created_at"`
	CompletedAt  time.Time `json:"completed_at,omitempty"`
	WinnerID     string    `json:"winner_id,omitempty"`
}

// ABTestResult represents A/B test results
type ABTestResult struct {
	TestID        string        `json:"test_id"`
	VariantA      VariantResult `json:"variant_a"`
	VariantB      VariantResult `json:"variant_b"`
	WinnerID      string        `json:"winner_id"`
	Confidence    float64       `json:"confidence"`    // Statistical confidence
	Improvement   float64       `json:"improvement"`   // Improvement percentage
	Recommendation string       `json:"recommendation"`
}

// VariantResult represents results for a single variant
type VariantResult struct {
	VariantID      string  `json:"variant_id"`
	Executions     int     `json:"executions"`
	SuccessRate    float64 `json:"success_rate"`
	AvgScore       float64 `json:"avg_score"`
	AvgLatency     float64 `json:"avg_latency_ms"`
	AvgTokens      float64 `json:"avg_tokens"`
}

// ABTestManager manages A/B tests for prompts
type ABTestManager struct {
	tests     map[string]*ABTest
	results   map[string]*ABTestResult
	manager   *PromptManager
	mu        sync.RWMutex
}

// NewABTestManager creates a new A/B test manager
func NewABTestManager(manager *PromptManager) *ABTestManager {
	return &ABTestManager{
		tests:   make(map[string]*ABTest),
		results: make(map[string]*ABTestResult),
		manager: manager,
	}
}

// CreateTest creates a new A/B test
func (m *ABTestManager) CreateTest(test *ABTest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if test.ID == "" {
		test.ID = generateTestID()
	}
	test.Status = "running"
	test.CreatedAt = time.Now()

	m.tests[test.ID] = test
	return nil
}

// AssignVariant assigns a variant for a request
func (m *ABTestManager) AssignVariant(testID string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	test, ok := m.tests[testID]
	if !ok {
		return "", fmt.Errorf("test not found: %s", testID)
	}

	if test.Status != "running" {
		return "", fmt.Errorf("test not running: %s", test.Status)
	}

	// Simple random assignment based on traffic split
	// In production, would use consistent hashing or other method
	if test.TrafficSplit >= 0.5 {
		return test.VariantBID, nil
	}
	return test.VariantAID, nil
}

// GetTest retrieves a test by ID
func (m *ABTestManager) GetTest(testID string) (*ABTest, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	test, ok := m.tests[testID]
	if !ok {
		return nil, fmt.Errorf("test not found: %s", testID)
	}
	return test, nil
}

// CompleteTest completes an A/B test
func (m *ABTestManager) CompleteTest(testID string) (*ABTestResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	test, ok := m.tests[testID]
	if !ok {
		return nil, fmt.Errorf("test not found: %s", testID)
	}

	// Get scores for both variants
	scoreA, err := m.manager.GetScore(test.VariantAID)
	if err != nil {
		return nil, fmt.Errorf("get score A: %w", err)
	}

	scoreB, err := m.manager.GetScore(test.VariantBID)
	if err != nil {
		return nil, fmt.Errorf("get score B: %w", err)
	}

	// Build result
	result := &ABTestResult{
		TestID: testID,
		VariantA: VariantResult{
			VariantID:   test.VariantAID,
			Executions:  scoreA.TotalExecutions,
			SuccessRate: scoreA.SuccessRate,
			AvgScore:    scoreA.AvgScore,
			AvgLatency:  scoreA.AvgLatency,
			AvgTokens:   scoreA.AvgTokens,
		},
		VariantB: VariantResult{
			VariantID:   test.VariantBID,
			Executions:  scoreB.TotalExecutions,
			SuccessRate: scoreB.SuccessRate,
			AvgScore:    scoreB.AvgScore,
			AvgLatency:  scoreB.AvgLatency,
			AvgTokens:   scoreB.AvgTokens,
		},
	}

	// Determine winner
	if scoreB.AvgScore > scoreA.AvgScore {
		result.WinnerID = test.VariantBID
		result.Improvement = (scoreB.AvgScore - scoreA.AvgScore) / scoreA.AvgScore * 100
	} else {
		result.WinnerID = test.VariantAID
		result.Improvement = (scoreA.AvgScore - scoreB.AvgScore) / scoreB.AvgScore * 100
	}

	// Calculate confidence (simplified)
	result.Confidence = m.calculateConfidence(scoreA, scoreB)

	// Build recommendation
	if result.WinnerID == test.VariantBID {
		result.Recommendation = fmt.Sprintf("Variant B performs %.1f%% better. Recommend adopting.", result.Improvement)
	} else {
		result.Recommendation = "Variant A performs better. Recommend keeping current version."
	}

	// Update test
	test.Status = "completed"
	test.CompletedAt = time.Now()
	test.WinnerID = result.WinnerID

	// Store result
	m.results[testID] = result

	return result, nil
}

// calculateConfidence calculates statistical confidence
func (m *ABTestManager) calculateConfidence(scoreA, scoreB *PromptScore) float64 {
	// Simplified confidence calculation
	// In production, would use proper statistical tests
	minExecutions := float64(min(scoreA.TotalExecutions, scoreB.TotalExecutions))
	if minExecutions < 10 {
		return 0.3
	}
	if minExecutions < 50 {
		return 0.5
	}
	if minExecutions < 100 {
		return 0.7
	}
	return 0.9
}

// GetResult retrieves test result
func (m *ABTestManager) GetResult(testID string) (*ABTestResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result, ok := m.results[testID]
	if !ok {
		return nil, fmt.Errorf("result not found: %s", testID)
	}
	return result, nil
}

// ListTests lists all tests
func (m *ABTestManager) ListTests() []*ABTest {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var tests []*ABTest
	for _, test := range m.tests {
		tests = append(tests, test)
	}
	return tests
}

// ============================================================
// Auto Prompt Generation
// ============================================================

// PromptGenerator automatically generates prompts
type PromptGenerator struct {
	llmClient llm.Client
	llmModel  string
	templates map[PromptType]string
}

// NewPromptGenerator creates a new prompt generator
func NewPromptGenerator() *PromptGenerator {
	g := &PromptGenerator{
		templates: make(map[PromptType]string),
	}

	// Set default templates
	g.templates[PromptTypeTask] = `你是一个专业的 {{role}}。

任务：{{task}}

请按照以下步骤完成任务：
1. 分析任务需求
2. 制定执行计划
3. 执行并验证结果

输出格式：
{{output_format}}`

	g.templates[PromptTypeReasoning] = `请分析以下问题：

{{problem}}

分析步骤：
1. 理解问题
2. 识别关键因素
3. 推理过程
4. 得出结论

请逐步思考并给出推理过程。`

	return g
}

// SetLLM sets LLM client
func (g *PromptGenerator) SetLLM(client llm.Client, model string) {
	g.llmClient = client
	g.llmModel = model
}

// Generate generates a prompt based on task description
func (g *PromptGenerator) Generate(ctx context.Context, task, role string, promptType PromptType) (*PromptVariant, error) {
	template, ok := g.templates[promptType]
	if !ok {
		template = g.templates[PromptTypeTask]
	}

	if g.llmClient != nil {
		return g.generateWithLLM(ctx, task, role, promptType)
	}

	// Use template
	rendered := strings.ReplaceAll(template, "{{role}}", role)
	rendered = strings.ReplaceAll(rendered, "{{task}}", task)

	return &PromptVariant{
		ID:        generateVariantID(),
		Name:      fmt.Sprintf("generated_%s", promptType),
		Type:      promptType,
		Template:  rendered,
		Variables: []string{"role", "task", "output_format"},
		CreatedAt: time.Now(),
	}, nil
}

// generateWithLLM generates prompt using LLM
func (g *PromptGenerator) generateWithLLM(ctx context.Context, task, role string, promptType PromptType) (*PromptVariant, error) {
	prompt := fmt.Sprintf(`请为以下任务生成一个优化的 Prompt 模板。

任务描述：%s
角色：%s
类型：%s

要求：
1. 清晰的任务说明
2. 明确的步骤指导
3. 具体的输出格式要求

请直接输出 Prompt 模板，使用 {{变量名}} 表示变量占位符。`, task, role, promptType)

	resp, err := g.llmClient.Chat(ctx, &llm.ChatRequest{
		Model:    g.llmModel,
		Messages: []llm.Message{{Role: "user", Content: prompt}},
	})
	if err != nil {
		return nil, err
	}

	// Extract variables
	template := strings.TrimSpace(resp.Content)
	variables := extractVariables(template)

	return &PromptVariant{
		ID:        generateVariantID(),
		Name:      fmt.Sprintf("llm_generated_%s", promptType),
		Type:      promptType,
		Template:  template,
		Variables: variables,
		CreatedAt: time.Now(),
	}, nil
}

// ============================================================
// Helper Functions
// ============================================================

func generateVariantID() string {
	return fmt.Sprintf("variant-%d", time.Now().UnixNano())
}

func generateExecutionID() string {
	return fmt.Sprintf("exec-%d", time.Now().UnixNano())
}

func generateTestID() string {
	return fmt.Sprintf("abtest-%d", time.Now().UnixNano())
}

func renderTemplate(template string, variables map[string]interface{}) (string, error) {
	result := template
	for key, value := range variables {
		placeholder := fmt.Sprintf("{{%s}}", key)
		result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%v", value))
	}
	return result, nil
}

func incrementVersion(version string) string {
	if version == "" {
		return "1.0"
	}
	// Simple version increment
	return version + ".1"
}

func extractVariables(template string) []string {
	var variables []string
	re := regexp.MustCompile(`\{\{(\w+)\}\}`)
	matches := re.FindAllStringSubmatch(template, -1)
	seen := make(map[string]bool)

	for _, match := range matches {
		if len(match) > 1 && !seen[match[1]] {
			variables = append(variables, match[1])
			seen[match[1]] = true
		}
	}

	return variables
}

func addStructure(template string) string {
	return fmt.Sprintf("## 任务\n%s\n\n## 要求\n请仔细完成任务。", template)
}

func addStepGuidance(template string) string {
	return template + "\n\n请按步骤执行。"
}

func addOutputGuidance(template string) string {
	return template + "\n\n请以清晰的格式输出结果。"
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
