# MCP 客户端实现计划

> 日期: 2026-07-13
> 目标: 让 mcp-service 作为 MCP 客户端，连接外部 MCP 服务器，代理远程工具给 agent 使用

## 1. 现状

- `Connect` 只存 map 元数据，不启动任何传输
- `Disconnect` 只删 map，不清理连接
- `ListTools` 只返回内置工具，忽略远程
- `CallTool` 只调内置，不路由到远程
- Gateway 缺 `DELETE /mcp/connections/:id` 路由
- 前端 MCP 页面无连接管理 UI
- Proto 不需要改（现有字段够用）

## 2. 架构

```
mcp-service (端口 50005)
├── 内置工具（现有 16 个）
└── MCP 客户端（新增）
    ├── 连接 A → GitHub MCP Server (stdio)
    │   └── 工具: create_issue, list_prs ...
    ├── 连接 B → DB MCP Server (Streamable HTTP)
    │   └── 工具: query_sql, list_tables ...
    └── ...
```

agent 调 `CallTool` 时，mcp-service 内部判断：
- 内置工具 → 本地执行
- 远程工具 → 通过 Transport 转发给外部 MCP 服务器

## 3. 新增文件

### 3.1 `pkg/mcp/` — MCP 协议层（已部分创建）

| 文件 | 内容 | 状态 |
|------|------|------|
| `doc.go` | 包文档 | ✅ 已创建 |
| `jsonrpc.go` | JSON-RPC 2.0 消息类型（Request/Response/Notification/Error） | ✅ 已创建 |
| `transport.go` | Transport 接口（Start/Send/Receive/Close/IsAlive） | ✅ 已创建 |
| `stdio.go` | stdio 传输（启动子进程，stdin 写 JSON-RPC，stdout 逐行读） | ✅ 已创建 |
| `streamable_http.go` | Streamable HTTP 传输（POST JSON-RPC，SSE 回调，Mcp-Session-Id） | ✅ 已创建 |
| `client.go` | MCP 客户端（Initialize 握手、tools/list、tools/call、ping、shutdown） | ✅ 已创建，需修语法错误 |

**待修复**: `client.go` 中 `ReadResource` 方法有重复 `if err :=` 行，需删除一行。

## 4. 修改文件

### 4.1 `services/mcp-service/internal/model/mcp.go`

Connection 结构体增加字段：

```go
type Connection struct {
    ID          string
    Name        string
    Type        string            // "stdio", "streamable-http"
    Command     string            // for stdio
    URL         string            // for streamable-http
    Env         map[string]string
    Status      string            // "connecting" | "connected" | "disconnected" | "error"
    ServerName  string            // 从 initialize 握手获取
    ServerVersion string          // 从 initialize 握手获取
    ToolCount   int               // 远程工具数量
    ErrorMsg    string            // 错误信息
}
```

### 4.2 `services/mcp-service/internal/service/mcp_service.go`

**核心改造**，改动最大：

#### MCPService 结构体增加

```go
type MCPService struct {
    // ... 现有字段 ...
    mcpClients   map[string]*mcp.Client  // connectionID → MCP client
}
```

#### Connect 改造

```
1. 根据 type 创建 Transport:
   - "stdio" → mcp.NewStdioTransport(command, args, env)
   - "streamable-http" → mcp.NewStreamableHTTPTransport(url, headers)
2. 创建 mcp.Client
3. 调 client.Initialize(ctx) — 3 步握手
4. 调 client.ListTools() — 发现远程工具
5. 注册远程工具到本地 tools map（带 connection_id 标记）
6. 存储 client 到 mcpClients map
7. 更新 Connection 状态为 "connected"
8. 如果失败，状态为 "error"，记录 ErrorMsg
```

#### Disconnect 改造

```
1. 从 mcpClients 取出 client
2. 调 client.Close() — 关闭 Transport
3. 从 tools map 移除该连接的所有远程工具
4. 从 toolExec map 移除对应的远程执行器
5. 从 mcpClients 删除
6. 从 connections 删除
7. 更新状态为 "disconnected"
```

#### ListTools 改造

```
1. 收集内置工具（现有逻辑）
2. 遍历 mcpClients，合并每个连接的远程工具
3. 远程工具的 name 加前缀 "connID__toolName" 避免冲突
4. 返回合并后的列表，每个工具带 connection_id 字段
```

#### CallTool 改造

```
1. 先查内置 toolExec（现有逻辑）
2. 没找到 → 解析 tool name 中的 connection 前缀
3. 找到对应 mcpClient → 调 client.CallTool(ctx, name, args)
4. 将 ToolCallResult 转换为 CallToolResponse 返回
5. 如果远程调用失败，返回 is_error=true + 错误信息
```

#### ListResources / ReadResource 改造

```
1. 如果指定了 connection_id → 路由到对应 client
2. 如果未指定 → 返回空（或合并所有连接的资源）
```

#### ListPrompts / GetPrompt 改造

```
1. 如果指定了 connection_id → 路由到对应 client
2. 如果未指定 → 返回空
```

### 4.3 `services/gateway/internal/handler/mcp_handler.go`

新增 `Disconnect` handler：

