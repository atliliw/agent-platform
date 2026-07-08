# 实施路线图

> 从 P0 到 P2 的完整实施步骤，按依赖关系排序，每步包含后端+前端的具体操作。

---

## 总览

```
Phase 1 (P0) ── 5-7天 ── 基础可用
  Step 1: Knowledge 接线 (1天)
  Step 2: Chat 真流式 (1.5天)
  Step 3: 人工审批接入 (1天)
  Step 4: 分布式追踪 (2天)
  
Phase 2 (P1) ── 6-8天 ── 能力提升
  Step 5: Harness 页面拆分 + API 统一 (0.5天)
  Step 6: 并行工具执行接入 (0.5天)
  Step 7: 分层记忆接入 (2天)
  Step 8: Checkpoint 持久化 (1.5天)
  Step 9: Workflow 图编排 (2-3天)
  
Phase 3 (P2) ── 按需 ── 锦上添花
  Step 10: 反思循环接入 (1天)
  Step 11: 实时干预接入 (1天)
  Step 12: Harness 补全 UI (2-3天)
```

---

## Phase 1：P0 基础可用

### Step 1：Knowledge 服务接线（1天）

**为什么先做这个**：改动最小（只改 Gateway），效果最明显（前端 Knowledge 页面瞬间可用），不依赖其他步骤。

#### 后端操作

```
1. 打开 services/gateway/internal/handler/all_handlers.go
2. KnowledgeHandler 结构体添加 knowledgepb.KnowledgeServiceClient 字段
3. 5 个方法全部重写：
   - Upload:   multipart form → protobuf UploadRequest → gRPC → JSON
   - ListDocuments:  query params → protobuf ListDocumentsRequest → gRPC → JSON
   - GetDocument:    path param id → protobuf GetDocumentRequest → gRPC → JSON
   - DeleteDocument: path param id → protobuf DeleteDocumentRequest → gRPC → JSON
   - Search:         body + query → protobuf SearchRequest → gRPC → JSON
4. 打开 services/gateway/cmd/main.go
5. 添加 knowledge-service gRPC 连接初始化：
   knowledgeConn := grpc.Dial("knowledge-service:50002", grpcOpts...)
   knowledgeClient := knowledgepb.NewKnowledgeServiceClient(knowledgeConn)
6. 传入 KnowledgeHandler
```

#### 前端操作

```
无需改动 — Knowledge 页面 UI 已完整，只是后端返回空数据
```

#### 构建部署

```bash
cd docker
docker compose build --no-cache gateway
docker compose up -d gateway
```

#### 验证

```
1. 前端打开 Knowledge 页面
2. 上传一个 PDF → 返回 document_id + chunk_count
3. 搜索 "关键词" → 返回匹配的文档片段
4. 列表 → 显示上传的文档
```

---

### Step 2：Chat 真流式（1.5天）

**依赖**：无。但最好在 Step 1 之后做，因为 Knowledge 可用后流式测试更有意义。

#### 第 1 天：后端

```
1. proto/chat/chat.proto — 扩展 ChatStreamChunk：
   添加 chunk_type, agent_id, step, tool_call 字段

2. proto/agent/agent.proto — 扩展 ExecuteStreamChunk：
   添加 chunk_type, agent_id, step 字段

3. 重新生成 pb.go：
   protoc --go_out=. --go-grpc_out=. proto/chat/chat.proto
   protoc --go_out=. --go-grpc_out=. proto/agent/agent.proto

4. services/agent-service/internal/service/agent_service.go
   重写 ExecuteStream：
   - 不再跑完 Execute 再 stream
   - 改为逐步推送：
     * LLM 调用前 → Send chunk_type="think"
     * LLM 流式 token → Send chunk_type="token"（逐 token）
     * 工具调用前 → Send chunk_type="tool_start"
     * 工具结果   → Send chunk_type="tool_result"
     * Handoff    → Send chunk_type="handoff"
     * 完成       → Send chunk_type="final" + done=true
   - 关键：LLM 调用改用 llmClient.ChatStream() 代替 llmClient.Chat()

5. services/chat-service/internal/handler/grpc_handler.go
   重写 ChatStream handler：
   - 如果启用 agent → 调用 agentClient.ExecuteStream()
   - 每收到一个 stream chunk → 转为 ChatStreamChunk 推送
   - 如果单 Agent → 本地 executeAgentLoop + 流式推送

6. services/chat-service/internal/service/chat_service.go
   chatWithMultiAgent 内部增加流式支持：
   - 保留原有完整执行逻辑（用于非流式 Chat RPC）
   - ChatStream RPC 调用新的流式路径

7. services/gateway/internal/handler/chat_handler.go
   SSE 代理改造：
   - 从 chat-service gRPC stream 逐 chunk 读取
   - 转为 SSE data: 行推送给前端
   - 添加 X-Trace-ID header
```

