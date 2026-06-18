# 02 - 架构设计

## 1. 整体架构

### 1.1 架构图

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              用户层                                          │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐                        │
│  │   Browser   │  │   Mobile    │  │    CLI      │                        │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘                        │
└─────────┼────────────────┼────────────────┼────────────────────────────────┘
          │                │                │
          ▼                ▼                ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                              接入层                                          │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │                        Nginx (反向代理)                                │  │
│  │                           :80 / :443                                  │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
│                                    │                                        │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │                     Gateway Service (API 网关)                         │  │
│  │   • HTTP 路由                                                          │  │
│  │   • 认证鉴权 (JWT)                                                      │  │
│  │   • 限流熔断                                                           │  │
│  │   • 租户隔离                                                           │  │
│  │   • gRPC 转发                                                          │  │
│  │                           :8080                                        │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    │ gRPC
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                              服务层                                          │
│                                                                             │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐        │
│  │ Chat Service│  │Knowledge Svc│  │Memory Service│ │ A2A Service │        │
│  │   :50001    │  │   :50002    │  │   :50003    │  │   :50004    │        │
│  │             │  │             │  │             │  │             │        │
│  │ • Agent执行 │  │ • 文件上传  │  │ • 记忆存储  │  │ • Agent发现 │        │
│  │ • 工具调用  │  │ • 文档解析  │  │ • 语义召回  │  │ • 任务发送  │        │
│  │ • 多Agent  │  │ • 智能分块  │  │ • 会话记忆  │  │ • 任务管理  │        │
│  │ • 会话管理  │  │ • 向量化   │  │             │  │ • Agent注册 │        │
│  │             │  │ • 多模检索  │  │             │  │             │        │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘        │
│         │                │                │                │               │
│  ┌──────┴────────────────┴────────────────┴────────────────┴──────┐        │
│  │                                                                 │        │
│  ┌──┴───────────┐  ┌─────────────┐                                │        │
│  │ MCP Service   │  │Harness Svc  │                                │        │
│  │   :50005      │  │   :50006    │                                │        │
│  │               │  │             │                                │        │
│  │ • MCP Server  │  │ • 规则引擎  │                                │        │
│  │ • MCP Client  │  │ • 护栏检查  │                                │        │
│  │ • 工具注册    │  │ • 评估测试  │                                │        │
│  │ • 资源管理    │  │ • A/B测试  │                                │        │
│  │               │  │ • SLO管理  │                                │        │
│  │               │  │ • 混沌工程 │                                │        │
│  │               │  │ • 自动回滚 │                                │        │
│  │               │  │ • 根因分析 │                                │        │
│  │               │  │ • 成本分析 │                                │        │
│  │               │  │ • Pipeline │                                │        │
│  └───────────────┘  └─────────────┘                                │        │
│                                                                    │        │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                            基础设施层                                        │
│                                                                             │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐        │
│  │   Qdrant    │  │   MongoDB   │  │   SQLite    │  │    Redis    │        │
│  │  (向量存储)  │  │  (文档存储) │  │  (元数据)    │  │   (缓存)    │        │
│  │   :6333     │  │   :27017    │  │   (各服务)   │  │   :6379     │        │
│  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘        │
│                                                                             │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐                         │
│  │ OpenAI API  │  │ DashScope   │  │  Embedding  │                         │
│  │    (LLM)    │  │    (LLM)    │  │   Service   │                         │
│  └─────────────┘  └─────────────┘  └─────────────┘                         │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 1.2 服务依赖关系

```
                    ┌─────────────┐
                    │   Gateway   │
                    └──────┬──────┘
                           │
        ┌──────────────────┼──────────────────┐
        │                  │                  │
        ▼                  ▼                  ▼
┌───────────────┐  ┌───────────────┐  ┌───────────────┐
│ Chat Service  │  │Knowledge Svc  │  │ A2A Service   │
└───────┬───────┘  └───────┬───────┘  └───────┬───────┘
        │                  │                  │
        │                  │                  │
        ▼                  ▼                  │
┌───────────────┐  ┌───────────────┐          │
│Memory Service │  │Memory Service │          │
└───────┬───────┘  └───────────────┘          │
        │                                       │
        └───────────────────┬───────────────────┘
                            │
                            ▼
                    ┌───────────────┐
                    │  MCP Service  │
                    └───────┬───────┘
                            │
                            ▼
                    ┌───────────────┐
                    │Harness Service│
                    └───────────────┘
```

## 2. 服务详细设计

### 2.1 Gateway Service（API 网关）

**职责**：
- HTTP 路由分发
- 认证鉴权（JWT）
- 限流熔断
- 租户隔离
- gRPC 请求转发
- 响应聚合

**端口**：8080 (HTTP)

**核心组件**：

```
gateway/
├── cmd/
│   └── main.go
├── internal/
│   ├── router/
│   │   └── router.go          # HTTP 路由
│   ├── middleware/
│   │   ├── auth.go            # JWT 认证
│   │   ├── ratelimit.go       # 限流
│   │   ├── tenant.go          # 租户提取
│   │   └── logging.go         # 日志
│   ├── handler/
│   │   ├── chat_handler.go    # 转发到 Chat Service
│   │   ├── knowledge_handler.go
│   │   └── ...
│   └── grpc_proxy/
│       └── proxy.go           # gRPC 转发
└── config.yaml
```

