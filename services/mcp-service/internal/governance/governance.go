// Package governance provides MCP governance capabilities
// Inspired by harness-service architecture: guardrail, rule, permission, SLO, A/B test
package governance

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ============================================================
// Guardrail - 输入输出安全检查
// ============================================================

// GuardrailViolation 表示违规信息
type GuardrailViolation struct {
	Type        string // "prompt_injection", "sensitive_info", "dangerous_tool"
	Pattern     string // 匹配的模式
	Severity    string // "high", "medium", "low"
	Description string // 详细描述
}

// Guardrail 护栏检查器
type Guardrail struct {
	promptInjectionPatterns []*regexp.Regexp
	sensitivePatterns       []*regexp.Regexp
	dangerousToolPrefixes   []string
}

// NewGuardrail 创建护栏检查器
func NewGuardrail() *Guardrail {
	g := &Guardrail{
		promptInjectionPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)ignore\s+(all\s+)?(previous|above|prior)\s+(instructions|directions|prompts?|commands?)`),
			regexp.MustCompile(`(?i)forget\s+(all\s+)?(previous|above|prior)\s+instructions`),
			regexp.MustCompile(`(?i)you\s+are\s+(not\s+)?(an?\s+)?(ai|assistant|chatbot)`),
			regexp.MustCompile(`(?i)system\s*prompt`),
			regexp.MustCompile(`(?i)do\s+what\s+i\s+say`),
			regexp.MustCompile(`(?i)disregard\s+(all\s+)?(previous|above)`),
			regexp.MustCompile(`(?i)override\s+(previous|default|system)`),
			regexp.MustCompile(`(?i)jailbreak`),
			regexp.MustCompile(`(?i)DAN\s+mode`),
		},
		sensitivePatterns: []*regexp.Regexp{
			regexp.MustCompile(`sk-[a-zA-Z0-9]{20,}`),                         // OpenAI API key
			regexp.MustCompile(`[aA][pP][iI]_?[kK][eE][yY]\s*[:=]\s*['"][a-zA-Z0-9_\-]{16,}['"]`),
			regexp.MustCompile(`(?i)password\s*[:=]\s*\S{8,}`),
			regexp.MustCompile(`-----BEGIN\s+(RSA\s+)?PRIVATE\s+KEY-----`),
			regexp.MustCompile(`(?i)secret[_-]?key\s*[:=]\s*['"][a-zA-Z0-9_\-]{16,}['"]`),
			regexp.MustCompile(`(?i)access[_-]?token\s*[:=]\s*['"][a-zA-Z0-9_\-]{16,}['"]`),
		},
		dangerousToolPrefixes: []string{
			"delete", "remove", "drop", "truncate", "exec", "shutdown",
			"rm", "kill", "format", "wipe", "destroy",
		},
	}
	return g
}

// CheckInput 检查输入内容
func (g *Guardrail) CheckInput(content string) []GuardrailViolation {
	var violations []GuardrailViolation

	// 检查 Prompt Injection
	for _, pattern := range g.promptInjectionPatterns {
		if pattern.MatchString(content) {
			violations = append(violations, GuardrailViolation{
				Type:        "prompt_injection",
				Pattern:     pattern.String(),
				Severity:    "high",
				Description: "检测到可能的 prompt injection 尝试",
			})
			break // 只记录一次
		}
	}

	// 检查敏感信息
	for _, pattern := range g.sensitivePatterns {
		if pattern.MatchString(content) {
			violations = append(violations, GuardrailViolation{
				Type:        "sensitive_info",
				Pattern:     pattern.String(),
				Severity:    "high",
				Description: "检测到敏感信息（API key、密码等）",
			})
			break
		}
	}

	return violations
}

// CheckOutput 检查输出内容
func (g *Guardrail) CheckOutput(content string) []GuardrailViolation {
	var violations []GuardrailViolation

	for _, pattern := range g.sensitivePatterns {
		if pattern.MatchString(content) {
			violations = append(violations, GuardrailViolation{
				Type:        "sensitive_info",
				Pattern:     pattern.String(),
				Severity:    "high",
				Description: "输出包含敏感信息，需要过滤",
			})
			break
		}
	}

	return violations
}

