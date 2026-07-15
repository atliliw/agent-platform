# 05 - API 设计

## 1. API 设计原则

### 1.1 设计规范

| 规范 | 说明 |
|------|------|
| RESTful | HTTP API 遵循 REST 设计原则 |
| 版本化 | 所有 API 使用 `/api/v2/` 前缀 |
| 统一响应 | 使用统一响应格式 |
| 错误处理 | HTTP 状态码 + 详细错误信息 |
| 认证 | JWT Bearer Token |
| 租户隔离 | Header: `X-Tenant-ID` |

### 1.2 统一响应格式

```json
// 成功响应
{
  "code": 0,
  "message": "success",
  "data": { ... }
}

// 错误响应
{
  "code": 10001,
  "message": "invalid parameter",
  "data": null
}
```

### 1.3 错误码定义

| 错误码 | 说明 |
|--------|------|
| 0 | 成功 |
| 10001 | 参数错误 |
| 10002 | 认证失败 |
| 10003 | 权限不足 |
| 10004 | 资源不存在 |
| 10005 | 服务内部错误 |
| 20001 | 护栏拦截 |
| 20002 | 规则拦截 |
| 20003 | 预算耗尽 |
| 20004 | 限流拦截 |

---

## 2. Chat API

### 2.1 对话接口

**POST `/api/v2/chat`**

请求：
```json
{
  "session_id": "session-123",        // 可选，不提供则创建新会话
  "message": "你好，请介绍一下这个项目",
  "model": "gpt-4",                   // 可选，默认使用配置
  "system_prompt": "...",             // 可选
  "tool_ids": ["search", "calculator"], // 可选，指定可用工具
  "stream": false                     // 是否流式响应
}
```

响应：
```json
{
  "code": 0,
  "data": {
    "session_id": "session-123",
    "content": "这个项目是一个...",
    "tool_calls": [
      {
        "id": "call-1",
        "name": "search",
        "arguments": "{\"query\": \"项目介绍\"}",
        "result": "找到相关文档..."
      }
    ],
    "total_tokens": 1500,
    "cost": 0.03
  }
}
```

### 2.2 流式对话接口

**POST `/api/v2/chat/stream`**

请求：同上，`stream: true`

响应：SSE 流
```
event: content
data: {"content": "这个"}

event: content
data: {"content": "项目"}

event: tool_call
data: {"id": "call-1", "name": "search", "arguments": "{}"}

event: tool_result
data: {"id": "call-1", "result": "找到相关文档..."}

event: done
data: {"total_tokens": 1500, "cost": 0.03}
```

### 2.3 会话管理接口

**GET `/api/v2/sessions`**

查询参数：
- `page`: 页码 (默认 1)
- `page_size`: 每页数量 (默认 20)

响应：
```json
{
  "code": 0,
  "data": {
    "sessions": [
      {
        "id": "session-123",
        "title": "项目讨论",
        "created_at": "2024-01-01T10:00:00Z",
        "updated_at": "2024-01-01T12:00:00Z",
        "message_count": 10
      }
    ],
    "pagination": {
      "total": 50,
      "page": 1,
      "page_size": 20
    }
  }
}
```

**GET `/api/v2/sessions/:id`**

响应：
```json
{
  "code": 0,
  "data": {
    "id": "session-123",
    "title": "项目讨论",
    "messages": [
      {
        "id": "msg-1",
        "role": "user",
        "content": "你好",
        "created_at": "2024-01-01T10:00:00Z"
      },
      {
        "id": "msg-2",
        "role": "assistant",
        "content": "你好！有什么可以帮助你的？",
        "tool_calls": [],
        "created_at": "2024-01-01T10:01:00Z"
      }
    ],
    "created_at": "2024-01-01T10:00:00Z",
    "updated_at": "2024-01-01T12:00:00Z"
  }
}
```

**DELETE `/api/v2/sessions/:id`**

响应：
```json
{
  "code": 0,
  "message": "session deleted"
}
```

### 2.4 多 Agent 对话接口

**POST `/api/v2/multi-agent/chat`**

请求：
```json
{
  "session_id": "session-123",
  "message": "请帮我分析这个项目的代码质量，并给出改进建议",
  "master_agent": "planner"        // 可选，默认使用 planner
}
```

响应：
```json
{
  "code": 0,
  "data": {
    "session_id": "session-123",
    "final_answer": "经过分析，代码质量整体良好...",
    "steps": [
      {
        "agent_id": "researcher",
        "action": "搜索项目代码相关资料",
        "result": "找到 10 篇相关文档..."
      },
      {
        "agent_id": "analyst",
        "action": "分析代码质量问题",
        "result": "发现以下问题：..."
      },
      {
        "agent_id": "writer",
        "action": "撰写改进建议",
        "result": "建议如下：..."
      }
    ]
  }
}
```