### 2.2 Chat Service（对话服务）

**职责**：
- Agent 执行（工具调用）
- 多 Agent 协作（Master-Worker）
- 会话管理
- 流式响应

**端口**：50001 (gRPC)

**核心流程**：

```
用户消息 → 会话加载 → 工具匹配 → Agent 执行 → 
调用 LLM → 工具执行 → 响应生成 → 会话保存
```

**依赖服务**：
- Memory Service（记忆）
- MCP Service（工具）
- Harness Service（治理）

### 2.3 Knowledge Service（知识库服务）

**职责**：
- 文件上传
- 文档解析（PDF/Word/Markdown/TXT/JSON/CSV）
- 智能分块（Token/Semantic/Markdown）
- 向量化（Embedding）
- 多模式检索（向量/BM25/混合）

**端口**：50002 (gRPC)

**核心流程**：

```
文件上传 → 解析内容 → 智能分块 → 批量 Embedding → 
存入 Qdrant + MongoDB → 返回结果

检索请求 → Query Embedding → 向量搜索 + BM25 搜索 → 
RRF 融合 → 返回结果
```

### 2.4 Memory Service（记忆服务）

**职责**：
- 短期记忆（会话消息）
- 长期记忆（重要信息）
- 语义召回
- 记忆过期清理

**端口**：50003 (gRPC)

**数据结构**：

```go
type MemoryEntry struct {
    ID         string
    SessionID  string
    AgentID    string
    Type       string    // important, summary, fact
    Content    string
    Vector     []float64
    Importance float64
    CreatedAt  time.Time
}
```

### 2.5 A2A Service（跨服务通信）

**职责**：
- Agent 发现（AgentCard）
- 任务发送/接收
- 任务生命周期管理
- 远程 Agent 注册

**端口**：50004 (gRPC)

**协议定义**：

```go
type AgentCard struct {
    ID           string
    Name         string
    Description  string
    Capabilities []string
    InputModes   []string
    OutputModes  []string
    URL          string
}

type TaskStatus string
const (
    TaskStatusSubmitted TaskStatus = "submitted"
    TaskStatusWorking   TaskStatus = "working"
    TaskStatusCompleted TaskStatus = "completed"
    TaskStatusFailed    TaskStatus = "failed"
    TaskStatusCancelled TaskStatus = "cancelled"
)
```

### 2.6 MCP Service（工具协议服务）

**职责**：
- MCP Server（对外暴露工具）
- MCP Client（连接外部工具）
- 工具注册与发现
- 资源管理
- 提示词管理

**端口**：50005 (gRPC)

**支持的工具**：
- search（知识库搜索）
- web_search（网络搜索）
- calculator（计算器）
- weather（天气）
- code_execute（代码执行）

### 2.7 Harness Service（运维治理服务）

**职责**：
- 规则引擎（约束检查）
- 输入输出护栏（内容过滤）
- 沙箱执行（权限控制）
- 评估测试（质量评分）
- A/B 测试（对比实验）
- SLO 管理（错误预算）
- 混沌工程（故障注入）
- 自动回滚
- 根因分析
- 成本分析
- 特性开关
- Pipeline 编排

**端口**：50006 (gRPC)

**Gate 流程**：

```
请求 → Gate 1: 输入护栏 → Gate 2: 规则预检 → 
Gate 3: 预算预检 → Gate 4: 权限检查 → 
执行 LLM → Gate 5: 输出护栏 → Gate 6: 成本记录 → 
Gate 7: 质量评分 → Gate 8: A/B 测试记录 → 
Gate 9: SLO 检查 → Gate 10: 回滚检查 → 返回响应
```

## 3. 数据流设计

### 3.1 对话流程

```
┌──────────┐     ┌─────────┐     ┌──────────┐     ┌──────────┐
│  User    │────▶│ Gateway │────▶│  Chat    │────▶│ Harness  │
└──────────┘     └─────────┘     │ Service  │     │ (Pre)    │
                                 └────┬─────┘     └────┬─────┘
                                      │                │
                                      │◀───────────────│
                                      │                │
                                 ┌────▼─────┐    ┌────▼─────┐
                                 │   MCP    │    │ Memory   │
                                 │ Service  │    │ Service  │
                                 └────┬─────┘    └────┬─────┘
                                      │                │
                                      │                │
                                 ┌────▼─────┐    ┌────▼─────┐
                                 │Knowledge │    │  Qdrant  │
                                 │ Service  │    │ (Vector) │
                                 └──────────┘    └──────────┘
```

### 3.2 知识库上传流程

