# 多 Agent 架构使用指南

## 概述

Agent Platform 现已支持多 Agent 协作架构，基于 OpenAI Swarm 的 Handoff 模式实现。

## 架构

```
用户请求 → Main Agent → (Handoff) → Researcher/Coder/Analyst Agent
                 ↑_________________________|
                      (完成或继续交接)
```

## 预设 Agent

| Agent | ID | 功能 | 工具 |
|-------|-----|------|------|
| Main Agent | main-agent | 主调度，理解意图并分配任务 | 无（只有 handoff 能力） |
| Researcher Agent | researcher-agent | 信息搜索和知识检索 | web_search, knowledge_search, weather |
| Coder Agent | coder-agent | 代码编写和执行 | code_execute, file_read, file_write, calculator |
| Analyst Agent | analyst-agent | 数据分析和可视化 | data_analysis, visualization, code_execute |

## API 使用

### 1. 列出所有 Agent

```bash
curl http://localhost:9000/api/v2/agents
```

响应：
```json
{
  "code": 0,
  "data": {
    "agents": [
      {"id": "main-agent", "name": "Main Agent", ...},
      {"id": "researcher-agent", "name": "Researcher Agent", ...},
      ...
    ]
  }
}
```

### 2. 执行多 Agent 对话

```bash
curl -X POST http://localhost:9000/api/v2/agents/execute \
  -H "Content-Type: application/json" \
  -d '{
    "message": "帮我研究一下 Go 语言的并发模型",
    "entry_agent": "main-agent"
  }'
```

响应：
```json
{
  "code": 0,
  "data": {
    "context_id": "ctx_xxx",
    "response": "Go 语言的并发模型基于 CSP...",
    "agent_history": [
      {
        "agent_id": "main-agent",
        "thought": "用户想研究 Go 并发模型...",
        "action": "handoff",
        "handoff_to": "researcher-agent"
      },
      {
        "agent_id": "researcher-agent",
        "thought": "需要搜索相关信息",
        "action": "web_search",
        "result": "找到了相关文章..."
      }
    ],
    "total_tokens": 2500,
    "status": "completed"
  }
}
```

### 3. 流式执行

```bash
curl -X POST http://localhost:9000/api/v2/agents/execute/stream \
  -H "Content-Type: application/json" \
  -d '{
    "message": "写一个 Python 快速排序",
    "entry_agent": "main-agent"
  }'
```

### 4. 注册自定义 Agent

```bash
curl -X POST http://localhost:9000/api/v2/agents \
  -H "Content-Type: application/json" \
  -d '{
    "id": "custom-agent",
    "name": "Custom Agent",
    "description": "自定义 Agent",
    "instructions": "你是一个自定义 Agent...",
    "tools": ["web_search", "calculator"],
    "handoffs": ["main-agent"]
  }'
```

## Agent 配置文件

Agent 配置位于 `configs/agents/` 目录，使用 YAML 格式：

```yaml
id: my-agent
name: My Agent
description: 自定义 Agent 描述
instructions: |
  你是一个专业的助手...
  
  ## 可用工具
  - tool1: 工具1描述
  - tool2: 工具2描述
  
  ## 工作流程
  1. 步骤1
  2. 步骤2
tools:
  - web_search
  - calculator
handoffs:
  - main-agent
  - researcher-agent
model: ""  # 空 = 使用默认模型
max_tokens: 4096
temperature: 0.7
```

## 工具列表

| 工具 | 描述 |
|------|------|
| web_search | 网络搜索 |
| knowledge_search | 知识库搜索 |
| weather | 天气查询 |
| calculator | 计算器 |
| code_execute | 代码执行 |
| file_read | 文件读取 |
| file_write | 文件写入 |
| data_analysis | 数据分析 |
| visualization | 可视化 |

## 示例场景

### 场景 1: 研究任务

```
用户: 帮我研究一下微服务架构的优缺点

流程:
1. Main Agent 分析请求 → Handoff to Researcher Agent
2. Researcher Agent 使用 web_search 搜索
3. Researcher Agent 整理并返回结果
4. Main Agent 接收并返回给用户
```

### 场景 2: 编程任务

```
用户: 写一个 Python 脚本计算斐波那契数列

流程:
1. Main Agent 分析请求 → Handoff to Coder Agent
2. Coder Agent 使用 code_execute 编写和测试代码
3. Coder Agent 返回代码
4. Main Agent 返回给用户
```

### 场景 3: 数据分析任务

```
用户: 分析这组数据的统计特征 [1, 2, 3, 4, 5, 6, 7, 8, 9, 10]

流程:
1. Main Agent 分析请求 → Handoff to Analyst Agent
2. Analyst Agent 使用 data_analysis 计算统计量
3. Analyst Agent 使用 visualization 生成图表配置
4. Main Agent 返回分析结果
```

## 注意事项

1. **最大步数限制**: 默认最多 10 步，防止无限循环
2. **Token 消耗**: 多 Agent 会增加 Token 消耗
3. **Handoff 验证**: 只能 handoff 给配置中声明的 Agent
4. **上下文共享**: Agent 间共享 ExecutionContext

## 配置环境变量

```bash
# .env
OPENAI_API_KEY=your-api-key
LLM_PROVIDER=openai
LLM_MODEL=gpt-4
```

## Docker 部署

```bash
docker-compose up -d
```

服务端口：
- Gateway: 9000
- Chat Service: 50001
- Knowledge Service: 50002
- Memory Service: 50003
- A2A Service: 50004
- MCP Service: 50005
- **Agent Service: 50006** (新增)
