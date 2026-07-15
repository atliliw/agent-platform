---
name: multi-agent-architecture-plan
description: 多Agent架构改造计划书
metadata:
  type: project
---

# 多 Agent 架构改造计划书

## 一、项目概述

### 1.1 背景

当前 Agent 平台是**单 Agent + 工具调用**架构：
```
用户请求 → Chat Agent → MCP 工具调用 → 返回结果
```

需要改造为**多 Agent 协作架构**，支持 Agent 间交接和协作。

### 1.2 目标

参考 OpenAI Swarm、CrewAI、MetaGPT 等优秀开源框架，实现：

1. **Handoff 机制**：Agent 间可以相互交接对话
2. **多 Agent 协作**：多个专业 Agent 协同完成任务
3. **Context 共享**：Agent 间共享执行上下文
4. **动态注册**：支持运行时注册新 Agent

### 1.3 架构对比

| 特性 | 当前架构 | 目标架构 |
|------|----------|----------|
| Agent 数量 | 1 个 | N 个 |
| 协作方式 | 无 | Handoff 交接 |
| 上下文 | 单 Agent 内 | 跨 Agent 共享 |
| Agent 定义 | 硬编码 | 配置 + 动态注册 |
| 工具调用 | MCP | MCP + Agent 交接 |

---

## 二、技术方案

### 2.1 核心架构设计

```
┌─────────────────────────────────────────────────────────────────────┐
│                           Gateway (HTTP API)                        │
└──────────────────────────────┬──────────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────────┐
│                         Agent Orchestrator                           │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │                    Agent Registry                            │   │
│  │   ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐       │   │
│  │   │  Main   │  │Researcher│  │  Coder  │  │ Analyst │       │   │
│  │   │  Agent  │  │  Agent   │  │  Agent  │  │  Agent  │       │   │
│  │   └────┬────┘  └────┬────┘  └────┬────┘  └────┬────┘       │   │
│  │        │            │            │            │             │   │
│  │        └────────────┴────────────┴────────────┘             │   │
│  │                     Handoff Graph                           │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                               │                                      │
│  ┌────────────────────────────┼────────────────────────────────┐   │
│  │                     Execution Engine                         │   │
│  │  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐     │   │
│  │  │  LLM Client │    │  Tool Router│    │  Context    │     │   │
│  │  │             │    │             │    │  Manager    │     │   │
│  │  └─────────────┘    └─────────────┘    └─────────────┘     │   │
│  └─────────────────────────────────────────────────────────────┘   │
└──────────────────────────────┬──────────────────────────────────────┘
                               │
          ┌────────────────────┼────────────────────┐
          │                    │                    │
          ▼                    ▼                    ▼
    ┌───────────┐        ┌───────────┐        ┌───────────┐
    │    MCP    │        │    A2A    │        │  Memory   │
    │  Service  │        │  Service  │        │  Service  │
    │  (Tools)  │        │ (Agents)  │        │  (RAG)    │
    └───────────┘        └───────────┘        └───────────┘
```

### 2.2 核心数据结构

#### 2.2.1 Agent 定义

```go
// Agent 定义
type Agent struct {
    ID           string            `json:"id"`
    Name         string            `json:"name"`
    Description  string            `json:"description"`
    Instructions string            `json:"instructions"`  // System prompt
    Tools        []string          `json:"tools"`         // 可用工具列表
    Handoffs     []string          `json:"handoffs"`      // 可交接的 Agent ID 列表
    Metadata     map[string]any    `json:"metadata"`      // 扩展元数据
    CreatedAt    time.Time         `json:"created_at"`
    UpdatedAt    time.Time         `json:"updated_at"`
}
```

#### 2.2.2 执行上下文

