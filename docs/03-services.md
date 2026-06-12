# 03 - 服务详细设计

## 1. Gateway Service（API 网关）

### 1.1 概述

Gateway Service 是系统的统一入口，负责：
- HTTP 路由分发
- 认证鉴权
- 限流熔断
- 租户隔离
- gRPC 请求转发

### 1.2 目录结构

```
services/gateway/
├── cmd/
│   └── main.go                    # 入口
│
├── internal/
│   ├── config/
│   │   └── config.go              # 配置加载
│   │
│   ├── router/
│   │   └── router.go              # HTTP 路由定义
│   │
│   ├── middleware/
│   │   ├── auth.go                # JWT 认证
│   │   ├── ratelimit.go           # 限流（基于 Redis）
│   │   ├── circuitbreaker.go      # 熔断
│   │   ├── tenant.go              # 租户提取
│   │   ├── logging.go             # 日志
│   │   ├── tracing.go             # OpenTelemetry
│   │   └── cors.go                # CORS
│   │
│   ├── handler/
│   │   ├── chat_handler.go        # 转发到 Chat Service
│   │   ├── knowledge_handler.go   # 转发到 Knowledge Service
│   │   ├── memory_handler.go      # 转发到 Memory Service
│   │   ├── a2a_handler.go         # 转发到 A2A Service
│   │   ├── mcp_handler.go         # 转发到 MCP Service
│   │   ├── harness_handler.go     # 转发到 Harness Service
│   │   └── health_handler.go      # 健康检查
│   │
│   └── grpc_client/
│   │   ├── chat_client.go         # Chat gRPC 客户端
│   │   ├── knowledge_client.go    # Knowledge gRPC 客户端
│   │   ├── memory_client.go       # Memory gRPC 客户端
│   │   ├── a2a_client.go          # A2A gRPC 客户端
│   │   ├── mcp_client.go          # MCP gRPC 客户端
│   │   └── harness_client.go      # Harness gRPC 客户端
│   │
│   └── model/
│       ├── request.go             # HTTP 请求模型
│       └── response.go            # HTTP 响应模型
│
├── Dockerfile
└── config.yaml
```

### 1.3 HTTP 路由表

| 路径 | 方法 | 目标服务 | 说明 |
|------|------|---------|------|
| `/api/v2/chat` | POST | Chat | 对话 |
| `/api/v2/chat/stream` | POST | Chat | 流式对话 |
| `/api/v2/sessions` | GET/POST/DELETE | Chat | 会话管理 |
| `/api/v2/multi-agent/chat` | POST | Chat | 多 Agent |
| `/api/v2/knowledge/upload` | POST | Knowledge | 文件上传 |
| `/api/v2/knowledge/documents` | GET/DELETE | Knowledge | 文档管理 |
| `/api/v2/knowledge/search` | POST | Knowledge | 检索 |
| `/api/v2/memory/:session_id` | GET/POST | Memory | 记忆管理 |
| `/api/v2/a2a/agents` | GET/POST | A2A | Agent 发现 |
| `/api/v2/a2a/tasks` | POST/GET | A2A | 任务管理 |
| `/api/v2/mcp/tools` | GET | MCP | 工具列表 |
| `/api/v2/mcp/call` | POST | MCP | 工具调用 |
| `/api/v2/harness/rules` | GET/POST | Harness | 规则管理 |
| `/api/v2/harness/eval` | POST | Harness | 评估测试 |
| `/api/v2/harness/chat` | POST | Harness | 治理对话 |
| `/health` | GET | Local | 健康检查 |

### 1.4 认证中间件

```go
func AuthMiddleware(jwtSecret string) gin.HandlerFunc {
    return func(c *gin.Context) {
        token := c.GetHeader("Authorization")
        if token == "" {
            c.AbortWithStatusJSON(401, gin.H{"error": "missing token"})
            return
        }

        // 去除 Bearer 前缀
        token = strings.TrimPrefix(token, "Bearer ")

        // 解析 JWT
        claims, err := jwt.Parse(token, jwtSecret)
        if err != nil {
            c.AbortWithStatusJSON(401, gin.H{"error": "invalid token"})
            return
        }

        // 设置用户信息到上下文
        c.Set("user_id", claims.UserID)
        c.Set("tenant_id", claims.TenantID)
        c.Set("roles", claims.Roles)

        c.Next()
    }
}
```

### 1.5 限流中间件

```go
func RateLimitMiddleware(rdb *redis.Client, limit int, window time.Duration) gin.HandlerFunc {
    return func(c *gin.Context) {
        tenantID := c.GetString("tenant_id")
        key := fmt.Sprintf("ratelimit:%s:%s", tenantID, c.FullPath())

        // 使用 Redis 滑动窗口
        count, err := rdb.Incr(c, key).Result()
        if err != nil {
            c.Next()
            return
        }

        if count == 1 {
            rdb.Expire(c, key, window)
        }

        if count > limit {
            c.AbortWithStatusJSON(429, gin.H{
                "error": "rate limit exceeded",
                "limit": limit,
                "window": window.String(),
            })
            return
        }

        c.Next()
    }
}
```

---

## 2. Chat Service（对话服务）

### 2.1 概述

Chat Service 是核心对话服务，负责：
- Agent 执行（工具调用）
- 多 Agent 协作
- 会话管理
- 流式响应

### 2.2 目录结构

