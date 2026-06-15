# Multi-Agent 架构深度解析

## 一、概述

Multi-Agent 架构实现了多个 AI Agent 之间的协作，基于 **Handoff（交接）机制**，让不同专业领域的 Agent 能够相互配合完成复杂任务。

### 核心概念

| 概念 | 说明 |
|------|------|
| **Agent** | 具有特定能力的 AI 助手，有自己的 System Prompt、可用工具、可交接的目标 |
| **Handoff** | Agent 之间的对话交接机制，将任务转交给更专业的 Agent |
| **Registry** | Agent 注册中心，管理所有 Agent 的注册、查询、验证 |
| **Engine** | 执行引擎，负责运行 Agent 循环、调用 LLM、执行工具、处理 Handoff |
| **ExecutionContext** | 执行上下文，跨 Agent 共享的状态、消息历史、执行记录 |

---

## 二、架构图

### 2.1 多 Agent 协作架构

```
                              用户请求
                                 │
                                 ▼
                    ┌────────────────────────┐
                    │      Main Agent        │
                    │    (主调度/路由)        │
                    │                        │
                    │  Handoffs:             │
                    │  → researcher-agent    │
                    │  → coder-agent         │
                    │  → analyst-agent       │
                    │  → browser-agent       │
                    └────────────┬───────────┘
                                 │
              ┌──────────────────┼──────────────────┐
              │                  │                  │
              ▼                  ▼                  ▼
    ┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐
    │ Researcher      │ │ Coder Agent     │ │ Analyst Agent   │
    │ Agent           │ │                 │ │                 │
    │                 │ │ Tools:          │ │ Tools:          │
    │ Tools:          │ │ - code_execute  │ │ - data_analysis │
    │ - web_search    │ │ - file_read     │ │ - visualization │
    │ - knowledge_    │ │ - file_write    │ │ - code_execute  │
    │   search        │ │ - calculator    │ │                 │
    │                 │ │                 │ │ Handoffs:       │
    │ Handoffs:       │ │ Handoffs:       │ │ → main-agent    │
    │ → main-agent    │ │ → main-agent    │ │                 │
    │ → coder-agent   │ │ → analyst-agent │ │                 │
    └────────┬────────┘ └────────┬────────┘ └────────┬────────┘
             │                   │                   │
             │         ┌─────────┴─────────┐         │
             │         │                   │         │
             ▼         ▼                   ▼         ▼
    ┌─────────────────────────────────────────────────────────┐
    │                                                         │
    │                    共享上下文                            │
    │              (ExecutionContext)                         │
    │                                                         │
    │  • Variables: 跨 Agent 共享变量                         │
    │  • Messages: 完整对话历史                               │
    │  • AgentHistory: 执行路径追踪                           │
    │  • ToolResults: 工具调用结果缓存                        │
    │                                                         │
    └─────────────────────────────────────────────────────────┘
```

### 2.2 Agent 交接流程（时序图）

```
用户: "帮我研究 Go 并发模型，然后写个示例代码"

     ┌──────────┐     ┌──────────┐     ┌──────────┐     ┌──────────┐
     │  用户    │     │  Main    │     │Researcher│     │  Coder   │
     └────┬─────┘     │  Agent   │     │  Agent   │     │  Agent   │
          │           └────┬─────┘     └────┬─────┘     └────┬─────┘
          │                │                │                │
          │  请求: 研究 Go 并发并写代码      │                │
          │───────────────>│                │                │
          │                │                │                │
          │                │ 分析: 这是研究任务              │
          │                │ 需要先研究，再写代码            │
          │                │                │                │
          │                │ handoff:       │                │
          │                │ transfer_to_   │                │
          │                │ researcher     │                │
          │                │───────────────>│                │
          │                │                │                │
          │                │                │ web_search     │
          │                │                │ "Go 并发"      │
          │                │                │───────┐        │
          │                │                │       │        │
          │                │                │<──────┘        │
          │                │                │ 结果: ...      │
          │                │                │                │
          │                │ handoff:       │                │
          │                │ transfer_to_   │                │
          │                │ coder          │                │
          │                │<───────────────│                │
          │                │                │                │
          │                │ handoff:       │                │
          │                │ transfer_to_   │                │
          │                │ coder          │                │
          │                │────────────────────────────────>│
          │                │                │                │
          │                │                │                │ code_execute
          │                │                │                │ 写示例代码
          │                │                │                │───────┐
          │                │                │                │       │
          │                │                │                │<──────┘
          │                │                │                │
          │  返回结果: Go 并发模型解释 + 示例代码             │
          │<───────────────────────────────────────────────────│
          │                │                │                │
```

### 2.3 系统架构图

```
┌─────────────────────────────────────────────────────────────────────┐
│                          Gateway (HTTP API)                          │
│                    POST /api/v2/agents/execute                      │
└──────────────────────────────┬──────────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────────┐
│                        Agent Service (gRPC)                         │
│                           Port: 50006                               │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │                      Engine (执行引擎)                        │   │
│  │                                                              │   │
│  │   ┌──────────────┐   ┌──────────────┐   ┌──────────────┐    │   │
│  │   │   Registry   │   │  LLM Client  │   │ Tool Executor│    │   │
│  │   │  (Agent注册) │   │  (调用LLM)   │   │  (执行工具)  │    │   │
│  │   └──────┬───────┘   └──────┬───────┘   └──────┬───────┘    │   │
│  │          │                  │                  │            │   │
│  │          └──────────────────┴──────────────────┘            │   │
│  │                         │                                   │   │
│  │                         ▼                                   │   │
│  │              ┌──────────────────────┐                       │   │
│  │              │  ExecutionContext    │                       │   │
│  │              │  (执行上下文/状态)    │                       │   │
│  │              └──────────────────────┘                       │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                                                                     │
└──────────────────────────────┬──────────────────────────────────────┘
                               │
          ┌────────────────────┼────────────────────┐
          │                    │                    │
          ▼                    ▼                    ▼
    ┌───────────┐        ┌───────────┐        ┌───────────┐
    │    MCP    │        │  MongoDB  │        │  SQLite   │
    │  Service  │        │  (Agent   │        │ (Context  │
    │  (工具)   │        │  存储)    │        │  存储)    │
    └───────────┘        └───────────┘        └───────────┘
```

