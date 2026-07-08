# 待实现功能 — P1（影响系统能力的关键缺失）

> 本文档记录 P1 优先级的待实现功能——缺了这些，系统能力受限但基本可用。

---

## P1-1：图编排 / 条件路由

### 问题

当前 Agent 执行只有两种模式：
1. **线性 ReAct 循环**：LLM → 工具 → 观察 → LLM → ...（单 Agent）
2. **Handoff 转交**：当前 Agent 把控制权交给另一个 Agent

无法表达复杂逻辑：
- "如果用户问技术问题，走 researcher-agent；如果问数据分析，走 analyst-agent"
- "先并行执行 A 和 B，然后汇总结果给 C"
- "循环执行直到满足条件"

### 目标

支持 LangGraph 风格的有向图编排：条件边、并行扇出、循环。

### 实现方案

#### Step 1：定义 Workflow DAG 数据结构

**新文件**：`pkg/agent/workflow/dag.go`

```go
package workflow

// Workflow 定义有向无环图工作流
type Workflow struct {
    ID          string
    Name        string
    Nodes       map[string]*Node
    Edges       []*Edge
    EntryNodeID string
}

// Node 定义工作流节点
type Node struct {
    ID           string
    Type         NodeType   // agent / tool / condition / parallel / merge
    AgentID      string     // agent 类型节点
    ToolName     string     // tool 类型节点
    Condition    string     // condition 类型节点的条件表达式
    Config       map[string]interface{}
}

// Edge 定义工作流边
type Edge struct {
    From        string
    To          string
    Condition   string     // 空表示无条件；表达式表示条件边
    Label       string     // 可选标签
}

type NodeType string

const (
    NodeAgent     NodeType = "agent"
    NodeTool      NodeType = "tool"
    NodeCondition NodeType = "condition"
    NodeParallel  NodeType = "parallel"
    NodeMerge     NodeType = "merge"
)
```

#### Step 2：实现 Workflow 执行器

**新文件**：`pkg/agent/workflow/executor.go`

```go
type WorkflowExecutor struct {
    engine   *Engine
    registry *Registry
}

// Execute 执行工作流
func (e *WorkflowExecutor) Execute(ctx context.Context, workflow *Workflow, input string) (*WorkflowResult, error) {
    currentNodeID := workflow.EntryNodeID
    results := make(map[string]interface{})
    
    for {
        node := workflow.Nodes[currentNodeID]
        
        switch node.Type {
        case NodeAgent:
            // 执行 Agent
            result, err := e.executeAgentNode(ctx, node, input, results)
            results[currentNodeID] = result
            if err != nil { return nil, err }
            
        case NodeCondition:
            // 评估条件，决定下一个节点
            nextNodeID, err := e.evaluateCondition(ctx, node, results)
            if err != nil { return nil, err }
            currentNodeID = nextNodeID
            continue
            
        case NodeParallel:
            // 并行执行多个分支
            branchResults, err := e.executeParallelBranches(ctx, workflow, node, input, results)
            results[currentNodeID] = branchResults
            if err != nil { return nil, err }
            
        case NodeMerge:
            // 合并并行分支结果
            merged := e.mergeResults(results, node)
            results[currentNodeID] = merged
        }
        
        // 找下一个节点（通过边）
        nextNodes := workflow.GetNextNodes(currentNodeID, results)
        if len(nextNodes) == 0 {
            break // 工作流结束
        }
        currentNodeID = nextNodes[0] // 条件边已过滤，只会有一个
    }
    
    return &WorkflowResult{Results: results}, nil
}

// evaluateCondition 评估条件表达式
func (e *WorkflowExecutor) evaluateCondition(ctx context.Context, node *Node, results map[string]interface{}) (string, error) {
    // 使用简单表达式引擎（如 expr 库）
    // 条件示例：`last_result.contains("技术")` → 走 researcher 分支
    // 条件示例：`last_result.contains("数据")` → 走 analyst 分支
    
    env := map[string]interface{}{
        "last_result": results[lastNodeID],
        "input":       input,
    }
    
    for _, edge := range workflow.Edges {
        if edge.From == node.ID && edge.Condition != "" {
            match, _ := expr.Eval(edge.Condition, env)
            if match.(bool) {
                return edge.To, nil
            }
        }
    }
    
    // 默认走第一条无条件边
    for _, edge := range workflow.Edges {
        if edge.From == node.ID && edge.Condition == "" {
            return edge.To, nil
        }
    }
    
    return "", fmt.Errorf("no matching edge from condition node %s", node.ID)
}
```

