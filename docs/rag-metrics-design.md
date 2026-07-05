# RAG 评测指标体系设计文档

> 版本: v2.0 | 更新日期: 2026-07-05
> 涉及模块: `services/harness-service/internal/rag/` + `frontend/src/pages/RAGMetrics/`

---

## 一、设计背景

RAG（Retrieval-Augmented Generation）系统的评测是衡量检索质量、生成质量、系统整体效果的关键环节。当前业界有三个主流开源评测框架：

| 框架 | 定位 | GitHub Stars | 特点 |
|------|------|-------------|------|
| **RAGAS** | RAG 专项评测 | ~8k | 最权威的 RAG 评测框架，RAGAS Score 定义者 |
| **DeepEval** | 通用 LLM 评测 | ~3k | 单元测试风格，支持 Hallucination 等安全指标 |
| **TruLens** | 可观测性 + 评测 | ~2k | 生产监控导向，RAG Triad（Groundedness + Context Relevance + Answer Relevance） |

我们的目标是**融合三大框架的核心指标**，形成比单一框架更完整的评测体系。

---

## 二、指标全景

### 2.1 检索质量指标（Retrieval Quality）

衡量 RAG 检索模块的效果：检索到的内容是否准、是否全、是否排序合理。

| # | 指标 | 英文名 | 含义 | 计算方式 | 需要GT | 对应开源 |
|---|------|--------|------|----------|:------:|----------|
| 1 | **上下文精确率** | Context Precision | 检索到的内容中有多少跟查询相关 | LLM 逐条判断相关性 → 相关数/总数 | ❌ | RAGAS ✅ DeepEval ✅ |
| 2 | **上下文召回率** | Context Recall | 需要的知识是否都被检索到 | GT 句子 vs 检索上下文的 embedding 相似度（阈值 0.8） | ✅ | RAGAS ✅ DeepEval ✅ |
| 3 | **上下文相关性** | Context Relevancy | 检索内容中废话占比 | 拆句 → LLM 判断每句相关性 → 相关句数/总句数 | ❌ | DeepEval ✅ TruLens ✅ |
| 4 | **实体召回率** 🆕 | Context Entity Recall | 关键实体是否被检索覆盖 | LLM 提取 GT 和 contexts 中的实体 → 交集/GT实体数 | ✅ | RAGAS ✅ |
| 5 | **噪声敏感度** 🆕 | Noise Sensitivity | 检索到无关内容时，模型是否跟着犯错 | 错误声明数/总声明数（需 GT 判断错误） | ✅ | RAGAS ✅ |
| 6 | **MRR** | Mean Reciprocal Rank | 第一个相关文档排第几位 | 1/首个相关文档排名 | ❌ | 自研 |
| 7 | **NDCG** | Normalized Discounted Cumulative Gain | 排序质量（相关的是否排前面） | DCG/IDCG，考虑位置折扣 | ❌ | 自研 |

### 2.2 生成质量指标（Generation Quality）

衡量 RAG 生成模块的效果：回答是否忠实、是否切题、是否全面、是否有幻觉。

| # | 指标 | 英文名 | 含义 | 计算方式 | 需要GT | 对应开源 |
|---|------|--------|------|----------|:------:|----------|
| 8 | **忠实度** | Faithfulness | 回答是否基于检索内容，有没有"编造" | LLM 提取声明 → 逐条验证是否被上下文支持 | ❌ | RAGAS ✅ DeepEval ✅ TruLens ✅ |
| 9 | **幻觉检测** 🆕 | Hallucination | 回答是否与上下文矛盾（反向检测） | LLM 判断每条上下文是否被回答矛盾 → 矛盾数/总数 | ❌ | DeepEval ✅ |
| 10 | **答案相关性** | Answer Relevancy | 回答是否切题 | LLM 从答案反推问题 → 原始问题与反推问题的 embedding 余弦相似度 | ❌ | RAGAS ✅ DeepEval ✅ TruLens ✅ |
| 11 | **答案正确性** | Answer Correctness | 回答跟参考答案有多接近 | 回答 vs GT 的 embedding 余弦相似度 | ✅ | 自研 |
| 12 | **答案相似度** | Answer Similarity | 两个答案的语义相似度 | embedding 余弦相似度 | ✅ | 自研 |
| 13 | **全面性** 🆕 | Comprehensiveness | 回答是否覆盖了所有重要信息 | LLM 从 GT 提取关键信息点 → 检查覆盖度 → 覆盖数/总数 | ✅ | TruLens ✅ |
| 14 | **连贯性** 🆕 | Coherence | 回答逻辑是否自洽、条理是否清晰 | LLM 评分 1-5 → 归一化到 0-1 | ❌ | TruLens ✅ |