#### 第 1.5 天：前端

```
1. frontend/src/api/chat.ts
   重写 chatStream 函数：
   - 添加 Authorization + X-Tenant-ID headers
   - 按 chunk_type 分发：token / think / tool_start / tool_result / handoff / final

2. frontend/src/stores/chatStore.ts
   重写 sendMessage：
   - 先添加空 assistant 消息占位
   - 调用 chatApi.chatStream()
   - onToken → 逐步追加 content
   - onAgentEvent → 更新 agent_trace / tool_calls
   - onDone → 标记 isStreaming=false

3. frontend/src/components/Chat/ChatMessage.tsx
   添加 StreamingMarkdown：
   - 流式中未闭合的代码块临时补上 ```
   - 避免渲染崩溃

4. frontend/src/pages/Chat/index.tsx
   添加流式状态栏：
   - 思考中：显示 Agent 名称 + "正在思考..."
   - 工具调用中：显示工具名称 tag
   - Handoff：显示转交提示

5. npx vite build && scp dist 到服务器
```

#### 构建部署

```bash
docker compose build --no-cache chat-service agent-service gateway
docker compose up -d chat-service agent-service gateway
# 前端单独 build + scp
```

#### 验证

```
1. 发送聊天请求 → 1秒内前端开始收到 token
2. Agent 思考 → 显示 "正在思考..."
3. 工具调用 → 显示工具名称
4. 工具结果 → 显示观察结果
5. Handoff → 显示转交提示
6. 完成 → 流式正常结束
7. 对比旧版：不再 9秒空白等待
```

---

### Step 3：人工审批接入（1天）

**依赖**：Step 2（流式做完后，审批请求可以实时推送通知）

#### 后端操作

```
1. pkg/agent/engine.go
   - Engine 结构体添加 approvalManager + ruleEngine 字段
   - NewEngine 初始化时创建：
     * ApprovalFlowManager + 默认规则（code_execute/browser_* 需审批）
     * RuleEngine + AddRule
   - executeLoop 中工具执行前插入审批检查（约第 253 行）：
     * ruleEngine.NeedsApproval() → 是否需要审批
     * approvalManager.CreateRequest() → 创建审批请求
     * approvalManager.WaitForApproval() → 阻塞等待
     * 检查 decision → approve 执行 / reject 跳过

2. services/agent-service/internal/service/agent_service.go
   - 初始化 Engine 时传入审批组件
   - 或在 agent_service adapter 中构建

3. services/gateway/internal/router/router.go
   添加审批路由：
   POST /api/v2/harness/approval/rules
   GET  /api/v2/harness/approval/rules
   POST /api/v2/harness/approval/pending
   POST /api/v2/harness/approval/:id/approve
   POST /api/v2/harness/approval/:id/reject

4. services/gateway/internal/handler/harness_handler.go
   添加 5 个审批 Handler 方法
   - 代理到 harness-service 或直接调用 approval 包

5. proto/harness/harness.proto
   添加审批相关 RPC 和消息定义（如果走 harness-service）