```
services/chat-service/
├── cmd/
│   └── main.go
│
├── internal/
│   ├── config/
│   │   └── config.go
│   │
│   ├── service/
│   │   ├── chat_service.go        # 对话核心逻辑
│   │   ├── agent_service.go       # Agent 执行
│   │   ├── multi_agent_service.go # 多 Agent 协作
│   │   └── session_service.go     # 会话管理
│   │
│   ├── handler/
│   │   └── grpc_handler.go        # gRPC Handler
│   │
│   ├── model/
│   │   ├── session.go             # 会话模型
│   │   ├── message.go             # 消息模型
│   │   ├── agent.go               # Agent 模型
│   │   └── tool_call.go           # 工具调用模型
│   │
│   ├── repository/
│   │   ├── session_repo.go        # 会话存储（SQLite）
│   │   └── message_repo.go        # 消息存储
│   │
│   └── client/
│   │   ├── memory_client.go       # Memory gRPC 客户端
│   │   ├── mcp_client.go          # MCP gRPC 客户端
│   │   └── harness_client.go      # Harness gRPC 客户端
│   │
│   └── agent/
│   │   ├── planner.go             # 规划 Agent
│   │   ├── master.go              # Master Agent
│   │   ├── specialist.go          # Specialist Agent
│   │   └── tool_executor.go       # 工具执行器
│   │
│   └── llm/
│   │   ├── client.go              # LLM 客户端接口
│   │   ├── openai.go              # OpenAI 实现
│   │   └── dashscope.go           # DashScope 实现
│   │
│   └── prompt/
│   │   ├── system_prompt.go       # 系统提示词
│   │   └── tool_prompt.go         # 工具提示词
│
├── Dockerfile
└── config.yaml
```

### 2.3 核心数据结构

```go
// 会话
type Session struct {
    ID        string
    Title     string
    UserID    string
    TenantID  string
    Messages  []Message
    Metadata  map[string]string
    CreatedAt time.Time
    UpdatedAt time.Time
}

// 消息
type Message struct {
    ID        string
    SessionID string
    Role      string    // user, assistant, system, tool
    Content   string
    ToolCalls []ToolCall
    CreatedAt time.Time
}

// 工具调用
type ToolCall struct {
    ID        string
    Name      string
    Arguments string    // JSON
    Result    string
    Status    string    // pending, success, error
}

// Agent 定义
type AgentDefinition struct {
    ID           string
    Name         string
    Description  string
    Category     string    // planner, specialist, utility
    Tools        []string
    SystemPrompt string
}
```

### 2.4 对话流程

```go
func (s *ChatService) Chat(ctx context.Context, req *pb.ChatRequest) (*pb.ChatResponse, error) {
    // 1. 加载会话
    session, err := s.sessionRepo.GetOrCreate(ctx, req.SessionId)
    if err != nil {
        return nil, err
    }

    // 2. 加载记忆
    memories, err := s.memoryClient.Recall(ctx, &pb.RecallMemoryRequest{
        Query:     req.Message,
        SessionId: session.ID,
        TopK:      5,
    })

    // 3. 构建上下文
    context := s.buildContext(session, memories)

    // 4. 检查是否需要工具
    tools := s.matchTools(req.Message, req.ToolIds)

    // 5. 调用 Harness (预处理)
    harnessResp, err := s.harnessClient.ChatPre(ctx, &pb.HarnessPreRequest{
        Message:  req.Message,
        AgentId:  "default",
        TenantId: session.TenantID,
    })
    if !harnessResp.Passed {
        return nil, fmt.Errorf("harness blocked: %v", harnessResp.Violations)
    }

    // 6. 调用 LLM
    llmResp, err := s.llmClient.Chat(ctx, context, req.Model, tools)

    // 7. 执行工具调用
    if len(llmResp.ToolCalls) > 0 {
        for _, tc := range llmResp.ToolCalls {
            result, err := s.executeTool(ctx, tc)
            if err != nil {
                tc.Status = "error"
                tc.Result = err.Error()
            } else {
                tc.Status = "success"
                tc.Result = result
            }
        }
    }

    // 8. 调用 Harness (后处理)
    harnessPostResp, err := s.harnessClient.ChatPost(ctx, &pb.HarnessPostRequest{
        Content:  llmResp.Content,
        AgentId:  "default",
        TenantId: session.TenantID,
    })

    // 9. 保存消息
    userMsg := Message{Role: "user", Content: req.Message}
    assistantMsg := Message{Role: "assistant", Content: llmResp.Content, ToolCalls: llmResp.ToolCalls}
    session.Messages = append(session.Messages, userMsg, assistantMsg)
    s.sessionRepo.Save(ctx, session)

    // 10. 返回响应
    return &pb.ChatResponse{
        SessionId:   session.ID,
        Content:     llmResp.Content,
        ToolCalls:   convertToolCalls(llmResp.ToolCalls),
        TotalTokens: llmResp.TotalTokens,
        Cost:        llmResp.Cost,
    }, nil
}
```

### 2.5 多 Agent 协作流程