---

## 三、核心组件详解

### 3.1 Agent（智能体）—— 最小执行单元

**定义：Agent 是一个具有特定能力的 AI 助手，由 Instructions（性格）、Tools（能力）、Handoffs（协作关系）三要素组成。**

```go
type Agent struct {
    // 核心三要素
    ID           string  // 唯一标识
    Name         string  // 显示名称
    Description  string  // 功能描述（给其他 Agent 看）
    Instructions string  // System Prompt（决定 Agent 的行为）
    
    Tools        []string          // 能力：能调用什么工具
    Handoffs     []string          // 协作：能交接给谁
    
    // 可选配置
    Model        string            // 指定 LLM（可以不同 Agent 用不同模型）
    MaxTokens    int               // 最大输出 Token
    Temperature  float64           // 创造性程度
    
    // 进阶配置
    ToolConfig   map[string]ToolSpecificConfig  // 工具特定配置（如 Browser Agent 的 API Key）
    Metadata     map[string]any                  // 自定义元数据
}
```

**三要素详解：**

| 要素 | 作用 | 示例 |
|------|------|------|
| **Instructions** | 定义 Agent 的"性格"、思考方式、行为准则 | "你是研究助手，搜索前先分析关键词..." |
| **Tools** | 定义 Agent 能调用的外部能力 | `["web_search", "knowledge_search"]` |
| **Handoffs** | 定义 Agent 能交接给谁（协作关系图） | `["coder-agent", "analyst-agent"]` |

**Agent 与 Tools/Handoffs 的关系：**

```
┌─────────────────────────────────────────────────────────┐
│                     Agent                                │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  Instructions (System Prompt)                           │
│  ┌─────────────────────────────────────────────────┐   │
│  │ "你是研究助手，根据问题选择合适的搜索策略..."    │   │
│  └─────────────────────────────────────────────────┘   │
│                                                         │
│  Tools (能力)                    Handoffs (协作)        │
│  ┌──────────────────┐          ┌──────────────────┐    │
│  │ • web_search     │          │ • coder-agent    │    │
│  │ • knowledge_     │          │ • main-agent     │    │
│  │   search         │          │                  │    │
│  │ • weather        │          │ Engine 自动注入: │    │
│  └──────────────────┘          │ transfer_to_xxx  │    │
│                                └──────────────────┘    │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

**重要：Handoffs 会自动转换为 Tools！**

```go
// handoffs: ["coder-agent"]
// Engine 自动注入一个特殊工具：
{
    "type": "function",
    "function": {
        "name": "transfer_to_coder-agent",  // 特殊命名前缀
        "description": "Transfer to Coder Agent. 编程 Agent，负责代码编写",
        "parameters": {}  // 无参数，交接就是交接
    }
}
```

### 3.2 Registry（注册中心）—— Agent 管理器

**Registry 负责管理所有 Agent，提供注册、查询、验证功能。**

```go
type Registry struct {
    agents      map[string]*Agent  // 内存缓存（快速访问）
    store       AgentStore         // MongoDB 持久化
    defaultID   string             // 默认 Agent ID
}
```

**核心功能：**

```
┌─────────────────────────────────────────────────────────┐
│                     Registry                             │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  注册/注销                                              │
│  ┌─────────────────────────────────────────────────┐   │
│  │ Register(agent)         → 添加到内存 + MongoDB   │   │
│  │ Unregister(id)          → 从内存 + MongoDB 删除  │   │
│  │ RegisterOrUpdate(agent) → 存在则更新，不存在则添加│   │
│  └─────────────────────────────────────────────────┘   │
│                                                         │
│  查询                                                   │
│  ┌─────────────────────────────────────────────────┐   │
│  │ Get(id)        → 获取单个 Agent                  │   │
│  │ GetDefault()   → 获取默认 Agent                  │   │
│  │ List()         → 获取所有 Agent                  │   │
│  │ Exists(id)     → 检查 Agent 是否存在             │   │
│  └─────────────────────────────────────────────────┘   │
│                                                         │
│  验证                                                   │
│  ┌─────────────────────────────────────────────────┐   │
│  │ ValidateHandoff(from, to) → 检查交接是否合法     │   │
│  │   1. from Agent 存在？                           │   │
│  │   2. from Agent 的 handoffs 包含 to？            │   │
│  │   3. to Agent 存在？                             │   │
│  └─────────────────────────────────────────────────┘   │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

**Handoff 验证逻辑：**

```go
func (r *Registry) ValidateHandoff(fromID, toID string) error {
    // 1. 检查源 Agent 存在
    fromAgent := r.agents[fromID]
    if fromAgent == nil {
        return ErrAgentNotFound
    }
    
    // 2. 检查目标在 handoffs 列表中（防止随意交接）
    if !fromAgent.CanHandoffTo(toID) {
        return ErrInvalidHandoff  // "main-agent 不能交接给 unknown-agent"
    }
    
    // 3. 检查目标 Agent 存在
    if r.agents[toID] == nil {
        return ErrAgentNotFound
    }
    
    return nil
}
```

### 3.3 Engine（执行引擎）—— 核心循环

**Engine 是整个 Multi-Agent 的心脏，负责协调 Agent、LLM、Tools 之间的交互。**

**核心概念：每次循环都会调用 LLM，问它"下一步干啥"。**

```go
type Engine struct {
    registry  *Registry       // Agent 注册中心
    llmClient LLMClient       // LLM 调用接口（每次循环都要调用！）
    tools     ToolExecutor    // 工具执行器（连接 MCP Service）
    store     ContextStore    // 执行上下文存储（SQLite）
    config    EngineConfig    // 配置（最大步数、历史长度）
}
```

**关键理解：**

