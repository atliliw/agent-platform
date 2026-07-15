---
name: open-source-agent-frameworks
description: 网上优秀开源Agent框架调研结果
metadata:
  type: reference
---

# 开源 Agent 框架调研

## 一、主流框架对比

| 框架 | Stars | 核心特点 | 适用场景 |
|------|-------|----------|----------|
| **MetaGPT** | 68.6k | 软件公司SOP模拟、多角色协作 | 软件开发、项目生成 |
| **CrewAI** | 35k+ | Crews+Flows双模式、YAML配置 | 团队协作任务 |
| **LangGraph** | 40k+ | 状态机、持久化、人在环中 | 复杂工作流 |
| **OpenAI Swarm** | 15k+ | 极简Handoff机制、轻量级 | 多Agent协作入门 |
| **AutoGPT** | 170k+ | 目标驱动、自反思、持续运行 | 自主任务执行 |
| **GPT-Researcher** | 30k+ | 规划者+执行者、多源搜索 | 研究报告生成 |
| **Dify** | 60k+ | 可视化工作流、50+内置工具 | 低代码Agent平台 |

---

## 二、核心架构模式

### 1. OpenAI Swarm - Handoff 模式（最简单）

```python
# 核心概念：Agent + Handoffs
# Agent 封装 instructions + tools
# 通过返回另一个 Agent 实现交接

from swarm import Swarm, Agent, Result

client = Swarm()

# 定义 Agent
sales_agent = Agent(
    name="Sales",
    instructions="You are a sales agent.",
    functions=[process_order]
)

support_agent = Agent(
    name="Support",
    instructions="You are a support agent.",
    functions=[create_ticket]
)

# Handoff 函数
def transfer_to_sales():
    return sales_agent

def transfer_to_support():
    return support_agent

# 主 Agent 可以将对话交接给其他 Agent
main_agent = Agent(
    name="Main",
    instructions="Route to appropriate agent.",
    functions=[transfer_to_sales, transfer_to_support]
)

# 运行
response = client.run(
    agent=main_agent,
    messages=[{"role": "user", "content": "I need to buy something"}]
)
```

**借鉴点**：
- 极简设计，只有 Agent + Handoff 两个原语
- Context Variables 跨 Agent 共享状态
- Result 对象支持同时返回值、切换 Agent、更新上下文

---

### 2. CrewAI - 角色+任务模式

```yaml
# agents.yaml - 定义角色
researcher:
  role: >
    {topic} Senior Data Researcher
  goal: >
    Uncover cutting-edge developments in {topic}
  backstory: >
    You're a seasoned researcher with a knack for uncovering the latest
    developments in {topic}.
  verbose: true

writer:
  role: >
    {topic} Reporting Analyst
  goal: >
    Create detailed reports based on {topic} data analysis
  backstory: >
    You're a meticulous analyst with a knack for uncovering trends.

# tasks.yaml - 定义任务
research_task:
  description: >
    Conduct a thorough research about {topic}
  expected_output: >
    A list with 10 bullet points of the most relevant information
  agent: researcher

reporting_task:
  description: >
    Review the content and create a final report
  expected_output: >
    A complete report with insights
  agent: writer
  output_file: report.md
```

```python
from crewai import Agent, Task, Crew

# 两种执行模式
# 1. Sequential - 任务按顺序执行
crew = Crew(
    agents=[researcher, writer],
    tasks=[research_task, reporting_task],
    process=Process.sequential
)

# 2. Hierarchical - Manager Agent 自动分配任务
crew = Crew(
    agents=[researcher, writer, editor],
    tasks=[task1, task2],
    process=Process.hierarchical,
    manager_llm="gpt-4"
)

result = crew.kickoff()
```

**借鉴点**：
- YAML 配置驱动，易于维护
- Sequential/Hierarchical 两种编排模式
- 任务依赖和输出文件自动处理

---

### 3. MetaGPT - SOP 流程模式

```python
# 核心理念：Code = SOP(Team)
# 将软件公司标准操作流程应用到 Agent 团队

from metagpt.roles import ProductManager, Architect, ProjectManager, Engineer

# 内置角色：产品经理 → 架构师 → 项目经理 → 工程师
company = Company()
company.hire([
    ProductManager(),
    Architect(),
    ProjectManager(),
    Engineer()
])

# 一行需求，输出完整项目
company.run_project("Create a CLI calculator app")

# 自动生成：
# - 用户故事
# - 竞争分析
# - 需求文档
# - 数据结构
# - API 设计
# - 代码实现
```

**借鉴点**：
- SOP（标准操作流程）驱动
- 角色专业化，各司其职
- 上下游角色自动衔接

---

### 4. LangGraph - 状态机模式

