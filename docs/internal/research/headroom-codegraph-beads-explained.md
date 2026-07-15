# Headroom / CodeGraph / Beads 原理详解

> 三个解决 Agent 上下文瓶颈的工具，各自从不同角度切入

---

## 核心问题

Agent 有一个共同的痛点：**上下文窗口不够用**

```
Agent 的上下文窗口就像一张桌子：
- 放太多东西 → 放不下（超出 token 限制）
- 东西乱放   → 找不到（信息丢失）
- 下次来桌子被清空 → 没记忆（跨会话丢失）
```

三个工具各自解决其中一个问题：

| 工具 | 解决什么 | 类比 |
|------|---------|------|
| **Headroom** | 东西太多放不下 | 压缩行李箱 |
| **CodeGraph** | 东西乱放找不到 | 画好地图 |
| **Beads** | 下次来桌子被清空 | 带个笔记本 |

---

## 1. Headroom — 上下文压缩

### 解决什么

Agent 调用工具后返回大量输出，塞满上下文窗口：

```
Agent 调用 grep 搜索代码 → 返回 100 行结果 = 17,000 tokens
Agent 读一个日志文件     → 65,000 tokens
Agent 查 GitHub issue   → 54,000 tokens

→ 上下文窗口被撑爆，Agent 忘了前面说了什么
```

### 工作原理

```
                Agent 调用工具返回的原始内容
                （工具输出、日志、RAG结果、文件内容）
                          │
                          ▼
            ┌─────────────────────────────┐
            │      Headroom 压缩层        │
            │                             │
            │  1. ContentRouter           │
            │     检测内容类型             │
            │     JSON? 代码? 纯文本?     │
            │          │                  │
            │  2. 选择压缩算法             │
            │     ├─ SmartCrusher (JSON)  │
            │     ├─ CodeCompressor (AST) │
            │     └─ Kompress-base (文本) │
            │          │                  │
            │  3. CacheAligner            │
            │     稳定前缀，命中 KV 缓存   │
            │          │                  │
            │  4. CCR 缓存               │
            │     原文存本地               │
            │     Agent 需要时可取回      │
            └─────────────────────────────┘
                          │
                          ▼
              压缩后的内容发给 LLM
              （原始内容本地留存）
```

### 三种压缩算法

#### SmartCrusher（压缩 JSON）

```
原始 JSON（2000 tokens）：
{
  "users": [
    {"id": 1, "name": "Alice", "email": "alice@example.com", "role": "admin"},
    {"id": 2, "name": "Bob",   "email": "bob@example.com",   "role": "user"},
    {"id": 3, "name": "Carol", "email": "carol@example.com", "role": "user"}
  ]
}

压缩后（400 tokens，省 80%）：
users: [id|name|email|role]
  1|Alice|alice@example.com|admin
  2|Bob|bob@example.com|user
  3|Carol|carol@example.com|user
```

原理：去掉重复的 key 名，用分隔符代替嵌套结构

#### CodeCompressor（压缩代码）

```
原始代码（1500 tokens）：
func (s *UserService) GetUser(ctx context.Context, id int64) (*User, error) {
    if id <= 0 {
        return nil, fmt.Errorf("invalid id: %d", id)
    }
    user, err := s.repo.FindByID(ctx, id)
    if err != nil {
        return nil, fmt.Errorf("find user: %w", err)
    }
    if user == nil {
        return nil, ErrUserNotFound
    }
    return user, nil
}

压缩后（300 tokens，省 80%）：
UserService.GetUser(ctx, id→int64) → (*User, error)
  guard: id>0, err!=nil, user!=nil
  core: s.repo.FindByID(ctx, id)
```

原理：基于 AST 解析，保留函数签名和核心逻辑，去掉样板代码

#### Kompress-base（压缩纯文本）

```
原始日志（5000 tokens）：
[2024-01-15 10:23:01] INFO  Starting server on port 8080
[2024-01-15 10:23:01] INFO  Connected to database
[2024-01-15 10:23:02] INFO  Migration complete
[2024-01-15 10:23:02] DEBUG Query: SELECT * FROM users WHERE id = 1
...（50行类似日志）

压缩后（800 tokens，省 84%）：
[10:23:01-02] INFO server:8080, db:connected, migration:done
[10:23:02] DEBUG SELECT * FROM users WHERE id=1
...（关键日志保留，重复模式合并）
```

原理：日志去重、时间戳压缩、DEBUG 级别选择性保留

### CacheAligner 原理

```
问题：每次压缩后前缀变了 → KV 缓存命中率低 → 浪费计算

解决：确保压缩后的内容前缀稳定

示例：
第1轮对话：
  [系统提示][工具输出A的压缩] → KV 缓存了前缀

第2轮对话：
  [系统提示][工具输出A的压缩][新内容] → 前缀命中 KV 缓存！
  
  而不是：
  [系统提示][完全不同的压缩] → KV 缓存全miss
```