```
┌─────────────────────────────────────────────────────────────────┐
│                    Engine 循环的本质                            │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│   每次循环 = 一次 LLM 调用                                       │
│                                                                 │
│   循环 1 → 调用 LLM（问 Main Agent）                            │
│   循环 2 → 调用 LLM（问 Researcher Agent）                      │
│   循环 3 → 调用 LLM（问 Researcher Agent）                      │
│   循环 4 → 调用 LLM（问 Coder Agent）                           │
│   ...                                                           │
│                                                                 │
│   每次调用都问：                                                 │
│     • 有工具调用？→ 继续！                                      │
│     • 无工具调用？→ 结束！返回结果                              │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

**执行循环详解：**

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Engine.Run()                                  │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  1. 创建 ExecutionContext                                          │
│     ┌──────────────────────────────────────────────────────────┐   │
│     │ execCtx = NewExecutionContext(sessionID)                  │   │
│     │ • ID: "ctx_20260615120000_abc123"                         │   │
│     │ • Variables: {}                                           │   │
│     │ • Messages: []                                            │   │
│     │ • AgentHistory: []                                        │   │
│     │ • Status: running                                         │   │
│     └──────────────────────────────────────────────────────────┘   │
│                                                                     │
│  2. 选择入口 Agent                                                  │
│     ┌──────────────────────────────────────────────────────────┐   │
│     │ currentAgent = registry.Get(entryAgent)                   │   │
│     │ 或 registry.GetDefault()                                  │   │
│     └──────────────────────────────────────────────────────────┘   │
│                                                                     │
│  3. 执行循环（最多 MaxSteps 步）                                    │
│     ┌──────────────────────────────────────────────────────────┐   │
│     │ for step := 0; step < MaxSteps; step++ {                  │   │
│     │                                                          │   │
│     │   ┌────────────────────────────────────────────────────┐ │   │
│     │   │ Step N                                               │ │   │
│     │   ├────────────────────────────────────────────────────┤ │   │
│     │   │                                                      │ │   │
│     │   │  3.1 构建 Prompt                                     │ │   │
│     │   │  messages = buildMessages(currentAgent, execCtx)    │ │   │
│     │   │  ┌────────────────────────────────────────────────┐│ │   │
│     │   │  │ System: Agent.Instructions                      ││ │   │
│     │   │  │         + 上下文变量                            ││ │   │
│     │   │  │         + 执行历史                              ││ │   │
│     │   │  │ User: 用户请求                                  ││ │   │
│     │   │  │ Assistant: 之前的响应                           ││ │   │
│     │   │  │ Tool: 工具执行结果                              ││ │   │
│     │   │  └────────────────────────────────────────────────┘│ │   │
│     │   │                                                      │ │   │
│     │   │  3.2 构建 Tools                                      │ │   │
│     │   │  tools = buildTools(currentAgent)                   │ │   │
│     │   │  ┌────────────────────────────────────────────────┐│ │   │
│     │   │  │ Agent.Tools:                                    ││ │   │
│     │   │  │   • web_search                                  ││ │   │
│     │   │  │   • knowledge_search                            ││ │   │
│     │   │  │                                                  ││ │   │
│     │   │  │ Handoffs（自动注入）:                           ││ │   │
│     │   │  │   • transfer_to_coder-agent                     ││ │   │
│     │   │  │   • transfer_to_main-agent                      ││ │   │
│     │   │  └────────────────────────────────────────────────┘│ │   │
│     │   │                                                      │ │   │
│     │   │  3.3 调用 LLM                                        │ │   │
│     │   │  response = llmClient.Chat(messages, tools)         │ │   │
│     │   │                                                      │ │   │
│     │   │  3.4 处理响应                                        │ │   │
│     │   │  if len(response.ToolCalls) == 0:                   │ │   │
│     │   │    → 返回 response.Content                          │ │   │
│     │   │                                                      │ │   │
│     │   │  for tc in response.ToolCalls:                      │ │   │
│     │   │    if IsHandoffTool(tc.Name):                       │ │   │
│     │   │      → 切换 Agent，继续循环                         │ │   │
│     │   │    else:                                             │ │   │
│     │   │      → 执行工具，添加结果到 Messages                │ │   │
│     │   │                                                      │ │   │
│     │   └────────────────────────────────────────────────────┘ │   │
│     │ }                                                        │   │
│     └──────────────────────────────────────────────────────────┘   │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

**buildMessages 详细逻辑：**

```go
func (e *Engine) buildMessages(agent *Agent, execCtx *ExecutionContext) []Message {
    messages := []
    
    // 1. System Prompt
    systemPrompt := agent.Instructions
    
    // 2. 添加上下文变量（让 Agent 知道当前状态）
    if len(execCtx.Variables) > 0 {
        systemPrompt += "\n\nCurrent context variables:\n" + json(execCtx.Variables)
    }
    
    // 3. 添加执行历史（让 Agent 知道之前发生了什么）
    if len(execCtx.AgentHistory) > 0 {
        systemPrompt += "\n\nPrevious agent actions:\n"
        for record in execCtx.AgentHistory {
            systemPrompt += "- {record.AgentName} ({record.Action}): {record.Result}\n"
        }
    }
    
    messages.append(Message{Role: "system", Content: systemPrompt})
    
    // 4. 添加最近的消息历史（限制长度，避免 Token 过多）
    start = max(0, len(execCtx.Messages) - MaxHistoryLength)
    for i := start; i < len(execCtx.Messages); i++ {
        messages.append(execCtx.Messages[i])
    }
    
    return messages
}
```

**buildTools 详细逻辑：**

```go
func (e *Engine) buildTools(ctx context.Context, agent *Agent) []Tool {
    tools := []
    
    // 1. 获取所有可用工具定义（从 MCP Service）
    allTools := e.tools.ListTools(ctx)
    
    // 2. 添加 Agent 自己的工具
    for toolName in agent.Tools {
        if allTools[toolName] exists {
            tools.append(allTools[toolName])
        }
    }
    
    // 3. 自动注入 Handoff 工具
    for targetID in agent.Handoffs {
        targetAgent := e.registry.Get(targetID)
        if targetAgent != nil {
            tools.append({
                type: "function",
                function: {
                    name: "transfer_to_" + targetID,
                    description: "Transfer to {targetAgent.Name}. {targetAgent.Description}",
                    parameters: {}  // 无参数
                }
            })
        }
    }
    
    return tools
}
```

### 3.4 Handoff（交接机制）—— Agent 协作的核心

**Handoff 是让 Agent 之间能够相互"对话交接"的机制。**

**工作原理：**

```
┌─────────────────────────────────────────────────────────────────────┐
│                     Handoff 机制详解                                 │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  1. 工具注入                                                        │
│     ┌──────────────────────────────────────────────────────────┐   │
│     │ Agent 配置:                                                │   │
│     │   handoffs: ["researcher-agent", "coder-agent"]           │   │
│     │                                                          │   │
│     │ Engine 自动注入工具:                                      │   │
│     │   {                                                       │   │
│     │     "name": "transfer_to_researcher-agent",               │   │
│     │     "description": "Transfer to Researcher Agent...",     │   │
│     │     "parameters": {}                                      │   │
│     │   }                                                       │   │
│     │   {                                                       │   │
│     │     "name": "transfer_to_coder-agent",                    │   │
│     │     "description": "Transfer to Coder Agent...",          │   │
│     │     "parameters": {}                                      │   │
│     │   }                                                       │   │
│     └──────────────────────────────────────────────────────────┘   │
│                                                                     │
│  2. LLM 决策                                                        │
│     ┌──────────────────────────────────────────────────────────┐   │
│     │ 用户: "帮我研究 Go 并发"                                   │   │
│     │                                                          │   │
│     │ Main Agent 的 Tools:                                      │   │
│     │   • transfer_to_researcher-agent                         │   │
│     │   • transfer_to_coder-agent                              │   │
│     │                                                          │   │
│     │ LLM 思考: "这是研究任务，应该交给 researcher-agent"        │   │
│     │                                                          │   │
│     │ Tool Call:                                                │   │
│     │   {                                                       │   │
│     │     "name": "transfer_to_researcher-agent",               │   │
│     │     "arguments": {}                                       │   │
│     │   }                                                       │   │
│     └──────────────────────────────────────────────────────────┘   │
│                                                                     │
│  3. Engine 处理                                                     │
│     ┌──────────────────────────────────────────────────────────┐   │
│     │ if IsHandoffTool("transfer_to_researcher-agent") {        │   │
│     │                                                          │   │
│     │   targetID = ParseHandoffTarget() // "researcher-agent"   │   │
│     │                                                          │   │
│     │   // 验证                                                 │   │
│     │   ValidateHandoff(currentAgent, targetID, registry)       │   │
│     │                                                          │   │
│     │   // 记录                                                 │   │
│     │   execCtx.AddAgentRecord({                               │   │
│     │     AgentID: "main-agent",                               │   │
│     │     Action: "handoff",                                   │   │
│     │     HandoffTo: "researcher-agent"                        │   │
│     │   })                                                     │   │
│     │                                                          │   │
│     │   // 切换                                                 │   │
│     │   currentAgent = registry.Get("researcher-agent")         │   │
│     │   execCtx.SetCurrentAgent("researcher-agent")             │   │
│     │                                                          │   │
│     │   // 继续循环（进入 researcher-agent 的执行）             │   │
│     │   continue                                                │   │
│     │ }                                                        │   │
│     └──────────────────────────────────────────────────────────┘   │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

