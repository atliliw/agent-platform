# Headroom / CodeGraph / Beads 底层原理

> 不讲类比，只讲机制。三个工具各自如何从工程层面解决 Agent 上下文瓶颈。

---

## 共同的问题空间

LLM Agent 的上下文窗口是有限资源。三个工具解决的是同一个问题的三个正交维度：

| 维度 | 症状 | 工具 |
|------|------|------|
| **容量** | 工具输出太大，塞不进窗口 | Headroom |
| **定位** | 不知道该读哪些文件，盲目搜索浪费 token | CodeGraph |
| **连续性** | 跨会话知识丢失，重复提问 | Beads |

三者可以独立使用，也可以组合——Headroom 压缩 CodeGraph 的查询结果，Beads 记住 Headroom 的压缩偏好。

---

## 1. Headroom — 上下文压缩引擎

### 1.1 核心问题

Agent 每次调用工具（grep、读文件、查 API）都会产生大量 token 输出。这些输出直接注入上下文窗口，导致：

1. **窗口溢出**：超出模型最大 token 限制，早期上下文被截断
2. **注意力稀释**：有效信息淹没在噪声中，模型推理质量下降
3. **KV 缓存失效**：上下文变化导致前缀不匹配，每次请求重新计算

### 1.2 架构

```
工具原始输出
    │
    ▼
┌──────────────────────────────────────────┐
│  ContentRouter                           │
│  检测内容类型 → 路由到对应压缩器          │
│  判断依据：MIME type / 结构特征 / 后缀   │
└──────────┬───────────────────────────────┘
           │
     ┌─────┼──────────┐
     ▼     ▼          ▼
 SmartCrusher  CodeCompressor  Kompress-base
 (JSON/结构化)  (源代码)        (纯文本/日志)
     │     │          │
     └─────┼──────────┘
           ▼
┌──────────────────────────────────────────┐
│  CacheAligner                            │
│  稳定前缀 → 最大化 KV 缓存命中率         │
└──────────┬───────────────────────────────┘
           ▼
┌──────────────────────────────────────────┐
│  CCR (Compressed Content Registry)       │
│  原文存本地 → 压缩版发给 LLM             │
│  Agent 需要原文时按 ID 取回              │
└──────────────────────────────────────────┘
```

### 1.3 SmartCrusher — JSON/结构化数据压缩

**原理**：结构化数据中，key 名是最大的冗余来源。

```
原始 JSON（每个对象重复相同的 key）：
[
  {"id": 1, "name": "Alice", "email": "alice@example.com", "role": "admin"},
  {"id": 2, "name": "Bob",   "email": "bob@example.com",   "role": "user"},
  {"id": 3, "name": "Carol", "email": "carol@example.com", "role": "user"}
]

压缩步骤：
1. 提取 schema（key 名只写一次）
2. 值用 | 分隔，每行一条记录
3. null/空值省略

压缩后：
id|name|email|role
1|Alice|alice@example.com|admin
2|Bob|bob@example.com|user
3|Carol|carol@example.com|user
```

**压缩率分析**：

| 场景 | 冗余来源 | 压缩率 |
|------|---------|--------|
| 100 条相同 schema 的记录 | key 名重复 100 次 | ~80% |
| 嵌套 3 层的 JSON | `{` `}` `"` `:` 等语法符号 | ~75% |
| API 分页响应 | status/message 等固定字段 | ~85% |

**关键实现细节**：
- Schema 提取时检测类型一致性：如果某字段在部分记录中缺失，用 `_` 占位
- 嵌套对象先展平（flatten），再按上述规则压缩
- 数组字段用逗号连接，不换行

### 1.4 CodeCompressor — 源代码压缩

**原理**：代码中大量 token 花在语法样板（boilerplate）上，真正传递语义的是函数签名和控制流。

