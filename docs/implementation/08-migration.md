# 08 - 迁移计划

## 1. 迁移来源

### 1.1 来源项目

| 项目 | 技术栈 | 迁移内容 |
|------|--------|---------|
| langchainRustAgent | Rust + Axum | A2A 协议、知识库分块、长期记忆 |
| mcpgo | Go + Gin | Chat 服务、MCP 协议、Harness 系统 |

### 1.2 迁移策略

| 类型 | 策略 | 说明 |
|------|------|------|
| Go 代码 | 直接复用 | mcpgo 中的 Go 代码可直接迁移 |
| Rust 代码 | 重写 | Rust 代码需要用 Go 重写 |
| 设计模式 | 借鉴 | 架构设计可以借鉴 |

---

## 2. 代码迁移清单

### 2.1 从 mcpgo 迁移（Go 代码）

| 模块 | 源路径 | 目标路径 | 迁移方式 |
|------|--------|---------|---------|
| LLM Client | `backend/pkg/llm/` | `pkg/llm/` | 直接复制 |
| Embedding Client | `backend/pkg/embedding/` | `pkg/embedding/` | 直接复制 |
| Qdrant Client | `backend/pkg/qdrant/` | `pkg/qdrant/` | 直接复制 |
| MongoDB Client | `backend/pkg/mongodb/` | `pkg/mongodb/` | 直接复制 |
| MCP Server | `backend/pkg/mcpserver/` | `services/mcp-service/internal/` | 直接复制 |
| Chat Service | `backend/internal/service/chat_service.go` | `services/chat-service/` | 适配修改 |
| Multi Agent | `backend/internal/service/multi_agent.go` | `services/chat-service/internal/agent/` | 适配修改 |
| Harness 全部 | `backend/internal/harness/` | `services/harness-service/internal/` | 直接复制 |
| Handler | `backend/internal/handler/` | `services/*/internal/handler/` | 拆分适配 |
| Middleware | `backend/internal/middleware/` | `services/gateway/internal/middleware/` | 直接复制 |
| Model | `backend/internal/model/` | 各服务 `internal/model/` | 拆分适配 |
| Config | `backend/internal/config/` | 各服务 `internal/config/` | 适配修改 |

### 2.2 从 langchainRustAgent 迁移（Rust → Go 重写）

| 模块 | 源路径 | 目标路径 | 重写要点 |
|------|--------|---------|---------|
| A2A Protocol | `src/services/a2a/protocol.rs` | `services/a2a-service/internal/model/` | 数据结构转换 |
| A2A Server | `src/services/a2a/server.rs` | `services/a2a-service/internal/service/` | HTTP → gRPC |
| A2A Client | `src/services/a2a/client.rs` | `services/a2a-service/internal/service/` | HTTP → gRPC |
| Token Chunker | `src/utils/chunkers.rs` | `services/knowledge-service/internal/chunker/` | 算法逻辑 |
| Semantic Chunker | `src/utils/chunkers.rs` | `services/knowledge-service/internal/chunker/` | Embedding 调用 |
| Document Parser | `src/utils/document_processor.rs` | `services/knowledge-service/internal/parser/` | 文件解析 |
| Memory Manager | `src/services/memory.rs` | `services/memory-service/internal/service/` | 存储逻辑 |
| Agent Registry | `src/services/agent_registry.rs` | `services/chat-service/internal/agent/` | 路由逻辑 |

---

## 3. 详细迁移步骤

### 3.1 Phase 1: 公共库迁移 (Day 1-3)

#### Step 1.1: LLM Client

**源文件**: `mcpgo/backend/pkg/llm/`

**目标文件**: `agent-platform/pkg/llm/`

**迁移操作**:
```bash
# 1. 复制文件
cp -r mcpgo/backend/pkg/llm/* agent-platform/pkg/llm/

# 2. 修改 import 路径
sed -i 's|mcpgo/backend|agent-platform|g' agent-platform/pkg/llm/*.go

# 3. 添加 gRPC 支持 (如果需要)
# 在 client.go 中添加 gRPC 方法
```

**需要修改的内容**:
- 更新 import 路径
- 添加 gRPC 兼容方法
- 统一错误处理