#### Step 3：Workflow 存储和 API

**文件**：`services/harness-service/` — 新增 Workflow Repository

```go
// MongoDB 存储 Workflow 定义
type WorkflowRepository struct {
    collection *mongo.Collection
}

func (r *WorkflowRepository) Save(ctx context.Context, wf *Workflow) error
func (r *WorkflowRepository) Get(ctx context.Context, id string) (*Workflow, error)
func (r *WorkflowRepository) List(ctx context.Context, tenantID string) ([]*Workflow, error)
func (r *WorkflowRepository) Delete(ctx context.Context, id string) error
```

#### Step 4：前端可视化编辑器

在 Harness 页面或新建 Workflow 页面中，实现拖拽式 DAG 编辑器：
- 左侧节点面板（Agent / Tool / Condition / Parallel / Merge）
- 中间画布（拖拽连线和布局）
- 右侧属性面板（节点配置）
- 底部测试面板（输入测试数据，模拟执行）

推荐使用 React Flow 库（`reactflow`）。

#### 关键文件

| 文件 | 改动类型 |
|------|---------|
| `pkg/agent/workflow/dag.go` | 新建（Workflow/Node/Edge 数据结构） |
| `pkg/agent/workflow/executor.go` | 新建（Workflow 执行器） |
| `pkg/agent/engine.go` | 修改（支持 Workflow 模式作为执行入口） |
| `services/harness-service/` | 修改（新增 Workflow Repository + Service） |
| `proto/harness/harness.proto` | 修改（新增 Workflow 相关 RPC） |
| `services/gateway/` | 修改（新增 Workflow HTTP 路由） |
| `frontend/src/pages/Workflow/` | 新建（可视化 DAG 编辑器页面） |

#### 验证

1. 创建一个条件路由工作流：用户输入 → Condition → 如果问技术走 researcher，否则走 analyst
2. 并行工作流：用户问题 → Parallel（同时走 web_search + knowledge_search）→ Merge → 总结
3. 前端可视化编辑器：拖拽连线、配置条件、保存/加载
4. 执行后 Session Replay 能正确展示图中的分支路径

---

## P1-2：并行工具执行

### 问题

代码已实现 `ParallelExecutor`（`pkg/agent/tools/parallel.go`），但 engine.go 串行执行所有工具调用。当 LLM 返回多个工具调用时，它们应该可以并行执行。

### 实现方案

#### 修改 engine.go 的工具执行循环

**文件**：`pkg/agent/engine.go`

当前代码（约第 213-298 行）：

```go
for _, tc := range llmResp.ToolCalls {
    // 串行执行每个工具
    result, err := e.tools.Execute(ctx, tc.Name, tc.Arguments, toolCfg)
    ...
}
```

改为：