```
原始代码：
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

压缩步骤：
1. AST 解析 → 提取函数签名
2. 识别 guard clause（提前返回的错误处理）
3. 提取核心逻辑行
4. 丢弃语法样板（括号、分号、类型声明的冗余部分）

压缩后：
UserService.GetUser(ctx, id→int64) → (*User, error)
  guard: id>0, err!=nil, user!=nil
  core: s.repo.FindByID(ctx, id)
```

**AST 解析流程**：

```
源代码字符串
    │
    ▼
词法分析 (Lexer)
    │  产生 token 流：func, (, s, *, UserService, ), GetUser, ...
    ▼
语法分析 (Parser)
    │  产生 AST：
    │  FuncDecl
    │  ├── Receiver: *UserService
    │  ├── Name: GetUser
    │  ├── Params: ctx(Context), id(int64)
    │  ├── Results: *User, error
    │  └── Body:
    │      ├── IfStmt (guard: id <= 0)
    │      ├── AssignStmt (user, err := s.repo.FindByID)
    │      ├── IfStmt (guard: err != nil)
    │      ├── IfStmt (guard: user == nil)
    │      └── ReturnStmt (return user, nil)
    ▼
语义提取
    │  从 AST 中提取：
    │  - 函数签名（接收者、名称、参数、返回值）
    │  - guard clauses（所有 if-return-nil 模式）
    │  - 核心逻辑（非 guard 的有效语句）
    │  - 调用关系（s.repo.FindByID 等）
    ▼
压缩输出
```

**guard clause 识别规则**：

```python
def is_guard_clause(node):
    """识别 guard clause 模式"""
    # 模式1: if condition { return nil, error }
    if node.type == "IfStmt" and node.body.has_early_return:
        return True
    # 模式2: if condition { return default_value }
    if node.type == "IfStmt" and node.body.has_early_return_with_default:
        return True
    # 模式3: if err != nil { return ..., err }
    if node.type == "IfStmt" and node.condition.is_nil_check("err"):
        return True
    return False
```

**不同语言的 AST 解析器**：

| 语言 | 解析器 | 说明 |
|------|--------|------|
| Go | `go/ast` | 原生标准库，最精确 |
| TypeScript | `tree-sitter-typescript` | 增量解析，支持 JSX |
| Python | `tree-sitter-python` | 处理缩进语法 |
| Rust | `tree-sitter-rust` | 处理宏和生命周期 |
| Java | `tree-sitter-java` | 处理注解和泛型 |

### 1.5 Kompress-base — 纯文本/日志压缩

**原理**：日志和纯文本的冗余来自重复模式和时间戳。

```
原始日志：
[2024-01-15 10:23:01] INFO  Starting server on port 8080
[2024-01-15 10:23:01] INFO  Connected to database
[2024-01-15 10:23:02] INFO  Migration complete
[2024-01-15 10:23:02] DEBUG Query: SELECT * FROM users WHERE id = 1
[2024-01-15 10:23:02] DEBUG Query: SELECT * FROM users WHERE id = 2
[2024-01-15 10:23:03] DEBUG Query: SELECT * FROM users WHERE id = 3
[2024-01-15 10:23:03] ERROR Connection timeout after 30s
[2024-01-15 10:23:03] ERROR Retry connection failed

压缩步骤：
1. 时间戳压缩：[2024-01-15 10:23:01] → [10:23:01]，连续时间合并为范围
2. 日志级别过滤：DEBUG 级别默认省略（可配置保留）
3. 重复模式合并：3 条相同结构的 DEBUG → 1 条 + 计数
4. 保留所有 ERROR/WARN

压缩后：
[10:23:01-02] INFO server:8080, db:connected, migration:done
[10:23:02] DEBUG SELECT * FROM users WHERE id=? (×3)
[10:23:03] ERROR Connection timeout after 30s
[10:23:03] ERROR Retry connection failed
```

**重复模式检测算法**：

