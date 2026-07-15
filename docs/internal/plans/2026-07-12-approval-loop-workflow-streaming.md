# 审批实时闭环 + Workflow 流式输出 实施计划

> 日期: 2026-07-12
> 范围: 把两个"已有半成品但没接通最后一公里"的功能补成真闭环

---

## 背景(已核实)

两个功能的后端机制都已存在,缺的是"接通":

| 功能 | 已有 | 缺口 |
|---|---|---|
| 审批闭环 | engine block 在 `WaitForApproval`、gateway `/approval/pending`+approve/reject、ApprovalPanel 审批按钮 | 推送/聊天页提示 + `approval` 流事件类型;`SetNotifier` 从未调用、ApprovalPanel 不轮询、ChatStream 无 approval chunk |
| Workflow 流式 | executor 有 `StreamEventFunc`+`onEvent` 发 node_started/completed/error | `SetStreamEventFunc` 从未调用(死代码)、`ExecuteWorkflow` 是 unary、前端 modal 只等最终结果 |

**传输方式**:gateway 已用 SSE(`text/event-stream` + `c.SSEvent`)推 chat 流。两个功能都复用 SSE。

---

## 功能 A:审批实时闭环

### 目标
聊天中 agent 调审批类工具(browser_navigate/click/publish)时,聊天界面**实时弹出审批卡片**,用户点批准/拒绝,agent 继续。不再干等 300s。

### 改动

**A1. `pkg/agent/engine_stream.go`** — 发审批事件
- `StreamEventType` 加 `EventApproval = "approval"`
- `StreamEvent` struct 加字段 `Approval *ApprovalEvent`(新 struct:`RequestID/ToolName/Reason`)
- 在 `CreateRequest`(line 279)之后、`WaitForApproval`(line 286)之前,`callback(StreamEvent{Type: EventApproval, Approval: {createdReq.ID, toolName, reason}})`
- WaitForApproval 返回后,按 decision 发 `EventToolResult`(批准继续 / 拒绝跳过),复用现有路径

**A2. `services/chat-service/internal/service/chat_service.go`** — 转发 approval chunk
- `chatStreamWithMultiAgent` 的 switch 加 `case "approval":`
- `chatChunk.Type = "approval"`,`chatChunk.Content` = JSON(`{request_id, tool_name, reason}`)
- 同样改 `chatWithSingleAgent` 的本地循环路径(若它也走 stream)

**A3. 不改 proto** — `ChatStreamChunk.type` 是 string,`content` 是 string,直接用 `type="approval"`

**A4. `frontend/src/api/chat.ts`** — SSE 解析识别 approval
- reader 循环里,收到 `type==="approval"` 时调 `onApproval(json)` 回调(新增参数)

**A5. `frontend/src/pages/Chat/`** — 聊天页审批卡片
- 收到 approval 事件 -> 在消息流里插一张审批卡片("Agent 想调用 browser_navigate → URL。[批准][拒绝]")
- 批准 -> `POST /api/v2/harness/approval/approve {request_id}`(已有端点)
- 拒绝 -> `POST /api/v2/harness/approval/reject {request_id, reason}`
- 提交后卡片置灰;agent 的 WaitForApproval 解除,流继续推送后续事件

**A6.(可选)`frontend/src/pages/Harness/ApprovalPanel.tsx`** — 加 `refetchInterval: 5000` 轮询,Harness 标签页也能及时看到 pending

### 关键正确性
- engine 在 `WaitForApproval` 阻塞时,gRPC stream 仍 open(已发出 approval chunk)。前端走**独立 HTTP POST** 提交决策 -> `SubmitDecision` -> channel 解除 -> engine 继续。不在 stream 上提交,无死锁。

---

## 功能 B:Workflow 流式输出

### 目标
执行 workflow 时,前端实时看到每个节点的 started/completed/error + 中间输出,不再黑盒等最终结果。

### 改动