---

## 3. Knowledge API

### 3.1 文件上传接口

**POST `/api/v2/knowledge/upload`**

请求：multipart/form-data
- `file`: 文件 (PDF/Word/MD/TXT/JSON/CSV)
- `chunk_strategy`: 分块策略 (token/semantic/markdown)
- `chunk_size`: 分块大小 (默认 512)
- `chunk_overlap`: 重叠大小 (默认 50)

响应：
```json
{
  "code": 0,
  "data": {
    "document_id": "doc-123",
    "filename": "project-guide.pdf",
    "chunk_count": 50,
    "chunk_ids": ["chunk-1", "chunk-2", ...],
    "status": "ready"
  }
}
```

### 3.2 文档管理接口

**GET `/api/v2/knowledge/documents`**

查询参数：
- `page`: 页码
- `page_size`: 每页数量
- `status`: 状态过滤 (ready/processing/error)

响应：
```json
{
  "code": 0,
  "data": {
    "documents": [
      {
        "id": "doc-123",
        "filename": "project-guide.pdf",
        "title": "项目指南",
        "chunk_count": 50,
        "status": "ready",
        "created_at": "2024-01-01T10:00:00Z"
      }
    ],
    "pagination": {
      "total": 100,
      "page": 1,
      "page_size": 20
    }
  }
}
```

**GET `/api/v2/knowledge/documents/:id`**

响应：
```json
{
  "code": 0,
  "data": {
    "id": "doc-123",
    "filename": "project-guide.pdf",
    "title": "项目指南",
    "content": "原始文档内容...",
    "chunk_count": 50,
    "chunks": [
      {
        "id": "chunk-1",
        "content": "第一个分块内容...",
        "chunk_index": 0
      }
    ],
    "metadata": {
      "author": "张三",
      "created_date": "2024-01-01"
    },
    "created_at": "2024-01-01T10:00:00Z"
  }
}
```

**DELETE `/api/v2/knowledge/documents/:id`**

响应：
```json
{
  "code": 0,
  "message": "document deleted"
}
```

### 3.3 检索接口

**POST `/api/v2/knowledge/search`**

请求：
```json
{
  "query": "如何配置这个项目",
  "top_k": 10,                       // 返回结果数量
  "search_type": "hybrid",           // vector/bm25/hybrid
  "score_threshold": 0.5,            // 最低分数阈值
  "filters": {                       // 可选过滤条件
    "document_id": "doc-123",
    "author": "张三"
  }
}
```

响应：
```json
{
  "code": 0,
  "data": {
    "results": [
      {
        "chunk_id": "chunk-5",
        "document_id": "doc-123",
        "content": "配置步骤如下...",
        "score": 0.85,
        "metadata": {
          "chunk_index": 5,
          "source_file": "project-guide.pdf"
        }
      }
    ],
    "total": 10
  }
}
```

---

## 4. Memory API

### 4.1 保存记忆接口

**POST `/api/v2/memory`**

请求：
```json
{
  "session_id": "session-123",
  "agent_id": "default",
  "type": "important",              // important/summary/fact
  "content": "用户偏好使用中文",
  "importance": 0.8                 // 重要程度 0-1
}
```

响应：
```json
{
  "code": 0,
  "data": {
    "id": "memory-123",
    "created_at": "2024-01-01T10:00:00Z"
  }
}
```

### 4.2 召回记忆接口

**POST `/api/v2/memory/recall`**

请求：
```json
{
  "query": "用户偏好",
  "session_id": "session-123",      // 可选，过滤特定会话
  "agent_id": "default",            // 可选，过滤特定 Agent
  "top_k": 5
}
```

响应：
```json
{
  "code": 0,
  "data": {
    "memories": [
      {
        "id": "memory-123",
        "session_id": "session-123",
        "agent_id": "default",
        "type": "important",
        "content": "用户偏好使用中文",
        "importance": 0.8,
        "created_at": "2024-01-01T10:00:00Z"
      }
    ]
  }
}
```

### 4.3 获取会话记忆接口

**GET `/api/v2/memory/session/:id`**

响应：
```json
{
  "code": 0,
  "data": {
    "memories": [
      {
        "id": "memory-123",
        "type": "important",
        "content": "用户偏好使用中文",
        "importance": 0.8
      }
    ]
  }
}
```

### 4.4 删除会话记忆接口