```go
// ExecutionContext 跨 Agent 共享的执行上下文
type ExecutionContext struct {
    ID              string                   `json:"id"`
    SessionID       string                   `json:"session_id"`
    Variables       map[string]any           `json:"variables"`       // 共享变量
    Messages        []Message                `json:"messages"`        // 对话历史
    AgentHistory    []AgentExecutionRecord   `json:"agent_history"`   // Agent 执行记录
    ToolResults     map[string]ToolResult    `json:"tool_results"`    // 工具执行结果
    CurrentAgent    string                   `json:"current_agent"`   // 当前 Agent
    Status          string                   `json:"status"`          // running, completed, error
    StartedAt       time.Time                `json:"started_at"`
    CompletedAt     *time.Time               `json:"completed_at"`
}

// AgentExecutionRecord Agent 执行记录
type AgentExecutionRecord struct {
    AgentID     string    `json:"agent_id"`
    Thought     string    `json:"thought"`
    Action      string    `json:"action"`
    Result      string    `json:"result"`
    HandoffTo   string    `json:"handoff_to,omitempty"`  // 交接给了谁
    StartedAt   time.Time `json:"started_at"`
    CompletedAt time.Time `json:"completed_at"`
}
```

#### 2.2.3 Handoff 结果

```go
// HandoffResult 交接结果
type HandoffResult struct {
    Type        string         `json:"type"`         // "response" | "handoff" | "tool_call"
    Content     string         `json:"content"`      // 响应内容
    NextAgent   string         `json:"next_agent"`   // 下一个 Agent ID
    Variables   map[string]any `json:"variables"`    // 更新的上下文变量
    ToolCalls   []ToolCall     `json:"tool_calls"`   // 工具调用
}
```

### 2.3 执行引擎核心逻辑

```go
// ExecutionEngine 执行引擎
type ExecutionEngine struct {
    registry      *AgentRegistry
    llmClient     llm.Client
    toolRouter    *ToolRouter
    contextStore  ContextStore
}

// Run 执行 Agent 循环
func (e *ExecutionEngine) Run(ctx context.Context, req *ExecutionRequest) (*ExecutionResult, error) {
    // 1. 获取或创建执行上下文
    execCtx := e.getOrCreateContext(req)

    // 2. 获取入口 Agent
    currentAgent := e.registry.Get(req.AgentID)
    if currentAgent == nil {
        currentAgent = e.registry.GetDefault()
    }

    // 3. Agent 循环（最大 10 步）
    for step := 0; step < 10; step++ {
        // 记录当前 Agent
        execCtx.CurrentAgent = currentAgent.ID

        // 构建 prompt（包含 handoff 工具）
        messages := e.buildMessages(currentAgent, execCtx)
        tools := e.buildTools(currentAgent)

        // 调用 LLM
        response, err := e.llmClient.Chat(ctx, &llm.ChatRequest{
            Messages:    messages,
            Tools:       tools,
            MaxTokens:   4096,
            Temperature: 0.7,
        })
        if err != nil {
            return nil, err
        }

        // 检查是否需要工具调用
        if len(response.ToolCalls) == 0 {
            // 无工具调用，直接返回
            return e.buildResult(execCtx, response.Content), nil
        }

        // 处理工具调用
        for _, tc := range response.ToolCalls {
            // 检查是否是 handoff
            if strings.HasPrefix(tc.Function.Name, "transfer_to_") {
                agentID := strings.TrimPrefix(tc.Function.Name, "transfer_to_")
                currentAgent = e.registry.Get(agentID)

                // 记录交接
                execCtx.AgentHistory = append(execCtx.AgentHistory, AgentExecutionRecord{
                    AgentID:   execCtx.CurrentAgent,
                    Thought:   response.Content,
                    Action:    "handoff",
                    Result:    fmt.Sprintf("Transferred to %s", agentID),
                    HandoffTo: agentID,
                })
                continue
            }

            // 执行普通工具
            result := e.toolRouter.Execute(ctx, tc.Function.Name, tc.Function.Arguments)

            // 添加到消息历史
            execCtx.Messages = append(execCtx.Messages, Message{
                Role:    "tool",
                Content: result,
                Name:    tc.Function.Name,
            })
        }
    }

    return e.buildResult(execCtx, "Maximum steps reached"), nil
}
```

