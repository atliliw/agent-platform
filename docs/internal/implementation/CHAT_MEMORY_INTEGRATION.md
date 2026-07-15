# Chat 记忆功能集成方案

## 当前问题

Chat Service 有短期记忆（最近 10 条消息），但没有集成 Memory Service 的长期记忆功能。

## 记忆架构设计

```
┌─────────────────────────────────────────────────────────────────┐
│                        Chat Service                              │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  用户消息 ──▶ 1. 检索长期记忆 (Memory Service)                    │
│                  ↓                                               │
│              2. 加载短期记忆 (最近 N 条消息)                       │
│                  ↓                                               │
│              3. 构建上下文                                        │
│                  ↓                                               │
│              4. 调用 LLM/Agent                                    │
│                  ↓                                               │
│              5. 提取重要信息，保存到长期记忆                       │
│                  ↓                                               │
│  响应 ◀───  6. 返回结果                                          │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## 实现步骤

### 步骤 1: 在 Chat Service 中添加 Memory Client

```go
// services/chat-service/internal/service/chat_service.go

import (
    memorypb "agent-platform/pkg/pb/memory"
)

type ChatService struct {
    // ... 现有字段
    memoryClient   memorypb.MemoryServiceClient  // 新增
    memoryConn     *grpc.ClientConn              // 新增
    enableMemory   bool                           // 新增：是否启用长期记忆
}
```

### 步骤 2: 初始化时连接 Memory Service

```go
func NewChatService(...) *ChatService {
    // 连接 Memory Service
    var memoryClient memorypb.MemoryServiceClient
    var memoryConn *grpc.ClientConn
    
    if memoryAddr := cfg.Services.Memory; memoryAddr != "" {
        conn, err := grpc.Dial(memoryAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
        if err == nil {
            memoryClient = memorypb.NewMemoryServiceClient(conn)
            memoryConn = conn
            fmt.Printf("Connected to Memory Service at %s\n", memoryAddr)
        }
    }
    
    return &ChatService{
        // ...
        memoryClient: memoryClient,
        memoryConn:   memoryConn,
        enableMemory: memoryClient != nil,
    }
}
```

### 步骤 3: 修改 buildMessages 方法

```go
func (s *ChatService) buildMessages(ctx context.Context, session *repository.Session, userMessage, systemPrompt string) []llm.Message {
    messages := make([]llm.Message, 0)
    
    // 1. 添加系统提示
    systemContent := systemPrompt
    
    // 2. ★ 检索长期记忆
    if s.enableMemory && s.memoryClient != nil {
        recalledMemories, err := s.recallMemories(ctx, userMessage, session.ID, session.TenantID)
        if err == nil && len(recalledMemories) > 0 {
            systemContent += "\n\n相关历史记忆:\n"
            for _, m := range recalledMemories {
                systemContent += fmt.Sprintf("- %s\n", m.Content)
            }
        }
    }
    
    if systemContent != "" {
        messages = append(messages, llm.Message{
            Role:    "system",
            Content: systemContent,
        })
    }
    
    // 3. 添加短期记忆（最近消息）
    maxHistory := 10  // 可配置
    start := 0
    if len(session.Messages) > maxHistory {
        start = len(session.Messages) - maxHistory
    }
    
    for i := start; i < len(session.Messages); i++ {
        m := session.Messages[i]
        messages = append(messages, llm.Message{
            Role:    m.Role,
            Content: m.Content,
        })
    }
    
    // 4. 添加当前用户消息
    messages = append(messages, llm.Message{
        Role:    "user",
        Content: userMessage,
    })
    
    return messages
}
```

### 步骤 4: 实现记忆检索方法

```go
// recallMemories 检索相关记忆
func (s *ChatService) recallMemories(ctx context.Context, query, sessionID, tenantID string) ([]*memorypb.MemoryEntry, error) {
    if s.memoryClient == nil {
        return nil, fmt.Errorf("memory service not available")
    }
    
    resp, err := s.memoryClient.Recall(ctx, &memorypb.RecallMemoryRequest{
        Query:     query,
        SessionId: sessionID,
        TenantId:  tenantID,
        TopK:      5,  // 检索最相关的 5 条记忆
    })
    if err != nil {
        return nil, err
    }
    
    return resp.Memories, nil
}
```

### 步骤 5: 对话后保存重要记忆

```go
// saveImportantMemory 保存重要信息到长期记忆
func (s *ChatService) saveImportantMemory(ctx context.Context, sessionID, tenantID, content string, importance float64) error {
    if s.memoryClient == nil {
        return nil
    }
    
    // 使用 LLM 判断是否值得记住
    if !s.isWorthRemembering(content) {
        return nil
    }
    
    _, err := s.memoryClient.Save(ctx, &memorypb.SaveMemoryRequest{
        SessionId:  sessionID,
        TenantId:   tenantID,
        Content:    content,
        Type:       memorypb.MemoryType_MEMORY_TYPE_FACT,
        Importance: importance,
    })
    
    return err
}

// isWorthRemembering 判断信息是否值得记忆
func (s *ChatService) isWorthRemembering(content string) bool {
    // 简单规则：包含关键信息
    keywords := []string{"我的名字是", "我喜欢", "我讨厌", "我是", "记住", "重要"}
    for _, kw := range keywords {
        if strings.Contains(content, kw) {
            return true
        }
    }
    return false
}
```

### 步骤 6: 修改 Chat 方法整合记忆

```go
func (s *ChatService) chatWithSingleAgent(ctx context.Context, req *pb.ChatRequest) (*pb.ChatResponse, error) {
    // 获取或创建会话
    session, err := s.getOrCreateSession(ctx, req.SessionId, req.TenantId, req.UserId)
    
    // ★ 构建消息（包含长期记忆）
    messages := s.buildMessages(ctx, session, req.Message, req.SystemPrompt)
    
    // ... 执行 Agent 循环
    
    // ★ 保存重要信息到长期记忆
    if s.enableMemory {
        s.saveImportantMemory(ctx, session.ID, req.TenantId, req.Message, 0.5)
        s.saveImportantMemory(ctx, session.ID, req.TenantId, finalContent, 0.3)
    }
    
    // ... 返回响应
}
```

## 配置更新

```yaml
# services/chat-service/config.yaml

# 记忆配置
memory:
  enabled: true
  max_short_term: 10        # 短期记忆最大条数
  recall_top_k: 5           # 检索记忆数量
  auto_save: true           # 自动保存重要信息
  
# 服务地址
services:
  mcp: mcp-service:50005
  agent: agent-service:50006
  memory: memory-service:50003  # 新增
```

## 记忆类型

| 类型 | 说明 | 示例 |
|------|------|------|
| `MEMORY_TYPE_IMPORTANT` | 用户明确要求记住的信息 | "记住我叫张三" |
| `MEMORY_TYPE_SUMMARY` | 对话摘要 | "用户询问了天气，推荐了北京旅游" |
| `MEMORY_TYPE_FACT` | 事实信息 | "用户喜欢编程" |

## 使用效果

### 没有长期记忆
```
用户: 我叫张三
AI: 你好张三！

用户: 我叫什么？
AI: 抱歉，我不记得你的名字。  ← 新会话会丢失
```

### 有长期记忆
```
用户: 我叫张三
AI: 你好张三！
[系统自动保存: "用户名字是张三" → 向量数据库]

用户: 我叫什么？
AI: 你叫张三。  ← 从长期记忆检索到
```

## 下一步

1. 实现 `extractImportantInfo` 用 LLM 提取关键信息
2. 添加记忆遗忘机制（根据 importance 和时间衰减）
3. 支持用户手动管理记忆（查看/删除）