```

#### 前端操作

```
1. frontend/src/api/approval.ts — 新建审批 API 模块
2. frontend/src/components/Chat/ApprovalNotification.tsx — 新建审批通知卡片
3. frontend/src/pages/Chat/index.tsx
   在消息列表中检测 tool_calls.status === 'pending_approval'
   插入 ApprovalNotification 组件
4. frontend/src/pages/Harness/index.tsx
   添加第 7 个 Tab "审批管理"
5. frontend/src/pages/Harness/ApprovalPanel.tsx — 新建
   规则列表 + 待审批列表（3秒轮询） + approve/reject 按钮

6. npx vite build && scp dist 到服务器
```

#### 构建部署

```bash
# protoc 重新生成 harness pb
docker compose build --no-cache agent-service gateway
docker compose up -d agent-service gateway
# 前端单独部署
```

#### 验证

```
1. 发送包含 code_execute 的对话
2. → 请求被拦截，Chat 页面显示审批卡片
3. 点击 "批准" → 工具继续执行
4. 点击 "拒绝" → Agent 收到拒绝反馈，调整策略
5. Harness 审批管理 Tab → 显示规则列表 + 待审批列表
6. web_search 等低风险工具 → 自动批准，无需等待
```

---

### Step 4：分布式追踪（2天）

**依赖**：Step 2（真流式做完后，SSE 推送中可以包含 trace_id）

#### 第 1 天：后端 OTEL 基础设施

```
1. 每个服务的 go.mod 添加依赖：
   go.opentelemetry.io/otel
   go.opentelemetry.io/otel/trace
   go.opentelemetry.io/otel/exporters/otlp/otlptracegrpc
   go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc

2. docker/docker-compose.yaml
   新增 otel-collector 服务：
   image: otel/opentelemetry-collector-contrib:0.96.0
   ports: 4317(gRPC) 4318(HTTP)

3. docker/otel-config.yaml — 新建
   receivers: otlp (grpc+http)
   exporters: prometheus(8889) + logging
   processors: batch

4. 每个服务的 cmd/main.go
   initTracer() 函数：
   - 创建 otlptracegrpc exporter → 连接 otel-collector:4317
   - 创建 TracerProvider + Batcher
   - otel.SetTracerProvider(tp)
   - defer tp.Shutdown()

5. 每个服务的 gRPC server/client 初始化
   添加 OTEL interceptor：
   - Server: grpc.UnaryInterceptor(otelgrpc.UnaryServerInterceptor())
   - Client: grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor())
```

#### 第 2 天：业务 Span + 前端

```
6. services/chat-service/internal/service/chat_service.go
   关键路径添加手动 Span：
   - ChatService.Chat → "chat-service.Chat"
   - chatWithMultiAgent → "chat-with-multi-agent"
   - checkRules → "check-guardrail"
   - recallMemories → "recall-memories"
   - 每个 Span 添加 attributes（session_id, model, agent_id）

7. pkg/agent/engine.go
   executeLoop 添加 Span：
   - "agent.execute-loop" + step attribute
   - "agent.llm-call" + model attribute
   - "agent.tool-call" + tool_name attribute

8. services/gateway/internal/handler/chat_handler.go
   SSE 推送添加 trace_id：
   c.Header("X-Trace-ID", span.SpanContext().TraceID().String())
   每个 SSE chunk 包含 trace_id 字段

9. frontend/src/pages/Observability/TraceViewer.tsx
   替换 mock 数据为真实 API：
   - GET /api/v2/observability/traces → 从 OTEL 获取
   - 降级：从 harness LLM metrics 构造 trace-like 数据
   - 添加 Trace 详情展开（Span 时间线）

10. frontend/src/pages/Observability/CostDashboard.tsx
    替换 mock 为 harness cost API

11. npx vite build && scp dist 到服务器
```

#### 构建部署

```bash
# 先启动 otel-collector
docker compose up -d otel-collector

# 重建所有服务（添加了 OTEL 依赖）
docker compose build --no-cache chat-service agent-service gateway harness-service memory-service knowledge-service
docker compose up -d