```go
// 分离 handoff 和普通工具调用
var handoffCalls []ToolCall
var regularCalls []ToolCall

for _, tc := range llmResp.ToolCalls {
    if IsHandoffTool(tc.Name) {
        handoffCalls = append(handoffCalls, tc)
    } else {
        regularCalls = append(regularCalls, tc)
    }
}

// 并行执行普通工具
if len(regularCalls) > 0 {
    parallelReqs := make([]tools.ParallelRequest, len(regularCalls))
    for i, tc := range regularCalls {
        parallelReqs[i] = tools.ParallelRequest{
            ToolName:   tc.Name,
            Parameters: parseArguments(tc.Arguments),
            ID:         tc.ID,
            Timeout:    30 * time.Second,
        }
    }
    
    parallelExecutor := tools.NewParallelExecutor(
        tools.NewComposer(e.tools),
        5,                    // maxWorkers
        60 * time.Second,     // 总超时
    )
    
    parallelResult, err := parallelExecutor.Execute(ctx, parallelReqs, toolCtx)
    if err != nil {
        // 部分失败处理
        for id, errMsg := range parallelResult.Errors {
            toolMessages = append(toolMessages, Message{
                Role:       "tool",
                Content:    fmt.Sprintf("工具执行失败: %s", errMsg),
                ToolCallID: id,
            })
        }
    }
    
    // 收集成功结果
    for id, result := range parallelResult.Results {
        toolMessages = append(toolMessages, Message{
            Role:       "tool",
            Content:    result.Output,
            ToolCallID: id,
        })
    }
}

// 串行处理 handoff（只能有一个）
for _, tc := range handoffCalls {
    targetAgent := ParseHandoffTarget(tc.Name)
    // ... 原有的 handoff 逻辑
}
```

#### 验证

1. LLM 返回 3 个工具调用（web_search + knowledge_search + weather）→ 3 个并行执行，总时间 ≈ 最慢的一个
2. LLM 返回 1 个 handoff + 2 个工具 → 工具先并行执行，再处理 handoff
3. 工具超时 → CircuitBreaker 记录失败，不影响其他工具
4. 工具失败 → 失败结果记录在 tool message 中，Agent 可以看到并调整策略

---

## P1-3：中间步骤持久化（Checkpoint）

### 问题

当前 Agent 执行过程中只有最终结果保存到 DB。如果执行到第 5 步崩溃（网络断开、服务重启），所有中间结果丢失，用户只能重新开始。

### 实现方案

#### Step 1：定义 Checkpoint 数据结构

**新文件**：`pkg/agent/checkpoint/checkpoint.go`

```go
package checkpoint

type Checkpoint struct {
    ID          string                 `bson:"_id"`
    SessionID   string                 `bson:"session_id"`
    Step        int                    `bson:"step"`
    AgentID     string                 `bson:"agent_id"`
    Messages    []Message              `bson:"messages"`
    Variables   map[string]string      `bson:"variables"`
    ToolResults map[string]string      `bson:"tool_results"`
    AgentHistory []AgentExecutionRecord `bson:"agent_history"`
    TotalTokens int                    `bson:"total_tokens"`
    CreatedAt   time.Time              `bson:"created_at"`
}
```

#### Step 2：在 engine.go 每步保存 Checkpoint

**文件**：`pkg/agent/engine.go`

```go
func (e *Engine) executeLoop(ctx context.Context, execCtx *ExecutionContext, ...) {
    for step := 0; step < e.config.MaxSteps; step++ {
        // ... LLM 调用 + 工具执行 ...
        
        // 每步完成后保存 Checkpoint
        if e.checkpointStore != nil {
            checkpoint := &checkpoint.Checkpoint{
                SessionID:    execCtx.SessionID,
                Step:         step,
                AgentID:      currentAgent.ID,
                Messages:     execCtx.Messages,
                Variables:    execCtx.Variables,
                ToolResults:  execCtx.ToolResults,
                AgentHistory: execCtx.AgentHistory,
                TotalTokens:  execCtx.TotalTokens,
            }
            if err := e.checkpointStore.Save(ctx, checkpoint); err != nil {
                log.Warnf("保存 checkpoint 失败: %v", err)
            }
        }
    }
}
```

#### Step 3：从 Checkpoint 恢复执行

**文件**：`pkg/agent/engine.go`