// IsDangerousTool 检查工具是否危险
func (g *Guardrail) IsDangerousTool(toolName string) bool {
	name := strings.ToLower(toolName)
	for _, prefix := range g.dangerousToolPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// SanitizeOutput 清理输出中的敏感信息
func (g *Guardrail) SanitizeOutput(content string) string {
	for _, pattern := range g.sensitivePatterns {
		content = pattern.ReplaceAllString(content, "[REDACTED]")
	}
	return content
}

// ============================================================
// Rule Engine - 工具调用规则
// ============================================================

// ToolRule 工具规则
type ToolRule struct {
	Name            string   // 规则名称
	ToolPattern     string   // 工具匹配模式（支持 * 通配符）
	Effect          string   // "allow" 或 "deny"
	MaxCalls        int      // 最大调用次数（0 = 无限制）
	RequiresApproval bool    // 是否需要人工审批
	Conditions      []string // 附加条件
}

// RuleCheckResult 规则检查结果
type RuleCheckResult struct {
	Passed     bool
	Violations []string
	Allowed    bool
	MaxCalls   int
	RequiresApproval bool
}

// RuleEngine 规则引擎
type RuleEngine struct {
	agentRules map[string][]ToolRule // 按 agent 类型分组的规则
	mu         sync.RWMutex
}

// NewRuleEngine 创建规则引擎
func NewRuleEngine() *RuleEngine {
	e := &RuleEngine{
		agentRules: make(map[string][]ToolRule),
	}
	e.setDefaults()
	return e
}

// setDefaults 设置默认规则
func (e *RuleEngine) setDefaults() {
	// 浏览器 Agent - 允许浏览器操作，限制危险操作
	e.agentRules["browser"] = []ToolRule{
		{Name: "allow_browser", ToolPattern: "browser_*", Effect: "allow", MaxCalls: 100},
		{Name: "allow_quick_fetch", ToolPattern: "quick_fetch", Effect: "allow", MaxCalls: 50},
		{Name: "deny_exec", ToolPattern: "exec_*", Effect: "deny", MaxCalls: 0},
		{Name: "deny_delete", ToolPattern: "delete_*", Effect: "deny", MaxCalls: 0},
	}

	// 搜索 Agent - 只允许搜索相关工具
	e.agentRules["search"] = []ToolRule{
		{Name: "allow_search", ToolPattern: "*_search", Effect: "allow", MaxCalls: 100},
		{Name: "allow_quick_fetch", ToolPattern: "quick_fetch", Effect: "allow", MaxCalls: 30},
		{Name: "deny_browser", ToolPattern: "browser_*", Effect: "deny", MaxCalls: 0},
		{Name: "deny_exec", ToolPattern: "exec_*", Effect: "deny", MaxCalls: 0},
	}

	// 写作 Agent - 允许发布和抓取
	e.agentRules["writer"] = []ToolRule{
		{Name: "allow_publish", ToolPattern: "*_publish", Effect: "allow", MaxCalls: 20},
		{Name: "allow_fetch", ToolPattern: "quick_fetch", Effect: "allow", MaxCalls: 50},
		{Name: "allow_browser", ToolPattern: "browser_*", Effect: "allow", MaxCalls: 30},
		{Name: "deny_exec", ToolPattern: "exec_*", Effect: "deny", MaxCalls: 0},
	}

	// 通用 Agent - 最宽松的规则
	e.agentRules["general"] = []ToolRule{
		{Name: "allow_all", ToolPattern: "*", Effect: "allow", MaxCalls: 0},
		{Name: "require_approval_dangerous", ToolPattern: "delete_*", Effect: "allow", MaxCalls: 5, RequiresApproval: true},
	}
}

// Check 检查工具调用规则
func (e *RuleEngine) Check(agentType, toolName string, callCount int) *RuleCheckResult {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := &RuleCheckResult{
		Passed:     true,
		Allowed:    true,
		Violations: []string{},
	}

	rules, ok := e.agentRules[agentType]
	if !ok {
		// 没有规则，默认允许
		return result
	}

	for _, rule := range rules {
		matched := simpleMatch(rule.ToolPattern, toolName)
		if !matched {
			continue
		}

		// 检查 deny
		if rule.Effect == "deny" {
			result.Passed = false
			result.Allowed = false
			result.Violations = append(result.Violations,
				fmt.Sprintf("工具 %s 被规则 %s 拒绝 (pattern: %s)", toolName, rule.Name, rule.ToolPattern))
			return result
		}

		// 检查调用次数
		if rule.MaxCalls > 0 && callCount >= rule.MaxCalls {
			result.Passed = false
			result.Violations = append(result.Violations,
				fmt.Sprintf("工具 %s 调用次数超限 (%d/%d)", toolName, callCount, rule.MaxCalls))
			return result
		}

		// 记录审批要求
		result.RequiresApproval = rule.RequiresApproval
		result.MaxCalls = rule.MaxCalls
	}

	return result
}

// AddRule 添加规则
func (e *RuleEngine) AddRule(agentType string, rule ToolRule) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.agentRules[agentType] = append(e.agentRules[agentType], rule)
}