```go
func (s *MultiAgentService) MultiAgentChat(ctx context.Context, req *pb.MultiAgentRequest) (*pb.MultiAgentResponse, error) {
    // 1. Master Agent 分析任务
    masterResp, err := s.masterAgent.Analyze(ctx, req.Message)

    // 2. 分解任务
    tasks := s.plannerAgent.Decompose(ctx, masterResp)

    // 3. 分配给 Specialist Agents
    var steps []*pb.AgentStep
    for _, task := range tasks {
        specialist := s.routeToSpecialist(task.AgentId)

        result, err := specialist.Execute(ctx, task)
        if err != nil {
            steps = append(steps, &pb.AgentStep{
                AgentId: task.AgentId,
                Action:  task.Action,
                Result:  fmt.Sprintf("error: %v", err),
            })
            continue
        }

        steps = append(steps, &pb.AgentStep{
            AgentId: task.AgentId,
            Action:  task.Action,
            Result:  result,
        })
    }

    // 4. Master Agent 整合结果
    finalAnswer, err := s.masterAgent.Summarize(ctx, req.Message, steps)

    return &pb.MultiAgentResponse{
        SessionId:    req.SessionId,
        FinalAnswer:  finalAnswer,
        Steps:        steps,
    }, nil
}
```

---

## 3. Knowledge Service（知识库服务）

### 3.1 概述

Knowledge Service 负责知识库管理：
- 文件上传
- 文档解析
- 智能分块
- 向量化
- 多模式检索

### 3.2 目录结构

```
services/knowledge-service/
├── cmd/
│   └── main.go
│
├── internal/
│   ├── config/
│   │   └── config.go
│   │
│   ├── service/
│   │   ├── upload_service.go     # 上传服务
│   │   ├── parser_service.go      # 解析服务
│   │   ├── chunker_service.go     # 分块服务
│   │   ├── index_service.go       # 索引服务
│   │   ├── search_service.go      # 检索服务
│   │   └── document_service.go    # 文档管理
│   │
│   ├── handler/
│   │   └── grpc_handler.go
│   │
│   ├── model/
│   │   ├── document.go            # 文档模型
│   │   ├── chunk.go               # 分块模型
│   │   └── search_result.go       # 检索结果模型
│   │
│   ├── repository/
│   │   ├── qdrant_repo.go         # Qdrant 存储
│   │   ├── mongodb_repo.go        # MongoDB 存储
│   │   └── sqlite_repo.go         # SQLite 元数据
│   │
│   ├── parser/
│   │   ├── pdf_parser.go          # PDF 解析
│   │   ├── markdown_parser.go     # Markdown 解析
│   │   ├── txt_parser.go          # TXT 解析
│   │   ├── json_parser.go         # JSON 解析
│   │   ├── csv_parser.go          # CSV 解析
│   │   └── docx_parser.go         # Word 解析
│   │
│   ├── chunker/
│   │   ├── token_chunker.go       # Token 分块
│   │   ├── semantic_chunker.go    # 语义分块
│   │   ├── markdown_chunker.go    # Markdown 分块
│   │   └── recursive_chunker.go   # 递归分块
│   │
│   └── embedding/
│   │   └── embedding_client.go    # Embedding 客户端
│
├── Dockerfile
└── config.yaml
```

### 3.3 核心数据结构

```go
// 文档
type Document struct {
    ID          string
    Title       string
    Filename    string
    Content     string    // 原文内容
    ChunkCount  int
    Status      string    // processing, ready, error
    Metadata    map[string]string
    CreatedAt   time.Time
}

// 分块
type Chunk struct {
    ID          string
    DocumentID  string
    Content     string
    ChunkIndex  int
    Vector      []float64
    Metadata    map[string]string
    CreatedAt   time.Time
}

// 分块策略
type ChunkStrategy struct {
    Type       string    // token, semantic, markdown, recursive
    ChunkSize  int       // 分块大小
    Overlap    int       // 重叠大小
}

// 检索结果
type SearchResult struct {
    ChunkID    string
    DocumentID string
    Content    string
    Score      float64
    Metadata   map[string]string
}
```

### 3.4 上传流程

```go
func (s *UploadService) Upload(ctx context.Context, req *pb.UploadRequest) (*pb.UploadResponse, error) {
    // 1. 创建文档记录
    doc := &Document{
        ID:       uuid.New().String(),
        Filename: req.Filename,
        Status:   "processing",
    }
    s.docRepo.Create(ctx, doc)

    // 2. 解析文件
    content, err := s.parser.Parse(ctx, req.Content, req.Filename)
    if err != nil {
        doc.Status = "error"
        s.docRepo.Update(ctx, doc)
        return nil, err
    }
    doc.Content = content

    // 3. 分块
    strategy := parseChunkStrategy(req.ChunkStrategy)
    chunks := s.chunker.Chunk(ctx, content, strategy)

    // 4. 向量化
    vectors, err := s.embeddingClient.EmbedBatch(ctx, chunks)
    if err != nil {
        return nil, err
    }

    // 5. 存储
    chunkIDs := []string{}
    for i, chunk := range chunks {
        chunkID := uuid.New().String()
        chunkIDs = append(chunkIDs, chunkID)

        // 存入 Qdrant
        s.qdrantRepo.Upsert(ctx, &pb.Point{
            ID:      chunkID,
            Vector:  vectors[i],
            Payload: map[string]interface{}{
                "content":     chunk,
                "document_id": doc.ID,
                "chunk_index": i,
            },
        })

        // 存入 MongoDB
        s.mongoRepo.InsertChunk(ctx, &Chunk{
            ID:         chunkID,
            DocumentID: doc.ID,
            Content:    chunk,
            ChunkIndex: i,
            Vector:     vectors[i],
        })
    }

    // 6. 更新文档状态
    doc.Status = "ready"
    doc.ChunkCount = len(chunks)
    s.docRepo.Update(ctx, doc)

    return &pb.UploadResponse{
        DocumentId:  doc.ID,
        ChunkCount:  int32(len(chunks)),
        ChunkIds:    chunkIDs,
    }, nil
}
```