**Handoff vs 普通工具调用：**

| 特性 | Handoff | 普通工具 |
|------|---------|----------|
| 命名 | `transfer_to_xxx` | `web_search`, `calculator` |
| 参数 | 无参数 | 有参数（如搜索关键词） |
| 效果 | 切换 Agent，改变执行主体 | 执行操作，返回结果 |
| 结果处理 | 继续循环，让新 Agent 处理 | 添加 Tool Message，当前 Agent 继续处理 |

### 3.5 ExecutionContext（执行上下文）—— 跨 Agent 共享状态

**ExecutionContext 是让多个 Agent 能够"协作"的关键——共享状态。**

```go
type ExecutionContext struct {
    // 唯一标识
    ID        string    // "ctx_20260615120000_abc123"
    SessionID string    // 关联用户会话
    
    // 跨 Agent 共享的数据
    Variables    map[string]any  // 共享变量
    Messages     []Message       // 完整对话历史
    AgentHistory []AgentExecutionRecord  // 执行路径
    
    // 当前状态
    CurrentAgent string        // 当前正在执行的 Agent
    Status       AgentStatus   // running, completed, error
    
    // 统计
    TotalTokens  int           // 总 Token 消耗
    TotalCost    float64       // 总成本
    StepCount    int           // 执行步数
}
```

**为什么需要 ExecutionContext？**

```
问题：多个 Agent 如何协作？

场景：
  用户: "研究 Go 并发，然后写示例代码"
  
  Step 1: Main Agent 分析 → handoff 到 Researcher
  Step 2: Researcher 搜索 → 找到信息
  
  问题：Researcher 找到的信息，Coder 怎么知道？
  
  解决：ExecutionContext！
  
  ┌─────────────────────────────────────────────────────────────┐
  │                 ExecutionContext                            │
  ├─────────────────────────────────────────────────────────────┤
  │                                                             │
  │  Variables:                                                 │
  │    {                                                        │
  │      "research_result": "Go 并发基于 CSP...",              │
  │      "search_keywords": ["goroutine", "channel"]           │
  │    }                                                        │
  │                                                             │
  │  Messages:                                                  │
  │    [                                                        │
  │      {role: "user", content: "研究 Go 并发，写代码"},       │
  │      {role: "assistant", content: "交給 researcher..."},   │
  │      {role: "tool", content: "找到文章..."},               │
  │      {role: "assistant", content: "交給 coder..."}         │
  │    ]                                                        │
  │                                                             │
  │  AgentHistory:                                              │
  │    [                                                        │
  │      {AgentID: "main", Action: "handoff", To: "researcher"},│
  │      {AgentID: "researcher", Action: "web_search"},         │
  │      {AgentID: "researcher", Action: "handoff", To: "coder"}│
  │    ]                                                        │
  │                                                             │
  └─────────────────────────────────────────────────────────────┘
  
  当 Coder Agent 开始执行时：
  
  buildMessages(coder, execCtx):
    System: "你是编程助手..."
            + "上下文变量: research_result=Go 并发基于 CSP..."
            + "之前的执行: researcher 搜索了 goroutine/channel..."
    
  Coder 就知道 Researcher 找到了什么！
```

**ExecutionContext 的生命周期：**