```
输入：N 条日志行
输出：合并后的日志行

1. 对每行提取模板：
   "[timestamp] LEVEL message" → 模板 = "LEVEL message_pattern"

2. 将常量部分替换为占位符：
   "SELECT * FROM users WHERE id = 1" → "SELECT * FROM users WHERE id = ?"

3. 相同模板的行合并：
   模板出现 K 次 → "模板 (×K)"

4. 保留阈值：
   - ERROR/WARN：始终保留
   - INFO：保留首次出现 + 合并计数
   - DEBUG：默认省略，除非配置保留
```

### 1.6 CacheAligner — KV 缓存对齐

**问题背景**：LLM 推理使用 KV 缓存加速。如果两次请求的前缀相同，已计算的部分可以复用，只计算新增部分。

```
没有 CacheAligner：
第1轮：[system][compressed_A]          → 计算全部 KV
第2轮：[system][compressed_B][new_msg] → compressed_B ≠ compressed_A
                                   → 前缀不匹配，KV 缓存全 miss
                                   → 重新计算全部 KV

有 CacheAligner：
第1轮：[system][stable_prefix][compressed_A] → 计算全部 KV
第2轮：[system][stable_prefix][compressed_B][new_msg]
                                   → stable_prefix 命中 KV 缓存
                                   → 只计算 compressed_B + new_msg
```

**实现机制**：

```
1. 稳定前缀提取：
   - 系统提示词（不变）
   - 工具定义（不变）
   - 对话历史摘要（缓慢变化）

2. 压缩输出排序：
   - 将压缩结果按"变化频率"排序
   - 不变的部分放前面
   - 频繁变化的部分放后面

3. 前缀哈希校验：
   - 每次压缩后计算前缀的哈希
   - 与上次比较，确认 KV 缓存可复用
   - 如果不匹配，触发重新对齐
```

### 1.7 CCR — 压缩内容注册表

**问题**：压缩后 Agent 可能需要查看原文的某个部分。

```
工作流程：

1. 工具输出 65,000 tokens
2. Headroom 压缩到 8,000 tokens
3. 原文写入本地文件：~/.headroom/cache/{hash}
4. 注册表记录：
   {
     "id": "ccr_abc123",
     "original_tokens": 65000,
     "compressed_tokens": 8000,
     "compression_ratio": 0.123,
     "content_type": "log",
     "file_path": "~/.headroom/cache/abc123",
     "line_map": {           // 压缩行 → 原文行的映射
       "1": "1-5",
       "2": "6-12",
       "3": "13-50"          // 合并的重复行
     }
   }
5. 压缩版发给 LLM，附带 CCR ID
6. Agent 需要原文 → "展开 ccr_abc123 行 13-50"
7. 从本地文件取回，注入上下文
```

**line_map 的作用**：压缩时多行原文可能合并为一行，line_map 记录这个映射关系，让 Agent 可以精确取回原文的特定行范围，而不是取回整个文件。

### 1.8 压缩率实测

| 内容类型 | 原始 tokens | 压缩后 | 节省 | 主要冗余来源 |
|----------|-----------|--------|------|-------------|
| JSON API 响应 | 17,000 | 2,800 | 84% | key 名重复 |
| Go 源代码 | 12,000 | 2,400 | 80% | 语法样板 + guard |
| 应用日志 | 65,000 | 9,100 | 86% | 重复模式 + 时间戳 |
| RAG 检索结果 | 8,000 | 2,400 | 70% | 文档格式标记 |
| GitHub Issue | 54,000 | 9,700 | 82% | HTML/Markdown 标记 |

---

## 2. CodeGraph — 代码知识图谱

### 2.1 核心问题

大项目中 Agent 需要理解代码结构，但只能通过逐文件搜索来探索。这导致：

1. **搜索成本高**：grep 一个函数名可能返回几十个文件，逐个阅读消耗大量 token
2. **遗漏风险**：grep 只能匹配字符串，无法理解调用链、继承关系
3. **无法回答结构化问题**："谁调用了 GetUser？" "User 类型在哪里定义？" "改了这个接口会影响哪些实现？"

### 2.2 架构