### 3.5 检索流程

```go
func (s *SearchService) Search(ctx context.Context, req *pb.SearchRequest) (*pb.SearchResponse, error) {
    // 1. Query 向量化
    queryVector, err := s.embeddingClient.Embed(ctx, req.Query)
    if err != nil {
        return nil, err
    }

    var results []*pb.SearchResult

    switch req.SearchType {
    case "vector":
        // 向量搜索
        results, err = s.vectorSearch(ctx, queryVector, req)

    case "bm25":
        // BM25 搜索
        results, err = s.bm25Search(ctx, req)

    case "hybrid":
        // 混合搜索
        results, err = s.hybridSearch(ctx, queryVector, req)

    default:
        return nil, fmt.Errorf("unknown search type: %s", req.SearchType)
    }

    if err != nil {
        return nil, err
    }

    return &pb.SearchResponse{
        Results: results,
        Total:   int64(len(results)),
    }, nil
}

// 混合搜索 (RRF 融合)
func (s *SearchService) hybridSearch(ctx context.Context, queryVector []float64, req *pb.SearchRequest) ([]*pb.SearchResult, error) {
    // 1. 向量搜索
    vectorResults, err := s.qdrantRepo.Search(ctx, queryVector, req.TopK*2)
    if err != nil {
        return nil, err
    }

    // 2. BM25 搜索
    bm25Results, err := s.mongoRepo.SearchBM25(ctx, req.Query, req.TopK*2)
    if err != nil {
        return nil, err
    }

    // 3. RRF 融合
    fused := s.rrfFusion(vectorResults, bm25Results, 60)

    // 4. 返回 TopK
    return fused[:min(len(fused), req.TopK)], nil
}

// RRF (Reciprocal Rank Fusion)
func (s *SearchService) rrfFusion(vectorResults, bm25Results []*pb.SearchResult, k int) []*pb.SearchResult {
    scores := make(map[string]float64)

    for i, r := range vectorResults {
        scores[r.ChunkId] += 1.0 / float64(k + i + 1)
    }

    for i, r := range bm25Results {
        scores[r.ChunkId] += 1.0 / float64(k + i + 1)
    }

    // 按分数排序
    var sorted []*pb.SearchResult
    for id, score := range scores {
        sorted = append(sorted, &pb.SearchResult{
            ChunkId: id,
            Score:   score,
        })
    }
    sort.Slice(sorted, func(i, j int) bool {
        return sorted[i].Score > sorted[j].Score
    })

    return sorted
}
```

### 3.6 分块策略

#### Token 分块

```go
func (c *TokenChunker) Chunk(ctx context.Context, content string, strategy ChunkStrategy) []string {
    // 估算 token 数 (中文 1 char ≈ 1 token, 英文 4 chars ≈ 1 token)
    chars := []rune(content)
    maxChars := strategy.ChunkSize * 2  // 粗略估算
    overlapChars := strategy.Overlap * 2

    var chunks []string
    start := 0

    for start < len(chars) {
        // 找分块边界
        end := c.findBoundary(chars, start, maxChars)
        chunk := string(chars[start:end])
        chunks = append(chunks, chunk)

        // 下一块起点 (考虑重叠)
        start = end - overlapChars
        if start < 0 {
            start = 0
        }
    }

    return chunks
}

func (c *TokenChunker) findBoundary(chars []rune, start, maxChars int) int {
    end := min(start+maxChars, len(chars))

    // 尝试在段落边界分块
    for i := end - 1; i >= start; i-- {
        if chars[i] == '\n' && i+1 < len(chars) && chars[i+1] == '\n' {
            return i + 2
        }
    }

    // 尝试在句子边界分块
    for i := end - 1; i >= start; i-- {
        if chars[i] == '。' || chars[i] == '！' || chars[i] == '？' ||
           chars[i] == '.' || chars[i] == '!' || chars[i] == '?' {
            return i + 1
        }
    }

    return end
}
```

#### 语义分块

```go
func (c *SemanticChunker) Chunk(ctx context.Context, content string, strategy ChunkStrategy) ([]string, error) {
    // 1. 分句
    sentences := c.splitSentences(content)

    // 2. 批量 Embedding
    vectors, err := c.embeddingClient.EmbedBatch(ctx, sentences)
    if err != nil {
        return nil, err
    }

    // 3. 计算相邻句子相似度
    var boundaries []int
    boundaries = append(boundaries, 0)

    currentLen := 0
    for i := 1; i < len(sentences); i++ {
        sim := cosineSimilarity(vectors[i-1], vectors[i])

        // 相似度低 = 话题边界
        if sim < 0.45 && currentLen >= strategy.ChunkSize {
            boundaries = append(boundaries, i)
            currentLen = 0
        } else if currentLen + len(sentences[i]) > strategy.ChunkSize * 2 {
            boundaries = append(boundaries, i)
            currentLen = 0
        } else {
            currentLen += len(sentences[i])
        }
    }
    boundaries = append(boundaries, len(sentences))

    // 4. 构建分块
    var chunks []string
    for i := 0; i < len(boundaries)-1; i++ {
        start := boundaries[i]
        end := boundaries[i+1]
        chunk := strings.Join(sentences[start:end], "")
        if chunk != "" {
            chunks = append(chunks, chunk)
        }
    }

    return chunks, nil
}
```