**DELETE `/api/v2/memory/session/:id`**

响应：
```json
{
  "code": 0,
  "message": "session memories deleted"
}
```

---

## 5. A2A API

### 5.1 Agent 发现接口

**POST `/api/v2/a2a/discover`**

请求：
```json
{
  "agent_url": "https://remote.example.com"
}
```

响应：
```json
{
  "code": 0,
  "data": {
    "card": {
      "id": "remote-agent-1",
      "name": "数据分析专家",
      "description": "擅长数据分析和可视化",
      "capabilities": ["data_analysis", "visualization"],
      "input_modes": ["text", "json"],
      "output_modes": ["text", "json"],
      "url": "https://remote.example.com"
    }
  }
}
```

### 5.2 Agent 注册接口

**POST `/api/v2/a2a/agents`**

请求：
```json
{
  "id": "local-agent-1",
  "name": "本地搜索专家",
  "description": "擅长知识库搜索",
  "capabilities": ["search", "rag"],
  "input_modes": ["text"],
  "output_modes": ["text"],
  "url": "https://local.example.com"
}
```

响应：
```json
{
  "code": 0,
  "message": "agent registered"
}
```

### 5.3 Agent 列表接口

**GET `/api/v2/a2a/agents`**

响应：
```json
{
  "code": 0,
  "data": {
    "agents": [
      {
        "id": "local-agent-1",
        "name": "本地搜索专家",
        "url": "https://local.example.com",
        "status": "active"
      },
      {
        "id": "remote-agent-1",
        "name": "数据分析专家",
        "url": "https://remote.example.com",
        "status": "active"
      }
    ]
  }
}
```

### 5.4 任务发送接口

**POST `/api/v2/a2a/tasks/send`**

请求：
```json
{
  "agent_id": "remote-agent-1",
  "message": {
    "role": "user",
    "content": "请分析这组数据"
  },
  "metadata": {
    "priority": "high"
  }
}
```

响应：
```json
{
  "code": 0,
  "data": {
    "task": {
      "id": "task-123",
      "agent_id": "remote-agent-1",
      "status": "working",
      "messages": [
        {
          "role": "user",
          "content": "请分析这组数据"
        }
      ],
      "created_at": "2024-01-01T10:00:00Z"
    }
  }
}
```

### 5.5 任务查询接口

**GET `/api/v2/a2a/tasks/:id`**

响应：
```json
{
  "code": 0,
  "data": {
    "task": {
      "id": "task-123",
      "agent_id": "remote-agent-1",
      "status": "completed",
      "messages": [
        {
          "role": "user",
          "content": "请分析这组数据"
        },
        {
          "role": "agent",
          "content": "分析结果如下..."
        }
      ],
      "result": "分析结果如下...",
      "created_at": "2024-01-01T10:00:00Z",
      "updated_at": "2024-01-01T10:05:00Z"
    }
  }
}
```

### 5.6 任务取消接口

**POST `/api/v2/a2a/tasks/:id/cancel`**

响应：
```json
{
  "code": 0,
  "data": {
    "task": {
      "id": "task-123",
      "status": "cancelled"
    }
  }
}
```

### 5.7 Agent Card 公开接口

**GET `/.well-known/agent.json`**

响应：
```json
{
  "id": "local-agent-1",
  "name": "本地 Agent 平台",
  "description": "提供知识库搜索和多 Agent 协作能力",
  "capabilities": ["search", "rag", "multi_agent"],
  "input_modes": ["text", "json"],
  "output_modes": ["text", "json"],
  "url": "https://local.example.com"
}
```

---

## 6. MCP API

### 6.1 工具列表接口

**GET `/api/v2/mcp/tools`**

查询参数：
- `connection_id`: 可选，指定 MCP 服务器

响应：
```json
{
  "code": 0,
  "data": {
    "tools": [
      {
        "name": "search",
        "description": "搜索知识库",
        "input_schema": {
          "type": "object",
          "properties": {
            "query": {
              "type": "string",
              "description": "搜索查询"
            }
          },
          "required": ["query"]
        }
      },
      {
        "name": "web_search",
        "description": "网络搜索",
        "input_schema": {
          "type": "object",
          "properties": {
            "query": {
              "type": "string",
              "description": "搜索查询"
            }
          },
          "required": ["query"]
        }
      }
    ]
  }
}
```

### 6.2 工具调用接口

**POST `/api/v2/mcp/call`**

请求：
```json
{
  "name": "search",
  "arguments": {
    "query": "项目配置"
  }
}
```

响应：
```json
{
  "code": 0,
  "data": {
    "is_error": false,
    "content": "找到 10 条相关结果..."
  }
}
```