#### Step 1.2: Embedding Client

**源文件**: `mcpgo/backend/pkg/embedding/`

**目标文件**: `agent-platform/pkg/embedding/`

**迁移操作**:
```bash
cp -r mcpgo/backend/pkg/embedding/* agent-platform/pkg/embedding/
```

#### Step 1.3: Qdrant Client

**源文件**: `mcpgo/backend/pkg/qdrant/`

**目标文件**: `agent-platform/pkg/qdrant/`

**迁移操作**:
```bash
cp -r mcpgo/backend/pkg/qdrant/* agent-platform/pkg/qdrant/
```

**需要增强的功能**:
- 批量插入优化
- 错误重试
- 连接池

#### Step 1.4: MongoDB Client

**源文件**: `mcpgo/backend/pkg/mongodb/`

**目标文件**: `agent-platform/pkg/mongodb/`

**迁移操作**:
```bash
cp -r mcpgo/backend/pkg/mongodb/* agent-platform/pkg/mongodb/
```

**需要增强的功能**:
- BM25 索引创建
- 文本搜索优化

---

### 3.2 Phase 2: Chat Service 迁移 (Day 4-7)

#### Step 2.1: 核心服务迁移

**源文件**: 
- `mcpgo/backend/internal/service/chat_service.go`
- `mcpgo/backend/internal/service/multi_agent.go`

**目标文件**: `agent-platform/services/chat-service/internal/service/`

**迁移操作**:

```go
// 1. 复制核心逻辑
// chat_service.go → chat_service.go

// 2. 修改服务初始化
// 原代码
func NewChatService(cfg *config.Config, llm *llm.Client, ...) *ChatService

// 新代码 (gRPC)
func NewChatService(cfg *config.Config, clients *Clients) *ChatService {
    return &ChatService{
        llmClient:      clients.LLM,
        memoryClient:   clients.Memory,   // gRPC 客户端
        mcpClient:      clients.MCP,      // gRPC 客户端
        harnessClient:  clients.Harness,  // gRPC 客户端
    }
}

// 3. 修改方法签名
// 原代码
func (s *ChatService) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)

// 新代码 (gRPC)
func (s *ChatService) Chat(ctx context.Context, req *pb.ChatRequest) (*pb.ChatResponse, error)
```

#### Step 2.2: 多 Agent 迁移

**源文件**: `mcpgo/backend/internal/service/multi_agent.go`

**迁移操作**:

```go
// 1. 复制 Master-Worker 逻辑
// 2. 修改 Agent 定义为 Proto 兼容类型
// 3. 添加 gRPC 调用支持

type MasterAgent struct {
    llmClient     *llm.Client
    planner       *Planner
    specialists   map[string]*Specialist
}

// 修改为
type MasterAgent struct {
    llmClient     pb.LLMServiceClient    // gRPC
    planner       *Planner
    specialists   map[string]pb.AgentServiceClient
}
```

---

### 3.3 Phase 3: Knowledge Service 迁移 (Day 8-10)

#### Step 3.1: 分块器重写 (Rust → Go)

**源文件**: `langchainRustAgent/src/utils/chunkers.rs`

**目标文件**: `agent-platform/services/knowledge-service/internal/chunker/`

**Rust 原代码**:
```rust
pub struct TokenTextSplitter {
    chunk_tokens: usize,
    overlap_tokens: usize,
}

impl TokenTextSplitter {
    fn find_split_end(&self, chars: &[char], start: usize) -> usize {
        // ... 边界查找逻辑
    }
}
```

**Go 重写**:
```go
type TokenChunker struct {
    chunkTokens  int
    overlapTokens int
}

func (c *TokenChunker) findSplitEnd(chars []rune, start int) int {
    maxChars := c.chunkTokens * 2
    end := min(start+maxChars, len(chars))
    
    if end == len(chars) {
        return end
    }
    
    // 找段落边界
    for i := end - 1; i >= start; i-- {
        if chars[i] == '\n' && i+1 < len(chars) && chars[i+1] == '\n' {
            return i + 2
        }
    }
    
    // 找句子边界
    for i := end - 1; i >= start; i-- {
        if chars[i] == '。' || chars[i] == '.' {
            return i + 1
        }
    }
    
    return end
}
```