# 前端单独部署
```

#### 验证

```
1. 发送聊天请求
2. otel-collector 日志 → 应收到完整 Trace
3. 前端 Observability → TraceViewer 显示真实 Trace 数据
4. 点击 Trace → 展开调用链（gateway → chat → agent → mcp）
5. 每个节点显示耗时
6. 错误请求 → Trace 中标记 Error Span
```

---

## Phase 2：P1 能力提升

### Step 5：Harness 页面拆分 + API 统一（0.5天）

**依赖**：无，但最好在 Phase 1 之后做，因为 Harness 页面要加审批 Tab（Step 3）

#### 操作

```
1. frontend/src/api/harness.ts — 新建统一 API 模块
   汇聚所有 Harness 相关端点（rules/ab/slo/cost/proposals/approval）

2. frontend/src/pages/Harness/index.tsx
   拆分 1076 行为独立组件文件：
   - DashboardPanel.tsx
   - RulesPanel.tsx
   - ABTestPanel.tsx
   - SLOPanel.tsx
   - CostPanel.tsx
   - ProposalsPanel.tsx
   - ApprovalPanel.tsx（Step 3 已创建）
   主页面只做 Tab 容器（~100行）

3. 每个 Panel 组件：
   - 将内联的 client.get/post 替换为 harnessApi.xxx()
   - 去掉 res?.data?.x || res?.x 的防御性代码（拦截器已处理）

4. npx vite build && scp dist 到服务器
```

#### 验证

```
1. Harness 页面各 Tab 功能不变
2. 代码可维护性提升（每个 Panel 150-300 行 vs 之前 1076 行）
3. API 调用风格统一
```

---

### Step 6：并行工具执行接入（0.5天）

**依赖**：无

#### 操作

```
1. pkg/agent/engine.go
   executeLoop 中工具执行循环（约第 213-298 行）：
   - 分离 handoffCalls 和 regularCalls
   - regularCalls → 构建 ParallelRequest 数组
   - 调用 ParallelExecutor.Execute() 并行执行
   - handoffCalls → 串行处理（原有逻辑）
   
2. services/agent-service/internal/service/agent_service.go
   初始化 Engine 时创建 ParallelExecutor

3. 构建部署：
   docker compose build --no-cache agent-service
   docker compose up -d agent-service
```

#### 验证

```
1. 发送需要多个工具的请求
2. 3 个工具（web_search + knowledge_search + weather）→ 并行执行
3. 总耗时 ≈ 最慢的工具（不是三者之和）
4. Handoff 仍然串行处理
5. 单工具调用 → 行为不变（退化为串行）
```

---

### Step 7：分层记忆接入（2天）

#### 第 1 天：后端

```
1. proto/memory/memory.proto
   添加新 RPC：
   - StoreEpisode / GetTimeline / GetSimilarEpisodes
   - StoreConcept / StoreRelation / RecallConcepts / GetRelatedConcepts / GetConceptPath
   - AddWorkingMessage / GetWorkingContext / GetWorkingMessagesForLLM / ClearWorkingContext
   - GetForgettingConfig / UpdateForgettingConfig / TriggerCleanup

2. 重新生成 pb.go

3. services/memory-service/internal/handler/grpc_handler.go
   - 注入 episodicMemory / semanticMemory / workingMemory 实例
   - 实现 15+ 个新 RPC handler
   - 切换基础 MemoryService 为 MemoryServiceWithForgetting

4. services/memory-service/cmd/main.go
   - 初始化 EpisodicMemory / SemanticMemory / WorkingMemory
   - 传入 grpc_handler

5. services/chat-service/internal/service/chat_service.go
   - buildMessages() 改为调用 workingMemory.GetMessagesForLLM()
   - chatWithMultiAgent 完成后调用 StoreEpisode()

6. services/gateway/internal/handler/memory_handler.go
   代理新的记忆 RPC 到 HTTP API

7. services/gateway/internal/router/router.go
   添加记忆增强路由