// simpleMatch 简单通配符匹配
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

// ============================================================
// Permission Matrix - 权限矩阵
// ============================================================

// Permission 权限定义
type Permission struct {
	AgentType   string   // Agent 类型
	AllowedTools []string // 允许的工具列表
	DeniedTools  []string // 禁止的工具列表
	MaxTokens    int      // 最大 token 数
	MaxCostUSD   float64  // 最大成本
	MaxDuration  int      // 最大执行时间（秒）
}

// PermissionMatrix 权限矩阵
type PermissionMatrix struct {
	permissions map[string]*Permission
	mu          sync.RWMutex
}

// NewPermissionMatrix 创建权限矩阵
func NewPermissionMatrix() *PermissionMatrix {
	m := &PermissionMatrix{
		permissions: make(map[string]*Permission),
	}
	m.setDefaults()
	return m
}

func (m *PermissionMatrix) setDefaults() {
	m.permissions["browser"] = &Permission{
		AgentType:    "browser",
		AllowedTools: []string{"browser_execute", "quick_fetch", "web_search", "calculator"},
		DeniedTools:  []string{"exec_*", "delete_*", "shutdown"},
		MaxTokens:    100000,
		MaxCostUSD:   5.0,
		MaxDuration:  600, // 10 分钟
	}

	m.permissions["search"] = &Permission{
		AgentType:    "search",
		AllowedTools: []string{"web_search", "knowledge_search", "quick_fetch", "calculator"},
		DeniedTools:  []string{"browser_*", "exec_*", "delete_*"},
		MaxTokens:    50000,
		MaxCostUSD:   1.0,
		MaxDuration:  120, // 2 分钟
	}

	m.permissions["writer"] = &Permission{
		AgentType:    "writer",
		AllowedTools: []string{"browser_execute", "quick_fetch", "csdn_publish", "web_search"},
		DeniedTools:  []string{"exec_*", "shutdown"},
		MaxTokens:    200000,
		MaxCostUSD:   10.0,
		MaxDuration:  900, // 15 分钟
	}

	m.permissions["general"] = &Permission{
		AgentType:    "general",
		AllowedTools: []string{"*"},
		DeniedTools:  []string{"shutdown"},
		MaxTokens:    50000,
		MaxCostUSD:   2.0,
		MaxDuration:  300, // 5 分钟
	}
}

// CheckPermission 检查权限
func (m *PermissionMatrix) CheckPermission(agentType, toolName string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	perm, ok := m.permissions[agentType]
	if !ok {
		return nil // 无权限配置，默认允许
	}

	// 检查禁止列表
	for _, denied := range perm.DeniedTools {
		if simpleMatch(denied, toolName) {
			return fmt.Errorf("agent %s 禁止使用工具 %s", agentType, toolName)
		}
	}

	// 检查允许列表
	for _, allowed := range perm.AllowedTools {
		if simpleMatch(allowed, toolName) {
			return nil
		}
	}

	return fmt.Errorf("agent %s 没有权限使用工具 %s", agentType, toolName)
}

// CheckCost 检查成本
func (m *PermissionMatrix) CheckCost(agentType string, costUSD float64) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	perm, ok := m.permissions[agentType]
	if !ok {
		return nil
	}

	if perm.MaxCostUSD > 0 && costUSD > perm.MaxCostUSD {
		return fmt.Errorf("agent %s 成本超限: $%.4f > $%.4f", agentType, costUSD, perm.MaxCostUSD)
	}

	return nil
}