#### Step 3.2: 语义分块重写

**Rust 原代码**:
```rust
pub struct SemanticChunker {
    embeddings: Arc<OpenAIEmbeddings>,
    min_chunk_chars: usize,
    max_chunk_chars: usize,
}

impl SemanticChunker {
    pub async fn split_document_semantic(&self, document: &Document) -> Result<Vec<Document>, ProcessError> {
        // 1. 分句
        // 2. 批量 Embedding
        // 3. 计算相似度找边界
        // 4. 构建分块
    }
}
```

**Go 重写**:
```go
type SemanticChunker struct {
    embeddingClient *embedding.Client
    minChunkChars   int
    maxChunkChars   int
}

func (c *SemanticChunker) Chunk(ctx context.Context, content string) ([]string, error) {
    // 1. 分句
    sentences := c.splitSentences(content)
    
    // 2. 批量 Embedding
    vectors, err := c.embeddingClient.EmbedBatch(ctx, sentences)
    if err != nil {
        return nil, err
    }
    
    // 3. 计算相似度找边界
    boundaries := c.findBoundaries(sentences, vectors)
    
    // 4. 构建分块
    return c.buildChunks(sentences, boundaries), nil
}

func (c *SemanticChunker) findBoundaries(sentences []string, vectors [][]float64) []int {
    boundaries := []int{0}
    currentLen := 0
    
    for i := 1; i < len(sentences); i++ {
        sim := cosineSimilarity(vectors[i-1], vectors[i])
        
        if sim < 0.45 && currentLen >= c.minChunkChars {
            boundaries = append(boundaries, i)
            currentLen = 0
        } else if currentLen + len(sentences[i]) > c.maxChunkChars {
            boundaries = append(boundaries, i)
            currentLen = 0
        } else {
            currentLen += len(sentences[i])
        }
    }
    
    boundaries = append(boundaries, len(sentences))
    return boundaries
}
```

---

### 3.4 Phase 4: Memory Service 迁移 (Day 11-12)

#### Step 4.1: 记忆管理器重写

**源文件**: `langchainRustAgent/src/services/memory.rs`

**Rust 原代码**:
```rust
pub struct MemoryManager {
    short_term: SqliteStore,
    long_term: QdrantStore,
    scratchpads: HashMap<String, Scratchpad>,
}

impl MemoryManager {
    pub async fn save_long_term_memory(&self, entry: &MemoryEntry) -> Result<(), Error> {
        let vector = self.embeddings.embed(&entry.content).await?;
        self.qdrant.upsert(entry.id, vector, payload).await?;
        self.db.insert(entry).await?;
        Ok(())
    }
    
    pub async fn recall_long_term_memory(&self, query: &str, top_k: usize) -> Result<Vec<MemoryEntry>, Error> {
        let vector = self.embeddings.embed(query).await?;
        let results = self.qdrant.search(vector, top_k).await?;
        // ...
    }
}
```

**Go 重写**:
```go
type MemoryManager struct {
    shortTerm  *SQLiteRepo
    longTerm   *QdrantRepo
    embedding  *embedding.Client
}

func (m *MemoryManager) SaveLongTerm(ctx context.Context, entry *MemoryEntry) error {
    // 1. 向量化
    vector, err := m.embedding.Embed(ctx, entry.Content)
    if err != nil {
        return err
    }
    
    // 2. 存入 Qdrant
    err = m.longTerm.Upsert(ctx, &pb.Point{
        ID:      entry.ID,
        Vector:  vector,
        Payload: entry.ToPayload(),
    })
    if err != nil {
        return err
    }
    
    // 3. 存入 SQLite
    return m.shortTerm.InsertEntry(ctx, entry)
}

func (m *MemoryManager) Recall(ctx context.Context, query string, topK int) ([]*MemoryEntry, error) {
    // 1. Query 向量化
    vector, err := m.embedding.Embed(ctx, query)
    if err != nil {
        return nil, err
    }
    
    // 2. 向量搜索
    results, err := m.longTerm.Search(ctx, vector, topK)
    if err != nil {
        return nil, err
    }
    
    // 3. 转换结果
    entries := make([]*MemoryEntry, len(results))
    for i, r := range results {
        entries[i] = MemoryEntryFromPayload(r.Payload)
    }
    
    return entries, nil
}
```