### 6.3 MCP 连接管理接口

**POST `/api/v2/mcp/connect`**

请求：
```json
{
  "name": "playwright-server",
  "type": "stdio",                  // stdio/sse/websocket
  "command": "npx -y @anthropic/mcp-server-playwright",
  "env": {
    "BROWSER": "chrome"
  }
}
```

响应：
```json
{
  "code": 0,
  "data": {
    "connection": {
      "id": "conn-123",
      "name": "playwright-server",
      "type": "stdio",
      "status": "connected"
    }
  }
}
```

**GET `/api/v2/mcp/connections`**

响应：
```json
{
  "code": 0,
  "data": {
    "connections": [
      {
        "id": "conn-123",
        "name": "playwright-server",
        "type": "stdio",
        "status": "connected"
      }
    ]
  }
}
```

**DELETE `/api/v2/mcp/connections/:id`**

响应：
```json
{
  "code": 0,
  "message": "connection closed"
}
```

### 6.4 资源接口

**GET `/api/v2/mcp/resources`**

响应：
```json
{
  "code": 0,
  "data": {
    "resources": [
      {
        "uri": "config://main",
        "name": "主配置",
        "description": "系统主配置",
        "mime_type": "application/json"
      }
    ]
  }
}
```

**GET `/api/v2/mcp/resources/:uri`**

响应：
```json
{
  "code": 0,
  "data": {
    "content": "{\"server\": {...}}",
    "mime_type": "application/json"
  }
}
```

---

## 7. Harness API

### 7.1 治理对话接口

**POST `/api/v2/harness/chat`**

请求：
```json
{
  "message": "你好",
  "model": "gpt-4",
  "system_prompt": "...",
  "agent_id": "default",
  "tenant_id": "tenant-123"
}
```

响应：
```json
{
  "code": 0,
  "data": {
    "content": "你好！有什么可以帮助你的？",
    "input_guard": {
      "passed": true,
      "violations": []
    },
    "output_guard": {
      "passed": true,
      "violations": []
    },
    "rule_check": {
      "passed": true,
      "violations": []
    },
    "cost": 0.01,
    "tokens": 100,
    "trace_id": "trace-123",
    "ab_test": {
      "test_id": "test-1",
      "group": "control"
    }
  }
}
```

### 7.2 规则管理接口

**POST `/api/v2/harness/rules`**

请求：
```json
{
  "agent_id": "default",
  "name": "token_limit",
  "type": "constraint",
  "config": {
    "max_tokens": 4000
  },
  "enabled": true
}
```

响应：
```json
{
  "code": 0,
  "data": {
    "id": "rule-123",
    "agent_id": "default",
    "name": "token_limit",
    "type": "constraint",
    "config": {"max_tokens": 4000},
    "enabled": true,
    "created_at": "2024-01-01T10:00:00Z"
  }
}
```

**GET `/api/v2/harness/rules`**

查询参数：
- `agent_id`: 可选，过滤特定 Agent

响应：
```json
{
  "code": 0,
  "data": {
    "rules": [
      {
        "id": "rule-123",
        "agent_id": "default",
        "name": "token_limit",
        "type": "constraint",
        "enabled": true
      }
    ]
  }
}
```

**PUT `/api/v2/harness/rules/:id`**

请求：同创建

**DELETE `/api/v2/harness/rules/:id`**

响应：
```json
{
  "code": 0,
  "message": "rule deleted"
}
```

### 7.3 护栏检查接口

**POST `/api/v2/harness/guardrail/check`**

请求：
```json
{
  "content": "你好，请帮我...",
  "type": "input"                    // input/output
}
```

响应：
```json
{
  "code": 0,
  "data": {
    "passed": true,
    "violations": []
  }
}
```

### 7.4 评估接口

**POST `/api/v2/harness/eval/suites`**

请求：
```json
{
  "name": "基础对话评估",
  "description": "评估基本对话能力",
  "cases": [
    {
      "name": "问候测试",
      "input": "你好",
      "expected": "包含问候语"
    }
  ]
}
```

响应：
```json
{
  "code": 0,
  "data": {
    "id": "suite-123",
    "name": "基础对话评估",
    "case_count": 1
  }
}
```

**POST `/api/v2/harness/eval/run`**

请求：
```json
{
  "suite_id": "suite-123",
  "model": "gpt-4",
  "compare_baseline": true
}
```

