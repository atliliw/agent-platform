# 待实现功能 — P0（不修不能称为可用产品）

> 本文档记录 P0 优先级的待实现功能——缺了这些，系统不能称为"可用产品"。

---

## P0-1：真流式输出

### 问题

当前 `ChatStream` 是假的：
- `chatWithMultiAgent` 先完整执行 Agent（可能 9-15 秒），然后把最终结果拆成几个 chunk 发送
- `chatWithSingleAgent` 同样先跑完 ReAct 循环再发
- 前端用户在这 9+ 秒内看到空白等待，没有任何反馈

### 目标

用户发送消息后，前端**立即**开始收到 token 流，类似 ChatGPT 体验。

### 实现方案

#### 方案：逐 Step 流式推送

**核心思想**：Agent 执行过程是 ReAct 循环（LLM 调用 → 工具调用 → 观察 → 再 LLM），每个 LLM 调用本身支持 token 级流式。我们把整个执行过程拆成多个阶段，每个阶段的输出实时推送。

#### Step 1：修改 chat-service gRPC proto

**文件**：`proto/chat/chat.proto`

当前 `ChatStream` 返回 `stream ChatStreamChunk`，Chunk 只有 `content` 和 `done` 字段。需要扩展：

```protobuf
message ChatStreamChunk {
  string content = 1;
  bool done = 2;
  
  // 新增字段
  string chunk_type = 3;      // "token" | "think" | "tool_start" | "tool_result" | "handoff" | "final"
  string agent_id = 4;        // 当前 Agent ID
  int32 step = 5;             // 当前步数
  AgentState agent_state = 6; // 完整的 AgentState（think/action/result）
  ToolCallInfo tool_call = 7; // 工具调用信息
  string trace_id = 8;        // 追踪 ID
}

message ToolCallInfo {
  string id = 1;
  string name = 2;
  string arguments = 3;
  string result = 4;
  string status = 5;
}
```

#### Step 2：修改 agent-service ExecuteStream

**文件**：`services/agent-service/internal/service/agent_service.go`

当前 `ExecuteStream` 只是跑完 `Execute()` 再 stream 结果。改为：

```go
func (s *AgentService) ExecuteStream(req *pb.ExecuteRequest, stream pb.AgentService_ExecuteStreamServer) error {
    // 1. 创建 Engine 和 ExecutionContext（同 Execute）
    // 2. 逐步执行 executeLoop，每个阶段发送一个 stream chunk：
    
    for step := 0; step < maxSteps; step++ {
        // LLM 调用开始 → 发送 chunk_type="think"
        stream.Send(&pb.ExecuteStreamChunk{
            ChunkType: "think",
            AgentId:   currentAgent.ID,
            Step:      step,
            Content:   "正在思考...",
        })
        
        // LLM 流式响应 → 逐 token 发送 chunk_type="token"
        // 使用 llmClient.ChatStream() 而不是 llmClient.Chat()
        for tokenChunk := range llmStreamCh {
            stream.Send(&pb.ExecuteStreamChunk{
                ChunkType: "token",
                Content:   tokenChunk.Content,
                AgentId:   currentAgent.ID,
                Step:      step,
            })
        }
        
        // 工具调用 → 发送 chunk_type="tool_start"
        for _, tc := range llmResp.ToolCalls {
            stream.Send(&pb.ExecuteStreamChunk{
                ChunkType: "tool_start",
                ToolCall:  &pb.ToolCallInfo{Name: tc.Name, Arguments: tc.Arguments},
                Step:      step,
            })
            
            // 执行工具
            result := tools.Execute(ctx, tc.Name, tc.Arguments, toolCfg)
            
            // 工具结果 → 发送 chunk_type="tool_result"
            stream.Send(&pb.ExecuteStreamChunk{
                ChunkType: "tool_result",
                ToolCall:  &pb.ToolCallInfo{Name: tc.Name, Result: result, Status: "completed"},
                Step:      step,
            })
        }
        
        // Handoff → 发送 chunk_type="handoff"
        // 完成 → 发送 chunk_type="final" + done=true
    }
}
```

#### Step 3：修改 chat-service ChatStream handler

**文件**：`services/chat-service/internal/handler/grpc_handler.go`