```
┌─────────────────────────────────────────────────────────────────────┐
│                   ExecutionContext 生命周期                         │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  创建                                                               │
│  ┌────────────────────────────────────────────────────────────┐    │
│  │ Engine.Run()                                                │    │
│  │   execCtx = NewExecutionContext(sessionID)                  │    │
│  │   • ID: 生成唯一 ID                                         │    │
│  │   • Variables: {}                                           │    │
│  │   • Messages: []                                            │    │
│  │   • Status: idle                                            │    │
│  └────────────────────────────────────────────────────────────┘    │
│                                                                     │
│  运行                                                               │
│  ┌────────────────────────────────────────────────────────────┐    │
│  │ execCtx.MarkRunning()                                       │    │
│  │ execCtx.AddMessage("user", userMessage)                     │    │
│  │                                                             │    │
│  │ 每次工具调用:                                                │    │
│  │   execCtx.AddToolMessage(toolName, result)                  │    │
│  │   execCtx.AddAgentRecord(record)                            │    │
│  │   execCtx.AddTokens(tokens)                                 │    │
│  │                                                             │    │
│  │ 每次 Handoff:                                                │    │
│  │   execCtx.SetCurrentAgent(newAgentID)                       │    │
│  │   execCtx.AddAgentRecord(handoffRecord)                     │    │
│  └────────────────────────────────────────────────────────────┘    │
│                                                                     │
│  完成/失败                                                          │
│  ┌────────────────────────────────────────────────────────────┐    │
│  │ execCtx.MarkCompleted()                                     │    │
│  │   • Status: completed                                       │    │
│  │   • CompletedAt: 设置时间                                   │    │
│  │                                                             │    │
│  │ execCtx.MarkError(err)                                      │    │
│  │   • Status: error                                           │    │
│  │   • Error: 错误信息                                         │    │
│  └────────────────────────────────────────────────────────────┘    │
│                                                                     │
│  存储                                                               │
│  ┌────────────────────────────────────────────────────────────┐    │
│  │ store.Save(ctx, execCtx)                                    │    │
│  │   → SQLite 存储                                             │    │
│  │   → 可用于后续恢复（Continue）                               │    │
│  └────────────────────────────────────────────────────────────┘    │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 四、完整执行流程示例

### 场景：用户请求 "帮我研究 Go 并发模型，然后写个示例代码"

**这是一个复杂任务，需要 Researcher 先研究，然后 Coder 写代码。**

```
┌─────────────────────────────────────────────────────────────────────┐
│                      完整执行流程                                    │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  用户请求: "研究 Go 并发模型，写个示例代码"                          │
│                                                                     │
│  ┌───────────────────────────────────────────────────────────────┐ │
│  │ Step 0: 初始化                                                 │ │
│  ├───────────────────────────────────────────────────────────────┤ │
│  │                                                               │ │
│  │  execCtx = NewExecutionContext("session-123")                 │ │
│  │  execCtx.AddMessage("user", "研究 Go 并发模型，写个示例代码")   │ │
│  │                                                               │ │
│  │  currentAgent = registry.Get("main-agent")                    │ │
│  │  execCtx.CurrentAgent = "main-agent"                          │ │
│  │                                                               │ │
│  └───────────────────────────────────────────────────────────────┘ │
│                                                                     │
│  ┌───────────────────────────────────────────────────────────────┐ │
│  │ Step 1: Main Agent                                             │ │
│  ├───────────────────────────────────────────────────────────────┤ │
│  │                                                               │ │
│  │  buildMessages:                                                │ │
│  │    System: "你是主调度 Agent，分析请求分配给专业 Agent..."      │ │
│  │    User: "研究 Go 并发模型，写个示例代码"                       │ │
│  │                                                               │ │
│  │  buildTools:                                                   │ │
│  │    • transfer_to_researcher-agent                             │ │
│  │    • transfer_to_coder-agent                                  │ │
│  │    • transfer_to_analyst-agent                                │ │
│  │                                                               │ │
│  │  LLM 思考:                                                     │ │
│  │    "这是一个复合任务：                         │ │
│  │     1. 先研究 Go 并发模型                                      │ │
│  │     2. 再写示例代码                                            │ │
│  │     应该先交给 Researcher Agent"                               │ │
│  │                                                               │ │
│  │  Tool Call:                                                    │ │
│  │    transfer_to_researcher-agent                                │ │
│  │                                                               │ │
│  │  Engine 处理:                                                  │ │
│  │    IsHandoffTool? → YES                                       │ │
│  │    ValidateHandoff? → OK                                      │ │
│  │                                                               │ │
│  │  execCtx.AddAgentRecord({                                     │ │
│  │    AgentID: "main-agent",                                     │ │
│  │    Thought: "这是复合任务，先研究...",                         │ │
│  │    Action: "handoff",                                         │ │
│  │    HandoffTo: "researcher-agent"                              │ │
│  │  })                                                           │ │
│  │                                                               │ │
│  │  currentAgent = registry.Get("researcher-agent")              │ │
│  │  execCtx.CurrentAgent = "researcher-agent"                    │ │
│  │                                                               │ │
│  │  → 继续循环                                                    │ │
│  │                                                               │ │
│  └───────────────────────────────────────────────────────────────┘ │
│                                                                     │
│  ┌───────────────────────────────────────────────────────────────┐ │
│  │ Step 2: Researcher Agent                                       │ │
│  ├───────────────────────────────────────────────────────────────┤ │
│  │                                                               │ │
│  │  buildMessages:                                                │ │
│  │    System: "你是研究助手，可以搜索信息..."                      │ │
│  │           + "之前执行: main-agent 交接给你"                    │ │
│  │    User: "研究 Go 并发模型，写个示例代码"                       │ │
│  │                                                               │ │
│  │  buildTools:                                                   │ │
│  │    • web_search                                                │ │
│  │    • knowledge_search                                         │ │
│  │    • transfer_to_coder-agent                                  │ │
│  │    • transfer_to_main-agent                                   │ │
│  │                                                               │ │
│  │  LLM 思考:                                                     │ │
│  │    "需要搜索 Go 并发的相关信息"                                │ │
│  │                                                               │ │
│  │  Tool Call 1:                                                  │ │
│  │    web_search({query: "Go concurrency model goroutine"})       │ │
│  │                                                               │ │
│  │  Engine 处理:                                                  │ │
│  │    IsHandoffTool? → NO                                        │ │
│  │    Execute Tool:                                              │ │
│  │      result = MCP.CallTool("web_search", args)                │ │
│  │      → "Go 并发基于 CSP 理论，goroutine 是轻量级线程..."        │ │
│  │                                                               │ │
│  │  execCtx.AddToolMessage("web_search", result)                 │ │
│  │  execCtx.AddAgentRecord({                                     │ │
│  │    AgentID: "researcher-agent",                               │ │
│  │    Action: "web_search",                                      │ │
│  │    Arguments: {"query": "Go concurrency..."},                 │ │
│  │    Result: "找到: Go 并发基于 CSP..."                          │ │
│  │  })                                                           │ │
│  │                                                               │ │
│  │  → 继续循环（同一个 Agent）                                    │ │
│  │                                                               │ │
│  └───────────────────────────────────────────────────────────────┘ │
│                                                                     │
│  ┌───────────────────────────────────────────────────────────────┐ │
│  │ Step 3: Researcher Agent (继续)                                │ │
│  ├───────────────────────────────────────────────────────────────┤ │
│  │                                                               │ │
│  │  buildMessages:                                                │ │
│  │    System: "你是研究助手..."                                   │ │
│  │    User: "研究 Go 并发模型..."                                 │ │
│  │    Tool: "web_search 结果: Go 并发基于 CSP..."                 │ │
│  │                                                               │ │
│  │  LLM 思考:                                                     │ │
│  │    "搜索完成，接下来需要让 Coder 写代码"                        │ │
│  │                                                               │ │
│  │  Tool Call:                                                    │ │
│  │    transfer_to_coder-agent                                     │ │
│  │                                                               │ │
│  │  Engine 处理:                                                  │ │
│  │    IsHandoffTool? → YES                                       │ │
│  │                                                               │ │
│  │  execCtx.AddAgentRecord({                                     │ │
│  │    AgentID: "researcher-agent",                               │ │
│  │    Action: "handoff",                                         │ │
│  │    HandoffTo: "coder-agent"                                   │ │
│  │  })                                                           │ │
│  │                                                               │ │
│  │  currentAgent = registry.Get("coder-agent")                   │ │
│  │  execCtx.CurrentAgent = "coder-agent"                         │ │
│  │                                                               │ │
│  │  → 继续循环                                                    │ │
│  │                                                               │ │
│  └───────────────────────────────────────────────────────────────┘ │
│                                                                     │
│  ┌───────────────────────────────────────────────────────────────┐ │
│  │ Step 4: Coder Agent                                            │ │
│  ├───────────────────────────────────────────────────────────────┤ │
│  │                                                               │ │
│  │  buildMessages:                                                │ │
│  │    System: "你是编程助手，可以写代码..."                        │ │
│  │           + "之前执行:                                        │ │
│  │               main-agent → handoff → researcher-agent         │ │
│  │               researcher-agent → web_search → Go 并发基于 CSP │ │
│  │               researcher-agent → handoff → coder-agent"       │ │
│  │           + "Tool 结果: Go 并发基于 CSP..."                    │ │
│  │    User: "研究 Go 并发模型，写个示例代码"                       │ │
│  │                                                               │ │
│  │  Coder 知道 Researcher 找到了什么！                            │ │
│  │                                                               │ │
│  │  buildTools:                                                   │ │
│  │    • code_execute                                             │ │
│  │    • file_write                                               │ │
│  │                                                               │ │
│  │  LLM 思考:                                                     │ │
│  │    "根据 Researcher 的研究结果，写一个 goroutine 示例"          │ │
│  │                                                               │ │
│  │  Tool Call:                                                    │ │
│  │    code_execute({                                             │ │
│  │      code: "                                                  │ │
│  │        package main                                           │ │
│  │        func main() {                                          │ │
│  │          ch := make(chan int)                                 │ │
│  │          go func() { ch <- 42 }()                             │ │
│  │          fmt.Println(ch)                                      │ │
│  │        }                                                      │ │
│  │      "                                                        │ │
│  │    })                                                         │ │
│  │                                                               │ │
│  │  Engine 处理:                                                  │ │
│  │    Execute Tool → 成功执行                                    │ │
│  │                                                               │ │
│  │  execCtx.AddToolMessage("code_execute", "执行成功: 输出 42")   │ │
│  │                                                               │ │
│  └───────────────────────────────────────────────────────────────┘ │
│                                                                     │
│  ┌───────────────────────────────────────────────────────────────┐ │
│  │ Step 5: Coder Agent (继续)                                     │ │
│  ├───────────────────────────────────────────────────────────────┤ │
│  │                                                               │ │
│  │  buildMessages:                                                │ │
│  │    ...包含之前的搜索结果和代码执行结果                          │ │
│  │                                                               │ │
│  │  LLM 思考:                                                     │ │
│  │    "任务完成，可以返回结果了"                                  │ │
│  │                                                               │ │
│  │  无 Tool Call，直接返回                                        │ │
│  │                                                               │ │
│  │  Final Response:                                              │ │
│  │    "Go 并发模型基于 CSP 理论...                               │ │
│  │     示例代码:                                                  │ │
│  │     package main                                              │ │
│  │     func main() {                                             │ │
│  │       ch := make(chan int)                                    │ │
│  │       go func() { ch <- 42 }()                                │ │
│  │       fmt.Println(ch)                                         │ │
│  │     }                                                         │ │
│  │     输出: 42"                                                  │ │
│  │                                                               │ │
│  └───────────────────────────────────────────────────────────────┘ │
│                                                                     │
│  ┌───────────────────────────────────────────────────────────────┐ │
│  │ 最终结果                                                       │ │
│  ├───────────────────────────────────────────────────────────────┤ │
│  │                                                               │ │
│  │  execCtx.MarkCompleted()                                      │ │
│  │  store.Save(ctx, execCtx)                                     │ │
│  │                                                               │ │
│  │  return ExecutionResult{                                      │ │
│  │    ContextID: "ctx_xxx",                                      │ │
│  │    Response: "Go 并发模型...",                                │ │
│  │    AgentHistory: [                                            │ │
│  │      {Agent: "main", Action: "handoff", To: "researcher"},    │ │
│  │      {Agent: "researcher", Action: "web_search"},             │ │
│  │      {Agent: "researcher", Action: "handoff", To: "coder"},   │ │
│  │      {Agent: "coder", Action: "code_execute"},                │ │
│  │    ],                                                         │ │
│  │    TotalTokens: 3500,                                         │ │
│  │    TotalCost: 0.08,                                           │ │
│  │    Status: "completed"                                        │ │
│  │  }                                                            │ │
│  │                                                               │ │
│  └───────────────────────────────────────────────────────────────┘ │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 五、Agent 存储（MongoDB）