---

### 3.5 Phase 5: A2A Service 迁移 (Day 15-16)

#### Step 5.1: 协议重写

**源文件**: `langchainRustAgent/src/services/a2a/protocol.rs`

**Rust 原代码**:
```rust
#[derive(Serialize, Deserialize)]
pub struct AgentCard {
    pub id: String,
    pub name: String,
    pub description: String,
    pub capabilities: Vec<String>,
    pub input_modes: Vec<String>,
    pub output_modes: Vec<String>,
    pub url: String,
}

#[derive(Serialize, Deserialize)]
pub struct A2ATask {
    pub id: String,
    pub status: TaskStatus,
    pub messages: Vec<A2AMessage>,
}

#[derive(Serialize, Deserialize)]
pub enum TaskStatus {
    Submitted,
    Working,
    Completed,
    Failed,
    Cancelled,
}
```

**Go 重写**:
```go
// 已经在 proto/a2a/a2a.proto 中定义

type AgentCard = pb.AgentCard
type A2ATask = pb.A2ATask
type TaskStatus = pb.TaskStatus

// 或者定义本地类型
type AgentCard struct {
    ID           string
    Name         string
    Description  string
    Capabilities []string
    InputModes   []string
    OutputModes  []string
    URL          string
}

func (c *AgentCard) ToProto() *pb.AgentCard {
    return &pb.AgentCard{
        Id:           c.ID,
        Name:         c.Name,
        Description:  c.Description,
        Capabilities: c.Capabilities,
        InputModes:   c.InputModes,
        OutputModes:  c.OutputModes,
        Url:          c.URL,
    }
}
```

#### Step 5.2: 客户端重写

**源文件**: `langchainRustAgent/src/services/a2a/client.rs`

**Rust 原代码**:
```rust
pub struct A2AClient {
    http_client: reqwest::Client,
}

impl A2AClient {
    pub async fn discover(&self, agent_url: &str) -> Result<AgentCard, Error> {
        let url = format!("{}/.well-known/agent.json", agent_url);
        let resp = self.http_client.get(&url).send().await?;
        let card: AgentCard = resp.json().await?;
        Ok(card)
    }
    
    pub async fn send_task(&self, agent_url: &str, task: &A2ATask) -> Result<A2ATask, Error> {
        let url = format!("{}/api/v2/a2a/tasks/send", agent_url);
        let resp = self.http_client.post(&url).json(task).send().await?;
        let task: A2ATask = resp.json().await?;
        Ok(task)
    }
}
```

**Go 重写**:
```go
type A2AClient struct {
    httpClient *http.Client
}

func (c *A2AClient) Discover(ctx context.Context, agentURL string) (*AgentCard, error) {
    url := fmt.Sprintf("%s/.well-known/agent.json", agentURL)
    
    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return nil, err
    }
    
    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    var card AgentCard
    if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
        return nil, err
    }
    
    return &card, nil
}

func (c *A2AClient) SendTask(ctx context.Context, agentURL string, task *A2ATask) (*A2ATask, error) {
    url := fmt.Sprintf("%s/api/v2/a2a/tasks/send", agentURL)
    
    body, err := json.Marshal(task)
    if err != nil {
        return nil, err
    }
    
    req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
    if err != nil {
        return nil, err
    }
    req.Header.Set("Content-Type", "application/json")
    
    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    var result A2ATask
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, err
    }
    
    return &result, nil
}
```

---

### 3.6 Phase 6: Harness Service 迁移 (Day 20-26)

#### Step 6.1: 直接复制

**源文件**: `mcpgo/backend/internal/harness/`

**目标文件**: `agent-platform/services/harness-service/internal/`

**迁移操作**:
```bash
# 直接复制整个目录
cp -r mcpgo/backend/internal/harness/* agent-platform/services/harness-service/internal/

# 更新 import 路径
find agent-platform/services/harness-service -name "*.go" -exec \
    sed -i 's|mcpgo/backend/internal|agent-platform/services/harness-service/internal|g' {} \;
```