8. 构建部署
```

#### 第 2 天：前端

```
1. frontend/src/pages/Memory/index.tsx
   添加 4 Tab：基础 / 情节 / 语义图谱 / 工作记忆

2. frontend/src/pages/Memory/EpisodicMemoryPanel.tsx — 新建
   时间线 + 统计卡片

3. frontend/src/pages/Memory/SemanticGraphPanel.tsx — 新建
   React Flow 知识图谱

4. frontend/src/pages/Memory/WorkingMemoryPanel.tsx — 新建
   Token 使用进度 + 消息列表 + 重要性标记

5. frontend/src/pages/Observability/MemoryManager.tsx
   替换 mock 数据为真实记忆 API

6. npx vite build && scp dist 到服务器
```

#### 验证

```
1. 多轮对话 → 工作记忆压缩上下文，Agent 理解早期内容
2. 对话完成 → 情节记忆自动存储时间线
3. 新对话 → 召回历史情节
4. Memory 页面 → 4 Tab 各显示对应数据
5. 语义图谱 → 概念和关系可视化
6. 遗忘 → 30天后低重要性记忆自动衰减
```

---

### Step 8：Checkpoint 持久化（1.5天）

#### 第 1 天：后端

```
1. pkg/agent/checkpoint/checkpoint.go — 新建
   Checkpoint 数据结构 + MongoDB Store

2. pkg/agent/engine.go
   - Engine 结构体添加 checkpointStore 字段
   - executeLoop 每步完成后保存 Checkpoint
   - 新方法 ResumeFromCheckpoint()

3. services/agent-service/internal/service/agent_service.go
   初始化 checkpointStore（MongoDB）

4. proto/harness/harness.proto 或 proto/agent/agent.proto
   添加 Checkpoint 相关 RPC

5. services/gateway/ — 添加 Checkpoint HTTP 路由
   GET  /api/v2/harness/session/:id/checkpoints
   GET  /api/v2/harness/session/:id/checkpoint/:cpid
   POST /api/v2/harness/session/:id/checkpoint/:cpid/resume

6. 构建部署
```

#### 第 0.5 天：前端

```
1. frontend/src/pages/Session/Replay.tsx
   - 添加 checkpoints state
   - Progress bar 上叠加紫色圆点标记
   - 点击圆点 → 弹出恢复确认框

2. frontend/src/api/session.ts
   添加 Checkpoint API 方法

3. npx vite build && scp dist
```

#### 验证

```
1. Agent 执行到第 3 步 → 查 DB 有 Checkpoint 记录
2. 服务重启 → 前端点击 "从 Step 3 恢复" → Agent 继续执行
3. Session Replay → Progress bar 上有紫色圆点
4. Checkpoint 列表 → 显示每步状态
```

---

### Step 9：Workflow 图编排（2-3天）

#### 第 1 天：后端

```
1. pkg/agent/workflow/dag.go — 新建
   Workflow / Node / Edge 数据结构

2. pkg/agent/workflow/executor.go — 新建
   WorkflowExecutor.Execute() — 按图拓扑顺序执行
   条件节点 → evaluateCondition (expr 库)
   并行节点 → goroutine fan-out
   合并节点 → 收集并行结果

3. services/harness-service/ — 新增 Workflow Repository (MongoDB)
   Save / Get / List / Delete

4. proto/harness/harness.proto — 添加 Workflow RPC
   CreateWorkflow / GetWorkflow / ListWorkflows / DeleteWorkflow / ExecuteWorkflow

5. services/gateway/ — 添加 Workflow HTTP 路由

6. 构建部署
```

#### 第 2-3 天：前端

```
1. npm install reactflow

2. frontend/src/pages/Workflow/index.tsx — 新建
   React Flow DAG 编辑器

3. frontend/src/pages/Workflow/nodes/ — 新建
   AgentNode / ToolNode / ConditionNode / ParallelNode / MergeNode

4. frontend/src/pages/Workflow/NodePanel.tsx — 新建
   左侧可拖拽节点类型列表

