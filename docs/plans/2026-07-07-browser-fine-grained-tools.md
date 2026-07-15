# 路线 A:细粒度浏览器工具 + 会话连续性(经 MCP)

日期:2026-07-07 · 分支:feature/optimize-browser-agent

## 问题

`pkg/agent/defaults.go` 的 browser-agent 声明了 6 个工具:
`browser_navigate / browser_click / browser_type / browser_extract / browser_scroll / browser_wait`

但这 6 个**双重失效**:
1. 未在 MCP `registerBuiltInTools` 注册 → `ListTools` 不返回 → engine `buildAgentTools`(engine.go:1087)只把 `ListTools` 里存在的工具加给 LLM → **LLM 根本看不到它们**。
2. 即便被调用,`s.toolExec[name]` 不存在 → MCP 返回 "Tool not found"。

目前唯一能用的是 `browser_execute`(自主 LLM 循环模式,一次自然语言任务内部多步决策)。

## 目标

让 6 个细粒度工具真正可用,且 `navigate → click → extract` 等多次调用**共享同一个浏览器页面**(保留 Cookie / 登录态)。

## 方案选型:A-mcp(经 MCP,非 engine 本地拦截)

| 维度 | A-mcp(选定) | A-engine(本地拦截) |
|---|---|---|
| 一致性 | 与 `browser_execute` 同处 MCP | 浏览器逻辑被拆到 engine |
| 治理 | 保留 MCP 护栏 / SLO / 权限 | 绕过 MCP 治理 |
| session_id | 走已有 `tool_config.Extra`(3 处小改) | engine 内联(无需传递) |
| engine 改动 | 仅注入 session_id(轻薄) | 加 def + 拦截 + 新 pkg |
| proto | **不改** | 不改 |

选 A-mcp:浏览器工具归属 MCP(与 `browser_execute` 一致)、保留治理与 SLO 可观测性、engine 保持轻薄。session_id 复用已有 `CallToolRequest.tool_config`(JSON)→ `ToolSpecificConfig.Extra` 字段,**无需改 proto、无需重新生成**。

## 会话连续性原理(已验证)

- `browseragent.NewBrowserFromPool(ctx)` 中 browser 的 `b.ctx` 派生自 acquire 传入的 ctx;所有操作(`GetState`/`ExecuteAction`)内部用 `b.ctx`,**不**用调用方 ctx。
- 因此保活的充要条件:**用长生命周期 ctx 去 acquire**(不能用单次请求的超时 ctx)。
- `BrowserSessionManager` 用 `context.Background()` 派生的长 ctx acquire,按 `session_id` 持有 `*browseragent.Browser`,跨调用复用;TTL 回收归还池。

## 改动清单

### 1. proto — 不改
session_id 经已有 `CallToolRequest.tool_config`(JSON)传递。`toolAdapter` 已 marshal `ToolSpecificConfig`(含 `Extra`),MCP 侧解析 `config["extra"]["session_id"]`。

### 2. 新增 `services/mcp-service/internal/tools/browser_session.go`(~280 行)

**`BrowserSessionManager`**
- `sessions map[string]*sessionEntry`(`sessionID` → `browserOps` + `lastUsed`),`sync.Mutex`,长生命周期 `ctx` + `cancel`,TTL goroutine(每 1 分钟扫描,闲置 > 15 分钟归还池)。
- `GetOrCreate(sessionID) (browserOps, error)`:命中则复用;否则 `browseragent.NewBrowserFromPool(managerCtx)`(长 ctx)+ `Start`,存入。
- `Release(sessionID)`:`browser.Close()`(归还池)+ 删 map。
- `Close()`:关闭全部(服务关停时)。
- 单例 `GetBrowserSessionManager()`。

**`browserOps` 接口**(便于测试注入 fake,`*browseragent.Browser` 天然满足):
```go
type browserOps interface {
    Start(ctx) error
    Close()
    GetState(ctx) (*browseragent.PageState, error)
    ExecuteAction(ctx, *browseragent.Action) (string, error)
    SetCookies([]browseragent.Cookie)
}
```

**6 个 executor**(实现 `tools.Executor`):`BrowserNavigateTool / BrowserClickTool / BrowserTypeTool / BrowserExtractTool / BrowserScrollTool / BrowserWaitTool`
- `GetInfo() ToolInfo`:name / description / JSON schema。
- `ExecuteWithConfig(ctx, args, config)`:
  1. 从 `config["extra"]["session_id"]` 读 `sessionID`。
  2. 有 `sessionID` → `manager.GetOrCreate(sessionID)`(连续性);无 → 一次性(`NewBrowserFromPool` + `Start` + 执行 + `Close`,无连续性但可用,覆盖 Path 2 / 无 session 调用方)。
  3. 调原语:
     - `navigate`:`SetCookies`(无 args cookies 时用 `cookie_loader` 按 URL 域名自动加载,与 `browser_execute` 行为一致)+ `ExecuteAction(ActionNavigate, url)`。
     - `click`:`ExecuteAction(ActionClick, elementIndex)`。
     - `type`:`ExecuteAction(ActionInput, elementIndex, text)`。
     - `scroll`:`ExecuteAction(ActionScroll, direction)`。
     - `wait`:`ExecuteAction(ActionWait, seconds)`。
     - `extract`:`GetState()` → 格式化 URL / 标题 / 元素列表(带索引,与自主 agent 契约一致,供 LLM 选 click/type 目标)/ 文本;可选 `selector` 参数走 JS 定向提取。