### CCR 缓存原理

```
问题：压缩后 Agent 需要看原文怎么办？

解决：CCR（Compressed Content Registry）

原文 → 存本地文件（不占上下文）
压缩版 → 发给 LLM
Agent 需要原文 → 通过 CCR ID 取回

流程：
1. 工具输出 65000 tokens
2. Headroom 压缩到 8000 tokens 发给 LLM
3. 存原文到 ~/.headroom/cache/abc123
4. Agent 想看原文 → "展开 CCR:abc123"
5. 取回原文的特定部分（按行范围）
```

### 实测效果

| 内容类型 | 原始 tokens | 压缩后 | 节省 |
|----------|-----------|--------|------|
| JSON API 响应 | 17,000 | 2,800 | 84% |
| 代码文件 | 12,000 | 2,400 | 80% |
| 日志输出 | 65,000 | 9,100 | 86% |
| RAG 检索结果 | 8,000 | 2,400 | 70% |
| GitHub Issue | 54,000 | 9,700 | 82% |

---

## 2. CodeGraph — 代码索引

### 解决什么

大项目中 Agent 不知道代码结构：

```
你：帮我改一下用户认证的逻辑

Agent：[搜索文件...] [读了20个文件...] [还是不知道认证在哪里...]

→ 浪费大量 token 在"找代码"上
→ 找到了也可能遗漏关键文件
```

### 工作原理

```
                    你的代码库
                        │
                        ▼
            ┌─────────────────────────────┐
            │     CodeGraph 索引引擎      │
            │                             │
            │  1. AST 解析                │
            │     每个文件解析语法树       │
            │          │                  │
            │  2. 提取实体                │
            │     函数/类/接口/类型/常量   │
            │          │                  │
            │  3. 关系构建                │
            │     调用关系 A→B            │
            │     继承关系 A extends B     │
            │     导入关系 A imports B     │
            │     实现关系 A implements B  │
            │          │                  │
            │  4. 生成知识图谱             │
            │     节点 = 代码实体          │
            │     边   = 关系             │
            └─────────────────────────────┘
                        │
                        ▼
              .codegraph/ 目录
              ├── index.json      # 图谱数据
              ├── symbols.json    # 符号表
              ├── relations.json  # 关系表
              └── metadata.json   # 元数据
```

### 索引内容

每个代码实体的信息：

```json
{
  "name": "GetUser",
  "type": "function",
  "file": "services/user/service.go",
  "line": 45,
  "signature": "func (s *UserService) GetUser(ctx context.Context, id int64) (*User, error)",
  "doc": "GetUser retrieves a user by ID",
  "calls": ["s.repo.FindByID", "fmt.Errorf"],
  "called_by": ["handlers.GetUserHandler", "services.auth.ValidateToken"],
  "references_type": ["User", "error"],
  "package": "services.user"
}
```

### 关系类型

| 关系 | 例子 | 用途 |
|------|------|------|
| **calls** | A 函数调用了 B 函数 | 改 B 时知道影响哪些 A |
| **called_by** | A 函数被谁调用 | 同上，反方向 |
| **imports** | A 文件导入了 B 包 | 理解依赖关系 |
| **inherits** | A 类继承 B 类 | 理解类层次 |
| **implements** | A 类实现 B 接口 | 理解接口实现 |
| **references_type** | A 使用了 B 类型 | 理解数据流 |

### Agent 怎么用

```
没有 CodeGraph：
你：帮我改一下 GetUser 函数

Agent：[搜索 GetUser...] 
       [读了10个文件...] 
       [不确定影响范围...] 
       → 花了 5000 tokens 找代码

有 CodeGraph：
你：帮我改一下 GetUser 函数

Agent：[查 CodeGraph]
       GetUser 在 services/user/service.go:45
       被 GetUserHandler 和 ValidateToken 调用
       返回 *User 类型
       调用了 repo.FindByID
       → 花了 200 tokens，精确知道全貌
```

### 自动同步

```
文件变更 → git hook / file watcher 触发
        → 只重新索引变更的文件（增量更新）
        → 更新 .codegraph/ 目录
        → Agent 下次查询自动拿到最新数据
```

### 支持的语言

| 语言 | AST 解析器 |
|------|-----------|
| Go | go/ast |
| TypeScript | tree-sitter-typescript |
| Python | tree-sitter-python |
| Rust | tree-sitter-rust |
| Java | tree-sitter-java |
| C/C++ | tree-sitter-c / tree-sitter-cpp |

---

## 3. Beads — Agent 记忆

### 解决什么

Agent 没有持久记忆，每次对话从零开始：