响应：
```json
{
  "code": 0,
  "data": {
    "run_id": "run-123",
    "results": [
      {
        "case_id": "case-1",
        "actual": "你好！有什么可以帮助你的？",
        "score": 9.5,
        "passed": true,
        "metrics": {
          "faithfulness": 10,
          "relevancy": 9,
          "completeness": 9.5
        }
      }
    ],
    "avg_score": 9.5,
    "regression_detected": false
  }
}
```

### 7.5 A/B 测试接口

**POST `/api/v2/harness/abtest`**

请求：
```json
{
  "name": "模型对比测试",
  "control_model": "gpt-3.5-turbo",
  "variant_model": "gpt-4",
  "traffic_split": 0.5,              // 50% 流量到 variant
  "status": "running"
}
```

响应：
```json
{
  "code": 0,
  "data": {
    "id": "test-123",
    "name": "模型对比测试",
    "control_model": "gpt-3.5-turbo",
    "variant_model": "gpt-4",
    "traffic_split": 0.5,
    "status": "running"
  }
}
```

**GET `/api/v2/harness/abtest/:id/result`**

响应：
```json
{
  "code": 0,
  "data": {
    "control_score": 7.5,
    "variant_score": 9.2,
    "delta": 1.7,
    "p_value": 0.001,
    "significant": true,
    "recommended": "variant",
    "auto_promote": false
  }
}
```

**POST `/api/v2/harness/abtest/:id/promote`**

响应：
```json
{
  "code": 0,
  "message": "variant promoted to default"
}
```

### 7.6 SLO 接口

**POST `/api/v2/harness/slo`**

请求：
```json
{
  "agent_id": "default",
  "name": "success_rate",
  "target": 0.99,                    // 99% 成功率
  "type": "success_rate"             // success_rate/latency_p99/quality_score
}
```

响应：
```json
{
  "code": 0,
  "data": {
    "id": "slo-123",
    "agent_id": "default",
    "name": "success_rate",
    "target": 0.99,
    "type": "success_rate"
  }
}
```

**GET `/api/v2/harness/slo/status`**

查询参数：
- `agent_id`: 可选

响应：
```json
{
  "code": 0,
  "data": {
    "statuses": [
      {
        "name": "success_rate",
        "current": 0.985,
        "target": 0.99,
        "budget_remaining": 0.15,
        "status": "at_risk"             // healthy/at_risk/exhausted
      }
    ]
  }
}
```

### 7.7 Trace 接口

**GET `/api/v2/harness/traces`**

查询参数：
- `trace_id`: 可选
- `session_id`: 可选
- `agent_id`: 可选
- `start_time`: 可选
- `end_time`: 可选
- `page`: 页码
- `page_size`: 每页数量

响应：
```json
{
  "code": 0,
  "data": {
    "traces": [
      {
        "trace_id": "trace-123",
        "session_id": "session-123",
        "agent_id": "default",
        "spans": [
          {
            "name": "input_guard",
            "duration_ms": 5,
            "status": "ok"
          },
          {
            "name": "llm_call",
            "duration_ms": 1500,
            "status": "ok"
          }
        ],
        "total_duration_ms": 2000,
        "created_at": "2024-01-01T10:00:00Z"
      }
    ],
    "pagination": {
      "total": 100,
      "page": 1,
      "page_size": 20
    }
  }
}
```

### 7.8 成本分析接口

**GET `/api/v2/harness/cost/report`**

查询参数：
- `tenant_id`: 租户 ID
- `start_time`: 开始时间
- `end_time`: 结束时间

响应：
```json
{
  "code": 0,
  "data": {
    "total_cost": 100.50,
    "total_tokens": 500000,
    "by_model": {
      "gpt-4": {
        "cost": 80.00,
        "tokens": 400000
      },
      "gpt-3.5-turbo": {
        "cost": 20.50,
        "tokens": 100000
      }
    },
    "by_agent": {
      "default": {
        "cost": 60.00,
        "tokens": 300000
      },
      "researcher": {
        "cost": 40.50,
        "tokens": 200000
      }
    },
    "idle_agents": ["unused-agent-1"],
    "recommendations": [
      "考虑将部分流量切换到 gpt-3.5-turbo 以降低成本"
    ]
  }
}
```

---

## 8. 健康检查 API

**GET `/health`**

响应：
```json
{
  "status": "healthy",
  "services": {
    "chat": "healthy",
    "knowledge": "healthy",
    "memory": "healthy",
    "a2a": "healthy",
    "mcp": "healthy",
    "harness": "healthy"
  },
  "dependencies": {
    "qdrant": "healthy",
    "mongodb": "healthy",
    "redis": "healthy"
  }
}
```

**GET `/ready`**

响应：
```json
{
  "ready": true
}
```