### 数据结构

```javascript
// MongoDB Collection: agents
{
    "_id": "main-agent",
    "name": "Main Agent",
    "description": "主调度 Agent",
    "instructions": "你是智能调度助手...",
    "tools": [],
    "handoffs": ["researcher-agent", "coder-agent", "analyst-agent"],
    "model": "",
    "max_tokens": 4096,
    "temperature": 0.7,
    "created_at": "2026-06-15T10:00:00Z",
    "updated_at": "2026-06-15T10:00:00Z"
}
```

### 存储流程

```
启动 Agent Service
        ↓
连接 MongoDB (mongodb://localhost:27017)
        ↓
检查 agents 集合
        ↓
┌───────────────────────────────────────┐
│ 空？→ 自动插入 5 个默认 Agent          │
│ 不空？→ 直接加载                       │
└───────────────────────────────────────┘
        ↓
加载到 Registry（内存缓存）
        ↓
服务就绪
        ↓
通过 API 注册/修改 Agent
        ↓
自动同步到 MongoDB
```

---

## 六、API 使用

### 6.1 执行 Multi-Agent 任务

```bash
POST /api/v2/agents/execute
Content-Type: application/json

{
    "session_id": "session-123",
    "message": "帮我研究 Go 并发模型",
    "entry_agent": "main-agent"  // 可选，默认 main-agent
}
```