当前 `ChatStream` 调用 `Chat()` 再分块发。改为：

```go
func (h *ChatHandler) ChatStream(req *pb.ChatStreamRequest, stream pb.ChatService_ChatStreamServer) error {
    // 如果启用 Agent：调用 agentClient.ExecuteStream()
    //   每收到一个 stream chunk，转换为 ChatStreamChunk 发给前端
    // 如果单 Agent：类似，但用本地 executeAgentLoop + 流式 LLM
}
```

#### Step 4：修改前端 SSE 解析

**文件**：`frontend/src/pages/Chat/index.tsx` 或 `frontend/src/api/chat.ts`

当前 SSE 解析只处理 `content` 字段。需要按 `chunk_type` 分发：

```typescript
// SSE 消息格式: data: {"content":"...", "chunk_type":"token", "agent_id":"...", "step":3}

eventSource.onmessage = (event) => {
  const chunk = JSON.parse(event.data);
  
  switch (chunk.chunk_type) {
    case 'token':
      // 拼接到当前响应
      setResponse(prev => prev + chunk.content);
      break;
    case 'think':
      // 显示思考指示器
      setThinkingAgent(chunk.agent_id);
      break;
    case 'tool_start':
      // 显示工具调用卡片
      addToolCall(chunk.tool_call);
      break;
    case 'tool_result':
      // 更新工具结果
      updateToolResult(chunk.tool_call);
      break;
    case 'handoff':
      // 显示 Agent 转交指示
      setCurrentAgent(chunk.agent_id);
      break;
    case 'final':
      // 完成
      setIsStreaming(false);
      break;
  }
};
```

#### 关键文件

| 文件 | 改动类型 |
|------|---------|
| `proto/chat/chat.proto` | 修改（扩展 ChatStreamChunk） |
| `proto/agent/agent.proto` | 修改（扩展 ExecuteStreamChunk） |
| `pkg/pb/chat/chat.pb.go` | 重新生成 |
| `pkg/pb/agent/agent.pb.go` | 重新生成 |
| `services/agent-service/internal/service/agent_service.go` | 修改（ExecuteStream 真流式） |
| `services/chat-service/internal/handler/grpc_handler.go` | 修改（ChatStream 真流式） |
| `services/chat-service/internal/service/chat_service.go` | 修改（流式 SSE 生成） |
| `services/gateway/internal/handler/chat_handler.go` | 修改（SSE 代理） |
| `frontend/src/api/chat.ts` | 修改（SSE chunk_type 解析） |
| `frontend/src/pages/Chat/index.tsx` | 修改（按类型显示流式状态） |

#### 验证

1. 发送聊天请求 → 前端应在 1 秒内开始看到 token 流
2. Agent 思考阶段 → 显示"正在思考..."指示器
3. 工具调用 → 显示工具名称和参数
4. 工具结果 → 显示观察结果
5. Handoff → 显示 Agent 转交
6. 完成 → 流式结束

---

## P0-2：Knowledge 服务接线

### 问题

前端有完整的 Knowledge 页面（文档上传、列表、搜索），但 Gateway 的 KnowledgeHandler 返回硬编码空数组。knowledge-service 在 Docker 中运行，有完整的 gRPC 实现（Upload/List/Search/Delete），但 Gateway 不代理到它。

### 实现方案

#### Step 1：替换 Gateway KnowledgeHandler

**文件**：`services/gateway/internal/handler/all_handlers.go`

当前 `KnowledgeHandler` 只有 `cfg *config.Config`，每个方法返回 stub JSON。改为注入 gRPC client：

```go
type KnowledgeHandler struct {
    cfg     *config.Config
    client  knowledgepb.KnowledgeServiceClient  // 新增
}
```

在 `main.go` 或 handler 初始化时创建 gRPC connection：

```go
knowledgeConn, err := grpc.Dial("knowledge-service:50002", grpcOpts...)
knowledgeClient := knowledgepb.NewKnowledgeServiceClient(knowledgeConn)
knowledgeHandler := &KnowledgeHandler{cfg: cfg, client: knowledgeClient}
```

#### Step 2：重写每个方法

**Upload**：