```
代码库
  │
  ▼
┌──────────────────────────────────────────┐
│  AST 解析层                              │
│  每个文件 → 语法树                        │
│  支持增量解析（只重新解析变更文件）        │
└──────────┬───────────────────────────────┘
           │
           ▼
┌──────────────────────────────────────────┐
│  实体提取层                              │
│  从 AST 中提取：                         │
│  - 函数/方法（签名、文档、位置）          │
│  - 类/结构体（字段、方法、继承）          │
│  - 接口（方法签名）                      │
│  - 类型定义（字段、关联类型）            │
│  - 常量/变量（类型、值）                 │
│  - 包/模块（导出符号）                   │
└──────────┬───────────────────────────────┘
           │
           ▼
┌──────────────────────────────────────────┐
│  关系构建层                              │
│  实体之间的边：                          │
│  - calls / called_by                     │
│  - imports / imported_by                 │
│  - inherits / inherited_by               │
│  - implements / implemented_by           │
│  - references_type / type_referenced_by  │
└──────────┬───────────────────────────────┘
           │
           ▼
┌──────────────────────────────────────────┐
│  存储层                                  │
│  .codegraph/                             │
│  ├── index.json    # 图谱数据（节点+边） │
│  ├── symbols.json  # 符号表（快速查找）  │
│  ├── relations.json # 关系表（图遍历）   │
│  └── metadata.json # 元数据（语言/版本） │
└──────────────────────────────────────────┘
```

### 2.3 AST 解析详解

以 Go 为例，展示从源代码到实体提取的完整过程：

```go
// 源代码
package user

import "context"

type UserService struct {
    repo UserRepository
}

func (s *UserService) GetUser(ctx context.Context, id int64) (*User, error) {
    if id <= 0 {
        return nil, fmt.Errorf("invalid id: %d", id)
    }
    return s.repo.FindByID(ctx, id)
}
```

```
AST 解析过程：

1. go/parser.ParseFile() → *ast.File

2. 遍历 ast.File.Decls：
   - *ast.GenDecl (type declaration)
     └── *ast.TypeSpec
         ├── Name: "UserService"
         ├── Type: *ast.StructType
         │   └── Fields: [{Names: ["repo"], Type: *ast.Ident("UserRepository")}]
         └── 提取实体：
             {
               "name": "UserService",
               "type": "struct",
               "file": "service.go",
               "line": 5,
               "fields": [{"name": "repo", "type": "UserRepository"}],
               "methods": []  // 后续扫描方法时填充
             }

   - *ast.FuncDecl (function declaration)
       ├── Recv: *ast.FieldList → *ast.StarExpr("UserService")
       ├── Name: "GetUser"
       ├── Params: [ctx context.Context, id int64]
       ├── Results: [*User, error]
       └── Body: ... (扫描调用关系)
           提取实体：
           {
             "name": "GetUser",
             "type": "method",
             "receiver": "*UserService",
             "file": "service.go",
             "line": 9,
             "signature": "func (s *UserService) GetUser(ctx context.Context, id int64) (*User, error)",
             "calls": ["s.repo.FindByID", "fmt.Errorf"],
             "references_type": ["User", "error", "context.Context"]
           }
```

### 2.4 关系构建算法

```
输入：所有文件的实体列表
输出：关系图（邻接表）

1. 构建符号表：
   symbol_table = {}
   for entity in all_entities:
       symbol_table[entity.name] = entity

2. 解析 calls 关系：
   for entity in all_entities:
       for call in entity.calls:
           target = resolve_call(call, symbol_table, entity.imports)
           if target:
               add_edge(entity, target, "calls")
               add_edge(target, entity, "called_by")

3. 解析 inherits 关系：
   for entity where entity.type == "class" or "struct":
       if entity.extends:
           parent = symbol_table[entity.extends]
           add_edge(entity, parent, "inherits")
           add_edge(parent, entity, "inherited_by")

4. 解析 implements 关系：
   for entity where entity.type == "class" or "struct":
       for iface in entity.implements_list:
           interface = symbol_table[iface]
           add_edge(entity, interface, "implements")
           add_edge(interface, entity, "implemented_by")

5. 解析 imports 关系：
   for entity in all_entities:
       for imp in entity.file_imports:
           add_edge(entity.file, imp, "imports")

6. 解析 references_type 关系：
   for entity in all_entities:
       for type_ref in entity.references_type:
           type_def = symbol_table[type_ref]
           if type_def:
               add_edge(entity, type_def, "references_type")
```

