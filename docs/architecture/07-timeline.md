# 07 - 时间规划

## 1. 项目总览

### 1.1 总时间估算

| 阶段 | 时间 | 说明 |
|------|------|------|
| Phase 1: 基础设施 | 3 天 | 项目骨架、Proto 定义、公共库 |
| Phase 2: 核心服务 | 2 周 | Chat、Knowledge、Memory 服务 |
| Phase 3: 扩展服务 | 1 周 | A2A、MCP 服务 |
| Phase 4: 治理服务 | 1 周 | Harness 服务 |
| Phase 5: 集成测试 | 3 天 | 服务联调、端到端测试 |
| Phase 6: 前端开发 | 1 周 | Vue 3 前端 |
| Phase 7: 部署上线 | 3 天 | Docker、K8s 配置 |

**总计：约 6 周**

---

## 2. 详细时间表

### Phase 1: 基础设施 (3 天)

#### Day 1: 项目初始化

| 任务 | 时间 | 负责人 | 产出 |
|------|------|--------|------|
| 创建项目目录结构 | 1h | - | 目录结构 |
| 初始化 Go Module | 0.5h | - | go.mod |
| 编写 Makefile | 0.5h | - | Makefile |
| 配置 CI/CD | 1h | - | .github/workflows/ |
| 编写 README | 1h | - | README.md |

#### Day 2: Protobuf 定义

| 任务 | 时间 | 负责人 | 产出 |
|------|------|--------|------|
| 定义 common.proto | 1h | - | proto/common/ |
| 定义 chat.proto | 2h | - | proto/chat/ |
| 定义 knowledge.proto | 2h | - | proto/knowledge/ |
| 定义 memory.proto | 1h | - | proto/memory/ |

#### Day 3: Protobuf 定义 (续) + 公共库

| 任务 | 时间 | 负责人 | 产出 |
|------|------|--------|------|
| 定义 a2a.proto | 1h | - | proto/a2a/ |
| 定义 mcp.proto | 1h | - | proto/mcp/ |
| 定义 harness.proto | 2h | - | proto/harness/ |
| 生成 Go 代码 | 0.5h | - | pkg/pb/ |
| 实现 gRPC 客户端工厂 | 1h | - | pkg/client/ |
| 实现 LLM 客户端 | 1h | - | pkg/llm/ |
| 实现 Embedding 客户端 | 0.5h | - | pkg/embedding/ |

---

### Phase 2: 核心服务 (2 周)

#### Week 1: Chat Service + Knowledge Service

**Day 4-5: Chat Service 基础**

| 任务 | 时间 | 产出 |
|------|------|------|
| 创建服务骨架 | 2h | services/chat-service/ |
| 实现 gRPC Handler | 2h | handler/grpc_handler.go |
| 实现会话管理 | 3h | service/session_service.go |
| 实现消息存储 | 2h | repository/ |
| 实现 LLM 调用 | 3h | llm/client.go |
| 单元测试 | 2h | *_test.go |

**Day 6-7: Chat Service 完善**

| 任务 | 时间 | 产出 |
|------|------|------|
| 实现工具调用逻辑 | 3h | service/tool_executor.go |
| 实现流式响应 | 2h | service/chat_stream.go |
| 集成 Memory 客户端 | 2h | client/memory_client.go |
| 集成 Harness 客户端 | 2h | client/harness_client.go |
| 多 Agent 框架 | 4h | agent/ |
| 集成测试 | 2h | integration_test.go |

**Day 8-10: Knowledge Service**