---

## 4. Memory Service（记忆服务）

### 4.1 概述

Memory Service 负责记忆管理：
- 短期记忆（会话消息）
- 长期记忆（重要信息）
- 语义召回

### 4.2 目录结构

```
services/memory-service/
├── cmd/
│   └── main.go
│
├── internal/
│   ├── config/
│   │   └── config.go
│   │
│   ├── service/
│   │   ├── memory_service.go     # 记忆核心逻辑
│   │   ├── short_term.go         # 短期记忆
│   │   ├── long_term.go          # 长期记忆
│   │   └── recall.go             # 召回
│   │
│   ├── handler/
│   │   └── grpc_handler.go
│   │
│   ├── model/
│   │   ├── memory_entry.go       # 记忆条目
│   │   └── session.go            # 会话
│   │
│   ├── repository/
│   │   ├── qdrant_repo.go        # Qdrant (长期记忆)
│   │   └── sqlite_repo.go        # SQLite (短期记忆)
│   │
│   └── embedding/
│   │   └── embedding_client.go
│
├── Dockerfile
└── config.yaml
```

### 4.3 核心数据结构

```go
// 记忆条目
type MemoryEntry struct {
    ID         string
    SessionID  string
    AgentID    string
    Type       MemoryType    // important, summary, fact
    Content    string
    Vector     []float64
    Importance float64       // 0.0 - 1.0
    CreatedAt  time.Time
}

type MemoryType string
const (
    MemoryTypeImportant MemoryType = "important"
    MemoryTypeSummary   MemoryType = "summary"
    MemoryTypeFact      MemoryType = "fact"
)

// 会话记忆
type SessionMemory struct {
    SessionID  string
    Messages   []Message
    Summaries  []string    // 历史摘要
    CreatedAt  time.Time
    UpdatedAt  time.Time
}
```

### 4.4 存储流程

```go
func (s *MemoryService) Save(ctx context.Context, req *pb.SaveMemoryRequest) (*pb.SaveMemoryResponse, error) {
    // 1. 创建记忆条目
    entry := &MemoryEntry{
        ID:         uuid.New().String(),
        SessionID:  req.SessionId,
        AgentID:    req.AgentId,
        Type:       MemoryType(req.Type),
        Content:    req.Content,
        Importance: req.Importance,
        CreatedAt:  time.Now(),
    }

    // 2. 向量化
    vector, err := s.embeddingClient.Embed(ctx, req.Content)
    if err != nil {
        return nil, err
    }
    entry.Vector = vector

    // 3. 存入 Qdrant (长期记忆)
    s.qdrantRepo.Upsert(ctx, &pb.Point{
        ID:      entry.ID,
        Vector:  vector,
        Payload: map[string]interface{}{
            "session_id":  entry.SessionID,
            "agent_id":    entry.AgentID,
            "type":        entry.Type,
            "content":     entry.Content,
            "importance":  entry.Importance,
            "created_at":  entry.CreatedAt.Unix(),
        },
    })

    // 4. 存入 SQLite (元数据)
    s.sqliteRepo.Create(ctx, entry)

    return &pb.SaveMemoryResponse{Id: entry.ID}, nil
}
```

### 4.5 召回流程

```go
func (s *MemoryService) Recall(ctx context.Context, req *pb.RecallMemoryRequest) (*pb.RecallMemoryResponse, error) {
    // 1. Query 向量化
    queryVector, err := s.embeddingClient.Embed(ctx, req.Query)
    if err != nil {
        return nil, err
    }

    // 2. 构建过滤条件
    filter := map[string]interface{}{}
    if req.SessionId != "" {
        filter["session_id"] = req.SessionId
    }
    if req.AgentId != "" {
        filter["agent_id"] = req.AgentId
    }

    // 3. 向量搜索
    results, err := s.qdrantRepo.Search(ctx, &pb.SearchRequest{
        Vector:   queryVector,
        Limit:    req.TopK,
        Filter:   filter,
        WithPayload: true,
    })
    if err != nil {
        return nil, err
    }

    // 4. 构建响应
    var memories []*pb.MemoryEntry
    for _, r := range results {
        memories = append(memories, &pb.MemoryEntry{
            Id:         r.ID,
            SessionId:  r.Payload["session_id"].(string),
            AgentId:    r.Payload["agent_id"].(string),
            Type:       r.Payload["type"].(string),
            Content:    r.Payload["content"].(string),
            Importance: r.Payload["importance"].(float64),
        })
    }

    return &pb.RecallMemoryResponse{Memories: memories}, nil
}
```

---

## 5. A2A Service（跨服务通信）

### 5.1 概述

A2A Service 实现 Agent-to-Agent 通信协议：
- Agent 发现
- 任务发送/接收
- 任务生命周期管理
- 远程 Agent 注册

### 5.2 目录结构

```
services/a2a-service/
├── cmd/
│   └── main.go
│
├── internal/
│   ├── config/
│   │   └── config.go
│   │
│   ├── service/
│   │   ├── discovery_service.go  # Agent 发现
│   │   ├── task_service.go       # 任务管理
│   │   ├── registry_service.go   # Agent 注册
│   │   └── a2a_client.go         # A2A 客户端
│   │
│   ├── handler/
│   │   └── grpc_handler.go
│   │   └── http_handler.go       # A2A HTTP Endpoint
│   │
│   ├── model/
│   │   ├── agent_card.go         # Agent 元数据
│   │   ├── task.go               # 任务
│   │   └── message.go            # 消息
│   │
│   ├── repository/
│   │   └── sqlite_repo.go        # 任务存储
│   │
│   └── protocol/
│   │   └── a2a_protocol.go       # 协议定义
│
├── Dockerfile
└── config.yaml
```

