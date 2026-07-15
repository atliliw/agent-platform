# API 参考

> Agent Platform HTTP API 参考。 [English](../en/api-reference.md)

- **Base URL：** `http://localhost:9000`
- **API 前缀：** 所有路由在 `/api/v2` 下，经过租户中间件。
- **健康检查：** `GET /health` · `GET /ready`

---

## Chat

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/chat` | 对话 |
| POST | `/chat/stream` | 流式对话 |
| GET | `/sessions` | 会话列表 |
| GET | `/sessions/:id` | 获取会话 |
| DELETE | `/sessions/:id` | 删除会话 |
| DELETE | `/sessions/empty` | 删除空会话 |
| POST | `/multi-agent/chat` | 多 Agent 对话 |

## Agents

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/agents` | 注册 Agent |
| GET | `/agents` | Agent 列表 |
| GET | `/agents/:id` | 获取 Agent |
| DELETE | `/agents/:id` | 注销 Agent |
| POST | `/agents/execute` | 执行 Agent |
| POST | `/agents/execute/stream` | 流式执行 Agent |
| GET | `/agents/context/:id` | 获取 Agent 上下文 |

## Skills

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/skills` | 创建技能 |
| GET | `/skills` | 技能列表 |
| POST | `/skills/import` | 导入技能 |
| GET | `/skills/:id` | 获取技能 |
| GET | `/skills/:id/export` | 导出技能 |
| PUT | `/skills/:id` | 更新技能 |
| DELETE | `/skills/:id` | 删除技能 |

## Knowledge

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/knowledge/upload` | 上传文档 |
| GET | `/knowledge/documents` | 文档列表 |
| GET | `/knowledge/documents/:id` | 获取文档 |
| DELETE | `/knowledge/documents/:id` | 删除文档 |
| POST | `/knowledge/search` | 检索知识库 |

## Memory - 基础

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/memory` | 保存记忆 |
| POST | `/memory/recall` | 召回记忆 |
| GET | `/memory/all` | 全部记忆 |
| GET | `/memory/session/:id` | 会话记忆 |
| DELETE | `/memory/session/:id` | 删除会话记忆 |
| DELETE | `/memory/session/clear` | 删除全部会话记忆 |
| DELETE | `/memory/:id` | 删除记忆 |

## Memory - 分层

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/memory/episodic` | 存储情景 |
| GET | `/memory/episodic` | 获取情景 |
| POST | `/memory/episodic/similar` | 相似情景 |
| POST | `/memory/semantic/concept` | 存储概念 |
| POST | `/memory/semantic/relation` | 存储关系 |
| GET | `/memory/semantic` | 召回概念 |
| POST | `/memory/semantic/related` | 相关概念 |
| POST | `/memory/working` | 添加工作消息 |
| GET | `/memory/working/:sessionId` | 工作上下文 |
| GET | `/memory/working/:sessionId/llm` | 供 LLM 的工作消息 |
| DELETE | `/memory/working/:sessionId` | 清空工作上下文 |
| GET | `/memory/forgetting/config` | 遗忘配置 |
| PUT | `/memory/forgetting/config` | 更新遗忘配置 |
| POST | `/memory/forgetting/cleanup` | 触发清理 |

## Memory - 增强

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/memory-enhanced/stats` | 记忆统计 |
| GET | `/memory-enhanced/timeline` | 记忆时间线 |
| GET | `/memory-enhanced/graph` | 记忆图谱 |
| GET | `/memory-enhanced/episodic` | 情景记忆 |
| GET | `/memory-enhanced/semantic` | 语义记忆 |
| GET | `/memory-enhanced/procedural` | 程序记忆 |
| POST | `/memory-enhanced/consolidate` | 整合记忆 |
| POST | `/memory-enhanced/search` | 搜索记忆 |

## A2A

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/a2a/discover` | 发现 Agent |
| POST | `/a2a/agents` | 注册 Agent |
| GET | `/a2a/agents` | Agent 列表 |
| DELETE | `/a2a/agents/:id` | 注销 Agent |
| POST | `/a2a/tasks/send` | 发送任务 |
| GET | `/a2a/tasks/:id` | 获取任务 |
| POST | `/a2a/tasks/:id/cancel` | 取消任务 |
| GET | `/a2a/tasks` | 任务列表 |