// SetPermission 设置权限
func (m *PermissionMatrix) SetPermission(perm *Permission) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.permissions[perm.AgentType] = perm
}

// ============================================================
// SLO Manager - 服务等级目标
// ============================================================

// SLODefinition SLO 定义
type SLODefinition struct {
	ID            string
	Name          string
	Type          string  // "latency", "success_rate", "availability"
	Target        float64 // 目标值（如 0.99 表示 99%）
	Window        time.Duration // 统计窗口
	AlertThreshold float64 // 告警阈值
}

// SLOStatus SLO 状态
type SLOStatus struct {
	Name            string
	Target          float64
	Current         float64
	BudgetRemaining float64
	Status          string // "healthy", "warning", "critical"
	BurnRate        float64
}

// SLOManager SLO 管理器
type SLOManager struct {
	definitions map[string]*SLODefinition
	metrics     map[string]*metricBuffer
	statuses    map[string]*sloStatusInternal
	mu          sync.RWMutex
}

type metricBuffer struct {
	latencies  []float64
	successes  []bool
	timestamps []time.Time
}

type sloStatusInternal struct {
	current         float64
	target          float64
	budgetRemaining float64
	status          string
	burnRate        float64
	totalRequests   int64
	failedRequests  int64
	totalLatency    float64
}

// NewSLOManager 创建 SLO 管理器
func NewSLOManager() *SLOManager {
	return &SLOManager{
		definitions: make(map[string]*SLODefinition),
		metrics:     make(map[string]*metricBuffer),
		statuses:    make(map[string]*sloStatusInternal),
	}
}

// RegisterSLO 注册 SLO
func (m *SLOManager) RegisterSLO(def *SLODefinition) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.definitions[def.ID] = def
	m.metrics[def.ID] = &metricBuffer{
		latencies:  make([]float64, 0, 1000),
		successes:  make([]bool, 0, 1000),
		timestamps: make([]time.Time, 0, 1000),
	}
	m.statuses[def.ID] = &sloStatusInternal{
		target:          def.Target,
		budgetRemaining: 1.0,
		status:          "healthy",
	}
}

// RecordToolCall 记录工具调用
func (m *SLOManager) RecordToolCall(sloID string, latencyMs float64, success bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	buf := m.metrics[sloID]
	if buf == nil {
		return
	}

	buf.latencies = append(buf.latencies, latencyMs)
	buf.successes = append(buf.successes, success)
	buf.timestamps = append(buf.timestamps, time.Now())

	// 保持最新 1000 条记录
	if len(buf.latencies) > 1000 {
		buf.latencies = buf.latencies[1:]
		buf.successes = buf.successes[1:]
		buf.timestamps = buf.timestamps[1:]
	}

	// 更新统计
	status := m.statuses[sloID]
	status.totalRequests++
	if !success {
		status.failedRequests++
	}
	status.totalLatency += latencyMs

	m.recalculateStatus(sloID)
}

// recalculateStatus 重新计算状态
func (m *SLOManager) recalculateStatus(sloID string) {
	def := m.definitions[sloID]
	buf := m.metrics[sloID]
	status := m.statuses[sloID]

	if def == nil || buf == nil || status == nil {
		return
	}

	switch def.Type {
	case "latency":
		if len(buf.latencies) == 0 {
			return
		}
		// 计算 P99
		status.current = calculatePercentile(buf.latencies, 99)
		if status.current > 0 {
			status.budgetRemaining = def.Target / status.current
		}

	case "success_rate":
		if len(buf.successes) == 0 {
			return
		}
		successCount := 0
		for _, s := range buf.successes {
			if s {
				successCount++
			}
		}
		status.current = float64(successCount) / float64(len(buf.successes))
		if def.Target < 1 {
			status.budgetRemaining = (status.current - def.Target) / (1 - def.Target)
		}

	case "availability":
		if status.totalRequests == 0 {
			return
		}
		status.current = 1 - float64(status.failedRequests)/float64(status.totalRequests)
		if def.Target < 1 {
			status.budgetRemaining = (status.current - def.Target) / (1 - def.Target)
		}
	}

	// 状态判定
	if status.budgetRemaining < 0 {
		status.status = "critical"
	} else if status.budgetRemaining < 0.2 {
		status.status = "warning"
	} else {
		status.status = "healthy"
	}

	// Burn rate
	if len(buf.timestamps) >= 2 {
		timeDiff := buf.timestamps[len(buf.timestamps)-1].Sub(buf.timestamps[0]).Hours()
		if timeDiff > 0 {
			status.burnRate = (1 - status.current) / timeDiff
		}
	}
}