### 3. 改 `services/mcp-service/internal/service/mcp_service.go` `registerBuiltInTools`
注册 6 个工具(`model.Tool` + `s.toolExec[name] = executor`)。细粒度工具**无需 LLM 配置**(不像 `browser_execute`)。

### 4. 改 `pkg/agent/engine.go`(session_id 注入,单点)
在共享的 `executeToolCalls` 中 `e.tools.Execute` 调用点(~line 525-541)前:
```go
if isFineGrainedBrowserTool(toolCall.Name) {
    toolCfg = injectSessionID(toolCfg, execCtx.SessionID) // 拷贝,不改 agent 配置
}
```
- `isFineGrainedBrowserTool(name)`:匹配 6 个名字。
- `injectSessionID(toolCfg, sessionID)`:返回**拷贝**的 `*ToolSpecificConfig`,`Extra` 为新 map(合并原 Extra + `session_id`),不污染 `currentAgent.ToolConfig`。
- 一旦 MCP 注册 6 个工具,`buildAgentTools` 自动经 `ListTools` 取到定义,**无需改 def 逻辑**。

### 5. `pkg/agent/defaults.go` — 不改
6 个工具已声明;改完后即生效。`browser_navigate` 的空 `ToolConfig`(APIKey/BaseURL/Model 空)可留可清,无害。

### 6. 测试
- **`pkg/agent`**:engine 注入测试 —— 验证 browser 工具的 `toolCfg.Extra["session_id"]` == `execCtx.SessionID`;验证 `currentAgent.ToolConfig` 未被 mutate;非 browser 工具不注入。
- **`mcp-service/internal/tools/browser_session_test.go`**:
  - `TestManager_GetOrCreate_ReusesAcrossCalls`(同 sessionID 复用同一 browser)
  - `TestManager_DifferentSessions_DifferentBrowsers`
  - `TestManager_TTLRelease`(模拟闲置 → 归还)
  - `TestNavigate/Click/Type/Scroll/Wait/Extract`(fake `browserOps`,验证参数解析与原语分发)
  - `TestOneShotFallback`(无 session_id → 一次性,调用 Close)
  - `TestExtract_ReturnsElementsWithIndices`

### 7. 前端 — 无需改
工具经 MCP `ListTools` 自动发现。(可选:browser-agent 的 prompt 模板可提示这些工具,属 Prompt Management UI 编辑范畴,不在本次范围。)

### 8. 部署
重建 `mcp-service` + `agent-service` Docker 镜像。**无前端同步、无 proto 重新生成。**

## 会话连续性流程
1. 用户聊天 → main-agent handoff → browser-agent。
2. engine 跑 browser-agent;LLM 调 `browser_navigate(url)`。
3. engine 注入 `execCtx.SessionID` 到 `toolCfg.Extra` → MCP `CallTool`。
4. MCP `BrowserNavigateTool` 读 session_id → `manager.GetOrCreate` → acquire browser(长 ctx)→ 导航。
5. LLM 调 `browser_click(idx)` → 同一 session_id → **同一 browser** → 点击。
6. LLM 调 `browser_extract` → 同一 browser → 返回页面状态。
7. Cookie / 登录态全程保留(同一页面)。
8. 闲置 > 15 分钟:manager 归还 browser 到池。

## 降级
无 session_id 的调用方(如 chat-service 本地循环 Path 2)→ 一次性 browser(每次 acquire+release,无连续性但可用)。

## 风险与注意
- 浏览器池 `maxSize=5`;并发会话多 = Chrome 进程多。TTL 缓解;可调 `BROWSER_POOL_SIZE`。
- `extract` 返回带索引的交互元素列表(与自主 agent 契约一致),LLM 据此选 click/type 目标。
- 不改 proto → 无跨服务重新生成涟漪。
- MCP 治理保留(护栏 / SLO 指标 / 权限)。

## 不做(YAGNI)
- 不加 `browser_close` 显式释放工具(TTL 已兜底;若实测需要再加)。
- 不改 browser-agent 的 prompt 模板内容(工具 description 已足够指引 LLM)。
- 不动 `browser_execute`(自主模式保持原样)。