## MCP

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/mcp/tools` | 工具列表 |
| POST | `/mcp/call` | 调用工具 |
| POST | `/mcp/connect` | 连接 MCP 服务器 |
| GET | `/mcp/connections` | 连接列表 |
| DELETE | `/mcp/connections/:id` | 断开连接 |

## Harness - 规则与护栏

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/harness/rules` | 创建规则 |
| GET | `/harness/rules` | 规则列表 |
| DELETE | `/harness/rules/:id` | 删除规则 |
| POST | `/harness/guardrail/check` | 护栏检查 |
| POST | `/harness/eval/run` | 运行评测 |
| GET | `/eval/suites` | 评测套件列表 |
| GET | `/eval/suites/:id/results` | 套件结果 |
| POST | `/eval/suites/:id/run` | 运行套件 |
| POST | `/harness/abtest` | 创建 A/B 测试 |
| POST | `/harness/abtest/list` | A/B 测试列表 |
| DELETE | `/harness/abtest/:id` | 删除 A/B 测试 |
| GET | `/harness/abtest/:id/result` | A/B 测试结果 |
| GET | `/harness/slo/status` | SLO 状态 |
| POST | `/harness/slo` | 创建 SLO |
| GET | `/harness/llm/metrics` | LLM 调用指标 |
| POST | `/harness/chat` | 治理对话 |

## Harness - 特性开关与成本

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/harness/flags` | 创建特性开关 |
| GET | `/harness/flags` | 开关列表 |
| PUT | `/harness/flags/toggle` | 切换开关 |
| POST | `/harness/flags/evaluate` | 评估开关 |
| GET | `/harness/cost/report` | 成本报告 |
| POST | `/harness/cost/pricing` | 设置模型定价 |
| GET | `/harness/cost/pricing` | 模型定价列表 |
| GET | `/harness/cost/recommendations` | 成本建议 |
| POST | `/harness/cost/usage` | 记录成本用量 |
| GET | `/cost/summary` | 成本汇总 |
| GET | `/cost/trend` | 成本趋势 |
| GET | `/cost/budgets` | 预算列表 |
| POST | `/cost/budgets` | 创建预算 |
| GET | `/cost/details` | 成本明细 |

## Harness - 提案、Golden Path、优化器、RCA

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/harness/proposals` | 创建提案 |
| GET | `/harness/proposals` | 提案列表 |
| POST | `/harness/proposals/:id/approve` | 批准提案 |
| POST | `/harness/proposals/:id/reject` | 拒绝提案 |
| POST | `/harness/proposals/:id/execute` | 执行提案 |
| POST | `/harness/proposals/analyze` | 分析并提案 |
| POST | `/harness/goldenpath/templates` | 创建 golden-path 模板 |
| GET | `/harness/goldenpath/templates` | 模板列表 |
| POST | `/harness/goldenpath/instantiate` | 实例化模板 |
| POST | `/harness/evolve/optimize` | 运行优化器 |
| POST | `/harness/rca/changes` | 记录变更 |
| GET | `/harness/rca/incidents/:id/analyze` | 分析事故 |

## Harness - 调度器

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/harness/scheduler/schedules` | 创建调度 |
| GET | `/harness/scheduler/schedules` | 调度列表 |
| GET | `/harness/scheduler/schedules/:id` | 获取调度 |
| PUT | `/harness/scheduler/schedules/:id/pause` | 暂停调度 |
| PUT | `/harness/scheduler/schedules/:id/resume` | 恢复调度 |
| DELETE | `/harness/scheduler/schedules/:id` | 删除调度 |
| POST | `/harness/scheduler/schedules/:id/run` | 立即运行 |
| GET | `/harness/scheduler/schedules/:id/results` | 调度结果 |
| GET | `/harness/scheduler/status` | 调度器状态 |
| POST | `/harness/scheduler/control` | 调度器控制 |
| GET | `/harness/scheduler/stats` | 调度器统计 |

## Harness - 会话回放与检查点

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/harness/session` | 创建会话 |
| GET | `/harness/session/list` | 回放列表 |
| GET | `/harness/session/stats` | 会话统计 |
| GET | `/harness/session/:id` | 获取回放 |
| GET | `/harness/session/:id/graph` | 会话图 |
| POST | `/harness/session/:id/replay` | 回放会话 |
| GET | `/harness/session/:id/export` | 导出会话 |
| DELETE | `/harness/session/:id` | 删除回放 |
| GET | `/harness/session/:id/checkpoints` | 检查点列表 |
| GET | `/harness/session/:id/checkpoint/:checkpointId` | 获取检查点 |
| POST | `/harness/session/:id/checkpoint/:checkpointId/resume` | 从检查点恢复 |

## Harness - Prompt 管理

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/harness/prompt` | 创建 Prompt |
| GET | `/harness/prompt/list` | Prompt 列表 |
| GET | `/harness/prompt/:key` | 获取 Prompt |
| DELETE | `/harness/prompt/:key` | 删除 Prompt |
| POST | `/harness/prompt/:key/versions` | 创建版本 |
| GET | `/harness/prompt/:key/versions` | 版本列表 |
| GET | `/harness/prompt/:key/active` | 当前激活版本 |
| POST | `/harness/prompt/version/activate` | 激活版本 |
| POST | `/harness/prompt/compare` | 对比版本 |
| POST | `/harness/prompt/render` | 渲染 Prompt |
| GET | `/harness/prompt/performance/:versionId` | Prompt 表现 |

## Harness - RAG 指标

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/harness/rag/evaluate` | 评估 RAG |
| POST | `/harness/rag/batch-evaluate` | 批量评估 |
| GET | `/harness/rag/metrics` | RAG 指标列表 |
| GET | `/harness/rag/metrics/:id` | 获取 RAG 指标 |
| POST | `/harness/rag/evaluation` | 创建评估 |
| GET | `/harness/rag/evaluations` | 评估列表 |
| POST | `/harness/rag/evaluation/:id/run` | 运行评估 |