5. frontend/src/pages/Workflow/PropertyPanel.tsx — 新建
   右侧选中节点属性编辑

6. frontend/src/api/workflow.ts — 新建
   Workflow API 模块

7. frontend/src/App.tsx
   添加 /workflow 和 /workflow/:id 路由

8. npx vite build && scp dist
```

#### 验证

```
1. 打开 /workflow → 看到空白画布 + 左侧节点面板
2. 拖拽 Agent 节点到画布 → 连线到 Condition → 连线到两个 Agent
3. 保存 → 后端存到 MongoDB
4. 执行 → Agent 按图拓扑顺序运行
5. 条件分支 → 根据结果走不同路径
6. Session Replay → 显示图中的分支路径
```

---

## Phase 3：P2 按需

### Step 10-12：按需实施

| Step | 功能 | 操作要点 |
|------|------|---------|
| 10 | 反思循环 | engine.go 添加 reflectionLoop 字段，executeLoop 三个插入点（postAction/error/complete），前端 Session Replay 展示反思结果 |
| 11 | 实时干预 | engine.go 添加 interventionManager，executeLoop 每步检查干预，前端 Chat 页面添加暂停/停止/恢复按钮 |
| 12 | Harness 补全 UI | Feature Flags / Scheduler / Catalog / Rollback Tab（标准 CRUD 面板） |

---

## 部署节奏

每个 Step 完成后的部署流程：

```bash
# 1. 重新生成 protobuf（如果改了 proto）
cd proto && make generate  # 或手动 protoc

# 2. 构建受影响的 Docker 服务
cd docker
docker compose build --no-cache <affected-services>

# 3. 启动新服务
docker compose up -d <affected-services>

# 4. 前端构建部署
cd frontend && npx vite build
scp -i ~/.ssh/demo_deploy_key -r dist root@192.168.10.100:/opt/agent-platform/frontend/

# 5. 清理旧 JS 文件（如果文件名 hash 变了）
ssh root@192.168.10.100 "rm -f /opt/agent-platform/frontend/dist/assets/*.js"

# 6. 验证
curl http://192.168.10.100:8888  # 前端
curl http://192.168.10.100:9000/health  # gateway
```

---

## 每步验收清单

### Step 1 验收 ✓
- [ ] Knowledge 上传返回 document_id
- [ ] 搜索返回匹配片段
- [ ] 列表显示文档

### Step 2 验收 ✓
- [ ] 发消息 1秒内看到第一个 token
- [ ] Agent 思考显示指示器
- [ ] 工具调用显示名称
- [ ] Handoff 显示转交
- [ ] 流式正常结束

### Step 3 验收 ✓
- [ ] code_execute 被拦截等待审批
- [ ] 批准后继续执行
- [ ] 拒绝后 Agent 调整策略
- [ ] Harness 审批 Tab 显示规则和待审批

### Step 4 验收 ✓
- [ ] OTEL collector 收到 Trace
- [ ] TraceViewer 显示真实数据
- [ ] CostDashboard 显示真实数据
- [ ] 调用链含 gateway→chat→agent→mcp

### Step 5 验收 ✓
- [ ] Harness 各 Tab 功能不变
- [ ] 代码拆分为独立 Panel
- [ ] API 调用统一用 harnessApi

### Step 6 验收 ✓
- [ ] 3 工具并行执行
- [ ] 总耗时 ≈ 最慢工具
- [ ] Handoff 仍串行

### Step 7 验收 ✓
- [ ] 工作记忆压缩上下文
- [ ] 情节记忆存储时间线
- [ ] 语义图谱可视化
- [ ] 遗忘机制生效

### Step 8 验收 ✓
- [ ] 每步有 Checkpoint
- [ ] 可从任意步恢复
- [ ] Replay 有紫色圆点

### Step 9 验收 ✓
- [ ] 可拖拽创建 DAG
- [ ] 条件分支工作
- [ ] 保存和加载
- [ ] 执行按图拓扑顺序