### 2.4 Handoff 工具注入

每个 Agent 的工具列表会自动注入 `transfer_to_xxx` 工具：

```go
// buildTools 构建工具列表，包含 handoff 工具
func (e *ExecutionEngine) buildTools(agent *Agent) []llm.ToolDefinition {
    tools := make([]llm.ToolDefinition, 0)

    // 添加 MCP 工具
    for _, toolName := range agent.Tools {
        if tool, ok := e.toolRouter.Get(toolName); ok {
            tools = append(tools, tool.ToDefinition())
        }
    }

    // 注入 Handoff 工具
    for _, targetAgentID := range agent.Handoffs {
        targetAgent := e.registry.Get(targetAgentID)
        if targetAgent != nil {
            tools = append(tools, llm.ToolDefinition{
                Type: "function",
                Function: llm.ToolFunction{
                    Name:        fmt.Sprintf("transfer_to_%s", targetAgentID),
                    Description: fmt.Sprintf("Transfer conversation to %s. %s", targetAgent.Name, targetAgent.Description),
                    Parameters: map[string]any{
                        "type":       "object",
                        "properties": map[string]any{},
                    },
                },
            })
        }
    }

    return tools
}
```

---

## 三、预设 Agent 配置

### 3.1 Main Agent（主调度 Agent）

```yaml
id: main-agent
name: Main Agent
description: 主调度 Agent，负责理解用户意图并分配任务
instructions: |
  你是一个智能调度助手。根据用户请求，决定应该交给哪个专业 Agent 处理：

  - Researcher Agent: 研究类任务，如搜索信息、查找资料
  - Coder Agent: 编程类任务，如写代码、调试程序
  - Analyst Agent: 分析类任务，如数据分析、生成报告

  分析用户请求，选择最合适的 Agent 进行交接。
tools: []
handoffs:
  - researcher-agent
  - coder-agent
  - analyst-agent
```

### 3.2 Researcher Agent（研究 Agent）

```yaml
id: researcher-agent
name: Researcher Agent
description: 研究 Agent，负责信息搜索和知识检索
instructions: |
  你是一个专业的研究助手。你可以：
  1. 使用 web_search 工具搜索网络信息
  2. 使用 knowledge_search 工具检索知识库

  根据用户的问题，进行全面的信息收集，并整理成清晰的答案。
tools:
  - web_search
  - knowledge_search
handoffs:
  - main-agent
  - coder-agent
```

### 3.3 Coder Agent（编程 Agent）

```yaml
id: coder-agent
name: Coder Agent
description: 编程 Agent，负责代码编写和执行
instructions: |
  你是一个专业的编程助手。你可以：
  1. 使用 code_execute 工具执行代码
  2. 使用 file_read/file_write 工具读写文件

  根据用户的需求，编写、调试或优化代码。
tools:
  - code_execute
  - file_read
  - file_write
handoffs:
  - main-agent
  - analyst-agent
```

### 3.4 Analyst Agent（分析 Agent）

```yaml
id: analyst-agent
name: Analyst Agent
description: 分析 Agent，负责数据分析和可视化
instructions: |
  你是一个专业的数据分析助手。你可以：
  1. 使用 data_analysis 工具进行数据分析
  2. 使用 visualization 工具生成图表

  根据用户提供的数据，进行深入分析并生成洞察报告。
tools:
  - data_analysis
  - visualization
handoffs:
  - main-agent
```

---

## 四、API 设计

### 4.1 Agent 注册 API