**响应：**

```json
{
    "context_id": "ctx_xxx",
    "session_id": "session-123",
    "response": "Go 并发模型基于 CSP...",
    "agent_history": [
        {
            "agent_id": "main-agent",
            "action": "handoff",
            "handoff_to": "researcher-agent"
        },
        {
            "agent_id": "researcher-agent",
            "action": "web_search",
            "result": "找到了..."
        }
    ],
    "total_tokens": 2500,
    "status": "completed"
}
```

### 6.2 注册新 Agent

```bash
POST /api/v2/agents
Content-Type: application/json

{
    "id": "custom-agent",
    "name": "Custom Agent",
    "description": "自定义 Agent",
    "instructions": "你是一个...",
    "tools": ["web_search"],
    "handoffs": ["main-agent"]
}
```

### 6.3 查看所有 Agent

```bash
GET /api/v2/agents
```

---

## 七、默认 Agent 配置

| Agent ID | 名称 | 功能 | 工具 | Handoffs |
|----------|------|------|------|----------|
| main-agent | Main Agent | 主调度，分配任务 | 无 | researcher, coder, analyst, browser |
| researcher-agent | Researcher Agent | 信息搜索 | web_search, knowledge_search | main, coder |
| coder-agent | Coder Agent | 编程 | code_execute, file_read, file_write | main, analyst |
| analyst-agent | Analyst Agent | 数据分析 | data_analysis, visualization | main |
| browser-agent | Browser Agent | 浏览器自动化 | browser_navigate, browser_click | main, researcher |

---

## 八、关键设计点

### 8.1 为什么用 Handoff 而不是固定流程？

```
传统做法（固定流程）：
┌────────────────────────────────────────────────────────────┐
│                                                          │
│  Workflow 定义:                                           │
│    Step 1: Researcher 搜索                                │
│    Step 2: Analyst 分析                                   │
│    Step 3: Coder 写代码                                   │
│                                                          │
│  问题:                                                    │
│    • 流程固定，不灵活                                      │
│    • 用户说"直接写代码"，但必须先 Researcher               │
│    • 新增 Agent 需要修改 Workflow                          │
│                                                          │
└────────────────────────────────────────────────────────────┘

Handoff 做法（LLM 决策）：
┌────────────────────────────────────────────────────────────┐
│                                                          │
│  Agent 定义:                                              │
│    Main: handoffs=[researcher, coder, analyst]            │
│    Researcher: handoffs=[coder]                           │
│                                                          │
│  用户: "直接写一个 Hello World"                            │
│                                                          │
│  Main Agent 思考:                                         │
│    "这是个简单编程任务，直接交给 Coder"                     │
│    → handoff to coder                                     │
│                                                          │
│  用户: "研究 Go 并发，写代码"                              │
│                                                          │
│  Main Agent 思考:                                         │
│    "先研究再写代码"                                        │
│    → handoff to researcher                                │
│    → researcher 搜索后 handoff to coder                   │
│                                                          │
│  优势:                                                    │
│    • LLM 智能决策，适应不同任务                            │
│    • 流程动态生成                                          │
│    • 新增 Agent 只需注册                                   │
│                                                          │
└────────────────────────────────────────────────────────────┘
```

### 8.2 如何防止无限循环？

```
┌────────────────────────────────────────────────────────────────────┐
│                     循环防护机制                                    │
├────────────────────────────────────────────────────────────────────┤
│                                                                    │
│  1. 最大步数限制（MaxSteps = 10）                                  │
│     ┌──────────────────────────────────────────────────────────┐  │
│     │ for step := 0; step < MaxSteps; step++ {                  │  │
│     │   ...                                                     │  │
│     │ }                                                         │  │
│     │ // 超过 10 步，强制停止                                    │  │
│     │ return "Maximum steps reached"                            │  │
│     └──────────────────────────────────────────────────────────┘  │
│                                                                    │
│  2. Handoff 验证                                                   │
│     ┌──────────────────────────────────────────────────────────┐  │
│     │ // 只能交接给声明的 Agent                                  │  │
│     │ if !fromAgent.CanHandoffTo(toAgentID) {                   │  │
│     │   return ErrInvalidHandoff                                │  │
│     │ }                                                         │  │
│     │                                                           │  │
│     │ // Agent 定义:                                            │  │
│     │ main-agent: handoffs=[researcher, coder]                  │  │
│     │                                                           │  │
│     │ // 如果 Main 尝试交接给 unknown-agent:                    │  │
│     │ → 错误！不允许                                             │  │
│     └──────────────────────────────────────────────────────────┘  │
│                                                                    │
│  3. 执行历史追踪                                                   │
│     ┌──────────────────────────────────────────────────────────┐  │
│     │ execCtx.AgentHistory 记录每一步                           │  │
│     │                                                           │  │
│     │ 可以检测循环:                                              │  │
│     │   main → researcher → main → researcher → main           │  │
│     │                                                           │  │
│     │ 未来可加: 循环检测，发现重复 Handoff 时警告               │  │
│     └──────────────────────────────────────────────────────────┘  │
│                                                                    │
└────────────────────────────────────────────────────────────────────┘
```