**resolve_call 的难点**：

```
调用表达式：s.repo.FindByID(ctx, id)

解析步骤：
1. s → 当前函数的接收者 *UserService
2. s.repo → UserService.repo 字段，类型 UserRepository
3. FindByID → UserRepository 上的方法
4. 最终定位：UserRepository.FindByID

跨包调用：
fmt.Errorf → import "fmt" → 标准库 fmt.Errorf

未解析的调用：
helper() → 当前包内查找 → 找不到 → 标记为 unresolved
```

### 2.5 存储格式

**index.json** — 图谱核心数据：

```json
{
  "version": "1.0",
  "language": "go",
  "root": "/path/to/project",
  "generated_at": "2024-01-15T10:23:00Z",
  "stats": {
    "files": 45,
    "entities": 312,
    "relations": 891
  }
}
```

**symbols.json** — 符号表（按名称索引）：

```json
{
  "GetUser": [
    {
      "id": "ent_001",
      "type": "method",
      "receiver": "*UserService",
      "file": "services/user/service.go",
      "line": 9,
      "signature": "func (s *UserService) GetUser(ctx context.Context, id int64) (*User, error)"
    }
  ],
  "User": [
    {
      "id": "ent_045",
      "type": "struct",
      "file": "models/user.go",
      "line": 3
    }
  ]
}
```

**relations.json** — 关系表（邻接表）：

```json
{
  "ent_001": {
    "calls": ["ent_023", "ent_089"],
    "called_by": ["ent_056", "ent_078"],
    "references_type": ["ent_045", "ent_090"]
  },
  "ent_045": {
    "referenced_by": ["ent_001", "ent_056", "ent_078"]
  }
}
```

### 2.6 增量更新机制

```
文件变更检测：

1. Git hook (post-commit / post-checkout)
   或
   File watcher (inotify / FSEvents / ReadDirectoryChangesW)

2. 变更文件列表：
   changed_files = git diff --name-only HEAD~1

3. 增量更新：
   for file in changed_files:
       old_entities = get_entities_for_file(file)
       new_entities = parse_and_extract(file)

       # 删除旧实体及其关系
       for entity in old_entities:
           remove_entity(entity.id)
           remove_relations(entity.id)

       # 添加新实体及其关系
       for entity in new_entities:
           add_entity(entity)
           add_relations(entity)

4. 写入 .codegraph/ 目录
```

**增量更新的关键**：只重新解析变更的文件，不影响其他文件的索引。关系重建时，只更新与变更文件相关的边。

### 2.7 Agent 查询接口

```
Agent 发出查询 → CodeGraph 返回结果

查询类型：

1. 符号查找：
   query: "GetUser"
   result: {file, line, signature, type}

2. 调用链查询：
   query: "callers of GetUser"
   result: [GetUserHandler, ValidateToken]

3. 影响分析：
   query: "impact of changing UserRepository.FindByID"
   result: [UserService.GetUser, AdminService.ListUsers, ...]

4. 类型查找：
   query: "implementations of UserRepository"
   result: [MongoUserRepo, PostgresUserRepo, MockUserRepo]

5. 路径查询：
   query: "path from Handler to Database"
   result: Handler → Service → Repository → DB
```

**token 节省对比**：

```
没有 CodeGraph：
  grep "GetUser" → 15 个文件匹配
  逐个读取 → 15 × 800 tokens = 12,000 tokens
  还可能遗漏间接调用

有 CodeGraph：
  查询 "GetUser" → 1 个结果 + 关系
  → 200 tokens，精确完整
```