```go
func (h *KnowledgeHandler) Upload(c *gin.Context) {
    // 1. 从 HTTP request 解析 multipart form
    file, header, err := c.Request.FormFile("file")
    // 2. 读取文件内容
    content, _ := io.ReadAll(file)
    // 3. 构建 protobuf UploadRequest
    req := &knowledgepb.UploadRequest{
        FileName:    header.Filename,
        Content:     content,
        ContentType: header.Header.Get("Content-Type"),
        TenantId:    c.GetString("tenant_id"),
        ChunkSize:   int32(chunkSize),  // 从 query param 获取
        ChunkOverlap: int32(overlap),
    }
    // 4. gRPC 调用
    resp, err := h.client.Upload(c.Request.Context(), req)
    // 5. 返回 JSON
    c.JSON(200, gin.H{
        "document_id": resp.DocumentId,
        "chunk_count": resp.ChunkCount,
        "status":      resp.Status,
    })
}
```

**ListDocuments / GetDocument / DeleteDocument / Search** — 类似模式：HTTP → protobuf → gRPC → JSON。

#### 关键文件

| 文件 | 改动类型 |
|------|---------|
| `services/gateway/internal/handler/all_handlers.go` | 修改（KnowledgeHandler 注入 gRPC client + 重写所有方法） |
| `services/gateway/cmd/main.go` | 修改（创建 knowledge-service gRPC connection） |
| `proto/knowledge/knowledge.proto` | 确认已有完整定义 |

#### 验证

1. 前端上传 PDF → Gateway → knowledge-service → 返回 document_id + chunk_count
2. 列表 → 返回真实文档列表
3. 搜索 → 返回向量搜索结果

---

## P0-3：人工审批接入

### 问题

`pkg/agent/approval/flow.go` 有完整的审批流程代码，但 `engine.go` 从不调用。敏感操作（删除数据、执行代码、浏览器操作）没有审批拦截。

### 实现方案

#### Step 1：在 Engine 中添加审批字段

**文件**：`pkg/agent/engine.go`

```go
type Engine struct {
    registry        *Registry
    llmClient       llm.Client
    tools           ToolExecutor
    config          EngineConfig
    store           ContextStore
    
    // 新增
    approvalManager *approval.ApprovalFlowManager
    ruleEngine      *approval.RuleEngine
}
```

初始化时创建：

```go
func NewEngine(registry *Registry, llmClient llm.Client, tools ToolExecutor, opts ...EngineOption) *Engine {
    e := &Engine{...}
    e.approvalManager = approval.NewApprovalFlowManager(approval.DefaultConfig())
    e.ruleEngine = approval.NewRuleEngine()
    
    // 注册默认规则：高风险操作需要审批
    e.ruleEngine.AddRule(&approval.ApprovalRule{
        Type:         approval.ApprovalTypeToolCall,
        ToolName:     "code_execute",
        RiskThreshold: 0.5,
        AutoApprove:  false,
        Enabled:      true,
    })
    e.ruleEngine.AddRule(&approval.ApprovalRule{
        Type:         approval.ApprovalTypeToolCall,
        ToolName:     "browser_*",  // 所有浏览器操作
        RiskThreshold: 0.3,
        AutoApprove:  false,
        Enabled:      true,
    })
}
```

#### Step 2：在 executeLoop 工具执行前插入审批检查

**位置**：`engine.go` 的 `executeLoop` 中，当前代码约第 253 行：

```go
// 当前代码（直接执行）
result, err := e.tools.Execute(ctx, tc.Name, tc.Arguments, toolCfg)
```

改为：