### 2.3 综合指标

| # | 指标 | 英文名 | 含义 | 计算方式 | 需要GT | 对应开源 |
|---|------|--------|------|----------|:------:|----------|
| 15 | **RAGAS Score** | RAGAS Score | RAG 系统整体质量 | Context Precision、Context Recall、Faithfulness、Answer Relevancy 的调和平均 | ✅ | RAGAS ✅ |

---

## 三、指标关系图

```
用户 Query ──────────────────────────────────────────────────
    │                                                        │
    ▼                                                        │
┌──────────────┐                                             │
│   检索模块    │                                             │
│ (Retrieval)  │                                             │
│              │                                             │
│  检索到 N 条 │                                             │
│  上下文文档   │                                             │
└──────┬───────┘                                             │
       │                                                     │
       │ 检索质量指标（衡量检索模块）                            │
       │ ┌──────────────────────────────────────────────┐    │
       │ │ Context Precision ── 检到的准不准？            │    │
       │ │ Context Recall ───── 该检的检到了没？          │    │
       │ │ Context Relevancy ── 检到的废话多不多？        │    │
       │ │ Entity Recall ────── 关键实体检到了没？  🆕    │    │
       │ │ Noise Sensitivity ── 有噪声时跟着犯错没？🆕    │    │
       │ │ MRR ──────────────── 相关的排前面没？          │    │
       │ │ NDCG ─────────────── 排序整体质量如何？        │    │
       │ └──────────────────────────────────────────────┘    │
       │                                                     │
       ▼                                                     │
┌──────────────┐                                             │
│   生成模块    │                                             │
│ (Generation) │                                             │
│              │                                             │
│  生成回答    │                                             │
└──────┬───────┘                                             │
       │                                                     │
       │ 生成质量指标（衡量生成模块）                            │
       │ ┌──────────────────────────────────────────────┐    │
       │ │ Faithfulness ────── 回答编造了没？            │    │
       │ │ Hallucination ───── 回答跟上下文矛盾了没？🆕  │    │
       │ │ Answer Relevancy ── 回答切题吗？              │    │
       │ │ Answer Correctness ─ 跟参考答案多接近？        │    │
       │ │ Answer Similarity ── 语义相似度？              │    │
       │ │ Comprehensiveness ── 回答遗漏了没？    🆕     │    │
       │ │ Coherence ────────── 回答逻辑连贯吗？  🆕     │    │
       │ └──────────────────────────────────────────────┘    │
       │                                                     │
       ▼                                                     │
   RAGAS Score = harmonic_mean(Precision, Recall, Faithfulness, Relevancy)
```

---

## 四、Faithfulness vs Hallucination 的区别

这是最容易混淆的两个指标。它们**互补但方向相反**：

| 维度 | Faithfulness | Hallucination |
|------|-------------|---------------|
| **检测方向** | 正向验证：声明是否被上下文**支持** | 反向检测：声明是否与上下文**矛盾** |
| **满分含义** | 1.0 = 所有声明都被支持（好） | 0.0 = 没有任何矛盾（好） |
| **零分含义** | 0.0 = 没有任何声明被支持（坏） | 1.0 = 所有上下文都被矛盾（坏） |
| **检测场景** | 漏了关键信息 | 编造了错误信息 |
| **举例** | 回答说"量子计算很慢"（上下文没提到慢） → Faithfulness 低 | 回答说"量子计算只能做加法"（上下文说能做复杂运算） → Hallucination 高 |