```
┌──────────┐     ┌─────────┐     ┌──────────┐
│  User    │────▶│ Gateway │────▶│Knowledge │
│ (Upload) │     └─────────┘     │ Service  │
└──────────┘                     └────┬─────┘
                                      │
                     ┌────────────────┼────────────────┐
                     │                │                │
                ┌────▼────┐     ┌────▼────┐     ┌────▼────┐
                │ Parser  │     │Chunker  │     │Embedding│
                │(PDF/MD) │     │(Token)  │     │  (API)  │
                └────┬────┘     └────┬────┘     └────┬────┘
                     │                │                │
                     └────────────────┼────────────────┘
                                      │
                     ┌────────────────┼────────────────┐
                     │                │                │
                ┌────▼────┐     ┌────▼────┐     ┌────▼────┐
                │ Qdrant  │     │ MongoDB │     │ SQLite  │
                │(Vector) │     │(Chunks) │     │(Meta)   │
                └─────────┘     └─────────┘     └─────────┘
```

### 3.3 A2A 跨服务流程

```
┌──────────────────────────────────────────────────────────────┐
│                      Agent Platform A                         │
│  ┌──────────┐     ┌─────────┐     ┌──────────┐              │
│  │  User    │────▶│ Gateway │────▶│   A2A    │              │
│  └──────────┘     └─────────┘     │ Service  │              │
│                                   └────┬─────┘              │
└────────────────────────────────────────┼─────────────────────┘
                                         │ HTTP/gRPC
                                         │
┌────────────────────────────────────────▼─────────────────────┐
│                      Agent Platform B                         │
│                                   ┌────┴─────┐              │
│                                   │   A2A    │              │
│                                   │ Service  │              │
│                                   └────┬─────┘              │
│                                        │                     │
│                                   ┌────▼─────┐              │
│                                   │   Chat   │              │
│                                   │ Service  │              │
│                                   └────┬─────┘              │
│                                        │                     │
│                                   ┌────▼─────┐              │
│                                   │ Response │              │
│                                   └──────────┘              │
└──────────────────────────────────────────────────────────────┘
```

## 4. 部署架构

### 4.1 开发环境（Docker Compose）

```yaml
services:
  gateway:
    build: ./services/gateway
    ports:
      - "8080:8080"
    depends_on:
      - chat-service
      - knowledge-service

  chat-service:
    build: ./services/chat-service
    ports:
      - "50001:50001"

  knowledge-service:
    build: ./services/knowledge-service
    ports:
      - "50002:50002"

  # ... 其他服务

  qdrant:
    image: qdrant/qdrant:latest
    ports:
      - "6333:6333"

  mongodb:
    image: mongo:7
    ports:
      - "27017:27017"

  redis:
    image: redis:7
    ports:
      - "6379:6379"
```

### 4.2 生产环境（Kubernetes）

```yaml
# Deployment 示例
apiVersion: apps/v1
kind: Deployment
metadata:
  name: chat-service
spec:
  replicas: 3
  selector:
    matchLabels:
      app: chat-service
  template:
    spec:
      containers:
      - name: chat-service
        image: agent-platform/chat-service:latest
        ports:
        - containerPort: 50001
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
---
apiVersion: v1
kind: Service
metadata:
  name: chat-service
spec:
  selector:
    app: chat-service
  ports:
  - port: 50001
    targetPort: 50001
```

## 5. 安全架构

### 5.1 认证流程

```
┌──────────┐     ┌─────────┐     ┌──────────┐
│  User    │────▶│ Gateway │────▶│  Auth    │
│          │     │         │     │ Service  │
└──────────┘     └─────────┘     └──────────┘
     │               │                │
     │  1. Login     │                │
     │──────────────▶│                │
     │               │  2. Verify     │
     │               │───────────────▶│
     │               │                │
     │               │  3. JWT Token  │
     │               │◀───────────────│
     │  4. Token     │                │
     │◀──────────────│                │
     │               │                │
     │  5. API Call  │                │
     │──────────────▶│  6. Validate   │
     │               │───────────────▶│
     │               │                │
     │               │  7. User Info  │
     │               │◀───────────────│
     │               │                │
     │  8. Response  │                │
     │◀──────────────│                │
```

### 5.2 租户隔离

- 每个请求携带 `tenant_id`
- 数据库行级隔离
- 配额限制
- 资源隔离（可选独立数据库）

## 6. 容错设计

### 6.1 服务降级

| 故障场景 | 降级策略 |
|---------|---------|
| Memory Service 故障 | 跳过长期记忆，仅使用会话记忆 |
| Knowledge Service 故障 | 跳过知识库检索 |
| Harness Service 故障 | 跳过治理检查（可配置） |
| Qdrant 故障 | 使用 MongoDB BM25 检索 |

### 6.2 重试策略

```go
type RetryConfig struct {
    MaxAttempts     int           // 最大重试次数
    InitialBackoff  time.Duration // 初始等待时间
    MaxBackoff      time.Duration // 最大等待时间
    BackoffMultiple float64      // 退避倍数
}

// 默认配置
var DefaultRetry = RetryConfig{
    MaxAttempts:     3,
    InitialBackoff:  100 * time.Millisecond,
    MaxBackoff:      5 * time.Second,
    BackoffMultiple: 2.0,
}
```