```http
POST /api/v2/agents
Content-Type: application/json

{
  "id": "researcher-agent",
  "name": "Researcher Agent",
  "description": "研究 Agent，负责信息搜索",
  "instructions": "你是一个专业的研究助手...",
  "tools": ["web_search", "knowledge_search"],
  "handoffs": ["main-agent", "coder-agent"]
}
```

### 4.2 多 Agent 对话 API

```http
POST /api/v2/chat/multi-agent
Content-Type: application/json

{
  "session_id": "session-123",
  "message": "帮我研究一下 Go 语言的并发模型",
  "entry_agent": "main-agent",  // 可选，默认 main-agent
  "context": {                   // 可选，初始上下文
    "user_name": "张三"
  }
}
```

### 4.3 响应格式

```json
{
  "code": 0,
  "data": {
    "session_id": "session-123",
    "final_response": "Go 语言的并发模型基于 CSP...",
    "execution_path": [
      {
        "agent_id": "main-agent",
        "thought": "用户想要研究 Go 并发模型...",
        "action": "handoff",
        "handoff_to": "researcher-agent"
      },
      {
        "agent_id": "researcher-agent",
        "thought": "需要搜索 Go 并发相关信息",
        "action": "web_search",
        "result": "找到了相关文章..."
      }
    ],
    "total_tokens": 2500,
    "cost": 0.05
  }
}
```

---

## 五、实现计划

### 5.1 阶段划分

```
Phase 1: 核心框架（3天）
├── Agent Registry（Agent 注册中心）
├── Execution Context（执行上下文）
├── Execution Engine（执行引擎）
└── Handoff 机制

Phase 2: 预设 Agent（2天）
├── Main Agent
├── Researcher Agent
├── Coder Agent
└── Analyst Agent

Phase 3: 工具扩展（2天）
├── code_execute 工具
├── file_read/write 工具
├── data_analysis 工具
└── visualization 工具

Phase 4: API & 集成（1天）
├── Agent 注册 API
├── 多 Agent 对话 API
└── 与现有系统集成

Phase 5: 测试 & 文档（1天）
├── 单元测试
├── 集成测试
└── 使用文档
```

### 5.2 详细任务清单

#### Phase 1: 核心框架（预计 3 天）

| 任务 | 文件 | 描述 |
|------|------|------|
| 1.1 | `pkg/agent/agent.go` | Agent 结构定义 |
| 1.2 | `pkg/agent/registry.go` | Agent 注册中心 |
| 1.3 | `pkg/agent/context.go` | 执行上下文管理 |
| 1.4 | `pkg/agent/engine.go` | 执行引擎核心逻辑 |
| 1.5 | `pkg/agent/handoff.go` | Handoff 工具注入和执行 |
| 1.6 | `pkg/agent/store.go` | 上下文持久化存储 |

#### Phase 2: 预设 Agent（预计 2 天）

| 任务 | 文件 | 描述 |
|------|------|------|
| 2.1 | `configs/agents/main.yaml` | Main Agent 配置 |
| 2.2 | `configs/agents/researcher.yaml` | Researcher Agent 配置 |
| 2.3 | `configs/agents/coder.yaml` | Coder Agent 配置 |
| 2.4 | `configs/agents/analyst.yaml` | Analyst Agent 配置 |
| 2.5 | `pkg/agent/loader.go` | YAML 配置加载器 |

#### Phase 3: 工具扩展（预计 2 天）

| 任务 | 文件 | 描述 |
|------|------|------|
| 3.1 | `services/mcp-service/internal/tools/code_execute.go` | 代码执行工具 |
| 3.2 | `services/mcp-service/internal/tools/file_ops.go` | 文件读写工具 |
| 3.3 | `services/mcp-service/internal/tools/data_analysis.go` | 数据分析工具 |
| 3.4 | `services/mcp-service/internal/tools/visualization.go` | 可视化工具 |

#### Phase 4: API & 集成（预计 1 天）