```go
// ResumeFromCheckpoint 从某个 Checkpoint 恢复执行
func (e *Engine) ResumeFromCheckpoint(ctx context.Context, checkpointID string) (*ExecutionResult, error) {
    cp, err := e.checkpointStore.Get(ctx, checkpointID)
    if err != nil {
        return nil, fmt.Errorf("checkpoint 不存在: %w", err)
    }
    
    // 重建 ExecutionContext
    execCtx := &ExecutionContext{
        SessionID:    cp.SessionID,
        Messages:     cp.Messages,
        Variables:    cp.Variables,
        ToolResults:  cp.ToolResults,
        AgentHistory: cp.AgentHistory,
        TotalTokens:  cp.TotalTokens,
    }
    
    // 从 cp.Step + 1 继续执行
    return e.executeLoop(ctx, execCtx, cp.Step+1)
}
```

#### Step 4：暴露 Checkpoint API

```
GET  /api/v2/harness/session/:id/checkpoints     — 列出 Session 的所有 Checkpoint
GET  /api/v2/harness/session/:id/checkpoint/:cpid — 获取某个 Checkpoint 详情
POST /api/v2/harness/session/:id/checkpoint/:cpid/resume — 从 Checkpoint 恢复执行
```

#### Step 5：前端 Checkpoint 列表

在 Session Replay 页面添加 Checkpoint 时间线：
- 每个 Checkpoint 显示为时间线上的一个点
- 点击可以查看该步骤的完整状态
- "从此处恢复"按钮

#### 验证

1. Agent 执行到第 3 步后服务重启 → Checkpoint 已保存
2. 用户在前端点击"从 Step 3 恢复" → Agent 从第 4 步继续
3. 执行完成后 → Checkpoint 列表显示每步状态
4. 中间状态可以回溯查看

---

## P1-4：分层记忆接入

### 问题

代码已实现情节记忆（Episodic）、语义记忆（Semantic）、工作记忆（Working），但 gRPC handler 只使用基础 MemoryService。Agent 缺乏跨会话的长期记忆和上下文压缩。

### 实现方案

#### Step 1：切换到 MemoryServiceWithForgetting

**文件**：`services/memory-service/internal/handler/grpc_handler.go`

当前 `NewMemoryService()` 创建基础版。改为：

```go
func NewMemoryHandler(repo repository.MemoryRepository, llmClient llm.Client) *MemoryHandler {
    // 使用增强版
    service := memory.NewMemoryServiceWithForgetting(repo, llmClient, &memory.ForgettingConfig{
        DecayRate:        0.01,   // 每天衰减 1%
        CleanupInterval:  24 * time.Hour,
        MinImportance:    0.1,
        MaxMemories:      10000,
    })
    return &MemoryHandler{service: service}
}
```

#### Step 2：扩展 Memory gRPC proto

**文件**：`proto/memory/memory.proto`

```protobuf
service MemoryService {
    // 现有 RPC
    rpc Save(SaveMemoryRequest) returns (SaveMemoryResponse);
    rpc Recall(RecallMemoryRequest) returns (RecallMemoryResponse);
    // ... 省略
    
    // 新增：情节记忆
    rpc StoreEpisode(StoreEpisodeRequest) returns (StoreEpisodeResponse);
    rpc GetTimeline(GetTimelineRequest) returns (TimelineResponse);
    rpc GetSimilarEpisodes(SimilarEpisodesRequest) returns (EpisodesResponse);
    
    // 新增：语义记忆
    rpc StoreConcept(StoreConceptRequest) returns (StoreConceptResponse);
    rpc StoreRelation(StoreRelationRequest) returns (StoreRelationResponse);
    rpc RecallConcepts(RecallConceptsRequest) returns (ConceptsResponse);
    rpc GetRelatedConcepts(GetRelatedConceptsRequest) returns (ConceptsResponse);
    rpc GetConceptPath(GetConceptPathRequest) returns (ConceptPathResponse);
    
    // 新增：工作记忆
    rpc AddWorkingMessage(AddWorkingMessageRequest) returns (WorkingMemoryResponse);
    rpc GetWorkingContext(GetWorkingContextRequest) returns (WorkingContextResponse);
    rpc GetWorkingMessagesForLLM(GetWorkingMessagesRequest) returns (WorkingMessagesResponse);
    rpc ClearWorkingContext(ClearWorkingContextRequest) returns (common.Empty);
    
    // 新增：遗忘配置
    rpc GetForgettingConfig(common.Empty) returns (ForgettingConfigResponse);
    rpc UpdateForgettingConfig(UpdateForgettingConfigRequest) returns (ForgettingConfigResponse);
    rpc TriggerCleanup(CleanupRequest) returns (CleanupResponse);
}
```