**B1. `proto/harness/harness.proto`** — 加流式 RPC
```protobuf
message WorkflowStreamChunk {
  string type = 1;            // node_started | node_completed | node_error | final | error
  string node_id = 2;
  string node_name = 3;
  string node_type = 4;
  string output = 5;
  string error = 6;
  ExecuteWorkflowResponse final_result = 7;  // 仅 final
}
rpc ExecuteWorkflowStream(ExecuteWorkflowRequest) returns (stream WorkflowStreamChunk);
```
- 用本地 protoc 重生成(Makefile 已修,输出到 `pkg/pb/harness/`):
  `protoc --go_out=. --go-grpc_out=. --go_opt=module=agent-platform --go-grpc_opt=module=agent-platform -I . proto/harness/*.proto`
- 验证不产生 stray(只在 `pkg/pb/harness/`)

**B2. `services/harness-service/internal/service/harness_service.go`** — 新 `ExecuteWorkflowStream`
- 复用现有 `ExecuteWorkflow` 的 executor 构造逻辑
- `executor.SetStreamEventFunc(func(eventType, nodeID, nodeName, nodeType, output, err) { stream.Send(&WorkflowStreamChunk{...}) })`
- 执行完发 `final`(带 `ExecuteWorkflowResponse`),出错发 `error`

**B3. `services/harness-service/internal/handler/grpc_handler.go`** — 加 `ExecuteWorkflowStream` passthrough

**B4. `services/gateway/internal/handler/harness_handler.go`** — SSE 端点
- 新 `ExecuteWorkflowStream(c *gin.Context)`:SSE headers,调 `client.ExecuteWorkflowStream`,逐 chunk `c.SSEvent("message", data)` + Flush
- **新路径** `POST /harness/workflows/:id/execute-stream`(保留原 `/execute` 不破坏)

**B5. `services/gateway/internal/router/router.go`** — 注册新路由

**B6. `frontend/src/api/harness.ts`(或 workflow api)** — `executeStream(id, input, onEvent)`
- fetch + getReader 解析 SSE(复用 chat.ts 的解析模式),每事件调 `onEvent(chunk)`

**B7. `frontend/src/pages/Workflow/index.tsx`** — 执行 modal 改造
- modal 里渲染**节点进度列表**:每个节点一行(图标+名称+状态 running/done/error+输出摘要)
- 事件到达实时更新;`final` 时显示完整结果 + 关闭加载态
- 保留 cancel 按钮(已有 `/executions/:id/cancel`)

---

## 验证

1. `go build ./...` 通过 + `go vet` 通过
2. `make proto-harness`(或直接 protoc)只更新 `pkg/pb/harness/`,无 stray
3. 单测:engine_stream 发 approval 事件、executor onEvent 转发(补 `_test.go`)
4. 部署:agent-service + chat-service + gateway + harness-service + frontend
5. 端到端审批:发一条触发 browser_navigate 的对话 -> 聊天页弹审批卡 -> 批准 -> agent 继续 -> 看到导航结果
6. 端到端 workflow:跑一个 3+ 节点 workflow -> modal 里看到节点逐个 started/completed -> 最终结果

---

## 风险

- **proto 重生成**:Makefile 已修但首次跑要确认无 stray;protoc-gen-go 版本可能引入 diff 噪声(可接受)
- **审批阻塞超时**:若用户不批,仍 300s 超时(原有行为,不恶化);可在卡片显示倒计时
- **SSE 连接中断**:网络断开时审批/工作流状态丢失(原有问题,本次不解决)
- **chatWithSingleAgent 本地路径**:若不走 agent-service stream,审批事件发不出;需确认单 agent 路径也走 stream 或降级(无 session_id browser 降级模式可参考)

## 不做(YAGNI)
- 不加 WebSocket(现有 SSE 够用)
- 不改 `ExecuteWorkflow` unary(保留兼容,只加新 stream 端点)
- 不做审批推送(用前端收到 approval chunk 触发即可,无需额外 SSE 通道)
- 不改 browser-agent prompt