**同时使用两个指标才能全面判断：Faithfulness 确保不遗漏，Hallucination 确保不编造。**

---

## 五、需要 Ground Truth 的指标

| 不需要 GT（生产环境可用） | 需要 GT（开发/测试环境） |
|:---|:---|
| Context Precision | Context Recall |
| Context Relevancy | Context Entity Recall 🆕 |
| MRR | Noise Sensitivity 🆕 |
| NDCG | Answer Correctness |
| Faithfulness | Answer Similarity |
| Hallucination 🆕 | Comprehensiveness 🆕 |
| Answer Relevancy | RAGAS Score |

**生产监控**：只需要 Query + Contexts + Answer，就能得到 7 个指标。
**深度评估**：加上 Ground Truth，还能额外得到 6 个指标。

---

## 六、与开源框架的覆盖度对比

```
                    RAGAS    DeepEval   TruLens    本系统
                    ─────    ────────   ────────   ──────
检索指标
  Ctx Precision      ✅        ✅          -         ✅
  Ctx Recall         ✅        ✅          -         ✅
  Ctx Relevancy      -         ✅          ✅         ✅
  Entity Recall      ✅        -           -         ✅ 🆕
  Noise Sensitivity  ✅        -           -         ✅ 🆕
  MRR                -         -           -         ✅
  NDCG               -         -           -         ✅

生成指标
  Faithfulness       ✅        ✅          ✅         ✅
  Hallucination      -         ✅          -         ✅ 🆕
  Answer Relevancy   ✅        ✅          ✅         ✅
  Answer Correctness -         -           -         ✅
  Answer Similarity  -         -           -         ✅
  Comprehensiveness  -         -           ✅         ✅ 🆕
  Coherence          -         -           ✅         ✅ 🆕

综合指标
  RAGAS Score        ✅        -           -         ✅

覆盖指标数:          7         5           5         15 ✅
```

**本系统覆盖 15 个指标，超越 RAGAS (7) + DeepEval (5) + TruLens (5) 任意单一框架。**

---

## 七、技术实现架构

### 7.1 评估流程

```
前端 Evaluate 页面
  → POST /api/v2/harness/rag/evaluate
    {query, contexts, answer, ground_truth}
  → Gateway (HTTP → gRPC)
  → Harness Service
  → RAGEvaluator.EvaluateAll()
    ├→ EvaluateContextPrecision()     ← LLM 判断每条 context 相关性
    ├→ EvaluateContextRecall()        ← embedding 相似度
    ├→ EvaluateContextRelevancy()     ← LLM 判断每句相关性
    ├→ EvaluateContextEntityRecall()  ← LLM 提取实体 → 实体覆盖率 🆕
    ├→ EvaluateNoiseSensitivity()    ← LLM 提取声明 → GT 对照 → 错误率 🆕
    ├→ EvaluateFaithfulness()        ← LLM 提取声明 → 上下文验证
    ├→ EvaluateHallucination()       ← LLM 判断是否矛盾 🆕
    ├→ EvaluateAnswerRelevancy()     ← LLM 反推问题 → embedding 相似度
    ├→ EvaluateAnswerCorrectness()   ← embedding 余弦相似度
    ├→ EvaluateComprehensiveness()   ← LLM 提取关键信息 → 覆盖率 🆕
    ├→ EvaluateCoherence()           ← LLM 评分 1-5 🆕
    ├→ calculateMRR()                ← 纯数学
    ├→ calculateNDCG()               ← 纯数学
    └→ calculateRagasScore()         ← 调和平均
  → 保存到数据库
  → 返回 pb.RAGMetrics
  → Gateway → 前端展示
```

### 7.2 关键文件