### 5.3 核心数据结构

```go
// Agent Card
type AgentCard struct {
    ID           string
    Name         string
    Description  string
    Capabilities []string
    InputModes   []string    // text, file, json
    OutputModes  []string    // text, json
    URL          string      // 远程地址
    Metadata     map[string]string
}

// 任务
type A2ATask struct {
    ID         string
    AgentID    string       // 目标 Agent
    Status     TaskStatus
    Messages   []A2AMessage
    Result     string
    Metadata   map[string]string
    CreatedAt  time.Time
    UpdatedAt  time.Time
}

type TaskStatus string
const (
    TaskStatusSubmitted TaskStatus = "submitted"
    TaskStatusWorking   TaskStatus = "working"
    TaskStatusCompleted TaskStatus = "completed"
    TaskStatusFailed    TaskStatus = "failed"
    TaskStatusCancelled TaskStatus = "cancelled"
)

// 消息
type A2AMessage struct {
    Role      string    // user, agent
    Content   string
    Metadata  map[string]string
}
```

### 5.4 发现流程

```go
func (s *DiscoveryService) Discover(ctx context.Context, req *pb.DiscoverRequest) (*pb.DiscoverResponse, error) {
    // 1. 发送 HTTP GET 到远程 Agent
    url := req.AgentUrl + "/.well-known/agent.json"

    resp, err := http.Get(url)
    if err != nil {
        return nil, err
    }

    // 2. 解析 AgentCard
    var card AgentCard
    if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
        return nil, err
    }

    // 3. 注册到本地
    s.registry.Register(ctx, &card)

    return &pb.DiscoverResponse{Card: &pb.AgentCard{
        Id:           card.ID,
        Name:         card.Name,
        Description:  card.Description,
        Capabilities: card.Capabilities,
        Url:          card.URL,
    }}, nil
}
```

### 5.5 任务发送流程

```go
func (s *TaskService) SendTask(ctx context.Context, req *pb.SendTaskRequest) (*pb.SendTaskResponse, error) {
    // 1. 查找目标 Agent
    card, err := s.registry.Get(ctx, req.AgentId)
    if err != nil {
        return nil, fmt.Errorf("agent not found: %s", req.AgentId)
    }

    // 2. 创建任务
    task := &A2ATask{
        ID:        uuid.New().String(),
        AgentID:   req.AgentId,
        Status:    TaskStatusSubmitted,
        Messages:  []A2AMessage{req.Message},
        Metadata:  req.Metadata,
        CreatedAt: time.Now(),
    }
    s.repo.Create(ctx, task)

    // 3. 发送到远程 Agent
    remoteReq := map[string]interface{}{
        "id":      task.ID,
        "message": req.Message,
    }

    resp, err := http.Post(card.URL+"/api/v2/a2a/tasks/send", remoteReq)
    if err != nil {
        task.Status = TaskStatusFailed
        s.repo.Update(ctx, task)
        return nil, err
    }

    // 4. 更新任务状态
    task.Status = TaskStatusWorking
    s.repo.Update(ctx, task)

    // 5. 等待结果 (异步)
    go s.waitForResult(task.ID, card.URL)

    return &pb.SendTaskResponse{Task: convertTask(task)}, nil
}
```

---

## 6. MCP Service（工具协议服务）

### 6.1 概述

MCP Service 实现 Model Context Protocol：
- MCP Server（对外暴露工具）
- MCP Client（连接外部工具）
- 工具注册与发现

### 6.2 目录结构

```
services/mcp-service/
├── cmd/
│   └── main.go
│
├── internal/
│   ├── config/
│   │   └── config.go
│   │
│   ├── service/
│   │   ├── mcp_server.go         # MCP Server
│   │   ├── mcp_client.go         # MCP Client
│   │   ├── tool_registry.go      # 工具注册
│   │   ├── tool_executor.go      # 工具执行
│   │   └── resource_service.go   # 资源管理
│   │
│   ├── handler/
│   │   └── grpc_handler.go
│   │   └── mcp_http_handler.go   # MCP JSON-RPC
│   │
│   ├── model/
│   │   ├── tool.go               # 工具定义
│   │   ├── resource.go           # 资源定义
│   │   ├── prompt.go             # 提示词定义
│   │   └── connection.go         # 连接定义
│   │
│   ├── tools/
│   │   ├── search_tool.go        # 知识库搜索
│   │   ├── web_search_tool.go    # 网络搜索
│   │   ├── calculator_tool.go    # 计算器
│   │   ├── weather_tool.go       # 天气
│   │   └── code_tool.go          # 代码执行
│   │
│   └── protocol/
│   │   └── jsonrpc.go            # JSON-RPC 2.0
│
├── Dockerfile
└── config.yaml
```

### 6.3 核心数据结构

```go
// 工具
type Tool struct {
    Name        string
    Description string
    InputSchema string    // JSON Schema
}

// 工具调用请求
type ToolCallRequest struct {
    Name      string
    Arguments map[string]interface{}
}

// 工具调用结果
type ToolCallResult struct {
    IsError bool
    Content string
}

// MCP 连接
type Connection struct {
    ID      string
    Name    string
    Type    string    // stdio, sse, websocket
    Status  string    // connected, disconnected
    Command string    // for stdio
    URL     string    // for sse/websocket
}
```