```python
from langgraph.graph import StateGraph, END

# 定义状态
class AgentState(TypedDict):
    messages: List[Message]
    next_agent: str

# 构建图
graph = StateGraph(AgentState)

# 添加节点
graph.add_node("researcher", researcher_node)
graph.add_node("writer", writer_node)
graph.add_node("reviewer", reviewer_node)

# 添加边
graph.add_edge("researcher", "writer")
graph.add_edge("writer", "reviewer")

# 条件路由
graph.add_conditional_edges(
    "reviewer",
    should_revise,
    {
        "revise": "writer",
        "finish": END
    }
)

# 持久化执行
from langgraph.checkpoint.memory import MemorySaver
checkpointer = MemorySaver()
app = graph.compile(checkpointer=checkpointer)

# 支持中断恢复
config = {"configurable": {"thread_id": "conversation-1"}}
result = app.invoke(input, config)
```

**借鉴点**：
- 状态机建模复杂工作流
- 持久化支持中断恢复
- 条件路由实现动态分支
- 人在环中（Human-in-the-loop）

---

### 5. GPT-Researcher - 规划者+执行者模式

```python
# 双 Agent 架构
# 1. Planner - 规划研究任务
# 2. Executor - 执行搜索和信息收集

class GPTResearcher:
    def __init__(self):
        self.planner = PlannerAgent()
        self.executor = ExecutorAgent()

    async def research(self, query: str):
        # 1. 规划阶段
        questions = await self.planner.generate_questions(query)

        # 2. 并行执行搜索
        tasks = [self.executor.search(q) for q in questions]
        results = await asyncio.gather(*tasks)

        # 3. 聚合和总结
        report = await self.planner.summarize(results)

        return report

# 特点：
# - 多源搜索（20+来源）
# - 并行化执行提升速度
# - 引用追踪
```

**借鉴点**：
- 规划者与执行者分离
- 并行任务执行
- 多源信息聚合

---

## 三、架构模式总结

### 模式 1: Handoff（交接）
- **代表**: OpenAI Swarm
- **特点**: 最简单，Agent 通过返回另一个 Agent 实现切换
- **适用**: 简单的多 Agent 协作

### 模式 2: 角色+任务
- **代表**: CrewAI
- **特点**: YAML 配置，角色专业化，任务驱动
- **适用**: 团队协作任务

### 模式 3: SOP 流程
- **代表**: MetaGPT
- **特点**: 标准流程驱动，角色按流程协作
- **适用**: 复杂业务流程

### 模式 4: 状态机
- **代表**: LangGraph
- **特点**: 图结构，持久化，条件路由
- **适用**: 复杂工作流，需要中断恢复

### 模式 5: 规划者+执行者
- **代表**: GPT-Researcher
- **特点**: 规划与执行分离，并行化
- **适用**: 研究型任务

---

## 四、对我们项目的建议

### 当前架构
```
用户请求 → Chat Agent → MCP 工具调用
```

### 建议改进：采用 Swarm + CrewAI 混合模式

```go
// 1. Agent 定义（借鉴 Swarm）
type Agent struct {
    ID           string
    Name         string
    Instructions string
    Tools        []Tool
    Handoffs     []string  // 可以交接给哪些 Agent
}

// 2. Agent 交接函数
func (a *Agent) HandoffTo(agentID string) *Result {
    return &Result{
        AgentID: agentID,
        Context: a.context,
    }
}

// 3. Agent 执行循环（借鉴 Swarm）
func (e *Engine) Run(ctx context.Context, agent *Agent, messages []Message) *Response {
    for {
        // 调用 LLM
        response := e.llm.Chat(ctx, agent, messages)

        // 检查是否需要工具调用
        if len(response.ToolCalls) > 0 {
            // 执行工具
            for _, tc := range response.ToolCalls {
                result := e.executeTool(ctx, tc)

                // 检查是否需要交接
                if result.Handoff != "" {
                    agent = e.agents[result.Handoff]
                    continue
                }

                messages = append(messages, result.Message)
            }
        } else {
            // 无工具调用，返回最终结果
            return response
        }
    }
}
```

### 新增 Agent 类型

| Agent | 角色 | 工具 | 可交接给 |
|-------|------|------|----------|
| Main Agent | 主调度 | 无 | Researcher, Coder, Analyst |
| Researcher | 研究 | web_search, knowledge_search | Main, Coder |
| Coder | 编程 | code_execute, file_write | Main, Analyst |
| Analyst | 分析 | data_analysis, visualization | Main |

---

## 五、实现优先级

1. **P0 - Handoff 机制**：实现 Agent 间交接
2. **P1 - Agent 注册**：支持动态注册新 Agent
3. **P2 - Context 共享**：跨 Agent 状态共享
4. **P3 - YAML 配置**：借鉴 CrewAI 的配置方式
5. **P4 - 状态持久化**：借鉴 LangGraph 的 checkpoint

---

## 六、参考资源

- [OpenAI Swarm](https://github.com/openai/swarm)
- [CrewAI](https://github.com/joaomdmoura/crewAI)
- [MetaGPT](https://github.com/geekan/MetaGPT)
- [LangGraph](https://github.com/langchain-ai/langgraph)
- [AutoGPT](https://github.com/Significant-Gravitas/AutoGPT)
- [GPT-Researcher](https://github.com/assafelovic/gpt-researcher)
- [Dify](https://github.com/langgenius/dify)