| 文件 | 作用 |
|------|------|
| `services/harness-service/internal/rag/evaluator.go` | 所有指标的计算方法（15 个评估方法） |
| `services/harness-service/internal/rag/metrics.go` | 数学计算工具（MRR, NDCG, 余弦相似度, RAGAS Score） |
| `services/harness-service/internal/rag/model.go` | 数据模型定义（RAGMetrics, EvaluationResult） |
| `services/harness-service/internal/rag/repository.go` | 数据库 CRUD（GORM + 内存模式） |
| `services/harness-service/internal/service/rag_methods.go` | gRPC service → PB 转换 |
| `proto/harness/harness.proto` | Protobuf 消息定义 |
| `services/gateway/internal/handler/harness_handler.go` | HTTP handler（超时 300s） |
| `frontend/src/api/rag.ts` | 前端 API + TypeScript 类型 |
| `frontend/src/pages/RAGMetrics/index.tsx` | Dashboard 页面 |
| `frontend/src/pages/RAGMetrics/Evaluate.tsx` | 评估页面 |
| `frontend/src/pages/RAGMetrics/Detail.tsx` | 详情页面 |

### 7.3 LLM 调用次数估算

| 指标 | LLM Chat 调用 | LLM Embed 调用 |
|------|:-------------:|:-------------:|
| Context Precision | N（每条 context 1 次） | 0 |
| Context Recall | 0 | N×GT（每对 1 次） |
| Context Relevancy | N×S（每句 1 次） | 0 |
| Entity Recall 🆕 | 2（提取实体） | 0 |
| Noise Sensitivity 🆕 | C+1（提取声明+判断） | 0 |
| Faithfulness | C+1（提取声明+逐条验证） | 0 |
| Hallucination 🆕 | N（每条 context 1 次） | 0 |
| Answer Relevancy | 1+3（生成问题） | 4（问题+GT embedding） |
| Answer Correctness | 0 | 2 |
| Comprehensiveness 🆕 | K+1（提取+逐条验证） | 0 |
| Coherence 🆕 | 1 | 0 |

**典型单次评估总 LLM 调用：~15-25 次 Chat + ~6 次 Embed**
**耗时估算：2-5 分钟（取决于 DashScope 响应速度）**

---

## 八、RAGAS Score 计算公式

```
RAGAS Score = harmonic_mean(
    Context Precision,
    Context Recall,
    Faithfulness,
    Answer Relevancy
)

调和平均 = n / Σ(1/xi)

特性：
- 任一指标为 0 → RAGAS Score = 0（最严格的综合指标）
- 所有指标必须 > 0 才能得到非零综合分
- 对低分指标惩罚更重（调和平均比算术平均更敏感）
```

---

## 九、未来扩展方向（低优先级，非 RAG 核心）

| 指标 | 优先级 | 说明 |
|------|--------|------|
| Toxicity | 🟢 低 | 内容安全，属于独立的安全评估体系 |
| Bias | 🟢 低 | 偏见检测，同上 |
| Summarization Score | 🟢 低 | 仅用于摘要场景，非通用 RAG 评估 |
| Tool Call Accuracy | 🟢 低 | Agent 评估，不属于 RAG 评估体系 |
| Agent Goal Accuracy | 🟢 低 | 同上 |
| Topic Adherence | 🟢 低 | 同上 |
| Multimodal Faithfulness | 🟢 低 | 多模态（图片+文本）场景，当前系统仅支持文本 |

---

## 十、验证方式

1. **API 测试**: `curl POST /api/v2/harness/rag/evaluate` 验证返回 15 个指标
2. **Dashboard 测试**: 访问 `/rag-metrics` 页面查看真实数据
3. **Evaluate 测试**: 填写表单 → 执行 → 显示完整指标
4. **Detail 测试**: 点击列表中的记录 → 查看完整指标详情
5. **仅 Query 评估**: 不填 Ground Truth → 应得到 7 个指标，GT 相关的 6 个不显示
6. **全填评估**: 填写所有字段 → 应得到全部 15 个指标
