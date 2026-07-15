# 阶段 1:接记忆到 engine 循环(P0 · 第一刀)

> 优先级: 最高(ROI 最高,东西都建好了,只差接线)
> 依据: `real-agent-analysis.md` §3.1①

---

## 1. 现状(为什么是孤岛)

| 证据 | 说明 |
|------|------|
| `services/agent-service/cmd/main.go:155` | `NewAgentService(registry, llmClient, mcpClient, store, cfg)` 无 memory 参数 |
| `services/agent-service/internal/service/agent_service.go:40` | `NewAgentService` 签名无 memory client |
| `pkg/agent/engine.go:67-81` | `Engine` struct 无 memory 字段 |
| `pkg/agent/engine.go:749` | `buildAgentMessages` 只读 Variables + AgentHistory,从不 recall |
| `pkg/agent/engine.go:531` | `executeLoop` 每步执行完从不 write |
| memory service `:50003` | episodic/semantic/working + Qdrant 全实现,但无人 dial |

## 2. 目标

agent 每步决策前 recall 相关记忆,每步执行后 write 经验。
**测试:把 context window 清空,agent 仍能比全新启动做得更好。**

## 3. 设计:MemoryClient 接口

新建 `pkg/agent/memory_client.go`:

```go
package agent

import "context"

// MemoryClient 是 agent 引擎与记忆服务之间的接口。
// 由 agent-service 注入具体实现(gRPC 到 memory-service)。
type MemoryClient interface {
    // Recall 在每步决策前调用,根据任务上下文召回相关记忆。
    // 返回的文本会作为 system message 注入。
    Recall(ctx context.Context, sessionID, tenantID, taskContext string) (string, error)

    // Write 在每步工具执行后调用,把"这一步学到什么"存进记忆。
    Write(ctx context.Context, sessionID, tenantID, memoryType, content string, metadata map[string]any) error
}
```

> 接口放在 `pkg/agent` 是为了不反向依赖 memory proto。具体 gRPC 实现在 agent-service 里。

## 4. 改动清单

### 4.1 `pkg/agent/engine.go`
- `Engine` struct(`:67`)加字段:`memoryClient MemoryClient`
- 加方法:`SetMemoryClient(mc MemoryClient)`
- `buildAgentMessages`(`:749`):在拼 system prompt 后,若 `e.memoryClient != nil`,调 `Recall(ctx, sessionID, tenantID, taskContext)`,把返回作为一条 system message 注入(或拼到 system prompt 末尾的"相关记忆"段)
- `executeLoop`(`:531`)和 `resumeLoop`(`:934`):每步 `processToolResults` 后,若 `e.memoryClient != nil`,把本步 thought+action+result 调 `Write` 存为 episodic 记忆
- taskContext 来源:`execCtx.Variables["task"]`,无则用最近 user message

### 4.2 `pkg/agent/memory_client.go`(新文件)
- 定义上面的 `MemoryClient` 接口

### 4.3 `services/agent-service/internal/service/agent_service.go`
- `AgentService` struct 加字段:`memoryClient agent.MemoryClient`
- `NewAgentService` 签名加 `memoryClient` 参数(或加 setter `SetMemoryClient`)
- 装配 engine 后调 `s.engine.SetMemoryClient(...)`
- 实现 `memoryAdapter`:把 `agent.MemoryClient` 适配到 memory-service gRPC client
  - `Recall` -> 调 memory proto 的 Recall RPC
  - `Write` -> 调 memory proto 的 Store/Save RPC

### 4.4 `services/agent-service/cmd/main.go`
- dial memory-service `:50003`(参考已有的 mcp dial 模式,`main.go:138`)
- 创建 `memorypb.NewMemoryServiceClient`
- 传给 `NewAgentService`(或调 `SetMemoryClient`)
- 失败时 `log.Printf` 警告但不 fatal(memory 不可用 agent 仍能跑,只是无记忆)

## 5. 实施步骤

1. 读 `proto/memory/memory.proto` 确认 Recall/Store RPC 签名和请求/响应字段
2. 新建 `pkg/agent/memory_client.go` 定义接口
3. 改 `engine.go`:加字段、setter、recall 注入点、write 调用点
4. 在 `agent_service.go` 写 `memoryAdapter`(gRPC 适配)
5. 改 `main.go`:dial memory + 注入
6. `go build ./...` 验证编译
7. 跑一个简单 agent 任务,确认 memory service 收到 Recall/Write 调用

## 6. 验收标准

- [ ] `go build ./...` 通过
- [ ] agent 执行时,memory service 日志出现 Recall 和 Write 请求
- [ ] 关掉 memory service,agent 仍能正常跑(降级,不 panic)
- [ ] 同一 session 第二次跑相似任务时,system prompt 里能看到召回的记忆