| 任务 | 文件 | 描述 |
|------|------|------|
| 4.1 | `proto/agent/agent.proto` | Agent gRPC 协议定义 |
| 4.2 | `services/agent-service/*` | Agent 服务实现 |
| 4.3 | `services/gateway/internal/handler/agent_handler.go` | HTTP API Handler |
| 4.4 | `docker-compose.yml` | 更新部署配置 |

#### Phase 5: 测试 & 文档（预计 1 天）

| 任务 | 文件 | 描述 |
|------|------|------|
| 5.1 | `pkg/agent/*_test.go` | 单元测试 |
| 5.2 | `tests/integration/agent_test.go` | 集成测试 |
| 5.3 | `docs/multi-agent-usage.md` | 使用文档 |
| 5.4 | `README.md` | 更新主文档 |

---

## 六、文件结构

```
agent-platform/
├── pkg/
│   └── agent/                    # Agent 核心包
│       ├── agent.go              # Agent 结构定义
│       ├── registry.go           # Agent 注册中心
│       ├── context.go            # 执行上下文
│       ├── engine.go             # 执行引擎
│       ├── handoff.go            # Handoff 机制
│       ├── store.go              # 上下文存储
│       ├── loader.go             # YAML 加载器
│       └── errors.go             # 错误定义
│
├── services/
│   ├── agent-service/            # 新增：Agent 服务
│   │   ├── cmd/
│   │   │   └── main.go
│   │   ├── internal/
│   │   │   ├── handler/
│   │   │   │   └── grpc_handler.go
│   │   │   └── service/
│   │   │       └── agent_service.go
│   │   └── Dockerfile
│   │
│   └── mcp-service/
│       └── internal/
│           └── tools/
│               ├── code_execute.go    # 新增
│               ├── file_ops.go        # 新增
│               ├── data_analysis.go   # 新增
│               └── visualization.go   # 新增
│
├── proto/
│   └── agent/
│       └── agent.proto           # 新增：Agent 协议定义
│
├── configs/
│   └── agents/                   # 新增：Agent 配置目录
│       ├── main.yaml
│       ├── researcher.yaml
│       ├── coder.yaml
│       └── analyst.yaml
│
└── docs/
    ├── agent-frameworks-research.md
    ├── multi-agent-architecture-plan.md
    └── multi-agent-usage.md      # 新增：使用文档
```

---

## 七、风险与对策

| 风险 | 影响 | 对策 |
|------|------|------|
| LLM 调用成本增加 | 多 Agent 会增加 LLM 调用次数 | 设置最大步数限制，优化 prompt |
| Agent 循环交接 | 陷入无限循环 | 设置最大步数（10 步），记录历史防止重复 |
| 上下文过大 | Token 超限 | 实现上下文压缩和摘要 |
| 工具执行失败 | Agent 无法继续 | 提供错误信息，让 Agent 重试或交接 |

---

## 八、验收标准

### 8.1 功能验收

- [ ] 支持注册自定义 Agent
- [ ] Agent 间可以相互交接
- [ ] 执行路径可追溯
- [ ] 支持 YAML 配置 Agent
- [ ] 提供完整的 HTTP API

### 8.2 性能验收

- [ ] 单次对话响应时间 < 10s
- [ ] 支持并发请求
- [ ] 内存使用稳定

### 8.3 文档验收

- [ ] API 文档完整
- [ ] 使用示例清晰
- [ ] 架构图准确

---

## 九、参考资源

- [OpenAI Swarm](https://github.com/openai/swarm) - Handoff 机制参考
- [CrewAI](https://github.com/joaomdmoura/crewAI) - YAML 配置参考
- [MetaGPT](https://github.com/geekan/MetaGPT) - Agent 角色设计参考
- [LangGraph](https://github.com/langchain-ai/langgraph) - 状态管理参考

---

**计划创建时间**: 2026-06-05
**预计完成时间**: 9 天
**负责人**: AI Agent Platform Team