### 6.4 MCP Server 实现

```go
func (s *MCPServer) HandleJSONRPC(ctx context.Context, req *JSONRPCRequest) (*JSONRPCResponse, error) {
    switch req.Method {
    case "tools/list":
        return s.handleToolsList(ctx, req)

    case "tools/call":
        return s.handleToolsCall(ctx, req)

    case "resources/list":
        return s.handleResourcesList(ctx, req)

    case "resources/read":
        return s.handleResourcesRead(ctx, req)

    case "prompts/list":
        return s.handlePromptsList(ctx, req)

    case "prompts/get":
        return s.handlePromptsGet(ctx, req)

    default:
        return &JSONRPCResponse{
            ID:    req.ID,
            Error: &JSONRPCError{Code: -32601, Message: "method not found"},
        }, nil
    }
}

func (s *MCPServer) handleToolsCall(ctx context.Context, req *JSONRPCRequest) (*JSONRPCResponse, error) {
    params := req.Params.(map[string]interface{})
    name := params["name"].(string)
    arguments := params["arguments"].(map[string]interface{})

    // 执行工具
    result, err := s.toolExecutor.Execute(ctx, name, arguments)
    if err != nil {
        return &JSONRPCResponse{
            ID:    req.ID,
            Result: map[string]interface{}{
                "isError": true,
                "content": []map[string]interface{}{
                    {"type": "text", "text": err.Error()},
                },
            },
        }, nil
    }

    return &JSONRPCResponse{
        ID:    req.ID,
        Result: map[string]interface{}{
            "isError": false,
            "content": []map[string]interface{}{
                {"type": "text", "text": result},
            },
        },
    }, nil
}
```

---

## 7. Harness Service（运维治理服务）

### 7.1 概述

Harness Service 是运维治理核心，包含：
- 规则引擎
- 输入输出护栏
- 沙箱执行
- 评估测试
- A/B 测试
- SLO 管理
- 混沌工程
- 自动回滚
- 根因分析
- 成本分析
- 特性开关
- Pipeline 编排

### 7.2 目录结构

```
services/harness-service/
├── cmd/
│   └── main.go
│
├── internal/
│   ├── config/
│   │   └── config.go
│   │
│   ├── service/
│   │   ├── harness_service.go    # 核心服务
│   │   └── harness_chat.go       # 治理对话包装
│   │
│   ├── handler/
│   │   └── grpc_handler.go
│   │
│   ├── model/
│   │   ├── rule.go
│   │   ├── eval.go
│   │   ├── abtest.go
│   │   ├── slo.go
│   │   ├── proposal.go
│   │   └── trace.go
│   │
│   ├── repository/
│   │   └── sqlite_repo.go
│   │
│   ├── rule/
│   │   ├── engine.go             # 规则引擎
│   │   ├── guardrail.go          # 护栏
│   │   ├── loader.go             # 规则加载
│   │   └── watcher.go            # 规则热更新
│   │
│   ├── sandbox/
│   │   ├── executor.go           # 沙箱执行器
│   │   ├── permission.go         # 权限矩阵
│   │   └── budget.go             # 预算执行器
│   │
│   ├── evaluate/
│   │   ├── runner.go             # 评估运行器
│   │   ├── scorer.go             # 评分器
│   │   ├── suite.go              # 测试套件
│   │   ├── comparator.go         # 对比器
│   │   └── regression.go         # 回归检测
│   │
│   ├── abtest/
│   │   ├── engine.go             # A/B 测试引擎
│   │   ├── statistics.go         # 统计显著性
│   │   └── traffic.go            # 流量分流
│   │
│   ├── slo/
│   │   ├── manager.go            # SLO 管理
│   │   └── burn_rate.go          # 错误预算
│   │
│   ├── chaos/
│   │   ├── engine.go             # 混沌引擎
│   │   └── injector.go           # 故障注入器
│   │
│   ├── rollback/
│   │   ├── engine.go             # 回滚引擎
│   │   └── snapshot.go           # 配置快照
│   │
│   ├── rca/
│   │   └── engine.go             # 根因分析
│   │
│   ├── cost/
│   │   └── analyzer.go           # 成本分析
│   │
│   ├── featureflag/
│   │   └── engine.go             # 特性开关
│   │
│   ├── pipeline/
│   │   ├── engine.go             # Pipeline 引擎
│   │   └── runners.go            # 各阶段执行器
│   │
│   └── trace/
│   │   ├── collector.go          # Trace 收集
│   │   └── span.go               # Span 定义
│   │
│   └── tenant/
│   │   ├── model.go
│   │   └── quota.go
│
├── Dockerfile
└── config.yaml
```

### 7.3 Gate 流程