```
第1次对话：
你：我喜欢用 JWT 做认证
Agent：好的，记住了
[对话结束，记忆消失]

第2次对话：
你：帮我做登录功能
Agent：请问你想用 JWT 还是 Session？
你：上次说过了啊... JWT
[Agent 忘了]
```

### 工作原理

```
                    对话内容
                       │
                       ▼
            ┌─────────────────────────────┐
            │       Beads 记忆层          │
            │                             │
            │  1. 提取                    │
            │     从对话中提取关键信息     │
            │     "用户偏好JWT"           │
            │     "项目用Go+gRPC"         │
            │          │                  │
            │  2. 分类                    │
            │     ├─ 偏好 (preference)    │
            │     ├─ 事实 (fact)          │
            │     ├─ 决策 (decision)      │
            │     ├─ 模式 (pattern)       │
            │     └─ 上下文 (context)     │
            │          │                  │
            │  3. 存储                    │
            │     Markdown 源文件         │
            │     + SQLite 全文索引       │
            │     + 向量嵌入（语义搜索）   │
            │          │                  │
            │  4. 精炼                    │
            │     定期合并重复记忆         │
            │     过时记忆标记失效         │
            │     重要记忆提升权重         │
            └─────────────────────────────┘
                       │
                       ▼
              下次对话自动注入相关记忆
```

### 存储格式

记忆以 Markdown 文件存储（人类可读、可编辑、可 Git）：

```markdown
# 用户偏好

## 认证方式
- 偏好 JWT，不用 Session
- 2024-01-15 添加

# 项目事实

## 技术栈
- Go + gRPC 微服务
- MongoDB + Redis
- Vue 3 + Element Plus 前端
- 2024-01-10 添加

# 决策记录

## 为什么选 MongoDB
- 灵活 schema，适合 Agent 数据
- 2024-01-12 添加
```

### 三层检索

```
Agent 请求记忆："认证相关"

第1层：关键词匹配（SQLite FTS5）
  → 搜索 "认证"、"JWT"、"auth"
  → 快速但只能精确匹配

第2层：语义搜索（向量嵌入）
  → "认证" ≈ "authentication" ≈ "登录验证"
  → 慢但能理解语义

第3层：关联扩展
  → 找到"认证" → 关联到"用户偏好" → 关联到"JWT"
  → 扩展上下文
```

### 精炼机制

```
问题：记忆越来越多 → 注入太多 → 浪费 token

解决：定期精炼

1. 合并重复
   "用户喜欢JWT" + "偏好JWT认证" → 合并为一条

2. 过时标记
   "用React" + "后来迁移到Vue3" → "用Vue3（从React迁移）"

3. 权重衰减
   30天没用的记忆 → 降低权重
   90天没用的记忆 → 标记为归档
   常用的记忆 → 提升权重

4. 压缩摘要
   10条关于前端框架的记忆 → 1条摘要
```

### 与 CLAUDE.md 的区别

| | CLAUDE.md | Beads |
|--|----------|-------|
| **谁写** | 你手动写 | Agent 自动提取 |
| **内容** | 项目规范 | 对话中产生的知识 |
| **更新** | 你手动改 | 自动精炼 |
| **搜索** | 全文加载 | 三层检索 |
| **跨会话** | 每次都加载 | 只加载相关的 |
| **类比** | 员工手册 | 工作笔记 |

### 跨 Agent 共享

```
Claude Code 的对话 → 提取记忆 → Beads 存储
                                  │
Codex 的对话     → 提取记忆 ─────┤
                                  │
Cursor 的对话    → 提取记忆 ─────┤
                                  ▼
                            同一份记忆库
                            所有 Agent 共享
```

---

## 三者配合使用

```
Agent 开始工作
      │
      ├─ Beads 注入相关记忆
      │  "这个项目用 Go+gRPC，用户偏好 JWT"
      │
      ├─ CodeGraph 提供代码索引
      │  "GetUser 在 service.go:45，被3个地方调用"
      │
      │  ... Agent 工作 ...
      │
      ├─ Headroom 压缩工具输出
      │  grep 返回 17000 tokens → 压缩到 2800 tokens
      │
      └─ Beads 保存新记忆
         "用户决定用 Redis 做 token 黑名单"
```

---

## 总结

| 工具 | 原理 | 效果 | 类比 |
|------|------|------|------|
| **Headroom** | AST解析/结构压缩/日志去重 → 只留关键信息 | 省 60-86% token | 压缩行李箱 |
| **CodeGraph** | AST解析 → 实体+关系 → 知识图谱 → 精确查询 | 省 90%+ 搜索 token | 项目地图 |
| **Beads** | 对话提取 → 分类存储 → 三层检索 → 自动精炼 | 跨会话不遗忘 | 工作笔记本 |

> 最后更新: 2026-06-25