// GetStatus 获取状态
func (m *SLOManager) GetStatus(sloID string) *SLOStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	def := m.definitions[sloID]
	status := m.statuses[sloID]
	if def == nil || status == nil {
		return nil
	}

	return &SLOStatus{
		Name:            def.Name,
		Target:          status.target,
		Current:         status.current,
		BudgetRemaining: status.budgetRemaining,
		Status:          status.status,
		BurnRate:        status.burnRate,
	}
}

// GetAlertStatus 获取告警状态
func (m *SLOManager) GetAlertStatus() map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	alerts := make(map[string]string)
	for id, status := range m.statuses {
		if status.status != "healthy" {
			if def, ok := m.definitions[id]; ok {
				alerts[def.Name] = status.status
			}
		}
	}
	return alerts
}

// ============================================================
// A/B Test Engine - 模型分流测试
// ============================================================

// ABTestDefinition A/B 测试定义
type ABTestDefinition struct {
	ID            string
	Name          string
	ControlModel  string
	VariantModel  string
	TrafficSplit  float64 // 变体流量比例（0-1）
	Status        string  // "running", "completed", "paused"
	CreatedAt     time.Time
}

// ABTestResult A/B 测试结果
type ABTestResult struct {
	TestID         string
	ControlScore   float64
	VariantScore   float64
	Delta          float64
	PValue         float64
	Significant    bool
	Recommended    string // "control", "variant", "continue"
	ControlCount   int
	VariantCount   int
}

// ABTestEngine A/B 测试引擎
type ABTestEngine struct {
	tests    map[string]*ABTestDefinition
	scores   map[string]*abTestScores
	trials   map[string]*abTrialData
	mu       sync.RWMutex
}

type abTestScores struct {
	controlScores []float64
	variantScores []float64
	controlTimes  []float64
	variantTimes  []float64
}

type abTrialData struct {
	controlCount int
	variantCount int
	splitRatio   float64
}

// NewABTestEngine 创建 A/B 测试引擎
func NewABTestEngine() *ABTestEngine {
	return &ABTestEngine{
		tests:  make(map[string]*ABTestDefinition),
		scores: make(map[string]*abTestScores),
		trials: make(map[string]*abTrialData),
	}
}

// CreateTest 创建测试
func (e *ABTestEngine) CreateTest(def *ABTestDefinition) {
	e.mu.Lock()
	defer e.mu.Unlock()

	def.CreatedAt = time.Now()
	def.Status = "running"
	e.tests[def.ID] = def
	e.scores[def.ID] = &abTestScores{
		controlScores: make([]float64, 0),
		variantScores: make([]float64, 0),
	}
	e.trials[def.ID] = &abTrialData{
		splitRatio: def.TrafficSplit,
	}
}

// AssignVariant 分配变体
func (e *ABTestEngine) AssignVariant(testID string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	trial := e.trials[testID]
	if trial == nil {
		return false // 默认 control
	}

	total := float64(trial.controlCount + trial.variantCount)
	if total == 0 {
		trial.controlCount++
		return false // 第一个请求走 control
	}

	currentRatio := float64(trial.variantCount) / total
	if currentRatio < trial.splitRatio {
		trial.variantCount++
		return true // 分配到变体
	}

	trial.controlCount++
	return false // 分配到 control
}

// RecordScore 记录得分
func (e *ABTestEngine) RecordScore(testID string, isVariant bool, score float64, latencyMs float64) {
	e.mu.Lock()
	defer e.mu.Unlock()

	scores := e.scores[testID]
	if scores == nil {
		return
	}

	if isVariant {
		scores.variantScores = append(scores.variantScores, score)
		scores.variantTimes = append(scores.variantTimes, latencyMs)
	} else {
		scores.controlScores = append(scores.controlScores, score)
		scores.controlTimes = append(scores.controlTimes, latencyMs)
	}
}