```go
// 审批检查
needsApproval, rule := e.ruleEngine.NeedsApproval(currentAgent.ID, tc.Name, tcContext)
if needsApproval {
    // 创建审批请求
    approvalReq := &approval.ApprovalRequest{
        Type:        approval.ApprovalTypeToolCall,
        Priority:    approval.PriorityMedium,
        AgentID:     currentAgent.ID,
        SessionID:   execCtx.SessionID,
        Description: fmt.Sprintf("Agent %s 请求执行工具 %s", currentAgent.Name, tc.Name),
        RiskLevel:   rule.RiskThreshold,
        Details:     tc.Arguments,
    }
    createdReq, err := e.approvalManager.CreateRequest(ctx, approvalReq)
    if err != nil {
        // 审批系统故障，根据配置决定是否自动批准
        if rule.AutoApproveOnError {
            result, err = e.tools.Execute(ctx, tc.Name, tc.Arguments, toolCfg)
        } else {
            result = "审批系统故障，操作被拒绝"
        }
    } else {
        // 等待审批决策
        decision, err := e.approvalManager.WaitForApproval(ctx, createdReq.ID)
        if err != nil || decision.Decision == approval.StatusRejected {
            // 拒绝 — 记录拒绝作为工具结果
            result = fmt.Sprintf("操作被拒绝: %s", decision.Reason)
            execCtx.AddMessage(fmt.Sprintf("工具 %s 的执行被拒绝", tc.Name), "system")
        } else {
            // 批准 — 可能修改了参数
            if decision.ModifiedParams != nil {
                tc.Arguments = decision.ModifiedParams
            }
            result, err = e.tools.Execute(ctx, tc.Name, tc.Arguments, toolCfg)
        }
    }
} else {
    // 不需要审批，直接执行
    result, err = e.tools.Execute(ctx, tc.Name, tc.Arguments, toolCfg)
}
```

#### Step 3：暴露审批 API

**文件**：`services/gateway/internal/router/router.go`

添加审批管理路由：

```go
// Approval management
api.POST("/api/v2/harness/approval/rules", harnessHandler.CreateApprovalRule)
api.GET("/api/v2/harness/approval/rules", harnessHandler.ListApprovalRules)
api.POST("/api/v2/harness/approval/pending", harnessHandler.ListPendingApprovals)
api.POST("/api/v2/harness/approval/:id/approve", harnessHandler.ApproveRequest)
api.POST("/api/v2/harness/approval/:id/reject", harnessHandler.RejectRequest)
```

#### Step 4：前端审批 UI

在 Chat 页面或 Harness 页面添加审批面板：
- 显示待审批请求列表
- Approve / Reject 按钮
- 可选：修改参数后批准

#### 关键文件

| 文件 | 改动类型 |
|------|---------|
| `pkg/agent/engine.go` | 修改（添加审批字段 + executeLoop 中插入审批检查） |
| `services/agent-service/internal/service/agent_service.go` | 修改（初始化审批 Manager 和 RuleEngine） |
| `services/gateway/internal/router/router.go` | 修改（添加审批 API 路由） |
| `services/gateway/internal/handler/harness_handler.go` | 修改（添加审批 Handler 方法） |
| `frontend/src/pages/Chat/index.tsx` | 修改（审批请求通知 UI） |
| `frontend/src/pages/Harness/index.tsx` | 修改（审批规则管理 + 待审批列表） |

#### 验证

1. 发送聊天请求，Agent 执行 code_execute → 请求被拦截，等待审批
2. 前端显示审批请求 → 用户点击 Approve → 工具继续执行
3. 用户点击 Reject → 工具不执行，Agent 收到拒绝反馈
4. 低风险工具（如 web_search）→ 自动批准，无需等待

---

## P0-4：分布式追踪（Tracing）

### 问题

当前只有 LLM Metrics（成本/延迟/Token），没有 Span/Trace 体系。无法看到一个请求在 gateway → chat-service → agent-service → mcp-service 之间的调用链和耗时分布。

### 实现方案

#### Step 1：引入 OpenTelemetry

**新增依赖**：每个 Go 服务添加：

```go
// go.mod
go.opentelemetry.io/otel
go.opentelemetry.io/otel/trace
go.opentelemetry.io/otel/exporters/otlp/otlptracegrpc
go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc
```

#### Step 2：配置 OTEL Collector

**文件**：`docker/docker-compose.yaml`

新增 OTEL Collector 服务：

```yaml
otel-collector:
  image: otel/opentelemetry-collector-contrib:0.96.0
  ports:
    - "4317:4317"   # gRPC receiver
    - "4318:4318"   # HTTP receiver
  volumes:
    - ./otel-config.yaml:/etc/otelcol-contrib/config.yaml
```

**新增文件**：`docker/otel-config.yaml`

```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318

exporters:
  prometheus:
    endpoint: "0.0.0.0:8889"
  logging:
    loglevel: debug

processors:
  batch:

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [logging]
    metrics:
      receivers: [otlp]
      processors: [batch]
      exporters: [prometheus, logging]
```