---

## 3. Beads — Agent 持久记忆系统

### 3.1 核心问题

LLM 是无状态的——每次对话从零开始。这导致：

1. **重复提问**：用户说过"用 JWT"，下次对话 Agent 又问
2. **重复探索**：Agent 上次已经发现项目结构，下次又从头 grep
3. **决策丢失**：上次讨论的架构决策，下次对话完全不知道

### 3.2 架构

```
对话内容
    │
    ▼
┌──────────────────────────────────────────┐
│  提取层 (Extraction)                     │
│  从对话中识别值得记住的信息              │
│  - 用户明确表达的偏好                    │
│  - Agent 发现的项目事实                  │
│  - 讨论中达成的决策                      │
│  - 反复出现的模式                        │
└──────────┬───────────────────────────────┘
           │
           ▼
┌──────────────────────────────────────────┐
│  分类层 (Classification)                 │
│  preference — 用户偏好                   │
│  fact       — 项目事实                   │
│  decision   — 架构/技术决策              │
│  pattern    — 代码/工作模式              │
│  context    — 当前任务上下文             │
└──────────┬───────────────────────────────┘
           │
           ▼
┌──────────────────────────────────────────┐
│  存储层 (Storage)                        │
│  三重存储：                              │
│  1. Markdown 文件 — 人类可读/可编辑/Git  │
│  2. SQLite FTS5   — 关键词全文搜索       │
│  3. 向量嵌入      — 语义搜索             │
└──────────┬───────────────────────────────┘
           │
           ▼
┌──────────────────────────────────────────┐
│  精炼层 (Refinement)                     │
│  定期维护记忆质量：                      │
│  - 合并重复                              │
│  - 标记过时                              │
│  - 权重衰减                              │
│  - 压缩摘要                              │
└──────────────────────────────────────────┘
```

### 3.3 提取层 — 如何从对话中识别记忆

**提取触发条件**：

```
1. 用户明确表达偏好：
   "我喜欢用 JWT" → preference: auth=JWT
   "不要用 ORM"   → preference: no_ORM

2. Agent 发现项目事实：
   [读取 go.mod] → fact: project_uses=Go_1.21+gRPC
   [读取目录结构] → fact: architecture=microservices

3. 讨论中达成决策：
   "我们决定用 Redis 做缓存" → decision: cache=Redis
   "API 用 REST 而不是 gRPC" → decision: api_style=REST

4. 反复出现的模式：
   Agent 连续 3 次使用某个工具组合 → pattern: debug_flow=grep→read→edit
```

**提取算法**：

```
输入：对话历史
输出：候选记忆列表

1. 对话分段：
   将对话按主题分段（每次用户提问视为新主题开始）

2. 每段提取：
   for segment in conversation_segments:
       candidates = llm_extract(segment, prompt="""
           从这段对话中提取值得跨会话记住的信息。
           类型：preference / fact / decision / pattern / context
           格式：{type, content, confidence, source_turn}
       """)

3. 置信度过滤：
   只保留 confidence > 0.7 的候选

4. 去重：
   与已有记忆比较，如果语义相似度 > 0.9，合并而非新增
```

### 3.4 存储层 — 三重索引

#### Markdown 文件存储

```markdown
---
id: mem_001
type: preference
confidence: 0.95
created: 2024-01-15
last_used: 2024-01-20
use_count: 5
weight: 0.9
---

# 认证方式

偏好 JWT，不用 Session
```

**为什么用 Markdown**：
- 人类可以直接编辑（修正 Agent 的错误记忆）
- 可以 Git 版本控制（追踪记忆变化历史）
- 不需要额外工具就能查看

#### SQLite FTS5 全文索引

