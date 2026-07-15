# Agent Loop (ReAct) 完全指南

> 研究理论、主流框架与实现原理

> **术语说明**: 学术界正式名称是 **ReAct** (Reasoning + Acting)，工业界常称为 **Agent Loop**。本文档使用 Agent Loop 作为标题，但核心概念等同于 ReAct。

---

## 目录

- [一、什么是 Agent Loop (ReAct)](#一什么是-agent-loopreact)
- [二、核心研究理论](#二核心研究理论)
- [三、主流框架介绍](#三主流框架介绍)
- [四、框架对比与选型](#四框架对比与选型)
- [五、实现原理](#五实现原理)
- [六、发展时间线](#六发展时间线)
- [七、参考资料](#七参考资料)

---

## 一、什么是 Agent Loop (ReAct)

### 1.1 定义与术语

**Agent Loop**（也叫 **ReAct**）是一种 AI Agent 执行模式。

> **核心思想**: Agent 在"思考-行动-观察"的循环中迭代执行，直到任务完成

**术语澄清**:

| 术语 | 使用场景 | 说明 |
|------|----------|------|
| **ReAct** | 学术界 | 论文正式名称，Reasoning + Acting |
| **Agent Loop** | 工业界 | 强调循环执行的特性 |
| **Agentic Loop** | LangChain | 同上，变体写法 |
| **Loop Agent** | ❌ 不正式 | 可能是误解或随意组合，不是标准术语 |

### 1.2 基本流程

```
        ┌──────────────────────────────────────┐
        │                                      │
        ▼                                      │
    ┌───────┐      ┌───────┐      ┌───────┐   │
    │ 思考  │ ───→ │ 行动  │ ───→ │ 观察  │ ───┘
    │ Think │      │ Act   │      │Observe│
    └───────┘      └───────┘      └───────┘
        │
        ▼
    任务完成？
        │
        ├─ 是 ──→ 输出最终答案
        │
        └─ 否 ──→ 继续循环
```

### 1.3 每个阶段的作用

| 阶段 | 英文 | 说明 | 示例 |
|------|------|------|------|
| 思考 | Think | LLM 分析当前状态，推理下一步行动 | "我需要先查询天气信息" |
| 行动 | Act | LLM 调用工具或做出决策 | 调用 `weather_api("北京")` |
| 观察 | Observe | 获取工具执行结果 | 返回 "25度，晴天" |
| 循环 | Loop | 将观察结果反馈给 LLM，继续思考 | "现在我有了天气信息，可以回答了" |

### 1.4 与普通对话的区别

**普通对话（一次性）：**

```
用户: "今天北京天气怎么样？"
AI: "抱歉，我无法获取实时天气信息"
```

**Loop Agent（迭代执行）：**

```
用户: "今天北京天气怎么样？"

[Step 1]
Thought: 用户想知道北京天气，我需要调用天气工具
Action: weather_api("北京")
Observation: {"temp": 25, "weather": "晴"}

[Step 2]
Thought: 我已经获取了天气信息，可以回答用户了
Action: finish("北京今天25度，天气晴朗")
Final Answer: 北京今天25度，天气晴朗
```

### 1.5 为什么需要 Loop Agent？

1. **复杂任务无法一步完成** — 一个任务可能需要调用多个工具、执行多个步骤
2. **动态决策** — 每一步的执行结果可能影响下一步的决策方向
3. **自我纠错** — 如果某一步执行失败，Agent 可以尝试替代方案
4. **适应性强** — 可以处理预定义流程无法覆盖的场景

### 1.6 终止条件

Loop Agent 需要明确的终止条件：

| 条件 | 说明 |
|------|------|
| **任务完成** | LLM 判断已获取足够信息，不再调用工具，直接输出答案 |
| **最大步数** | 达到预设的最大循环次数，防止无限循环 |
| **发生错误** | 遇到无法恢复的错误，被迫终止 |
| **用户中断** | 用户主动取消任务 |

---

## 二、核心研究理论

### 2.1 ReAct (Reasoning + Acting)

**论文**: *ReAct: Synergizing Reasoning and Acting in Language Models*

**发表**: ICLR 2023, Google Research

**论文链接**: https://arxiv.org/abs/2210.03629

**核心思想**:

让 LLM 交替进行"推理（Reasoning）"和"行动（Acting）"，在思考中行动，在行动中思考。

**ReAct 执行示例**:

```
Question: 除了苹果遥控器，还有哪些设备可以控制苹果电视？

Thought 1: 我需要搜索苹果电视支持的遥控器和控制设备
Action 1: Search[Apple TV remote control devices]
Observation 1: 苹果电视支持以下控制设备...

Thought 2: 我需要确认除了官方遥控器还有哪些第三方设备
Action 2: Search[third party Apple TV remote]
Observation 2: 第三方支持包括 iPhone、iPad、Apple Watch...

Thought 3: 我已经获取了足够的信息来回答问题
Action 3: Finish[除了苹果遥控器，您还可以使用 iPhone、iPad、Apple Watch、支持 IR 的通用遥控器等设备控制苹果电视]
```

**ReAct 的优势**:

| 优势 | 说明 |
|------|------|
| 可解释性 | 每个 Thought 都展示了模型的推理过程 |
| 可靠性 | Action 和 Observation 交错，减少幻觉 |
| 可追溯 | 完整的执行轨迹便于调试 |
| 可纠正 | 基于 Observation 可以调整后续 Action |

**ReAct vs 其他方法对比**:

```
Standard Prompting:
Q: 问题
A: 答案（可能产生幻觉）

CoT (Chain-of-Thought):
Q: 问题
A: 思考过程... 答案（仍可能产生幻觉）

ReAct:
Q: 问题
A: Thought → Action → Observation → Thought → ... → 答案
   （通过外部工具验证，减少幻觉）
```

---

### 2.2 Chain-of-Thought (CoT)

**论文**: *Chain-of-Thought Prompting Elicits Reasoning in Large Language Models*

**发表**: NeurIPS 2022, Google Research

**论文链接**: https://arxiv.org/abs/2201.11903

**核心思想**:

让模型"一步步思考"，通过展示中间推理过程来提高复杂任务的准确性。

**CoT 示例**:

```
标准提示:
Q: Roger 有 5 个网球。他又买了 2 罐网球，每罐有 3 个。他现在有多少网球？
A: 11

CoT 提示:
Q: Roger 有 5 个网球。他又买了 2 罐网球，每罐有 3 个。他现在有多少网球？
A: Roger 起初有 5 个网球。
   2 罐网球，每罐 3 个，所以是 2 × 3 = 6 个新网球。
   5 + 6 = 11。
   答案是 11。
```

**CoT 的变体**:

| 变体 | 说明 |
|------|------|
| **Zero-shot CoT** | 添加"Let's think step by step"提示，无需示例 |
| **Manual CoT** | 手动编写推理示例作为 few-shot |
| **Auto CoT** | 自动生成推理示例 |
| **Self-Consistency CoT** | 多次采样推理路径，投票选择答案 |

**与 Loop Agent 的关系**:

CoT 提供了"思考"的能力，是 Loop Agent 中 Thought 阶段的基础。

---

### 2.3 Tool Use / Function Calling

**论文**: *Toolformer: Language Models Can Teach Themselves to Use Tools*

**发表**: NeurIPS 2023, Meta AI

**论文链接**: https://arxiv.org/abs/2302.04761

**核心思想**:

让 LLM 学会在适当的时候调用外部工具（API），并将工具结果整合到生成过程中。

**Toolformer 工作流程**:

```
1. 输入文本: "上周的电影票房是多少？"

2. 模型决定调用工具:
   "上周的电影票房是多少？[API_call(search, "上周电影票房")]"

3. 执行工具获取结果:
   Result: "上周票房冠军是《xxx》，票房2亿..."

4. 将结果插入文本:
   "上周的电影票房是多少？[Result: 上周票房冠军是《xxx》，票房2亿...]"

5. 模型生成最终回答:
   "根据票房数据，上周《xxx》获得冠军，总票房2亿元。"
```

**其他重要研究**:

| 论文 | 团队 | 核心贡献 |
|------|------|----------|
| *Gorilla: Large Language Model Connected with Massive APIs* | UC Berkeley | 训练专门的工具调用模型 |
| *ToolLLM: Facilitating Large Language Models to Master 16000+ Real-world APIs* | 清华大学 | 大规模工具学习 |
| *Chameleon: Plug-and-Play Compositional Reasoning with Large Language Models* | Microsoft | 组合式工具调用 |

---

### 2.4 Self-Reflection

**论文**: *Reflexion: Language Agents with Verbal Reinforcement Learning*

**发表**: NeurIPS 2023, Northeastern University, MIT

**论文链接**: https://arxiv.org/abs/2303.11366

**核心思想**:

Agent 执行任务后进行自我反思（Reflection），从失败中学习，改进后续尝试。

**Reflexion 工作流程**:

```
┌─────────────────────────────────────────────────────────┐
│                     Reflexion Loop                       │
│                                                          │
│   ┌──────────┐    ┌──────────┐    ┌──────────┐         │
│   │  Actor   │ →  │ Evaluator│ →  │Self-Reflect│        │
│   │ (执行任务) │    │ (评估结果) │    │ (自我反思)  │        │
│   └──────────┘    └──────────┘    └──────────┘         │
│        ↑                                 │              │
│        └─────────────────────────────────┘              │
│                    (保存反思经验)                          │
└─────────────────────────────────────────────────────────┘
```

**反思示例**:

```
Task: 编写一个排序算法

Attempt 1:
Action: 写代码
Result: 测试失败，数组越界错误
Reflection: 我在实现时没有考虑空数组的情况，导致索引越界。
            下次需要添加边界条件检查。

Attempt 2:
Action: 修改代码，添加边界检查
Result: 测试通过
Final Answer: 排序算法实现完成
```

**其他自我改进研究**:

| 论文 | 核心思想 |
|------|----------|
| *Self-Refine* | 迭代自我改进输出 |
| *CRITIC* | 使用工具验证和纠正输出 |
| *Tree of Thoughts* | 探索多条思考路径 |

---

### 2.5 Plan-and-Solve

**论文**: *Plan-and-Solve Prompting: Improving Zero-Shot Chain-of-Thought Reasoning by Large Language Models*

**发表**: ACL 2023

**论文链接**: https://arxiv.org/abs/2305.11903

**核心思想**:

先制定完整计划，再逐步执行，避免"走一步看一步"的盲目性。

**Plan-and-Solve 流程**:

```
┌─────────────────────────────────────────┐
│              Planning Phase              │
│                                          │
│  Input: "帮我订一张明天去上海的机票"        │
│                                          │
│  Plan:                                   │
│    1. 查询明天北京到上海的航班             │
│    2. 筛选合适的航班时间                  │
│    3. 比较价格                           │
│    4. 完成预订                           │
└─────────────────────────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────┐
│              Solving Phase               │
│                                          │
│  Execute Step 1: 查询航班 ✓              │
│  Execute Step 2: 筛选航班 ✓              │
│  Execute Step 3: 比较价格 ✓              │
│  Execute Step 4: 完成预订 ✓              │
└─────────────────────────────────────────┘
```

**与其他方法对比**:

| 方法 | 特点 |
|------|------|
| **ReAct** | 边思考边行动，灵活但可能走偏 |
| **Plan-and-Solve** | 先规划后执行，有条理但可能不够灵活 |
| **Plan-and-Execute** | Plan 和 Execute 分离，可并行优化 |

---

### 2.6 理论总结

| 理论 | 核心贡献 | 与 Loop Agent 的关系 |
|------|----------|----------------------|
| **ReAct** | 思考-行动循环范式 | Loop Agent 的核心执行模式 |
| **CoT** | 链式思考推理 | Thought 阶段的推理基础 |
| **Tool Use** | 工具调用能力 | Action 阶段的能力支撑 |
| **Self-Reflection** | 自我反思学习 | 支持错误恢复和持续改进 |
| **Plan-and-Solve** | 先规划后执行 | 复杂任务的规划策略 |

---

## 三、主流框架介绍

### 3.1 LangChain

**官网**: https://python.langchain.com/

**GitHub**: https://github.com/langchain-ai/langchain

**Stars**: 90k+ (2024)

**简介**:

LangChain 是最流行的 LLM 应用开发框架，提供了完整的 Agent 开发工具链。

**核心组件**:

```
┌─────────────────────────────────────────────────────┐
│                    LangChain 架构                    │
├─────────────────────────────────────────────────────┤
│                                                      │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐│
│  │  Model  │  │  Prompt │  │  Memory │  │  Index  ││
│  │   I/O   │  │ Template│  │         │  │  (RAG)  ││
│  └─────────┘  └─────────┘  └─────────┘  └─────────┘│
│                                                      │
│  ┌──────────────────────────────────────────────┐  │
│  │                    Chain                      │  │
│  │         (组合多个组件的执行流程)                │  │
│  └──────────────────────────────────────────────┘  │
│                                                      │
│  ┌──────────────────────────────────────────────┐  │
│  │                    Agent                      │  │
│  │    (自主决策，使用工具，动态执行)               │  │
│  └──────────────────────────────────────────────┘  │
│                                                      │
│  ┌──────────────────────────────────────────────┐  │
│  │                   Tools                       │  │
│  │         (搜索、计算、API调用等)                │  │
│  └──────────────────────────────────────────────┘  │
│                                                      │
└─────────────────────────────────────────────────────┘
```

**Agent 实现示例**:

```python
from langchain.agents import initialize_agent, AgentType
from langchain.tools import Tool
from langchain_openai import ChatOpenAI

# 定义工具
tools = [
    Tool(
        name="Calculator",
        func=calculator,
        description="用于数学计算"
    ),
    Tool(
        name="Search",
        func=search,
        description="用于搜索信息"
    )
]

# 创建 LLM
llm = ChatOpenAI(model="gpt-4")

# 创建 Agent
agent = initialize_agent(
    tools=tools,
    llm=llm,
    agent=AgentType.ZERO_SHOT_REACT_DESCRIPTION,
    verbose=True
)

# 执行
result = agent.run("北京今天天气如何？")
```

**LangChain Agent 执行流程**:

```python
# AgentExecutor 的核心循环 (简化版)
def agent_loop(agent, tools, input):
    messages = [input]

    while True:
        # 1. LLM 思考并决定下一步行动
        action = agent.plan(messages)

        # 2. 判断是否完成
        if action.is_final():
            return action.output

        # 3. 执行工具
        observation = tools[action.tool_name](action.tool_input)

        # 4. 记录观察结果
        messages.append(observation)

        # 5. 继续循环
```

**LangChain Agent 类型**:

| Agent 类型 | 说明 |
|------------|------|
| `ZERO_SHOT_REACT_DESCRIPTION` | 基于 ReAct 的零样本 Agent |
| `CHAT_ZERO_SHOT_REACT_DESCRIPTION` | 聊天模型版本的 ReAct Agent |
| `OPENAI_FUNCTIONS` | 使用 OpenAI Function Calling |
| `STRUCTURED_CHAT_ZERO_SHOT_REACT_DESCRIPTION` | 支持多输入参数的聊天 Agent |
| `PLAN_AND_EXECUTE` | 先规划后执行的 Agent |

---

### 3.2 AutoGPT

**GitHub**: https://github.com/Significant-Gravitas/AutoGPT

**Stars**: 165k+ (2024)

**简介**:

AutoGPT 是第一个"完全自主"的 AI Agent，只需给定一个目标，它会自主规划和执行直到目标达成。

**核心特点**:

```
用户: "帮我创办一家咖啡店"

AutoGPT 自主执行:
┌─────────────────────────────────────────────────────┐
│ Thought: 我需要研究咖啡店市场                        │
│ Action: 搜索咖啡店市场分析                           │
│ Observation: [市场数据...]                          │
├─────────────────────────────────────────────────────┤
│ Thought: 我需要制定商业计划                          │
│ Action: 写入文件 "business_plan.md"                  │
│ Observation: 文件已保存                             │
├─────────────────────────────────────────────────────┤
│ Thought: 我需要寻找合适的选址                        │
│ Action: 搜索 "咖啡店选址建议"                        │
│ Observation: [选址建议...]                          │
├─────────────────────────────────────────────────────┤
│ ...自动继续...                                       │
└─────────────────────────────────────────────────────┘
```

**AutoGPT 架构**:

```
┌─────────────────────────────────────────────────────────┐
│                      AutoGPT 架构                        │
├─────────────────────────────────────────────────────────┤
│                                                          │
│  ┌────────────┐     ┌────────────┐     ┌────────────┐  │
│  │   Goals    │     │   Memory   │     │   Tools    │  │
│  │  (目标)    │     │  (记忆)    │     │  (工具)    │  │
│  └────────────┘     └────────────┘     └────────────┘  │
│         │                  │                  │         │
│         └──────────────────┼──────────────────┘         │
│                            │                            │
│                            ▼                            │
│                    ┌──────────────┐                     │
│                    │    Agent     │                     │
│                    │   (大脑)     │                     │
│                    └──────────────┘                     │
│                            │                            │
│                            ▼                            │
│                    ┌──────────────┐                     │
│                    │  Workspace   │                     │
│                    │  (工作空间)   │                     │
│                    └──────────────┘                     │
│                                                          │
└─────────────────────────────────────────────────────────┘
```

**AutoGPT vs 普通 Agent**:

| 特点 | 普通 Agent | AutoGPT |
|------|------------|---------|
| **用户交互** | 每步需要用户输入 | 一次性给定目标，自主执行 |
| **任务范围** | 单一任务 | 复杂、长期的任务 |
| **执行时间** | 几秒到几分钟 | 可持续几小时甚至几天 |
| **自主性** | 低 | 高 |

---

### 3.3 BabyAGI

**GitHub**: https://github.com/yoheinakajima/babyagi

**Stars**: 20k+ (2024)

**简介**:

BabyAGI 是一个任务驱动的自主 Agent 框架，核心思想是通过任务队列来管理执行流程。

**核心流程**:

```
┌─────────────────────────────────────────────────────────┐
│                     BabyAGI 循环                         │
│                                                          │
│                    ┌──────────────┐                      │
│                    │  Task Queue  │                      │
│                    │   任务队列    │                      │
│                    └──────┬───────┘                      │
│                           │                              │
│                           ▼                              │
│    ┌──────────────────────────────────────────┐         │
│    │              1. 执行任务                   │         │
│    │         Task Execution Agent             │         │
│    └──────────────────────────────────────────┘         │
│                           │                              │
│                           ▼                              │
│    ┌──────────────────────────────────────────┐         │
│    │              2. 存储结果                   │         │
│    │           Store in Memory                │         │
│    └──────────────────────────────────────────┘         │
│                           │                              │
│                           ▼                              │
│    ┌──────────────────────────────────────────┐         │
│    │              3. 创建新任务                 │         │
│    │         Task Creation Agent              │         │
│    └──────────────────────────────────────────┘         │
│                           │                              │
│                           ▼                              │
│    ┌──────────────────────────────────────────┐         │
│    │              4. 任务优先级排序             │         │
│    │        Prioritization Agent              │         │
│    └──────────────────────────────────────────┘         │
│                           │                              │
│                           └──────→ 返回任务队列           │
│                                                          │
└─────────────────────────────────────────────────────────┘
```

**BabyAGI 代码示例**:

```python
import babyagi

# 定义目标
objective = "研究如何提高咖啡店利润"

# 运行 BabyAGI
babyagi.run(objective, max_iterations=10)

# 执行过程:
# Task 1: 搜索咖啡店利润提升方法
# Task 2: 分析竞争对手定价策略
# Task 3: 制定营销计划
# Task 4: 评估成本控制方案
# ...
```

**三个核心 Agent**:

| Agent | 职责 |
|-------|------|
| **Task Execution Agent** | 执行当前任务，调用工具完成具体操作 |
| **Task Creation Agent** | 根据执行结果，创建新的相关任务 |
| **Prioritization Agent** | 对任务队列进行优先级排序 |

---

### 3.4 CrewAI

**官网**: https://www.crewai.com/

**GitHub**: https://github.com/joaomdmoura/crewAI

**Stars**: 18k+ (2024)

**简介**:

CrewAI 是一个多 Agent 协作框架，让多个 AI Agent 像团队一样协作完成任务。

**核心概念**:

```
┌─────────────────────────────────────────────────────────┐
│                      CrewAI 架构                         │
├─────────────────────────────────────────────────────────┤
│                                                          │
│   Agent 1          Agent 2          Agent 3             │
│  ┌─────────┐      ┌─────────┐      ┌─────────┐         │
│  │研究员    │      │ 分析师   │      │ 作家    │         │
│  │Researcher│      │Analyst  │      │ Writer  │         │
│  └────┬────┘      └────┬────┘      └────┬────┘         │
│       │                │                │               │
│       └────────────────┼────────────────┘               │
│                        │                                │
│                        ▼                                │
│                ┌──────────────┐                         │
│                │     Task     │                         │
│                │    (任务)    │                         │
│                └──────────────┘                         │
│                        │                                │
│                        ▼                                │
│                ┌──────────────┐                         │
│                │    Crew      │                         │
│                │   (团队)     │                         │
│                └──────────────┘                         │
│                                                          │
└─────────────────────────────────────────────────────────┘
```

**CrewAI 代码示例**:

```python
from crewai import Agent, Task, Crew, Process
from langchain_openai import ChatOpenAI

# 定义 LLM
llm = ChatOpenAI(model="gpt-4")

# 定义 Agent
researcher = Agent(
    role="研究员",
    goal="研究并收集最新信息",
    backstory="你是一位经验丰富的市场研究员",
    llm=llm
)

analyst = Agent(
    role="分析师",
    goal="分析数据并提取洞察",
    backstory="你是一位数据分析专家",
    llm=llm
)

writer = Agent(
    role="作家",
    goal="撰写高质量报告",
    backstory="你是一位专业的内容创作者",
    llm=llm
)

# 定义任务
research_task = Task(
    description="研究2024年AI市场趋势",
    agent=researcher
)

analysis_task = Task(
    description="分析收集的市场数据",
    agent=analyst
)

writing_task = Task(
    description="撰写市场分析报告",
    agent=writer
)

# 组建团队
crew = Crew(
    agents=[researcher, analyst, writer],
    tasks=[research_task, analysis_task, writing_task],
    process=Process.sequential  # 顺序执行
)

# 启动
result = crew.kickoff()
```

**协作模式**:

| 模式 | 说明 |
|------|------|
| **Sequential** | 任务按顺序执行，前一个任务的输出作为下一个任务的输入 |
| **Hierarchical** | 存在管理者 Agent，负责分配和监督任务 |
| **Consensual** | 多个 Agent 共同决策 |

---

### 3.5 Microsoft AutoGen

**GitHub**: https://github.com/microsoft/autogen

**Stars**: 28k+ (2024)

**简介**:

AutoGen 是微软开源的多 Agent 对话框架，支持 Agent 之间的自主对话协作。

**核心概念**:

```
┌─────────────────────────────────────────────────────────┐
│                     AutoGen 对话                         │
├─────────────────────────────────────────────────────────┤
│                                                          │
│  ┌──────────────┐              ┌──────────────┐         │
│  │ UserProxy    │◄────────────►│ Assistant    │         │
│  │ Agent        │              │ Agent        │         │
│  │ (用户代理)    │              │ (助手)       │         │
│  └──────────────┘              └──────────────┘         │
│        │                              │                 │
│        │                              │                 │
│        ▼                              ▼                 │
│  ┌──────────────┐              ┌──────────────┐         │
│  │ Critic       │◄────────────►│ Coder        │         │
│  │ Agent        │              │ Agent        │         │
│  │ (评论家)     │              │ (程序员)     │         │
│  └──────────────┘              └──────────────┘         │
│                                                          │
└─────────────────────────────────────────────────────────┘
```

**AutoGen 代码示例**:

```python
import autogen

# 配置 LLM
config_list = [{
    "model": "gpt-4",
    "api_key": "your-api-key"
}]

# 创建助手 Agent
assistant = autogen.AssistantAgent(
    name="assistant",
    llm_config={"config_list": config_list}
)

# 创建用户代理 Agent
user_proxy = autogen.UserProxyAgent(
    name="user_proxy",
    human_input_mode="NEVER",  # 不需要人工输入
    max_consecutive_auto_reply=10,
    code_execution_config={"work_dir": "coding"}
)

# 启动对话
user_proxy.initiate_chat(
    assistant,
    message="帮我写一个Python爬虫，抓取新闻标题"
)

# 执行过程:
# User: 帮我写一个Python爬虫
# Assistant: 好的，我来编写代码... [代码块]
# User Proxy: [执行代码] 结果: ...
# Assistant: 代码运行成功，需要我优化吗？
# ...
```

**Agent 类型**:

| Agent 类型 | 说明 |
|------------|------|
| **AssistantAgent** | 默认助手，负责生成回复和代码 |
| **UserProxyAgent** | 用户代理，可执行代码、提供人工输入 |
| **ConversableAgent** | 基类，可自定义行为 |

---

### 3.6 Microsoft Semantic Kernel

**GitHub**: https://github.com/microsoft/semantic-kernel

**Stars**: 22k+ (2024)

**简介**:

Semantic Kernel 是微软开发的企业级 AI 编排框架，支持 C#、Python、Java 多种语言。

**核心概念**:

```
┌─────────────────────────────────────────────────────────┐
│                   Semantic Kernel 架构                   │
├─────────────────────────────────────────────────────────┤
│                                                          │
│  ┌─────────────────────────────────────────────────────┐│
│  │                    Kernel                           ││
│  │                  (核心容器)                          ││
│  └─────────────────────────────────────────────────────┘│
│                           │                              │
│         ┌─────────────────┼─────────────────┐           │
│         │                 │                 │           │
│         ▼                 ▼                 ▼           │
│  ┌────────────┐    ┌────────────┐    ┌────────────┐    │
│  │  Skills    │    │  Planners  │    │  Memories  │    │
│  │  (技能)    │    │  (规划器)   │    │  (记忆)    │    │
│  └────────────┘    └────────────┘    └────────────┘    │
│                                                          │
└─────────────────────────────────────────────────────────┘
```

**C# 代码示例**:

```csharp
using Microsoft.SemanticKernel;
using Microsoft.SemanticKernel.Planning;

// 创建 Kernel
var kernel = Kernel.Builder()
    .WithOpenAIChatCompletion("gpt-4", "your-api-key")
    .Build();

// 导入技能
kernel.ImportSkill(new EmailSkill(), "email");
kernel.ImportSkill(new CalendarSkill(), "calendar");

// 创建规划器
var planner = new SequentialPlanner(kernel);

// 执行
var plan = await planner.CreatePlan("发送会议邀请给 team@company.com");
var result = await plan.InvokeAsync();
```

**Python 代码示例**:

```python
import semantic_kernel as sk
from semantic_kernel.planning import SequentialPlanner

# 创建 Kernel
kernel = sk.Kernel()
kernel.add_chat_service("gpt-4", OpenAIChatCompletion("gpt-4", "your-api-key"))

# 导入技能
kernel.import_skill(EmailSkill(), "email")

# 创建规划并执行
planner = SequentialPlanner(kernel)
plan = await planner.create_plan("发送邮件给 boss@company.com")
result = await plan.invoke()
```

---

### 3.7 LlamaIndex

**官网**: https://www.llamaindex.ai/

**GitHub**: https://github.com/run-llama/llama_index

**Stars**: 33k+ (2024)

**简介**:

LlamaIndex 专注于数据连接，将外部数据与 LLM 连接，同时提供 Agent 能力。

**核心架构**:

```
┌─────────────────────────────────────────────────────────┐
│                    LlamaIndex 架构                       │
├─────────────────────────────────────────────────────────┤
│                                                          │
│                    ┌──────────────┐                      │
│                    │    Agent     │                      │
│                    │   (代理)     │                      │
│                    └──────┬───────┘                      │
│                           │                              │
│         ┌─────────────────┼─────────────────┐           │
│         │                 │                 │           │
│         ▼                 ▼                 ▼           │
│  ┌────────────┐    ┌────────────┐    ┌────────────┐    │
│  │   Query    │    │   Index    │    │   Tools    │    │
│  │  Engine    │    │  (索引)    │    │  (工具)    │    │
│  └────────────┘    └────────────┘    └────────────┘    │
│         │                 │                 │           │
│         └─────────────────┼─────────────────┘           │
│                           │                              │
│                           ▼                              │
│                    ┌──────────────┐                      │
│                    │  Data Sources│                      │
│                    │  (数据源)    │                      │
│                    └──────────────┘                      │
│                                                          │
└─────────────────────────────────────────────────────────┘
```

**LlamaIndex Agent 示例**:

```python
from llama_index.core.agent import ReActAgent
from llama_index.core.tools import FunctionTool
from llama_index.llms.openai import OpenAI

# 定义工具
def multiply(a: int, b: int) -> int:
    """Multiply two numbers"""
    return a * b

def add(a: int, b: int) -> int:
    """Add two numbers"""
    return a + b

tools = [
    FunctionTool.from_defaults(fn=multiply),
    FunctionTool.from_defaults(fn=add)
]

# 创建 Agent
llm = OpenAI(model="gpt-4")
agent = ReActAgent.from_tools(tools, llm=llm, verbose=True)

# 执行
response = agent.chat("计算 (5 + 3) * 2")
```

---

## 四、框架对比与选型

### 4.1 功能对比

| 框架 | Agent 类型 | 多 Agent | 工具支持 | 记忆 | RAG | 企业级 |
|------|------------|----------|----------|------|-----|--------|
| **LangChain** | ReAct, Plan | ✓ | ✓ | ✓ | ✓ | ✓ |
| **AutoGPT** | 自主 Agent | ✗ | ✓ | ✓ | ✗ | ✗ |
| **BabyAGI** | 任务驱动 | ✗ | ✓ | ✓ | ✗ | ✗ |
| **CrewAI** | 协作 Agent | ✓✓ | ✓ | ✓ | ✗ | ✗ |
| **AutoGen** | 对话 Agent | ✓✓ | ✓ | ✓ | ✗ | ✓ |
| **Semantic Kernel** | 规划 Agent | ✗ | ✓ | ✓ | ✓ | ✓✓ |
| **LlamaIndex** | ReAct | ✗ | ✓ | ✓ | ✓✓ | ✓ |

### 4.2 适用场景

| 场景 | 推荐框架 | 原因 |
|------|----------|------|
| **快速原型开发** | LangChain | 生态丰富，文档完善 |
| **完全自主任务** | AutoGPT | 自主性强，无需人工干预 |
| **任务规划执行** | BabyAGI | 任务驱动，流程清晰 |
| **团队协作模拟** | CrewAI | 多 Agent 协作原生支持 |
| **对话式协作** | AutoGen | Agent 间对话自然 |
| **企业应用集成** | Semantic Kernel | 多语言支持，企业级特性 |
| **RAG + Agent** | LlamaIndex | 数据连接能力强 |

### 4.3 技术栈选择

```
选择决策树:

                    你的项目类型？
                         │
         ┌───────────────┼───────────────┐
         │               │               │
      原型/MVP       企业应用        研究/实验
         │               │               │
         ▼               ▼               ▼
    LangChain     Semantic Kernel    AutoGPT
         │               │           BabyAGI
         │               │               │
         ▼               ▼               ▼
    需要多Agent?    需要多Agent?    需要协作?
         │               │               │
    ┌────┴────┐     ┌────┴────┐     ┌────┴────┐
   是        否    是        否    是        否
    │         │     │         │     │         │
    ▼         ▼     ▼         ▼     ▼         ▼
 CrewAI   LlamaIndex AutoGen  SK   CrewAI   AutoGPT
 AutoGen                   LlamaIndex       BabyAGI
```

---

## 五、实现原理

### 5.1 Loop Agent 核心伪代码

```python
def loop_agent(llm, tools, user_input, max_steps=10):
    """
    Loop Agent 核心执行循环
    """
    messages = [{"role": "user", "content": user_input}]

    for step in range(max_steps):
        # 1. 思考阶段：LLM 分析并决定行动
        response = llm.chat(messages, tools=tools)

        # 2. 检查是否完成
        if not response.tool_calls:
            # 没有工具调用，任务完成
            return response.content

        # 3. 行动阶段：执行工具
        for tool_call in response.tool_calls:
            tool_name = tool_call.name
            tool_args = tool_call.arguments

            # 执行工具
            result = execute_tool(tools, tool_name, tool_args)

            # 4. 观察阶段：记录结果
            messages.append({
                "role": "tool",
                "name": tool_name,
                "content": result
            })

    # 达到最大步数，强制结束
    return llm.chat(messages, force_final_answer=True)
```

### 5.2 ReAct Prompt 模板

```python
REACT_PROMPT = """
你是一个能使用工具的智能助手。请按照以下格式思考和行动：

Question: 用户的问题
Thought: 你的思考过程
Action: 工具名称[工具参数]
Observation: 工具返回结果
... (这个 Thought/Action/Observation 可以重复 N 次)
Thought: 我现在知道最终答案了
Final Answer: 对用户问题的最终回答

可用工具：
{tool_descriptions}

开始！

Question: {input}
Thought: {agent_scratchpad}
"""
```

### 5.3 工具调用流程

```
┌─────────────────────────────────────────────────────────┐
│                    工具调用流程                          │
├─────────────────────────────────────────────────────────┤
│                                                          │
│  1. LLM 生成工具调用请求                                 │
│     {                                                    │
│       "name": "weather_api",                            │
│       "arguments": {"city": "北京"}                     │
│     }                                                    │
│                                                          │
│  2. 框架解析请求                                         │
│     tool_name = "weather_api"                           │
│     tool_args = {"city": "北京"}                        │
│                                                          │
│  3. 执行工具                                             │
│     result = weather_api(city="北京")                   │
│     result = {"temp": 25, "weather": "晴"}              │
│                                                          │
│  4. 返回结果给 LLM                                       │
│     Observation: {"temp": 25, "weather": "晴"}          │
│                                                          │
│  5. LLM 继续思考                                         │
│     Thought: 我已经获取了天气信息...                     │
│                                                          │
└─────────────────────────────────────────────────────────┘
```

### 5.4 记忆管理

```python
class AgentMemory:
    """
    Agent 记忆管理
    """

    def __init__(self, max_messages=50):
        self.messages = []
        self.max_messages = max_messages

    def add_message(self, role, content):
        """添加消息"""
        self.messages.append({"role": role, "content": content})

        # 超出限制时，保留系统消息和最近的对话
        if len(self.messages) > self.max_messages:
            self._truncate()

    def _truncate(self):
        """截断历史"""
        # 保留第一条系统消息
        system_msg = self.messages[0] if self.messages[0]["role"] == "system" else None

        # 保留最近的对话
        recent = self.messages[-(self.max_messages-1):]

        if system_msg:
            self.messages = [system_msg] + recent
        else:
            self.messages = recent

    def get_context(self):
        """获取上下文"""
        return self.messages
```

### 5.5 错误处理

```python
def execute_with_retry(tool, args, max_retries=3):
    """
    带重试的工具执行
    """
    for attempt in range(max_retries):
        try:
            result = tool(**args)
            return {"success": True, "result": result}

        except TimeoutError:
            # 超时错误，可以重试
            if attempt < max_retries - 1:
                time.sleep(2 ** attempt)  # 指数退避
                continue
            return {"success": False, "error": "Tool timeout after retries"}

        except AuthError:
            # 认证错误，无法通过重试解决
            return {"success": False, "error": "Authentication failed"}

        except Exception as e:
            # 未知错误
            return {"success": False, "error": str(e)}
```

---

## 六、发展时间线

```
2022
│
├── 01月 ─ Chain-of-Thought (Google)
│         "让模型一步步思考"
│
├── 10月 ─ ReAct (Google)
│         "思考 + 行动 循环范式"
│
│
2023
│
├── 01月 ─ Toolformer (Meta)
│         "让模型学会使用工具"
│
├── 03月 ─ AutoGPT 发布
│         "第一个完全自主的 AI Agent"
│
├── 04月 ─ BabyAGI 发布
│         "任务驱动的自主 Agent"
│         Reflexion 论文
│         "自我反思学习"
│
├── 06月 ─ LangChain Agent 稳定版
│         "通用 Agent 开发框架"
│
├── 08月 ─ Semantic Kernel (Microsoft)
│         "企业级 AI 编排框架"
│
├── 10月 ─ AutoGen (Microsoft)
│         "多 Agent 对话框架"
│         CrewAI 发布
│         "多 Agent 协作框架"
│
│
2024
│
├── 01月 ─ OpenAI GPTs & Assistants API
│         "官方 Agent 解决方案"
│
├── 03月 ─ Claude Tool Use
│         "Anthropic Agent 能力"
│
├── 06月 ─ LangGraph (LangChain)
│         "有状态的 Agent 工作流"
│
├── 10月 ─ Swarm (OpenAI)
│         "轻量级多 Agent 框架"
│
│
2025+
│
└── Agentic Workflow 成为主流
    "Agent 工作流自动化"
```

---

## 七、参考资料

### 7.1 核心论文

| 论文 | 链接 |
|------|------|
| ReAct | https://arxiv.org/abs/2210.03629 |
| Chain-of-Thought | https://arxiv.org/abs/2201.11903 |
| Toolformer | https://arxiv.org/abs/2302.04761 |
| Reflexion | https://arxiv.org/abs/2303.11366 |
| Plan-and-Solve | https://arxiv.org/abs/2305.11903 |
| Gorilla | https://arxiv.org/abs/2305.15334 |
| ToolLLM | https://arxiv.org/abs/2307.16789 |

### 7.2 框架文档

| 框架 | 文档链接 |
|------|----------|
| LangChain | https://python.langchain.com/docs/get_started/introduction |
| AutoGPT | https://docs.agpt.co/ |
| CrewAI | https://docs.crewai.com/ |
| AutoGen | https://microsoft.github.io/autogen/ |
| Semantic Kernel | https://learn.microsoft.com/en-us/semantic-kernel/ |
| LlamaIndex | https://docs.llamaindex.ai/en/stable/ |

### 7.3 学习资源

| 资源 | 链接 |
|------|------|
| LangChain 官方教程 | https://python.langchain.com/docs/tutorials/ |
| DeepLearning.AI Agent 课程 | https://www.deeplearning.ai/short-courses/ |
| Andrew Ng Agent 课程 | https://www.deeplearning.ai/courses/ |

---

## 八、总结

### 8.1 核心要点

1. **Loop Agent 本质**: 思考 → 行动 → 观察 → 循环
2. **理论基础**: ReAct、CoT、Tool Use、Self-Reflection
3. **主流框架**: LangChain、AutoGPT、CrewAI、AutoGen 等
4. **选择依据**: 根据场景需求（自主性、协作、企业级）选择合适框架

### 8.2 未来趋势

- **更强的自主性**: Agent 将更加智能地规划和执行任务
- **更好的协作**: 多 Agent 协作将成为常态
- **更低的门槛**: 框架将更加易用，降低开发成本
- **更多的应用**: Agent 将渗透到各行各业

---

*文档版本: 1.0*
*最后更新: 2024年*