#### Step 3：每个服务初始化 TracerProvider

**文件**：每个服务的 `cmd/main.go`

```go
func initTracer() (*sdktrace.TracerProvider, error) {
    exporter, err := otlptracegrpc.New(context.Background(),
        otlptracegrpc.WithEndpoint("otel-collector:4317"),
        otlptracegrpc.WithInsecure(),
    )
    if err != nil {
        return nil, err
    }
    tp := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(exporter),
        sdktrace.WithResource(resource.NewWithAttributes(
            semconv.ServiceNameKey.String("chat-service"),
        )),
    )
    otel.SetTracerProvider(tp)
    return tp, nil
}
```

#### Step 4：gRPC Interceptor 自动追踪

**文件**：每个服务的 gRPC server/client 初始化

```go
// Server interceptor — 自动为每个 gRPC 方法创建 Span
srv := grpc.NewServer(
    grpc.UnaryInterceptor(otelgrpc.UnaryServerInterceptor()),
    grpc.StreamInterceptor(otelgrpc.StreamServerInterceptor()),
)

// Client interceptor — 自动为每个 gRPC 调用创建 Span
conn, err := grpc.Dial(
    target,
    grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()),
    grpc.WithStreamInterceptor(otelgrpc.StreamClientInterceptor()),
)
```

#### Step 5：关键业务逻辑手动添加 Span

**文件**：`services/chat-service/internal/service/chat_service.go`

```go
func (s *ChatService) Chat(ctx context.Context, req *pb.ChatRequest) (*pb.ChatResponse, error) {
    ctx, span := otel.Tracer("chat-service").Start(ctx, "ChatService.Chat",
        trace.WithAttributes(attribute.String("session_id", req.SessionId)),
    )
    defer span.End()
    
    // guardrail 检查
    ctx, guardSpan := otel.Tracer("chat-service").Start(ctx, "checkGuardrail")
    err := s.checkRules(ctx, req.Message)
    guardSpan.End()
    if err != nil { span.SetStatus(codes.Error, err.Error()) }
    
    // agent 执行
    ctx, agentSpan := otel.Tracer("chat-service").Start(ctx, "chatWithMultiAgent")
    resp, err := s.chatWithMultiAgent(ctx, req)
    agentSpan.End()
}
```

#### Step 6：Gateway SSE 追踪

**文件**：`services/gateway/internal/handler/chat_handler.go`

在 SSE 流式响应中添加 trace_id header：

```go
func (h *ChatHandler) ChatStream(c *gin.Context) {
    traceID := otel.Tracer("gateway").Start(c.Request.Context(), "ChatStream").SpanContext().TraceID()
    c.Header("X-Trace-ID", traceID.String())
    // SSE 推送中每个 chunk 包含 trace_id
}
```

#### Step 7：前端展示 Trace

**文件**：`frontend/src/pages/Observability/TraceViewer.tsx`

当前 TraceViewer 是 mock。改为从真实 Trace API 查询：

```typescript
// 替换 mock 数据为真实 API
const traces = await client.get("/api/v2/observability/traces", {
  params: { limit: 50, service: "chat-service" }
});
```

#### 关键文件

| 文件 | 改动类型 |
|------|---------|
| 每个 `cmd/main.go` | 修改（初始化 TracerProvider） |
| 每个 gRPC server/client | 修改（添加 OTEL interceptor） |
| `docker/docker-compose.yaml` | 修改（新增 otel-collector） |
| `docker/otel-config.yaml` | 新建 |
| `services/chat-service/internal/service/chat_service.go` | 修改（关键路径添加 Span） |
| `services/agent-service/` | 修改（engine 循环添加 Span） |
| `services/gateway/internal/handler/chat_handler.go` | 修改（SSE 添加 trace_id） |
| `frontend/src/pages/Observability/TraceViewer.tsx` | 修改（从真实 API 查数据） |

#### 验证

1. 发送聊天请求 → OTEL Collector 收到完整 Trace（gateway → chat → agent → mcp）
2. 前端 Observability 页面 → 显示真实 Trace 数据（调用链 + 耗时）
3. 错误请求 → Trace 中标记 Error Span
4. 性能慢的请求 → Trace 中显示耗时瓶颈在哪一步