```sql
CREATE VIRTUAL TABLE memory_fts USING fts5(
    id,
    type,
    content,
    tokenize='unicode61'  -- 支持中文分词
);

-- 插入
INSERT INTO memory_fts VALUES('mem_001', 'preference', '认证方式 偏好 JWT 不用 Session');

-- 查询
SELECT id, rank FROM memory_fts WHERE memory_fts MATCH 'JWT 认证'
ORDER BY rank LIMIT 5;
```

**FTS5 的优势**：
- 毫秒级响应
- 支持布尔查询（AND/OR/NOT）
- 支持短语查询
- 内置 BM25 排名

#### 向量嵌入索引

```
记忆内容 → Embedding 模型 → 向量（1536维 / 768维）

存储：
{
  "id": "mem_001",
  "embedding": [0.012, -0.034, 0.056, ...],  // 768 或 1536 维
  "metadata": {"type": "preference", "weight": 0.9}
}

查询：
query = "认证相关"
query_embedding = embed("认证相关")

results = vector_search(
    query_embedding,
    top_k=5,
    threshold=0.7  // 余弦相似度阈值
)
```

**向量搜索的语义能力**：

```
关键词搜索（FTS5）：
  "认证" → 只匹配包含"认证"的记忆
  "auth" → 只匹配包含"auth"的记忆
  "认证" ≠ "auth" ≠ "登录验证"

语义搜索（向量）：
  "认证" ≈ "auth" ≈ "authentication" ≈ "登录验证"
  因为这些词在向量空间中距离很近
```

### 3.5 三层检索流程

```
Agent 请求："认证相关的记忆"

第1层：关键词搜索（FTS5）
  ├── 查询："认证" OR "JWT" OR "auth"
  ├── 耗时：< 1ms
  ├── 结果：精确匹配的记忆 ID 列表
  └── 局限：只能匹配字面量

第2层：语义搜索（向量）
  ├── 查询：embed("认证相关")
  ├── 耗时：~50ms（含 embedding 计算）
  ├── 结果：语义相关的记忆 ID 列表
  └── 优势：能匹配"auth"、"登录验证"等同义表达

第3层：关联扩展
  ├── 从第1、2层结果出发
  ├── 沿关联边扩展
  │   "认证" → 关联到 "JWT" → 关联到 "token过期策略"
  │   "认证" → 关联到 "用户偏好" → 关联到 "不用Session"
  └── 结果：扩展后的记忆集合

合并与排序：
  final_results = merge_and_dedup(layer1, layer2, layer3)
  sort by: weight × recency × relevance_score
  limit to: token_budget（不超过上下文预算）
```

**关联图的构建**：

```
记忆之间的关联边：

1. 共现关联：
   如果两条记忆经常在同一对话中被使用 → 关联强度 +1

2. 语义关联：
   两条记忆的向量余弦相似度 > 0.7 → 建立关联

3. 类型关联：
   同一 type 的记忆之间有弱关联

4. 时间关联：
   同一对话中提取的记忆之间有时间关联

关联强度衰减：
   每次精炼时，关联强度 × 0.95
   如果衰减到 < 0.3，删除关联
```

### 3.6 精炼层 — 记忆维护

**为什么需要精炼**：记忆无限增长会导致：
- 注入太多无关记忆，浪费 token
- 搜索变慢
- 过时记忆误导 Agent

**精炼操作**：

```
1. 合并重复：
   mem_001: "用户喜欢JWT"
   mem_045: "偏好JWT认证"
   → 合并为 mem_001: "偏好 JWT 认证（不用 Session）"
   → 删除 mem_045

   合并条件：语义相似度 > 0.9 且 type 相同

2. 过时标记：
   mem_012: "项目用 React"
   mem_078: "前端迁移到 Vue 3"
   → mem_012.status = "superseded"
   → mem_012.superseded_by = "mem_078"
   → mem_078.content = "前端用 Vue 3（从 React 迁移）"

   检测方式：新记忆与旧记忆矛盾时，标记旧记忆为过时

3. 权重衰减：
   weight = initial_weight × decay_factor^(days_since_last_use)

   decay_factor = 0.98  // 每天衰减 2%

   30 天没用：weight × 0.98^30 ≈ 0.545
   90 天没用：weight × 0.98^90 ≈ 0.163
   180 天没用：weight × 0.98^180 ≈ 0.027 → 归档

4. 压缩摘要：
   10 条关于前端框架的记忆 → 1 条摘要

   原始：
   - "用 Vue 3"
   - "用 Composition API"
   - "用 Pinia 状态管理"
   - "用 Vue Router 4"
   - "用 Vite 构建"
   - ...

   摘要：
   "前端：Vue 3 + Composition API + Pinia + Vue Router 4 + Vite"

   触发条件：同一 category 下记忆数 > 8
```