| 任务 | 时间 | 产出 |
|------|------|------|
| 创建服务骨架 | 1h | services/knowledge-service/ |
| 实现 gRPC Handler | 2h | handler/ |
| 实现文件解析器 | 4h | parser/*.go |
| 实现 Token 分块 | 2h | chunker/token_chunker.go |
| 实现语义分块 | 3h | chunker/semantic_chunker.go |
| 实现 Markdown 分块 | 2h | chunker/markdown_chunker.go |
| 实现 Embedding 批处理 | 2h | embedding/ |
| 实现 Qdrant 存储 | 2h | repository/qdrant_repo.go |
| 实现 MongoDB 存储 | 2h | repository/mongodb_repo.go |
| 实现向量检索 | 2h | service/search.go |
| 实现 BM25 检索 | 2h | service/bm25_search.go |
| 实现混合检索 (RRF) | 2h | service/hybrid_search.go |
| 单元测试 | 2h | *_test.go |

#### Week 2: Memory Service + Gateway

**Day 11-12: Memory Service**

| 任务 | 时间 | 产出 |
|------|------|------|
| 创建服务骨架 | 1h | services/memory-service/ |
| 实现 gRPC Handler | 2h | handler/ |
| 实现短期记忆存储 | 2h | service/short_term.go |
| 实现长期记忆存储 | 2h | service/long_term.go |
| 实现语义召回 | 3h | service/recall.go |
| 实现重要信息提取 | 2h | service/extractor.go |
| 单元测试 | 2h | *_test.go |

**Day 13-14: Gateway Service**

| 任务 | 时间 | 产出 |
|------|------|------|
| 创建服务骨架 | 1h | services/gateway/ |
| 实现 HTTP 路由 | 2h | router/router.go |
| 实现 JWT 认证中间件 | 2h | middleware/auth.go |
| 实现限流中间件 | 2h | middleware/ratelimit.go |
| 实现租户中间件 | 1h | middleware/tenant.go |
| 实现 gRPC 客户端 | 3h | grpc_client/ |
| 实现 HTTP-gRPC 转换 | 3h | handler/ |
| 健康检查 | 1h | handler/health.go |
| 单元测试 | 2h | *_test.go |

---

### Phase 3: 扩展服务 (1 周)

#### Day 15-16: A2A Service

| 任务 | 时间 | 产出 |
|------|------|------|
| 创建服务骨架 | 1h | services/a2a-service/ |
| 实现 AgentCard 模型 | 1h | model/agent_card.go |
| 实现任务模型 | 1h | model/task.go |
| 实现 Agent 发现 | 2h | service/discovery.go |
| 实现 Agent 注册 | 2h | service/registry.go |
| 实现任务发送 | 2h | service/task_sender.go |
| 实现任务管理 | 2h | service/task_manager.go |
| 实现 HTTP Endpoint | 2h | handler/http_handler.go |
| 单元测试 | 2h | *_test.go |

#### Day 17-18: MCP Service

| 任务 | 时间 | 产出 |
|------|------|------|
| 创建服务骨架 | 1h | services/mcp-service/ |
| 实现 JSON-RPC Handler | 2h | handler/jsonrpc.go |
| 实现工具注册 | 2h | service/tool_registry.go |
| 实现内置工具 | 3h | tools/*.go |
| 实现 MCP Server | 2h | service/mcp_server.go |
| 实现 MCP Client | 2h | service/mcp_client.go |
| 实现连接管理 | 2h | service/connection.go |
| 单元测试 | 2h | *_test.go |

#### Day 19: 服务集成

| 任务 | 时间 | 产出 |
|------|------|------|
| 服务间调用测试 | 3h | 集成测试 |
| 错误处理统一 | 2h | 错误码定义 |
| 日志格式统一 | 1h | 日志配置 |
| 配置文件完善 | 2h | config.yaml |

---

### Phase 4: 治理服务 (1 周)

#### Day 20-21: Harness 核心组件

| 任务 | 时间 | 产出 |
|------|------|------|
| 创建服务骨架 | 1h | services/harness-service/ |
| 实现规则引擎 | 3h | rule/engine.go |
| 实现护栏检查 | 3h | rule/guardrail.go |
| 实现权限矩阵 | 2h | sandbox/permission.go |
| 实现预算执行器 | 2h | sandbox/budget.go |
| 单元测试 | 2h | *_test.go |

#### Day 22-23: Harness 评估组件

| 任务 | 时间 | 产出 |
|------|------|------|
| 实现评估运行器 | 2h | evaluate/runner.go |
| 实现评分器 | 3h | evaluate/scorer.go |
| 实现测试套件管理 | 2h | evaluate/suite.go |
| 实现回归检测 | 2h | evaluate/regression.go |
| 实现 A/B 测试引擎 | 3h | abtest/engine.go |
| 实现统计显著性 | 2h | abtest/statistics.go |
| 单元测试 | 2h | *_test.go |

#### Day 24-25: Harness 运维组件

| 任务 | 时间 | 产出 |
|------|------|------|
| 实现 SLO 管理器 | 2h | slo/manager.go |
| 实现混沌引擎 | 2h | chaos/engine.go |
| 实现回滚引擎 | 2h | rollback/engine.go |
| 实现根因分析 | 2h | rca/engine.go |
| 实现成本分析 | 2h | cost/analyzer.go |
| 实现 Pipeline 引擎 | 3h | pipeline/engine.go |
| 实现 Trace 收集 | 2h | trace/collector.go |
| 单元测试 | 2h | *_test.go |

#### Day 26: Harness Chat 包装

| 任务 | 时间 | 产出 |
|------|------|------|
| 实现 HarnessChatService | 3h | service/harness_chat.go |
| 实现 Gate 流程 | 3h | service/gate.go |
| 集成测试 | 2h | integration_test.go |

---

### Phase 5: 集成测试 (3 天)

#### Day 27-29: 端到端测试

| 任务 | 时间 | 产出 |
|------|------|------|
| 编写测试用例 | 4h | test/e2e/*.go |
| 对话流程测试 | 2h | test/e2e/chat_test.go |
| 知识库流程测试 | 2h | test/e2e/knowledge_test.go |
| A2A 流程测试 | 2h | test/e2e/a2a_test.go |
| Harness 流程测试 | 2h | test/e2e/harness_test.go |
| 性能测试 | 3h | test/perf/*.go |
| Bug 修复 | 4h | - |

---

### Phase 6: 前端开发 (1 周)

#### Day 30-32: 前端基础

| 任务 | 时间 | 产出 |
|------|------|------|
| 创建 Vue 3 项目 | 1h | frontend/ |
| 配置 Element Plus | 1h | 基础组件 |
| 实现布局组件 | 2h | layouts/ |
| 实现登录页面 | 2h | views/Login.vue |
| 实现会话列表 | 3h | views/Sessions.vue |
| 实现对话界面 | 4h | views/Chat.vue |
| 实现 SSE 流式渲染 | 2h | components/MessageList.vue |

#### Day 33-34: 前端功能

| 任务 | 时间 | 产出 |
|------|------|------|
| 实现知识库管理 | 3h | views/Knowledge.vue |
| 实现文件上传 | 2h | components/FileUpload.vue |
| 实现检索测试 | 2h | views/Search.vue |
| 实现 A2A 管理 | 2h | views/A2A.vue |
| 实现监控仪表盘 | 3h | views/Monitor.vue |
| 实现 Harness 配置 | 2h | views/Harness.vue |

#### Day 35: 前端完善

| 任务 | 时间 | 产出 |
|------|------|------|
| 样式调整 | 2h | - |
| 响应式适配 | 2h | - |
| 国际化 | 2h | i18n/ |
| 性能优化 | 2h | - |

---

### Phase 7: 部署上线 (3 天)

#### Day 36-37: Docker 配置

| 任务 | 时间 | 产出 |
|------|------|------|
| 编写 Dockerfile | 3h | **/Dockerfile |
| 编写 docker-compose.dev | 2h | docker/docker-compose.dev.yaml |
| 编写 docker-compose.prod | 2h | docker/docker-compose.yaml |
| 配置 Nginx | 1h | docker/nginx/ |
| 配置 Prometheus | 2h | docker/prometheus/ |
| 配置 Grafana | 2h | docker/grafana/ |

