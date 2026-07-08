# 未接入功能清单

> 本文档记录**代码已完整实现但未接入运行时**的功能。这些是最容易"激活"的功能——代码已经有了，只需要接线。

---

## 一、问题总览

| # | 功能 | 代码位置 | 未接入原因 | 接入工作量 |
|---|------|---------|-----------|-----------|
| 1 | 人工审批 | `pkg/agent/approval/` | engine.go 未调用 | 1 天 |
| 2 | 反思循环 | `pkg/agent/reflection/` | engine.go 未调用 | 1 天 |
| 3 | 实时干预 | `pkg/agent/intervention/` | engine.go 未调用 | 1 天 |
| 4 | 并行工具执行 | `pkg/agent/tools/parallel.go` | engine.go 未调用 | 半天 |
| 5 | 工具组合 | `pkg/agent/tools/composer.go` | engine.go 未调用 | 半天 |
| 6 | Prompt 优化 | `pkg/agent/optimization/` | 未被任何服务调用 | 1 天 |
| 7 | 情节记忆 | `memory/internal/episodic/` | gRPC handler 未使用 | 1 天 |
| 8 | 语义记忆 | `memory/internal/semantic/` | gRPC handler 未使用 | 1 天 |
| 9 | 工作记忆 | `memory/internal/working/` | gRPC handler 未使用 | 1 天 |
| 10 | 案例推理 | `memory/internal/cases/` | gRPC handler 未使用 | 1 天 |
| 11 | 遗忘机制 | `memory_service_with_forgetting.go` | gRPC handler 用的是基础版 | 半天 |

---

## 二、逐项详情

### 2.1 人工审批（Approval）

**代码位置**：`pkg/agent/approval/flow.go`

**已有类型**：
- `ApprovalFlowManager` — 审批流程管理器
- `ApprovalRequest` — 审批请求（ID/Type/Priority/AgentID/SessionID/RiskLevel/...）
- `ApprovalDecision` — 审批决策（approve/reject + 修改参数）
- `ApprovalRule` — 规则定义（AgentID/ToolName/Condition/RiskThreshold/AutoApprove）
- `RuleEngine` — 规则评估引擎
- `ApprovalHooks` — 生命周期回调接口
- `ApprovalNotifier` — 通知接口

**关键函数**：
- `RuleEngine.NeedsApproval(agentID, toolName string, context map[string]interface{}) (bool, *ApprovalRule)`
- `ApprovalFlowManager.CreateRequest(ctx, *ApprovalRequest) (*ApprovalRequest, error)`
- `ApprovalFlowManager.WaitForApproval(ctx, requestID string) (*ApprovalDecision, error)`

**当前状态**：engine.go 在执行工具前不检查是否需要审批，直接执行。

---

### 2.2 反思循环（Reflection）

**代码位置**：`pkg/agent/reflection/loop.go`

**已有类型**：
- `ReflectionLoop` — 反思循环管理器
- `ReflectionResult` — 反思结果（Score/Strengths/Weaknesses/Suggestions/LessonsLearned）
- `ReflectionContext` — 反思输入（Task/Goal/ActionsTaken/Errors/TokenUsage/ElapsedTime/Success）
- `ErrorAnalyzer` — 错误分析器（LLM-based 根因分析）
- `StrategyAdjuster` — 策略调整器（规则驱动）

**关键函数**：
- `ReflectionLoop.Reflect(ctx, sessionID, phase, context_) (*ReflectionResult, error)`
- `ErrorAnalyzer.Analyze(ctx, sessionID, stepNum, errorMsg, errorType) (*ErrorAnalysis, error)`
- `StrategyAdjuster.Adjust(ctx, sessionID, result *ReflectionResult) (*StrategyAdjustment, error)`

**反思阶段**：`PhasePreAction` / `PhasePostAction` / `PhaseComplete` / `PhaseError`

**当前状态**：engine.go 工具执行后不反思，错误后不分析根因。

---

### 2.3 实时干预（Intervention）

**代码位置**：`pkg/agent/intervention/handler.go`

**已有类型**：
- `InterventionManager` — 干预管理器
- `InterventionRequest` — 干预请求（pause/resume/stop/modify/inject/override）
- `SessionState` — Session 状态追踪（running/paused/stopped/completed）
- `InterventionEvent` — 事件通知
- `FeedbackCollector` — 用户反馈收集

**关键函数**：
- `InterventionManager.RegisterSession(sessionID, agentID string)`
- `InterventionManager.WaitForEvent(ctx, sessionID) (*InterventionEvent, error)`
- `InterventionManager.LogExecution(sessionID, entry)`
- `InterventionManager.UnregisterSession(sessionID)`

**当前状态**：engine.go 执行后不可暂停、不可修改参数、不可注入消息。

---