### 3.7 记忆注入策略

```
新对话开始
    │
    ▼
1. 提取用户意图
   user_query → intent_keywords

2. 检索相关记忆
   三层检索 → candidate_memories

3. 预算控制
   token_budget = context_window × 0.1  // 最多占 10%
   selected = []
   for mem in sorted(candidates, by=weight×relevance):
       if total_tokens(selected) + tokens(mem) > token_budget:
           break
       selected.append(mem)

4. 格式化注入
   注入到系统提示词中：
   """
   <memory>
   [preference] 认证方式：JWT，不用 Session
   [fact] 技术栈：Go + gRPC + MongoDB + Redis
   [decision] 缓存策略：Redis，TTL 30min
   </memory>
   """

5. 更新使用统计
   for mem in selected:
       mem.last_used = now
       mem.use_count += 1
```

### 3.8 跨 Agent 共享

```
Beads 的记忆存储是文件系统上的 Markdown + SQLite + 向量索引

共享机制：
1. 同一项目目录 → 所有 Agent 共享 .beads/ 目录
2. 文件锁（flock / LockFileEx）→ 防止并发写入冲突
3. 写入时广播（可选）→ 通知其他 Agent 刷新缓存

Claude Code 的对话 → 提取记忆 → 写入 .beads/
Codex 的对话       → 提取记忆 → 写入 .beads/  ← 同一目录
Cursor 的对话      → 提取记忆 → 写入 .beads/  ← 同一目录

读取时：
任何 Agent → 查询 .beads/ → 获得所有 Agent 积累的记忆
```

---

## 4. 三者协同

```
Agent 开始新对话
    │
    ├─ Beads 注入相关记忆
    │  "这个项目用 Go+gRPC，用户偏好 JWT"
    │  → Agent 不需要重新探索项目结构
    │
    ├─ CodeGraph 提供代码索引
    │  "GetUser 在 service.go:45，被 3 个地方调用"
    │  → Agent 不需要 grep 搜索
    │
    │  ... Agent 工作中 ...
    │
    ├─ Headroom 压缩工具输出
    │  grep 返回 17,000 tokens → 压缩到 2,800 tokens
    │  → 上下文窗口不被撑爆
    │
    └─ Beads 保存新记忆
       "用户决定用 Redis 做 token 黑名单"
       → 下次对话自动知道
```

**数据流**：

```
CodeGraph 查询结果 → Headroom 压缩 → 注入上下文
Beads 检索结果    → Headroom 压缩 → 注入上下文
工具输出          → Headroom 压缩 → 注入上下文
对话结束          → Beads 提取    → 持久化记忆
代码变更          → CodeGraph 增量更新 → 索引刷新
```

---

## 5. 实现参考

| 工具 | 核心依赖 | 语言 | 开源参考 |
|------|---------|------|---------|
| Headroom | tree-sitter, tiktoken | Go/Rust | Aider 的 repo-map, Sourcegraph 的 scip-code |
| CodeGraph | go/ast, tree-sitter, bluge/bleve | Go | Sourcegraph Code Intelligence, LSIF |
| Beads | SQLite FTS5, sentence-transformers | Go/Python | MemGPT, LangChain Memory, Zep |

> 最后更新: 2026-06-26