```go
func (h *MCPHandler) Disconnect(c *gin.Context) {
    connID := c.Param("id")
    // 调 mcpClient.Disconnect
}
```

### 4.4 `services/gateway/internal/router/router.go`

新增路由：

```go
api.DELETE("/mcp/connections/:id", mcpHandler.Disconnect)
```

### 4.5 `frontend/src/types/index.ts`

补全 MCPConnection 类型：

```typescript
export interface MCPConnection {
  id: string;
  name: string;
  type: 'stdio' | 'streamable-http';
  status: 'connecting' | 'connected' | 'disconnected' | 'error';
  command?: string;
  url?: string;
  env?: Record<string, string>;
  serverName?: string;
  serverVersion?: string;
  toolCount?: number;
  errorMsg?: string;
}
```

### 4.6 `frontend/src/pages/Agents/MCP.tsx`

新增连接管理区域：

1. **添加连接表单**
   - 选择 type（stdio / streamable-http）
   - type=stdio 时：输入 command（如 `npx @modelcontextprotocol/server-github`）
   - type=streamable-http 时：输入 url（如 `https://mcp.example.com/sse`）
   - 环境变量键值对（可选，用于 API key 等）

2. **连接列表**
   - 显示所有连接：名称、类型、状态、服务器信息、工具数量
   - 每行有"断开"按钮
   - 状态实时显示（connecting → connected / error）

3. **工具列表改造**
   - 区分"内置工具"和"远程工具"
   - 远程工具显示来源连接名
   - 按连接分组展示

### 4.7 `frontend/src/api/` (新增或扩展)

在现有 API 模块中添加：

```typescript
// 连接管理
connectMCP: (params: { name, type, command?, url?, env? }) => client.post('/mcp/connect', params),
disconnectMCP: (id: string) => client.delete(`/mcp/connections/${id}`),
listMCPConnections: () => client.get('/mcp/connections'),
```

## 5. MCP 协议握手流程

```
Connect(type=stdio, command="npx @anthropic/mcp-server-github", env={GITHUB_TOKEN: "xxx"})
  ↓
1. 启动子进程 (exec.Command)
2. 发 initialize 请求:
   { "jsonrpc": "2.0", "id": 1, "method": "initialize",
     "params": { "protocolVersion": "2025-06-18",
                 "capabilities": { "roots": { "listChanged": true } },
                 "clientInfo": { "name": "agent-platform-mcp-client", "version": "1.0.0" } } }
3. 收到服务器响应:
   { "jsonrpc": "2.0", "id": 1, "result": {
       "protocolVersion": "2025-06-18",
       "capabilities": { "tools": { "listChanged": true } },
       "serverInfo": { "name": "name": "github-mcp-server", "version": "1.0.0" } } }
4. 发 initialized 通知:
   { "jsonrpc": "2.0", "method": "notifications/initialized" }
5. 调 tools/list:
   { "jsonrpc": "2.0", "id": 2, "method": "tools/list" }
6. 收到工具列表:
   { "tools": [{ "name": "create_issue", "description": "...", "inputSchema": {...} }, ...] }
7. 注册到本地 tools map
8. 状态 = connected
```

## 6. 工具名冲突处理

远程工具名加前缀：`{connectionID}__{toolName}`

- 内置工具：`web_search`、`browser_navigate`（无前缀，保持不变）
- 远程工具：`conn123__create_issue`、`conn123__list_prs`

agent 调用时：
1. 先查内置（无前缀匹配）
2. 再查远程（解析前缀找到对应 client）
3. 去掉前缀后转发给远程服务器

## 7. 错误处理

| 场景 | 处理 |
|------|------|
| 子进程启动失败 | Connection 状态 = "error"，ErrorMsg 记录原因 |
| 握手超时（30s） | 关闭 Transport，状态 = "error" |
| tools/list 失败 | 非致命，连接仍可用，ToolCount = 0 |
| tools/call 远程失败 | 返回 is_error=true + 错误文本 |
| Transport 断开 | 检测到 IsAlive=false，标记 "disconnected"，移除远程工具 |
| 服务重启 | 连接丢失（内存存储），需重新连接 |

## 8. 实施顺序

| 步骤 | 内容 | 依赖 |
|------|------|------|
| 1 | 修复 pkg/mcp/ 语法错误 + 编译验证 | 无 |
| 2 | 改造 model/mcp.go（Connection 增加字段） | 无 |
| 3 | 改造 mcp_service.go（Connect/Disconnect/ListTools/CallTool） | 步骤 1, 2 |
| 4 | 补 Gateway Disconnect 路由 + handler | 步骤 3 |
| 5 | 前端类型 + API + MCP 页面改造 | 步骤 4 |
| 6 | 编译 + 部署 + 验证 | 步骤 5 |

## 9. 不做的事

- **不改 proto**：现有 Connection 消息已有 type/command/url/env 字段
- **不做持久化**：连接保持内存存储，重启需重连（后续可加 MongoDB）
- **不做 sampling/elicitation**：只实现 tools，resources/prompts 为可选
- **不做 WebSocket**：MCP 规范只有 stdio 和 Streamable HTTP
- **不做自动重连**：断开后需手动重连（后续可加）