// GetResult 获取结果
func (e *ABTestEngine) GetResult(testID string) *ABTestResult {
	e.mu.RLock()
	defer e.mu.RUnlock()

	test := e.tests[testID]
	scores := e.scores[testID]
	trial := e.trials[testID]

	if test == nil || scores == nil || trial == nil {
		return nil
	}

	controlAvg := average(scores.controlScores)
	variantAvg := average(scores.variantScores)
	delta := variantAvg - controlAvg

	// 统计显著性
	nControl := len(scores.controlScores)
	nVariant := len(scores.variantScores)
	significant := nControl >= 30 && nVariant >= 30

	pValue := calculatePValue(scores.controlScores, scores.variantScores)

	recommended := "continue"
	if significant {
		if delta > 0.05 {
			recommended = "variant"
		} else if delta < -0.05 {
			recommended = "control"
		}
	}

	return &ABTestResult{
		TestID:        testID,
		ControlScore:  controlAvg,
		VariantScore:  variantAvg,
		Delta:         delta,
		PValue:        pValue,
		Significant:   significant,
		Recommended:   recommended,
		ControlCount:  nControl,
		VariantCount:  nVariant,
	}
}

// ============================================================
// Governance Pipeline - 综合治理流水线
// ============================================================

// GovernanceRequest 治理请求
type GovernanceRequest struct {
	AgentType    string
	ToolName     string
	InputContent string
	CallCount    int
	CostUSD      float64
	Model        string // 可选，用于 A/B 测试
	TestID       string // 可选，A/B 测试 ID
}

// GovernanceResponse 沧治响应
type GovernanceResponse struct {
	Passed          bool
	Blocked         bool
	BlockReason     string
	InputViolations []GuardrailViolation
	OutputViolations []GuardrailViolation
	RuleViolations  []string
	RequiresApproval bool
	AssignedModel   string // A/B 测试分配的模型
	IsVariant       bool   // 是否分配到变体
	SLOID           string // 关联的 SLO ID
}

// GovernancePipeline 沧治流水线
type GovernancePipeline struct {
	Guardrail   *Guardrail
	RuleEngine  *RuleEngine
	Permission  *PermissionMatrix
	SLOManager  *SLOManager
	ABEngine    *ABTestEngine
}

// NewGovernancePipeline 创建治理流水线
func NewGovernancePipeline() *GovernancePipeline {
	return &GovernancePipeline{
		Guardrail:   NewGuardrail(),
		RuleEngine:  NewRuleEngine(),
		Permission:  NewPermissionMatrix(),
		SLOManager:  NewSLOManager(),
		ABEngine:    NewABTestEngine(),
	}
}

// CheckInput 检查输入（Gate 1）
func (p *GovernancePipeline) CheckInput(req *GovernanceRequest) *GovernanceResponse {
	resp := &GovernanceResponse{
		Passed: true,
	}

	// Gate 1: 输入护栏
	resp.InputViolations = p.Guardrail.CheckInput(req.InputContent)
	if len(resp.InputViolations) > 0 {
		for _, v := range resp.InputViolations {
			if v.Severity == "high" {
				resp.Passed = false
				resp.Blocked = true
				resp.BlockReason = fmt.Sprintf("输入护栏违规: %s", v.Description)
				return resp
			}
		}
	}

	return resp
}

// CheckTool 检查工具调用（Gate 2-3）
func (p *GovernancePipeline) CheckTool(req *GovernanceRequest) *GovernanceResponse {
	resp := &GovernanceResponse{
		Passed: true,
	}

	// Gate 2: 权限检查
	if err := p.Permission.CheckPermission(req.AgentType, req.ToolName); err != nil {
		resp.Passed = false
		resp.Blocked = true
		resp.BlockReason = err.Error()
		return resp
	}

	// Gate 3: 规则检查
	ruleResult := p.RuleEngine.Check(req.AgentType, req.ToolName, req.CallCount)
	if !ruleResult.Passed {
		resp.Passed = false
		resp.Blocked = true
		resp.RuleViolations = ruleResult.Violations
		resp.BlockReason = strings.Join(ruleResult.Violations, "; ")
		return resp
	}

	resp.RequiresApproval = ruleResult.RequiresApproval

	// Gate 4: 成本检查
	if err := p.Permission.CheckCost(req.AgentType, req.CostUSD); err != nil {
		resp.Passed = false
		resp.Blocked = true
		resp.BlockReason = err.Error()
		return resp
	}

	return resp
}

