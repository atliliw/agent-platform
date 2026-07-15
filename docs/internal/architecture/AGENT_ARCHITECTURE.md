# Agent 架构说明

## 概述

本平台实现了完整的 AI Agent 能力，不是简单的服务拆分，而是真正的 Agent 循环执行。

## 架构图

```
┌─────────────────────────────────────────────────────────────────┐
│                         用户请求                                 │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Chat Service                               │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │                    Agent Loop                              │  │
│  │                                                            │  │
│  │   ┌──────────┐    ┌──────────┐    ┌──────────┐           │  │
│  │   │  Thought │───▶│  Action  │───▶│Observation│           │  │
│  │   │  (思考)  │    │ (工具调用)│    │ (观察结果)│           │  │
│  │   └──────────┘    └──────────┘    └──────────┘           │  │
│  │        ▲                                    │             │  │
│  │        └────────────────────────────────────┘             │  │
│  │                    (循环直到完成)                          │  │
│  └───────────────────────────────────────────────────────────┘  │
│                                │                                 │
│                   ┌────────────┴────────────┐                   │
│                   ▼                         ▼                   │
│          ┌──────────────┐          ┌──────────────┐            │
│          │  LLM Client  │          │  MCP Client  │            │
│          │  (通义千问)   │          │  (工具服务)   │            │
│          └──────────────┘          └──────────────┘            │
└─────────────────────────────────────────────────────────────────┘
                                │
                   ┌────────────┴────────────┐
                   ▼                         ▼
          ┌──────────────┐          ┌──────────────┐
          │ MCP Service  │          │  Knowledge   │
          │  (工具执行)   │          │   Service    │
          │              │          │  (知识检索)   │
          │ - search     │          │              │
          │ - web_search │          └──────────────┘
          │ - calculator │
          │ - weather    │
          │ - time       │
          │ - code_exec  │
          └──────────────┘
```

## 核心流程

### 1. Agent Loop (思考-行动-观察循环)

```go
for step := 0; step < maxSteps; step++ {
    // 1. Thought: LLM 思考下一步
    response := llm.Chat(messages, tools)
    
    // 2. 判断是否需要工具
    if len(response.ToolCalls) == 0 {
        // 无工具调用，返回最终答案
        return response.Content
    }
    
    // 3. Action: 执行工具
    for _, toolCall := range response.ToolCalls {
        result := mcpClient.CallTool(toolCall.Name, toolCall.Arguments)
        
        // 4. Observation: 将结果加入上下文
        messages = append(messages, 
            Message{Role: "tool", Content: result})
    }
}
```

### 2. 示例对话流程

**用户**: "北京今天天气怎么样？"

**Agent 执行**:
```
Step 1:
  Thought: 用户想知道北京的天气，我需要使用 weather 工具
  Action: weather(location="北京")
  Observation: 北京今天晴，25°C，湿度60%

Step 2:
  Thought: 我已经获取到天气信息，可以回答用户了
  Action: 无（直接回答）
  
最终回答: 北京今天天气晴朗，气温25°C，湿度60%，适合外出活动。
```

## 服务协作

### Chat Service → MCP Service

Chat 服务通过 gRPC 调用 MCP 服务执行工具：

```go
// Chat Service 初始化时连接 MCP
mcpClient := client.NewMCPClient("mcp-service:50005")

// 在 Agent Loop 中调用工具
result := mcpClient.CallTool(ctx, &CallToolRequest{
    Name:      "weather",
    Arguments: `{"location": "北京"}`,
})
```

### MCP Service 工具注册

```go
// 内置工具
tools := map[string]Tool{
    "search":     {Description: "搜索知识库"},
    "web_search": {Description: "网络搜索"},
    "calculator": {Description: "数学计算"},
    "weather":    {Description: "天气查询"},
    "time":       {Description: "时间查询"},
    "code_exec":  {Description: "代码执行"},
}
```

## LLM 工具调用

LLM (通义千问) 支持 Function Calling，Chat Service 将 MCP 工具转换为 LLM 工具定义：

```go
// 从 MCP 获取工具列表
mcpTools := mcpClient.ListTools()

// 转换为 LLM 工具格式
llmTools := []llm.ToolDefinition{
    {
        Type: "function",
        Function: {
            Name:        "weather",
            Description: "获取指定地点的天气信息",
            Parameters: {
                "type": "object",
                "properties": {
                    "location": {
                        "type": "string",
                        "description": "地点名称",
                    },
                },
                "required": ["location"],
            },
        },
    },
}

// 发送给 LLM
response := llm.Chat(messages, llmTools)
```

## 数据存储

### Agent 执行轨迹

每次对话保存完整的 Agent 执行轨迹：

```go
type Message struct {
    Role       string        // user, assistant, tool
    Content    string        // 消息内容
    AgentTrace []AgentState  // Agent 执行轨迹
    ToolCalls  []ToolCall    // 工具调用记录
}

type AgentState struct {
    Thought   string  // 思考过程
    Action    string  // 执行的动作/工具
    Arguments map     // 工具参数
    Result    string  // 执行结果
    Step      int     // 步骤编号
}
```

## 多 Agent 协作 (A2A)

### A2A 协议

支持跨服务 Agent 通信：

```
┌─────────────┐     A2A Protocol     ┌─────────────┐
│  Agent A    │◀───────────────────▶│  Agent B    │
│ (Chat Svc)  │                     │ (External)  │
└─────────────┘                     └─────────────┘
```

### Agent Card

```json
{
  "id": "local-agent-platform",
  "name": "Local Agent Platform",
  "capabilities": ["chat", "search", "multi_agent", "tool_calling"],
  "input_modes": ["text", "json"],
  "output_modes": ["text", "json"],
  "url": "http://localhost:8080"
}
```

## 配置

### 启用 Agent 模式

```yaml
# config.yaml
agent:
  enabled: true
  max_steps: 5          # 最大执行步骤
  timeout: 60s          # 单次执行超时
  tools:
    - search
    - web_search
    - calculator
    - weather
```

### API 调用

```bash
# 普通对话 (不使用 Agent)
curl -X POST http://localhost:9000/api/v2/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "你好"}'

# Agent 对话 (自动调用工具)
curl -X POST http://localhost:9000/api/v2/chat \
  -H "Content-Type: application/json" \
  -d '{
    "message": "北京今天天气怎么样？",
    "enable_agent": true
  }'
```

## 前端展示

前端会展示 Agent 的执行过程：

```tsx
// ChatMessage.tsx
{message.agent_trace?.map((state, i) => (
  <div key={i} className="agent-step">
    <div className="thought">💭 {state.thought}</div>
    <div className="action">🔧 {state.action}</div>
    <div className="result">📊 {state.result}</div>
  </div>
))}
```

## 总结

这是一个**真正的 Agent 系统**：

| 特性 | 状态 |
|------|------|
| ✅ Agent Loop | 思考-行动-观察循环 |
| ✅ 工具调用 | 通过 MCP 服务执行 |
| ✅ LLM Function Calling | 通义千问原生支持 |
| ✅ 执行轨迹 | 完整记录每一步 |
| ✅ 多步推理 | 最多 5 步可配置 |
| ✅ A2A 协议 | 跨服务 Agent 通信 |
| ✅ 知识检索 | Knowledge Service 集成 |

**不是**简单的服务拆分，而是完整的 Agent 执行引擎！