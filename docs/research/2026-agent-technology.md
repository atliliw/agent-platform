# 2026年 AI Agent 技术全景

> 最新架构、最佳实践与核心突破

---

## 目录

- [一、核心概念](#一核心概念)
- [二、Deep Agents 架构](#二deep-agents-架构)
- [三、循环工程](#三循环工程)
- [四、Agent Harness 设计](#四agent-harness-设计)
- [五、容错机制](#五容错机制)
- [六、沙盒隔离](#六沙盒隔离)
- [七、自我评估与纠错](#七自我评估与纠错)
- [八、记忆系统](#八记忆系统)
- [九、成本控制](#九成本控制)
- [十、技术栈与工具](#十技术栈与工具)
- [十一、最佳实践](#十一最佳实践)

---

## 一、核心概念

### 1.1 Agent 定义

```
Agent = Model + Harness

Model: 推理引擎（Claude、GPT等）
Harness: 连接模型与真实世界的脚手架
```

### 1.2 Deep Agents vs 浅层 Agents

| 维度 | 浅层 Agent | Deep Agent |
|------|------------|------------|
| **执行时间** | 分钟级 | 小时/天级 |
| **任务复杂度** | 单一任务 | 复杂多步骤 |
| **规划能力** | 简单规划 | 深度规划 |
| **记忆** | 会话内 | 跨会话 |
| **子代理** | 无 | 支持委托 |
| **文件系统** | 无 | 持久化存储 |

### 1.3 核心价值主张

```
Deep Agents 能够:
1. 深入探索主题
2. 规划复杂任务
3. 长时间范围内执行
4. 自动纠错和改进
```

---

## 二、Deep Agents 架构

### 2.1 四大关键特征

```
┌─────────────────────────────────────────────────────────┐
│                    Deep Agent 架构                       │
├─────────────────────────────────────────────────────────┤
│                                                          │
│  ┌─────────────────────────────────────────────────┐    │
│  │           1. 详细系统提示词                       │    │
│  │    • 工具使用说明                                 │    │
│  │    • 行为示例                                     │    │
│  │    • 任务特定指导                                 │    │
│  └─────────────────────────────────────────────────┘    │
│                                                          │
│  ┌─────────────────────────────────────────────────┐    │
│  │           2. 规划工具                             │    │
│  │    • Todo list 工具                              │    │
│  │    • 保持 Agent 正轨                              │    │
│  │    • 上下文工程策略                               │    │
│  └─────────────────────────────────────────────────┘    │
│                                                          │
│  ┌─────────────────────────────────────────────────┐    │
│  │           3. 子代理                               │    │
│  │    • 任务拆分                                     │    │
│  │    • 专注特定任务                                 │    │
│  │    • 上下文管理                                   │    │
│  └─────────────────────────────────────────────────┘    │
│                                                          │
│  ┌─────────────────────────────────────────────────┐    │
│  │           4. 文件系统                             │    │
│  │    • 存储笔记                                     │    │
│  │    • 管理工作上下文                               │    │
│  │    • 共享工作空间                                 │    │
│  └─────────────────────────────────────────────────┘    │
│                                                          │
└─────────────────────────────────────────────────────────┘
```

### 2.2 开源实现

```bash
# 安装
pip install deepagents

# 基础用法
from deepagents import DeepAgent

agent = DeepAgent(
    model="claude-sonnet-4-6",
    tools=[...],
    filesystem="./workspace"
)

result = agent.run("复杂任务描述")
```

---

## 三、循环工程

### 3.1 四层循环架构

```
┌─────────────────────────────────────────────────────────┐
│                                                          │
│   ┌─────────────────────────────────────────────────┐   │
│   │              Loop 4: 爬山循环                     │   │
│   │                                                  │   │
│   │   分析 traces → 改进 harness 配置                │   │
│   │   自动化改进的关键                               │   │
│   │                                                  │   │
│   └─────────────────────────────────────────────────┘   │
│                         ▲                               │
│                         │                               │
│   ┌─────────────────────────────────────────────────┐   │
│   │              Loop 3: 事件驱动循环                 │   │
│   │                                                  │   │
│   │   连接生态系统                                   │   │
│   │   cron schedules, webhooks, channels             │   │
│   │                                                  │   │
│   └─────────────────────────────────────────────────┘   │
│                         ▲                               │
│                         │                               │
│   ┌─────────────────────────────────────────────────┐   │
│   │              Loop 2: 验证循环                     │   │
│   │                                                  │   │
│   │   grader 检查输出是否符合 rubric                  │   │
│   │   失败 → 返回反馈 → 重试                          │   │
│   │                                                  │   │
│   └─────────────────────────────────────────────────┘   │
│                         ▲                               │
│                         │                               │
│   ┌─────────────────────────────────────────────────┐   │
│   │              Loop 1: Agent 循环                   │   │
│   │                                                  │   │
│   │   模型在循环中调用工具直到任务完成                │   │
│   │   基础执行层                                     │   │
│   │                                                  │   │
│   └─────────────────────────────────────────────────┘   │
│                                                          │
└─────────────────────────────────────────────────────────┘
```

### 3.2 各层详解

#### Loop 1: Agent 循环

```python
# 最基础的循环
# 模型调用工具 → 观察结果 → 继续调用 → 直到完成

while not task_complete:
    response = model.chat(messages, tools=tools)
    
    if response.tool_calls:
        for tool_call in response.tool_calls:
            result = execute_tool(tool_call)
            messages.append(result)
    else:
        return response.content
```

#### Loop 2: 验证循环

```python
# 添加输出验证
# 不符合标准则重试

max_retries = 3

for attempt in range(max_retries):
    output = agent.run(task)
    
    # 验证输出
    grade = grader.evaluate(output, rubric)
    
    if grade.passed:
        return output
    
    # 失败，添加反馈重试
    feedback = grade.feedback
    task = f"{task}\n\n之前的尝试问题: {feedback}"

raise ValidationError("无法通过验证")
```

#### Loop 3: 事件驱动循环

```python
# 连接外部系统
# 支持定时触发和 webhook

from langsmith import Deployment

deployment = Deployment(
    agent=my_agent,
    triggers=[
        {"type": "cron", "schedule": "0 9 * * *"},  # 每天9点
        {"type": "webhook", "path": "/trigger"},
    ]
)

deployment.start()
```

#### Loop 4: 爬山循环

```python
# 自动化改进
# 分析运行 traces 优化配置

from langsmith import Engine

engine = Engine(
    agent=my_agent,
    improvement_cycle={
        "analyze_traces": True,
        "suggest_improvements": True,
        "auto_apply": False,  # 需要人工审批
    }
)

# 引擎会自动分析并建议改进
# 如: 调整 prompt、添加工具、修改 grader
```

### 3.3 人类监督点

```
每个循环层次都可加入人类监督:

Loop 1: 敏感操作前确认
Loop 2: 人工 grader
Loop 3: 输出审批
Loop 4: harness 改进审批
```

---

## 四、Agent Harness 设计

### 4.1 Harness 定义

```
Agent = Model + Harness

Harness 是连接模型与真实世界的脚手架
职责: 在每一步为模型提供正确的上下文
```

### 4.2 Middleware 中间件机制

```python
from langchain.agents import create_agent

# 基础用法
agent = create_agent(
    model="anthropic:claude-sonnet-4-6",
    tools=tools,
    system_prompt="你是一个智能助手..."
)

# 自定义 Harness
agent = create_agent(
    model="anthropic:claude-sonnet-4-6",
    tools=tools,
    middleware=[
        # 上下文管理
        SummarizationMiddleware(max_tokens=4000),
        ContextEditingMiddleware(),
        
        # 记忆系统
        FilesystemMiddleware(root="./workspace"),
        MemoryMiddleware(db=vector_db),
        
        # 任务执行
        ShellToolMiddleware(allowed_commands=["git", "npm"]),
        CodeInterpreterMiddleware(),
        
        # 子代理
        SubAgentMiddleware(),
        TodoListMiddleware(),
        
        # 错误处理
        ToolRetryMiddleware(max_retries=3),
        ModelFallbackMiddleware(fallback_model="claude-haiku-4-5"),
        
        # 策略执行
        PIIMiddleware(action="redact"),
        HumanInTheLoopMiddleware(approval_required=["delete", "send"]),
        
        # 成本控制
        ModelCallLimitMiddleware(daily_limit=1000),
        PromptCachingMiddleware(),
    ]
)
```

### 4.3 四大能力杠杆

| 杠杆 | 说明 | 示例 Middleware |
|------|------|-----------------|
| **确定性逻辑** | 业务逻辑、策略执行 | `PIIMiddleware`, `HumanInTheLoopMiddleware` |
| **Tools 管理** | 工具生命周期管理 | `ShellToolMiddleware`, `CodeInterpreterMiddleware` |
| **自定义状态** | 在 hook 间追踪状态 | `TodoListMiddleware`, `MemoryMiddleware` |
| **流处理器** | 拦截和转换输出流 | `StreamFilterMiddleware`, `OutputFormatterMiddleware` |

### 4.4 Middleware 生命周期

```python
class CustomMiddleware:
    """自定义中间件示例"""
    
    def on_start(self, state):
        """Agent 启动时调用"""
        pass
    
    def before_model(self, messages, state):
        """模型调用前"""
        return messages
    
    def after_model(self, response, state):
        """模型调用后"""
        return response
    
    def before_tool(self, tool_call, state):
        """工具执行前"""
        return tool_call
    
    def after_tool(self, result, state):
        """工具执行后"""
        return result
    
    def on_end(self, output, state):
        """Agent 结束时"""
        return output
    
    def on_error(self, error, state):
        """错误处理"""
        pass
```

### 4.5 Task-Harness Fit

```
不同任务需要不同的 Harness 配置:

简单查询:
  - 基础 Agent 循环
  - 少量工具

数据分析:
  - CodeInterpreterMiddleware
  - FilesystemMiddleware
  - MemoryMiddleware

复杂工作流:
  - SubAgentMiddleware
  - TodoListMiddleware
  - HumanInTheLoopMiddleware

企业应用:
  - PIIMiddleware
  - ModelCallLimitMiddleware
  - 审计日志 Middleware
```

---

## 五、容错机制

### 5.1 三大支柱

```
┌─────────────────────────────────────────────────────────┐
│                    容错三大支柱                          │
├─────────────────────────────────────────────────────────┤
│                                                          │
│  ┌───────────────┐  ┌───────────────┐  ┌─────────────┐  │
│  │   Retries     │  │   Timeouts    │  │   Error     │  │
│  │   重试机制    │  │   超时控制    │  │   Handlers  │  │
│  │               │  │               │  │   错误处理  │  │
│  └───────────────┘  └───────────────┘  └─────────────┘  │
│                                                          │
│  • 瞬态故障重试     • 防止无限等待     • 分类错误类型   │
│  • 指数退避策略     • 模型调用超时     • 降级策略       │
│  • 最大重试次数     • 工具执行超时     • 人工介入       │
│                                                          │
└─────────────────────────────────────────────────────────┘
```

### 5.2 重试机制

```python
class RetryMiddleware:
    """重试中间件"""
    
    def __init__(self, max_retries=3, base_delay=1.0):
        self.max_retries = max_retries
        self.base_delay = base_delay
    
    def after_tool(self, result, state):
        if result.error and result.retry_count < self.max_retries:
            # 指数退避
            delay = self.base_delay * (2 ** result.retry_count)
            time.sleep(delay)
            
            # 标记重试
            result.retry_count += 1
            result.should_retry = True
        
        return result
```

### 5.3 超时控制

```python
class TimeoutMiddleware:
    """超时控制中间件"""
    
    def __init__(
        self,
        model_timeout=60,      # 模型调用超时（秒）
        tool_timeout=300,      # 工具执行超时（秒）
        total_timeout=3600     # 总任务超时（秒）
    ):
        self.model_timeout = model_timeout
        self.tool_timeout = tool_timeout
        self.total_timeout = total_timeout
    
    def before_model(self, messages, state):
        state.model_start_time = time.time()
        return messages
    
    def after_model(self, response, state):
        elapsed = time.time() - state.model_start_time
        if elapsed > self.model_timeout:
            raise TimeoutError(f"模型调用超时: {elapsed}s > {self.model_timeout}s")
        return response
```

### 5.4 错误处理器

```python
class ErrorHandlerMiddleware:
    """错误处理中间件"""
    
    def on_error(self, error, state):
        error_type = self.classify_error(error)
        
        handler = {
            "timeout": self.handle_timeout,
            "rate_limit": self.handle_rate_limit,
            "tool_error": self.handle_tool_error,
            "model_error": self.handle_model_error,
            "unknown": self.handle_unknown,
        }.get(error_type, self.handle_unknown)
        
        return handler(error, state)
    
    def classify_error(self, error):
        """错误分类"""
        if isinstance(error, TimeoutError):
            return "timeout"
        elif "rate limit" in str(error).lower():
            return "rate_limit"
        elif "tool" in str(error).lower():
            return "tool_error"
        elif "model" in str(error).lower():
            return "model_error"
        return "unknown"
    
    def handle_timeout(self, error, state):
        return {"action": "retry", "delay": 60}
    
    def handle_rate_limit(self, error, state):
        return {"action": "wait", "delay": 300}
    
    def handle_tool_error(self, error, state):
        return {"action": "try_alternative"}
    
    def handle_model_error(self, error, state):
        return {"action": "fallback", "fallback_model": "claude-haiku-4-5"}
    
    def handle_unknown(self, error, state):
        return {"action": "escalate", "notify_human": True}
```

---

## 六、沙盒隔离

### 6.1 为什么需要沙盒

```
Agent 的"致命三要素" (Lethal Trifecta):

当以下三个条件同时满足时，攻击者可以窃取数据:

1. 访问敏感数据
   ↓
2. 接触不可信内容
   ↓
3. 能够外部通信
   ↓
   = 数据泄露风险！

沙盒隔离切断这个链条
```

### 6.2 沙盒核心功能

```
┌─────────────────────────────────────────────────────────┐
│                    沙盒核心功能                          │
├─────────────────────────────────────────────────────────┤
│                                                          │
│  1. 隔离的文件系统                                       │
│     • 仅包含 Agent 工作所需数据                          │
│     • 无法访问宿主机敏感文件                             │
│                                                          │
│  2. 受限的网络访问                                       │
│     • 仅允许访问指定的外部端点                           │
│     • 白名单机制                                         │
│                                                          │
│  3. 资源限制                                             │
│     • CPU 限制                                           │
│     • 内存限制                                           │
│     • 执行时间限制                                       │
│                                                          │
│  4. 可控的可复用性                                       │
│     • 决定是否跨执行重用沙盒                             │
│     • 每次执行后清理                                     │
│                                                          │
│  5. 内核级隔离                                           │
│     • 微虚拟机 (microVM)                                 │
│     • 防止 Agent 绕过控制                                │
│                                                          │
└─────────────────────────────────────────────────────────┘
```

### 6.3 LangSmith Sandboxes

```python
from langsmith import Sandbox

# 创建沙盒
sandbox = Sandbox(
    cpu_limit=2,           # CPU 核心
    memory_limit="4GB",    # 内存限制
    timeout=3600,          # 超时（秒）
    network_whitelist=[    # 网络白名单
        "api.anthropic.com",
        "api.openai.com",
    ],
    filesystem="./workspace",  # 隔离文件系统
)

# 在沙盒中执行
result = sandbox.run(agent, task)

# 清理
sandbox.destroy()
```

### 6.4 安全最佳实践

```
1. 最小权限原则
   - 只授予完成任务所需的最小权限
   - 网络访问仅限必要端点

2. 敏感数据隔离
   - 不在沙盒中存储密钥
   - 使用授权代理注入凭证

3. 审计日志
   - 记录所有操作
   - 监控异常行为

4. 定期清理
   - 任务完成后销毁沙盒
   - 不跨任务复用
```

---

## 七、自我评估与纠错

### 7.1 Rubric 评估机制

```python
from langchain.rubrics import Rubric, RubricMiddleware

# 定义评估标准
rubric = Rubric(
    criteria=[
        {"name": "准确性", "description": "答案是否正确", "weight": 0.4},
        {"name": "完整性", "description": "是否覆盖所有要点", "weight": 0.3},
        {"name": "格式", "description": "格式是否符合要求", "weight": 0.3},
    ],
    passing_score=0.7,
)

# 使用 RubricMiddleware
agent = create_agent(
    model="claude-sonnet-4-6",
    middleware=[
        RubricMiddleware(
            rubric=rubric,
            max_retries=3,
            grader_model="claude-sonnet-4-6",  # 评估模型
        )
    ]
)
```

### 7.2 评估流程

```
┌─────────────────────────────────────────────────────────┐
│                    评估流程                              │
├─────────────────────────────────────────────────────────┤
│                                                          │
│   Agent 执行任务                                         │
│        │                                                 │
│        ▼                                                 │
│   生成输出                                               │
│        │                                                 │
│        ▼                                                 │
│   ┌─────────────────┐                                   │
│   │   Grader 评估    │                                   │
│   └────────┬────────┘                                   │
│            │                                             │
│       ┌────┴────┐                                       │
│       │         │                                       │
│    通过      不通过                                      │
│       │         │                                       │
│       ▼         ▼                                       │
│   返回结果   生成反馈                                    │
│                │                                         │
│                ▼                                         │
│           Agent 重试                                     │
│                │                                         │
│                └──→ 返回评估步骤                         │
│                                                          │
└─────────────────────────────────────────────────────────┘
```

### 7.3 自定义 Grader

```python
class CustomGrader:
    """自定义评估器"""
    
    def evaluate(self, output, rubric):
        scores = {}
        
        for criterion in rubric.criteria:
            score = self._evaluate_criterion(output, criterion)
            scores[criterion["name"]] = score
        
        # 加权平均
        total_score = sum(
            scores[c["name"]] * c["weight"]
            for c in rubric.criteria
        )
        
        return {
            "passed": total_score >= rubric.passing_score,
            "score": total_score,
            "details": scores,
            "feedback": self._generate_feedback(scores, rubric),
        }
    
    def _evaluate_criterion(self, output, criterion):
        # 使用 LLM 评估
        prompt = f"""
        评估以下输出在"{criterion['name']}"维度的得分（0-1）。
        
        评估标准: {criterion['description']}
        输出: {output}
        
        只返回得分数字。
        """
        return float(self.llm.chat(prompt))
    
    def _generate_feedback(self, scores, rubric):
        # 生成改进反馈
        low_scores = [
            name for name, score in scores.items()
            if score < 0.7
        ]
        
        if not low_scores:
            return "所有维度表现良好"
        
        return f"以下维度需要改进: {', '.join(low_scores)}"
```

---

## 八、记忆系统

### 8.1 记忆类型

```
┌─────────────────────────────────────────────────────────┐
│                    Agent 记忆系统                        │
├─────────────────────────────────────────────────────────┤
│                                                          │
│  ┌─────────────────────────────────────────────────┐    │
│  │           工作记忆 (Working Memory)              │    │
│  │                                                  │    │
│  │  • 当前会话上下文                                │    │
│  │  • 最近的消息历史                                │    │
│  │  • 临时状态                                      │    │
│  │                                                  │    │
│  │  存储: 内存                                      │    │
│  │  容量: 受上下文窗口限制                           │    │
│  └─────────────────────────────────────────────────┘    │
│                                                          │
│  ┌─────────────────────────────────────────────────┐    │
│  │           短期记忆 (Short-term Memory)           │    │
│  │                                                  │    │
│  │  • 当前任务的中间结果                            │    │
│  │  • 任务进度                                      │    │
│  │                                                  │    │
│  │  存储: 文件系统 / 数据库                         │    │
│  │  生命周期: 任务期间                              │    │
│  └─────────────────────────────────────────────────┘    │
│                                                          │
│  ┌─────────────────────────────────────────────────┐    │
│  │           长期记忆 (Long-term Memory)            │    │
│  │                                                  │    │
│  │  • 用户偏好                                      │    │
│  │  • 历史交互                                      │    │
│  │  • 学到的知识                                    │    │
│  │                                                  │    │
│  │  存储: 向量数据库                                │    │
│  │  生命周期: 永久                                  │    │
│  └─────────────────────────────────────────────────┘    │
│                                                          │
└─────────────────────────────────────────────────────────┘
```

### 8.2 记忆中间件

```python
from langchain.memory import MemoryMiddleware

memory_middleware = MemoryMiddleware(
    # 工作记忆
    working_memory={
        "max_messages": 50,
    },
    
    # 短期记忆
    short_term={
        "storage": "filesystem",
        "path": "./workspace/memory",
    },
    
    # 长期记忆
    long_term={
        "storage": "vectordb",
        "embed_model": "text-embedding-3-small",
        "top_k": 5,
    }
)

agent = create_agent(
    model="claude-sonnet-4-6",
    middleware=[memory_middleware]
)
```

### 8.3 跨会话记忆

```python
class CrossSessionMemory:
    """跨会话记忆管理"""
    
    def __init__(self, db):
        self.db = db
    
    def save_session(self, session_id, state):
        """保存会话状态"""
        self.db.save(f"session:{session_id}", {
            "messages": state.messages,
            "progress": state.progress,
            "timestamp": datetime.now(),
        })
    
    def load_session(self, session_id):
        """恢复会话"""
        data = self.db.load(f"session:{session_id}")
        return AgentState.from_dict(data)
    
    def remember_fact(self, user_id, fact, importance=0.5):
        """记住重要事实"""
        embedding = self.embed(fact)
        self.db.vector_store.upsert(
            collection="facts",
            id=f"{user_id}:{hash(fact)}",
            vector=embedding,
            metadata={
                "user_id": user_id,
                "fact": fact,
                "importance": importance,
            }
        )
    
    def recall_facts(self, user_id, query, top_k=5):
        """回忆相关事实"""
        embedding = self.embed(query)
        return self.db.vector_store.search(
            collection="facts",
            vector=embedding,
            filter={"user_id": user_id},
            top_k=top_k,
        )
```

---

## 九、成本控制

### 9.1 LLM Gateway

```python
from langsmith import LLMGateway

gateway = LLMGateway(
    # 预算配置
    budgets={
        "organization": {"monthly": 100000},  # 组织月度预算
        "workspace:dev": {"weekly": 5000},     # 开发工作空间
        "workspace:prod": {"monthly": 50000},  # 生产工作空间
        "user:*": {"daily": 500},              # 每用户日预算
    },
    
    # 警告阈值
    alert_threshold=0.8,  # 80% 时警告
    
    # 硬性限制
    hard_limit=True,
)
```

### 9.2 成本优化策略

```python
# 1. Prompt Caching
agent = create_agent(
    model="claude-sonnet-4-6",
    middleware=[
        PromptCachingMiddleware(
            cache_ttl=3600,  # 1小时缓存
        )
    ]
)

# 2. 模型分层
agent = create_agent(
    model="claude-sonnet-4-6",  # 主模型
    middleware=[
        ModelFallbackMiddleware(
            fallback_chain=[
                "claude-haiku-4-5",  # 便宜模型
                "claude-sonnet-4-6",  # 回退
            ]
        )
    ]
)

# 3. 调用限制
agent = create_agent(
    middleware=[
        ModelCallLimitMiddleware(
            daily_limit=1000,
            per_task_limit=50,
        )
    ]
)
```

### 9.3 成本监控

```python
from langsmith import CostMonitor

monitor = CostMonitor(
    metrics=[
        "total_tokens",
        "input_tokens",
        "output_tokens",
        "cost",
    ],
    
    granularity=["per_request", "per_session", "per_day"],
    
    alerts=[
        {
            "condition": "daily_cost > 100",
            "action": "notify",
            "recipients": ["admin@company.com"]
        },
        {
            "condition": "daily_cost > 500",
            "action": "throttle",
            "rate": 0.5
        }
    ]
)
```

### 9.4 三大经验教训

```
1. 模型定价比静态表格更复杂
   - 需考虑缓存
   - Token 层级差异
   - 频繁的价格变动

2. 并非所有客户端都能通过 Gateway 路由
   - 需测量 Gateway 捕获与实际使用差额
   - 监控绕过 Gateway 的调用

3. 硬性限制需要配套工作流
   - 提前警告
   - 快速审批流程
   - 可审计的限额调整
```

---

## 十、技术栈与工具

### 10.1 完整技术栈

```
┌─────────────────────────────────────────────────────────┐
│                    Agent 技术栈                          │
├─────────────────────────────────────────────────────────┤
│                                                          │
│  ┌─────────────────────────────────────────────────┐    │
│  │              Deep Agents 工具套件                │    │
│  │                                                  │    │
│  │  • deepagents 包                                │    │
│  │  • 虚拟文件系统                                  │    │
│  │  • 子代理管理                                    │    │
│  └─────────────────────────────────────────────────┘    │
│                         │                               │
│                         ▼                               │
│  ┌─────────────────────────────────────────────────┐    │
│  │              LangGraph (编排运行时)              │    │
│  │                                                  │    │
│  │  • 持久化执行                                    │    │
│  │  • 人机协作                                      │    │
│  │  • 完整记忆                                      │    │
│  │  • 流式处理                                      │    │
│  │  • 状态机                                        │    │
│  └─────────────────────────────────────────────────┘    │
│                         │                               │
│                         ▼                               │
│  ┌─────────────────────────────────────────────────┐    │
│  │              LangChain (模型/工具集成)           │    │
│  │                                                  │    │
│  │  • 多模型支持                                    │    │
│  │  • 工具集成                                      │    │
│  │  • Middleware 机制                               │    │
│  └─────────────────────────────────────────────────┘    │
│                         │                               │
│                         ▼                               │
│  ┌─────────────────────────────────────────────────┐    │
│  │              LangSmith (可观测性平台)            │    │
│  │                                                  │    │
│  │  • LLM Gateway                                   │    │
│  │  • Sandboxes                                     │    │
│  │  • Engine (自动化改进)                           │    │
│  │  • 监控和调试                                    │    │
│  └─────────────────────────────────────────────────┘    │
│                                                          │
└─────────────────────────────────────────────────────────┘
```

### 10.2 MCP 协议

```
Model Context Protocol (MCP)

定位: "AI 应用的 USB-C 接口"

核心概念:
- 标准化的工具调用协议
- 一次开发，到处集成
- 支持 Claude、ChatGPT、VS Code、Cursor 等

架构:

  AI Client (Claude/ChatGPT)
          │
          │ MCP 协议
          │
    ┌─────┼─────┬─────────┐
    │     │     │         │
  数据源  工具  工作流    应用
```

### 10.3 安装和使用

```bash
# 安装核心包
pip install langchain
pip install langgraph
pip install deepagents

# MCP 支持
pip install mcp
```

```python
# 完整示例
from langchain.agents import create_agent
from langchain.memory import MemoryMiddleware
from langchain.rubrics import RubricMiddleware
from langchain.fault_tolerance import RetryMiddleware, TimeoutMiddleware

agent = create_agent(
    model="claude-sonnet-4-6",
    tools=[...],
    system_prompt="你是一个智能助手...",
    middleware=[
        # 记忆
        MemoryMiddleware(),
        
        # 容错
        RetryMiddleware(max_retries=3),
        TimeoutMiddleware(model_timeout=60, tool_timeout=300),
        
        # 评估
        RubricMiddleware(rubric=my_rubric),
    ]
)

# 运行
result = agent.run("复杂任务描述")
```

---

## 十一、最佳实践

### 11.1 架构设计原则

```
1. 模型中立
   - 不要绑定特定模型
   - 支持多模型切换
   - 适配不同能力等级

2. 渐进式复杂度
   - 从简单循环开始
   - 按需添加验证层
   - 逐步引入事件驱动

3. 容错优先
   - 假设一切都会失败
   - 设计重试和降级
   - 保存检查点

4. 可观测性
   - 记录所有操作
   - 监控关键指标
   - 支持调试和审计

5. 安全隔离
   - 使用沙盒
   - 最小权限原则
   - 敏感操作人工确认
```

### 11.2 常见模式

```python
# 模式 1: 简单 Agent
simple_agent = create_agent(
    model="claude-sonnet-4-6",
    tools=[search_tool, calculator_tool],
)

# 模式 2: 带 Retry 的 Agent
retry_agent = create_agent(
    model="claude-sonnet-4-6",
    tools=[...],
    middleware=[RetryMiddleware(max_retries=3)]
)

# 模式 3: 带验证的 Agent
verified_agent = create_agent(
    model="claude-sonnet-4-6",
    tools=[...],
    middleware=[
        RetryMiddleware(),
        RubricMiddleware(rubric=quality_rubric),
    ]
)

# 模式 4: 长期运行 Agent
long_running_agent = create_agent(
    model="claude-sonnet-4-6",
    tools=[...],
    middleware=[
        MemoryMiddleware(),
        FilesystemMiddleware(),
        RetryMiddleware(),
        TimeoutMiddleware(),
        HumanInTheLoopMiddleware(),
    ]
)

# 模式 5: 企业级 Agent
enterprise_agent = create_agent(
    model="claude-sonnet-4-6",
    tools=[...],
    middleware=[
        MemoryMiddleware(),
        PIIMiddleware(),
        ModelCallLimitMiddleware(),
        HumanInTheLoopMiddleware(approval_required=["delete", "send"]),
        AuditLogMiddleware(),
    ]
)
```

### 11.3 调试技巧

```python
# 1. 启用详细日志
import logging
logging.basicConfig(level=logging.DEBUG)

# 2. 使用 LangSmith 追踪
from langsmith import trace

@trace
def agent_run(task):
    return agent.run(task)

# 3. 保存中间状态
agent = create_agent(
    middleware=[
        CheckpointMiddleware(save_every_n_steps=1)
    ]
)

# 4. 回放执行
from langsmith import replay

trace_id = "trace_xxx"
replay(trace_id)
```

---

## 参考资料

### 官方文档

- [LangGraph 文档](https://docs.langchain.com/langgraph/)
- [LangChain Blog](https://www.langchain.com/blog)
- [MCP 协议](https://modelcontextprotocol.io/)
- [Claude 文档](https://docs.anthropic.com/)

### 关键文章

- The Art of Loop Engineering
- Deep Agents
- Building Custom Agent Harness
- Fault Tolerance in LangGraph
- Rubrics: Self-Evaluating Agents
- How to Choose the Right Sandbox

---

*文档版本: 1.0*
*最后更新: 2026年6月*