#### Step 6.2: 适配修改

**需要修改的部分**:

1. **服务初始化**:
```go
// 原代码
func NewHarnessService(db *gorm.DB, llm *llm.Client, ...) *HarnessService

// 新代码
func NewHarnessService(db *gorm.DB, chatClient pb.ChatServiceClient, ...) *HarnessService
```

2. **Chat 调用**:
```go
// 原代码
resp, err := s.innerChatService.Chat(ctx, req)

// 新代码
resp, err := s.chatClient.Chat(ctx, req)  // gRPC 调用
```

3. **配置加载**:
```go
// 修改为独立的配置文件
type Config struct {
    Database DatabaseConfig
    Chat     ChatClientConfig
    // ...
}
```

---

## 4. 数据迁移

### 4.1 数据库迁移

| 数据类型 | 源存储 | 目标存储 | 迁移方式 |
|---------|--------|---------|---------|
| 会话数据 | SQLite | SQLite | 文件复制 |
| 文档数据 | MongoDB | MongoDB | mongodump/restore |
| 向量数据 | Qdrant | Qdrant | collection 快照 |
| 缓存数据 | Redis | Redis | 无需迁移 |

### 4.2 迁移脚本

```bash
#!/bin/bash
# migrate_data.sh

# MongoDB 迁移
mongodump --host source-mongo --db agent_platform --out /tmp/mongo_backup
mongorestore --host target-mongo --db agent_platform /tmp/mongo_backup/agent_platform

# Qdrant 迁移
# 在源 Qdrant 创建快照
curl -X POST "http://source-qdrant:6333/collections/documents/snapshots"
# 下载快照
curl "http://source-qdrant:6333/collections/documents/snapshots/snapshot_name" -o snapshot.tar
# 上传到目标
curl -X PUT "http://target-qdrant:6333/collections/documents/snapshots/upload" -T snapshot.tar
# 恢复快照
curl -X PUT "http://target-qdrant:6333/collections/documents/snapshots/recover?snapshot=snapshot_name"
```

---

## 5. 测试验证

### 5.1 单元测试迁移

**Rust 测试 → Go 测试**:

```rust
// Rust 测试
#[tokio::test]
async fn test_chunker() {
    let chunker = TokenTextSplitter::new(100, 10);
    let result = chunker.split_text("test content");
    assert!(result.len() > 0);
}
```

```go
// Go 测试
func TestChunker(t *testing.T) {
    chunker := NewTokenChunker(100, 10)
    result := chunker.Chunk(context.Background(), "test content")
    assert.Greater(t, len(result), 0)
}
```

### 5.2 集成测试验证

| 测试场景 | 验证方法 |
|---------|---------|
| 对话流程 | 发送消息，验证响应 |
| 知识库上传 | 上传文件，验证分块数 |
| 知识库检索 | 发送查询，验证召回 |
| A2A 通信 | 发送任务，验证结果 |
| Harness 治理 | 验证护栏拦截 |

---

## 6. 回滚计划

### 6.1 回滚触发条件

- 关键功能不可用
- 性能下降超过 50%
- 数据丢失

### 6.2 回滚步骤

1. 停止新服务
2. 恢复数据库快照
3. 启动旧服务
4. 验证功能正常

---

## 7. 迁移检查清单

### 7.1 代码迁移

- [ ] 公共库迁移完成
- [ ] Chat Service 迁移完成
- [ ] Knowledge Service 迁移完成
- [ ] Memory Service 迁移完成
- [ ] A2A Service 迁移完成
- [ ] MCP Service 迁移完成
- [ ] Harness Service 迁移完成
- [ ] Gateway 迁移完成

### 7.2 测试验证

- [ ] 单元测试通过
- [ ] 集成测试通过
- [ ] 端到端测试通过
- [ ] 性能测试通过

### 7.3 数据迁移

- [ ] 会话数据迁移完成
- [ ] 文档数据迁移完成
- [ ] 向量数据迁移完成
- [ ] 数据完整性验证

### 7.4 部署验证

- [ ] Docker 构建成功
- [ ] K8s 部署成功
- [ ] 健康检查通过
- [ ] 监控告警正常