### 2.4 并行工具执行（Parallel）

**代码位置**：`pkg/agent/tools/parallel.go`

**已有类型**：
- `ParallelExecutor` — 并行执行器（goroutine pool）
- `ParallelRequest` — 并行请求（ToolName/Parameters/ID/Timeout）
- `ParallelResult` — 并行结果（TotalRequests/Successful/Failed/Results/Errors）
- `RetryExecutor` — 重试执行器
- `CircuitBreaker` — 熔断器
- `RateLimiter` — 令牌桶限速
- `ToolRegistry` — 工具注册表
- `StatsCollector` — 统计收集

**关键函数**：
- `ParallelExecutor.Execute(ctx, []ParallelRequest, *ToolContext) (*ParallelResult, error)`

**当前状态**：engine.go 的 `for _, tc := range llmResp.ToolCalls` 是串行执行每个工具。

---

### 2.5 情节记忆（Episodic）

**代码位置**：`services/memory-service/internal/episodic/episodic.go`

**已有类型**：
- `EpisodicMemory` — 情节记忆管理器（全内存 map）
- `Episode` — 事件单元（SessionID/AgentID/Type/Title/Description/StartTime/EndTime/Outcome/Importance/Vector）
- `EpisodeSequence` — 事件序列（有序 Episode 组）
- `Timeline` — 时间线（按时间排列 + 统计）

**关键方法**：
- `Store(ctx, *Episode)` / `Get(ctx, id)` / `GetBySession(ctx, sessionID)`
- `GetByTimeRange(ctx, start, end)` / `GetTimeline(ctx, sessionID)`
- `GetSimilarEpisodes(ctx, embedding, topK)` / `Search(ctx, query)`

**当前状态**：gRPC handler 只使用基础 `MemoryService`，不使用 EpisodicMemory。

---

### 2.6 语义记忆（Semantic）

**代码位置**：`services/memory-service/internal/semantic/semantic.go`

**已有类型**：
- `SemanticMemory` — 语义记忆门面
- `KnowledgeGraph` — 知识图谱（Concept + Relation + Edges）
- `Concept` — 概念节点（entity/fact/rule/procedure/concept + Properties + Vector）
- `Relation` — 关系边（is_a/has_a/part_of/related_to/causes/precedes/...）

**关键方法**：
- `Store(ctx, *Concept)` / `StoreRelation(ctx, *Relation)`
- `Recall(ctx, query, topK)` / `RecallByEmbedding(ctx, embedding, topK)`
- `GetRelated(ctx, conceptID, relationType)`
- `KnowledgeGraph.GetConceptPath(ctx, fromID, toID)` — BFS 路径查找
- `KnowledgeGraph.ExtractKnowledge(ctx, text)` — 从文本提取概念和关系（stub）

**当前状态**：完全未接入。

---

### 2.7 工作记忆（Working）

**代码位置**：`services/memory-service/internal/working/working.go`

**已有类型**：
- `WorkingMemory` — 工作记忆管理器（per-session 上下文）
- `WorkingMemoryContext` — 会话上下文（Messages/TotalTokens/MaxTokens/KeyMessages）
- `Message` — 消息（Type/Content/Role/Summary/Tokens/Importance/IsKey）
- `LLMCompressor` — LLM 压缩接口
- `CompressionStrategy` — none/summary/selective/hybrid

**关键方法**：
- `Add(ctx, sessionID, message)` / `Get(ctx, sessionID)` / `GetMessages(ctx, sessionID)`
- `GetMessagesForLLM(ctx, sessionID)` — 压缩后的 LLM-ready 消息
- `Clear(ctx, sessionID)` / `MarkKey(ctx, sessionID, messageID)`
- `GetTokenUsage(ctx, sessionID)` — Token 使用情况

**当前状态**：完全未接入。chat-service 的 `buildMessages()` 只保留最近 10 条消息。

---

### 2.8 遗忘机制（Forgetting）

**代码位置**：`services/memory-service/internal/service/memory_service_with_forgetting.go`

**已有类型**：
- `MemoryServiceWithForgetting` — 增强版 MemoryService
- `calculateImportance()` — 关键词加权重要性评分
- 时间衰减公式：`math.Exp(-rate * age_hours/24)`
- `Cleanup()` — 自动清理低重要性记忆

**当前状态**：gRPC handler 使用 `MemoryService`（基础版），不使用 `MemoryServiceWithForgetting`。

---

### 2.9 案例推理 / Prompt 优化

- **案例推理** (`memory/internal/cases/`)：`CaseLibrary` + `CaseRetriever` + `CaseLearner`，完全未接入
- **Prompt 优化** (`pkg/agent/optimization/`)：Prompt 生成/优化，完全未接入
- **工具组合** (`pkg/agent/tools/composer.go`)：工具编排，完全未接入