```go
func (s *HarnessChatService) Chat(ctx context.Context, req *pb.HarnessChatRequest) (*pb.HarnessChatResponse, error) {
    result := &pb.HarnessChatResponse{}

    // Gate 1: 输入护栏
    inputGuard, err := s.guardrail.CheckInput(ctx, req.Message)
    if err != nil || !inputGuard.Passed {
        result.InputGuard = inputGuard
        result.Content = "输入被护栏拦截"
        return result, nil
    }

    // Gate 2: 规则预检
    ruleCheck, err := s.ruleEngine.PreCheck(ctx, req.AgentId, req.Message)
    if err != nil || !ruleCheck.Passed {
        result.RuleCheck = ruleCheck
        result.Content = "请求被规则拦截"
        return result, nil
    }

    // Gate 3: 预算预检
    budgetCheck, err := s.budget.PreCheck(ctx, req.TenantId)
    if err != nil || !budgetCheck.Passed {
        result.Content = "预算耗尽"
        return result, nil
    }

    // Gate 4: 权限检查
    permCheck, err := s.permission.Check(ctx, req.AgentId, req.ToolIds)
    if err != nil || !permCheck.Passed {
        result.Content = "权限不足"
        return result, nil
    }

    // A/B 测试分流
    abTestInfo := s.abtest.Route(ctx, req.AgentId)

    // 执行 LLM (调用 Chat Service)
    llmResp, err := s.innerClient.Chat(ctx, req.Message, abTestInfo.Model, req.SystemPrompt)
    if err != nil {
        result.Error = err.Error()
        return result, nil
    }

    // Gate 5: 输出护栏
    outputGuard, err := s.guardrail.CheckOutput(ctx, llmResp.Content)
    if err != nil || !outputGuard.Passed {
        result.OutputGuard = outputGuard
        // 可能触发回滚
        s.rollback.Trigger(ctx, req.AgentId, "output_guardrail_violation")
        return result, nil
    }

    // Gate 6: 成本记录
    s.cost.Record(ctx, req.TenantId, llmResp.Tokens, llmResp.Cost)

    // Gate 7: 质量评分
    qualityScore := s.scorer.Score(ctx, req.Message, llmResp.Content)

    // Gate 8: A/B 测试记录
    if abTestInfo.TestId != "" {
        s.abtest.Record(ctx, abTestInfo.TestId, abTestInfo.Group, qualityScore)
    }

    // Gate 9: SLO 检查
    s.slo.Record(ctx, req.AgentId, qualityScore)

    // Gate 10: 回滚检查
    if s.shouldRollback(ctx, req.AgentId, qualityScore) {
        s.rollback.Trigger(ctx, req.AgentId, "quality_regression")
    }

    // 构建响应
    result.Content = llmResp.Content
    result.InputGuard = inputGuard
    result.OutputGuard = outputGuard
    result.RuleCheck = ruleCheck
    result.Cost = llmResp.Cost
    result.Tokens = llmResp.Tokens
    result.TraceId = s.trace.GetTraceID(ctx)

    return result, nil
}
```

### 7.4 护栏实现

```go
func (g *Guardrail) CheckInput(ctx context.Context, content string) (*GuardCheckResult, error) {
    violations := []string{}

    // 1. Prompt Injection 检测
    if g.detectPromptInjection(content) {
        violations = append(violations, "prompt_injection_detected")
    }

    // 2. 敏感信息检测
    if g.detectSensitiveInfo(content) {
        violations = append(violations, "sensitive_info_detected")
    }

    // 3. 自定义规则检测
    for _, rule := range g.customRules {
        if !rule.Check(content) {
            violations = append(violations, rule.Name)
        }
    }

    return &GuardCheckResult{
        Passed:    len(violations) == 0,
        Violations: violations,
    }, nil
}

func (g *Guardrail) detectPromptInjection(content string) bool {
    patterns := []string{
        "ignore previous instructions",
        "disregard all above",
        "forget everything",
        "system prompt",
        "you are now",
    }

    lowerContent := strings.ToLower(content)
    for _, pattern := range patterns {
        if strings.Contains(lowerContent, pattern) {
            return true
        }
    }
    return false
}

func (g *Guardrail) detectSensitiveInfo(content string) bool {
    // API Key 检测
    apiKeyPatterns := []string{
        `sk-[a-zA-Z0-9]{20}`,       // OpenAI
        `[a-zA-Z0-9]{32}`,          // Generic
    }

    for _, pattern := range apiKeyPatterns {
        if regexp.MustCompile(pattern).MatchString(content) {
            return true
        }
    }

    return false
}
```

### 7.5 评估实现

```go
func (s *Scorer) Score(ctx context.Context, input, output string) (float64, error) {
    // 1. 关键词匹配评分 (快速)
    keywordScore := s.keywordScore(input, output)

    // 2. LLM-as-Judge 评分 (可选)
    if s.useLLMJudge {
        llmScore, err := s.llmJudge(ctx, input, output)
        if err == nil {
            return llmScore, nil
        }
    }

    return keywordScore, nil
}

func (s *Scorer) llmJudge(ctx context.Context, input, output string) (float64, error) {
    prompt := fmt.Sprintf(`
Evaluate the following AI response:

User Question: %s
AI Response: %s

Rate the response on a scale of 0-10 for:
1. Faithfulness (does it contain hallucinations?)
2. Relevancy (does it answer the question?)
3. Completeness (is the answer complete?)

Return JSON: {"faithfulness": x, "relevancy": x, "completeness": x}
`, input, output)

    resp, err := s.llmClient.Chat(ctx, prompt, "gpt-4")
    if err != nil {
        return 0, err
    }

    // 解析 JSON
    var scores struct {
        Faithfulness  float64
        Relevancy     float64
        Completeness  float64
    }
    json.Unmarshal([]byte(resp), &scores)

    return (scores.Faithfulness + scores.Relevancy + scores.Completeness) / 3.0, nil
}
```