## Harness - 审批

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/harness/approval/pending` | 待审批 |
| GET | `/harness/approval/rules` | 审批规则 |
| POST | `/harness/approval/approve` | 批准 |
| POST | `/harness/approval/reject` | 拒绝 |
| POST | `/harness/approval/rules` | 添加规则 |

## Harness - LLM 网关

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/harness/gateway/chat` | 网关对话 |
| POST | `/harness/gateway/config` | 创建网关配置 |
| GET | `/harness/gateway/config` | 配置列表 |
| GET | `/harness/gateway/config/:id` | 获取配置 |
| PUT | `/harness/gateway/config/:id` | 更新配置 |
| DELETE | `/harness/gateway/config/:id` | 删除配置 |
| GET | `/harness/gateway/stats` | 网关统计 |
| POST | `/harness/gateway/route` | 创建路由 |
| GET | `/harness/gateway/routes` | 路由列表 |
| DELETE | `/harness/gateway/route/:id` | 删除路由 |
| POST | `/harness/gateway/strategy` | 设置负载均衡策略 |

## Harness - Playground

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/harness/playground/execute` | 执行 |
| POST | `/harness/playground/compare` | 对比模型 |
| POST | `/harness/playground/stream` | 流式 |
| GET | `/harness/playground/history` | 历史 |
| GET | `/harness/playground/stats` | 统计 |

## Harness - 工作流

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/harness/workflows` | 创建工作流 |
| GET | `/harness/workflows` | 列表 |
| GET | `/harness/workflows/:id` | 获取 |
| PUT | `/harness/workflows/:id` | 更新 |
| DELETE | `/harness/workflows/:id` | 删除 |
| POST | `/harness/workflows/:id/execute` | 执行 |
| POST | `/harness/workflows/:id/execute-stream` | 流式执行 |
| POST | `/harness/workflows/validate` | 校验 |
| GET | `/harness/workflows/:id/executions` | 执行列表 |
| GET | `/harness/workflows/executions/:executionId` | 获取执行 |
| POST | `/harness/workflows/executions/:executionId/cancel` | 取消执行 |

## Harness - 干预

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/harness/session/:id/intervene` | 干预会话 |
| GET | `/harness/session/:id/state` | 会话状态 |
| POST | `/harness/session/:id/resume` | 恢复会话 |
| POST | `/harness/session/:id/inject` | 注入消息 |

## 可观测性

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/observability/traces` | trace 列表 |
| GET | `/observability/traces/:id` | 获取 trace |
| GET | `/observability/metrics` | 指标 |
| GET | `/observability/profile/:id` | 获取 profile |
| GET | `/observability/stats` | 统计 |

## Cookies

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/cookies` | 保存 cookies |
| GET | `/cookies` | 获取 cookies |
| GET | `/cookies/all` | 全部 cookies |
| DELETE | `/cookies` | 删除 cookies |

## User

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/user/info` | 用户信息 |
| GET | `/user/settings` | 用户设置 |
| PUT | `/user/settings` | 更新设置 |

---

## 示例

### 对话

```bash
curl -X POST http://localhost:9000/api/v2/chat \
  -H "Content-Type: application/json" \
  -d '{"session_id":"s1","message":"你好，你能做什么？"}'
```

### 知识库检索

```bash
curl -X POST http://localhost:9000/api/v2/knowledge/search \
  -H "Content-Type: application/json" \
  -d '{"query":"agent 记忆","top_k":5}'
```

### MCP 工具调用

```bash
curl -X POST http://localhost:9000/api/v2/mcp/call \
  -H "Content-Type: application/json" \
  -d '{"tool":"web_search","arguments":{"query":"golang gRPC"}}'
```

### 创建并执行工作流

```bash
# 创建
curl -X POST http://localhost:9000/api/v2/harness/workflows \
  -H "Content-Type: application/json" \
  -d '{"name":"my-flow","definition":{...}}'
# -> { "id": "wf_xxx" }

# 执行
curl -X POST http://localhost:9000/api/v2/harness/workflows/wf_xxx/execute \
  -H "Content-Type: application/json" \
  -d '{"input":{}}'
```

## 延伸阅读

- [架构](./architecture.md)
- [配置](./configuration.md)