### 8.3 Agent 之间如何共享信息？

```
┌────────────────────────────────────────────────────────────────────┐
│                   上下文共享机制                                    │
├────────────────────────────────────────────────────────────────────┤
│                                                                    │
│  ExecutionContext 包含:                                           │
│                                                                    │
│  1. Variables（共享变量）                                          │
│     ┌──────────────────────────────────────────────────────────┐  │
│     │ map[string]any                                             │  │
│     │                                                           │  │
│     │ Agent A 设置:                                              │  │
│     │   execCtx.SetVariable("search_result", "...")              │  │
│     │                                                           │  │
│     │ Agent B 读取:                                              │  │
│     │   result := execCtx.GetVariable("search_result")           │  │
│     │                                                           │  │
│     │ 用于: 传递结构化数据                                        │  │
│     └──────────────────────────────────────────────────────────┘  │
│                                                                    │
│  2. Messages（对话历史）                                           │
│     ┌──────────────────────────────────────────────────────────┐  │
│     │ []Message                                                  │  │
│     │                                                           │  │
│     │ 包含:                                                      │  │
│     │   • 用户原始请求                                           │  │
│     │   • 所有 Agent 的响应                                      │  │
│     │   • 所有工具执行结果                                       │  │
│     │                                                           │  │
│     │ 新 Agent 加入时，能看到完整的对话历史                       │  │
│     │                                                           │  │
│     │ 用于: 保持对话连贯性                                        │  │
│     └──────────────────────────────────────────────────────────┘  │
│                                                                    │
│  3. AgentHistory（执行路径）                                       │
│     ┌──────────────────────────────────────────────────────────┐  │
│     │ []AgentExecutionRecord                                     │  │
│     │                                                           │  │
│     │ 记录:                                                      │  │
│     │   • 哪个 Agent                                             │  │
│     │   • 做了什么（handoff 或 tool）                            │  │
│     │   • 结果是什么                                             │  │
│     │                                                           │  │
│     │ 新 Agent 加入时，System Prompt 会包含这些信息              │  │
│     │                                                           │  │
│     │ 用于: 让 Agent 知道"之前发生了什么"                         │  │
│     └──────────────────────────────────────────────────────────┘  │
│                                                                    │
│  示例:                                                             │
│                                                                    │
│  用户: "研究 Go 并发，写代码"                                      │
│                                                                    │
│  Researcher 执行后:                                               │
│    Variables: {search_result: "Go 并发基于 CSP..."}               │
│    Messages: [user请求, web_search结果]                           │
│    AgentHistory: [main→handoff, researcher→web_search]           │
│                                                                    │
│  Coder 开始时，buildMessages:                                     │
│    System: "你是编程助手...                                        │
│             上下文: search_result=Go 并发基于 CSP...              │
│             之前: researcher 搜索了相关信息"                       │
│                                                                    │
│  Coder 就知道 Researcher 找到了什么！                              │
│                                                                    │
└────────────────────────────────────────────────────────────────────┘
```

---

## 九、代码位置

| 文件 | 功能 |
|------|------|
| `pkg/agent/agent.go` | Agent 结构定义 |
| `pkg/agent/registry.go` | Agent 注册中心（支持 MongoDB） |
| `pkg/agent/mongo_store.go` | MongoDB 存储实现 |
| `pkg/agent/defaults.go` | 默认 Agent 定义 |
| `pkg/agent/engine.go` | 执行引擎（核心循环） |
| `pkg/agent/handoff.go` | Handoff 机制实现 |
| `pkg/agent/context.go` | 执行上下文 |
| `services/agent-service/cmd/main.go` | 服务启动入口 |

---

## 十、总结

Multi-Agent 架构的核心是：

```
┌────────────────────────────────────────────────────────────────────┐
│                   核心设计思想                                      │
├────────────────────────────────────────────────────────────────────┤
│                                                                    │
│  1. Agent = Instructions + Tools + Handoffs                       │
│     • Instructions: Agent 的"性格"                                │
│     • Tools: Agent 的"能力"                                       │
│     • Handoffs: Agent 的"协作关系"                                │
│                                                                    │
│  2. Handoff = 特殊 Tool                                           │
│     • Engine 自动注入                                             │
│     • LLM 决策何时调用                                            │
│     • 触发 Agent 切换                                             │
│                                                                    │
│  3. Engine = Agent 循环                                           │
│     • buildMessages → buildTools → LLM → 处理响应                │
│     • Handoff → 切换 Agent，继续循环                             │
│     • Tool → 执行，添加结果，继续循环                             │
│                                                                    │
│  4. ExecutionContext = 共享状态                                   │
│     • Variables: 结构化数据                                       │
│     • Messages: 对话历史                                          │
│     • AgentHistory: 执行路径                                      │
│                                                                    │
│  5. MongoDB = 持久化                                              │
│     • Agent 配置存数据库                                          │
│     • 动态注册/修改                                                │
│                                                                    │
└────────────────────────────────────────────────────────────────────┘
```

这套架构参考了 **OpenAI Swarm** 的设计思想：

- **轻量级**：没有复杂的工作流定义
- **灵活性**：LLM 自己决定交接时机
- **协作性**：通过 ExecutionContext 共享状态
- **可扩展**：新增 Agent 只需注册并声明 handoffs

适合构建协作型 AI 应用，如：研究助手、编程助手、数据分析助手等场景。