#### Day 38: Kubernetes 配置 + 上线

| 任务 | 时间 | 产出 |
|------|------|------|
| 编写 K8s Namespace | 0.5h | k8s/namespace.yaml |
| 编写 K8s ConfigMap | 1h | k8s/configmap.yaml |
| 编写 K8s Secret | 0.5h | k8s/secret.yaml |
| 编写 K8s Deployments | 2h | k8s/deployments.yaml |
| 编写 K8s Services | 1h | k8s/services.yaml |
| 编写 K8s StatefulSets | 1h | k8s/statefulsets.yaml |
| 部署测试 | 2h | - |
| 文档完善 | 1h | docs/ |

---

## 3. 里程碑

| 里程碑 | 时间点 | 验收标准 |
|--------|--------|---------|
| M1: 基础设施完成 | Day 3 | Proto 定义完成，公共库可用 |
| M2: Chat Service 可用 | Day 7 | 基本对话功能正常 |
| M3: Knowledge Service 可用 | Day 10 | 文件上传、检索功能正常 |
| M4: Memory Service 可用 | Day 12 | 记忆存储、召回正常 |
| M5: Gateway 可用 | Day 14 | 所有 HTTP API 可访问 |
| M6: A2A Service 可用 | Day 16 | 跨服务通信正常 |
| M7: MCP Service 可用 | Day 18 | 工具调用正常 |
| M8: Harness Service 可用 | Day 26 | 治理功能正常 |
| M9: 集成测试通过 | Day 29 | 所有端到端测试通过 |
| M10: 前端完成 | Day 35 | 所有页面可用 |
| M11: 部署上线 | Day 38 | 生产环境可访问 |

---

## 4. 风险评估

| 风险 | 可能性 | 影响 | 缓解措施 |
|------|--------|------|---------|
| Protobuf 定义变更 | 高 | 中 | 提前评审，版本管理 |
| 服务间通信问题 | 中 | 高 | 统一错误处理，完善日志 |
| 性能瓶颈 | 中 | 中 | 提前压测，优化关键路径 |
| 第三方 API 不稳定 | 中 | 中 | 重试机制，降级策略 |
| 需求变更 | 中 | 中 | 敏捷开发，迭代调整 |

---

## 5. 人力资源

| 角色 | 人数 | 职责 |
|------|------|------|
| 架构师 | 1 | 架构设计、技术选型、代码评审 |
| 后端开发 | 2 | 服务开发、测试 |
| 前端开发 | 1 | 前端开发 |
| DevOps | 1 | 部署、运维、监控 |

---

## 6. 验收标准

### 6.1 功能验收

- [ ] 用户可以发起对话，收到正常响应
- [ ] 用户可以上传文件到知识库
- [ ] 用户可以检索知识库内容
- [ ] 系统可以记住用户偏好
- [ ] Agent 可以跨服务通信
- [ ] 工具调用功能正常
- [ ] Harness 治理功能正常

### 6.2 性能验收

- [ ] API P99 延迟 < 500ms（不含 LLM 调用）
- [ ] 支持并发 100 QPS
- [ ] 知识库检索 < 200ms

### 6.3 可用性验收

- [ ] 服务可用性 > 99.9%
- [ ] 单服务故障不影响其他服务
- [ ] 数据库故障有降级策略