#### Step 3：在 gRPC handler 中实现新 RPC

**文件**：`services/memory-service/internal/handler/grpc_handler.go`

每个新 RPC 调用对应的内部包：

```go
func (h *MemoryHandler) StoreEpisode(ctx context.Context, req *pb.StoreEpisodeRequest) (*pb.StoreEpisodeResponse, error) {
    episode := &episodic.Episode{
        SessionID:   req.SessionId,
        AgentID:     req.AgentId,
        Type:        episodic.EpisodeType(req.Type),
        Title:       req.Title,
        Description: req.Description,
        Outcome:     req.Outcome,
        Importance:  req.Importance,
    }
    err := h.episodicMemory.Store(ctx, episode)
    return &pb.StoreEpisodeResponse{EpisodeId: episode.ID}, err
}
```

#### Step 4：在 chat-service 中集成工作记忆

**文件**：`services/chat-service/internal/service/chat_service.go`

当前 `buildMessages()` 只保留最近 10 条消息。改为使用工作记忆：

```go
func (s *ChatService) buildMessages(ctx context.Context, sessionID string, messages []Message) ([]*pb.Message, error) {
    // 调用工作记忆获取压缩后的消息
    workingResp, err := s.memoryClient.GetWorkingMessagesForLLM(ctx, &memorypb.GetWorkingMessagesRequest{
        SessionId: sessionID,
        TenantId:  tenantID,
    })
    if err == nil && len(workingResp.Messages) > 0 {
        // 使用工作记忆压缩后的消息
        return convertWorkingMessages(workingResp.Messages), nil
    }
    
    // 降级：原有的最近 10 条逻辑
    return buildMessagesLegacy(messages), nil
}
```

#### Step 5：在 chatWithMultiAgent 中存储情节

```go
// Agent 执行完成后，存储 episode
s.memoryClient.StoreEpisode(ctx, &memorypb.StoreEpisodeRequest{
    SessionId:   sessionID,
    AgentId:     "multi-agent",
    Type:        "conversation",
    Title:       truncate(req.Message, 50),
    Description: resp.Response,
    Outcome:     "completed",
    Importance:  calculateImportance(resp),
})
```

#### 关键文件

| 文件 | 改动类型 |
|------|---------|
| `proto/memory/memory.proto` | 修改（新增情节/语义/工作/遗忘 RPC） |
| `pkg/pb/memory/memory.pb.go` | 重新生成 |
| `services/memory-service/internal/handler/grpc_handler.go` | 修改（注入 episodic/semantic/working 实例，实现新 RPC） |
| `services/memory-service/cmd/main.go` | 修改（初始化各记忆子系统） |
| `services/chat-service/internal/service/chat_service.go` | 修改（集成工作记忆和情节存储） |
| `services/gateway/internal/handler/memory_handler.go` | 修改（代理新的记忆 API） |
| `services/gateway/internal/router/router.go` | 修改（新增记忆 API 路由） |
| `frontend/src/pages/Memory/index.tsx` | 修改（显示情节时间线、知识图谱、工作记忆状态） |

#### 验证

1. 多轮对话 → 工作记忆自动压缩长上下文，Agent 仍能理解早期内容
2. 对话完成 → 情节记忆自动存储，包含时间线
3. 新对话 → 召回相关历史情节，Agent 能参考过去经验
4. 语义记忆 → 概念和关系图谱可视化
5. 遗忘机制 → 30 天后低重要性记忆自动衰减清理