// AssignModel 分配模型（A/B 测试）
func (p *GovernancePipeline) AssignModel(testID, defaultModel string) (model string, isVariant bool) {
	if testID == "" {
		return defaultModel, false
	}

	test := p.ABEngine.tests[testID]
	if test == nil || test.Status != "running" {
		return defaultModel, false
	}

	isVariant = p.ABEngine.AssignVariant(testID)
	if isVariant {
		return test.VariantModel, true
	}
	return test.ControlModel, false
}

// CheckOutput 检查输出（Gate 5）
func (p *GovernancePipeline) CheckOutput(output string) *GovernanceResponse {
	resp := &GovernanceResponse{
		Passed: true,
	}

	resp.OutputViolations = p.Guardrail.CheckOutput(output)
	if len(resp.OutputViolations) > 0 {
		resp.Passed = false
		// 不阻断，而是标记需要清理
	}

	return resp
}

// SanitizeOutput 清理输出
func (p *GovernancePipeline) SanitizeOutput(output string) string {
	return p.Guardrail.SanitizeOutput(output)
}

// RecordMetrics 记录指标
func (p *GovernancePipeline) RecordMetrics(sloID string, latencyMs float64, success bool) {
	p.SLOManager.RecordToolCall(sloID, latencyMs, success)
}

// RecordABScore 记录 A/B 测试得分
func (p *GovernancePipeline) RecordABScore(testID string, isVariant bool, score float64, latencyMs float64) {
	p.ABEngine.RecordScore(testID, isVariant, score, latencyMs)
}

// GetSLOStatus 获取 SLO 状态
func (p *GovernancePipeline) GetSLOStatus(sloID string) *SLOStatus {
	return p.SLOManager.GetStatus(sloID)
}

// GetABResult 获取 A/B 测试结果
func (p *GovernancePipeline) GetABResult(testID string) *ABTestResult {
	return p.ABEngine.GetResult(testID)
}

// ============================================================
// Helper Functions
// ============================================================

func average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func variance(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	mean := average(values)
	var sum float64
	for _, v := range values {
		sum += (v - mean) * (v - mean)
	}
	return sum / float64(len(values))
}

func calculatePercentile(values []float64, percentile float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sorted := make([]float64, len(values))
	copy(sorted, values)

	// 简单排序
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j] < sorted[j-1]; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}

	index := int(float64(len(sorted)-1) * percentile / 100)
	if index >= len(sorted) {
		index = len(sorted) - 1
	}

	return sorted[index]
}

func calculatePValue(control, variant []float64) float64 {
	n1 := float64(len(control))
	n2 := float64(len(variant))

	if n1 < 2 || n2 < 2 {
		return 1.0
	}

	mean1 := average(control)
	mean2 := average(variant)
	var1 := variance(control)
	var2 := variance(variant)

	// Pooled standard error
	se := var1/n1 + var2/n2
	if se == 0 {
		return 1.0
	}

	// Z-score
	z := (mean2 - mean1) / se

	// P-value approximation (two-tailed)
	return 2 * (1 - normalCDF(z))
}

func normalCDF(x float64) float64 {
	// Abramowitz & Stegun approximation
	const (
		a1 = 0.254829592
		a2 = -0.284496736
		a3 = 1.421413741
		a4 = -1.453152027
		a5 = 1.061405429
		p  = 0.3275911
	)

	sign := 1.0
	if x < 0 {
		sign = -1.0
	}
	x = x / 1.41421356237 // sqrt(2)

	t := 1.0 / (1.0 + p*x)
	y := 1.0 - (((((a5*t+a4)*t)+a3)*t+a2)*t+a1)*t*exp(-x*x)

	return 0.5 * (1.0 + sign*y)
}

func exp(x float64) float64 {
	// 简单的 e^x 近似
	result := 1.0
	term := 1.0
	for i := 1; i <= 20; i++ {
		term *= x / float64(i)
		result += term
		if term < 1e-10 {
			break
		}
	}
	return result
}