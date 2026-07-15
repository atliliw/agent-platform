# RAG 评测指标体系设计文档

> 版本: v3.0 | 更新日期: 2026-07-06
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

## 三、指标详解与实例

> 用一个统一场景贯穿所有指标，帮助你直观理解每个指标在"挑什么毛病"。
>
> **统一场景：用户问"阿司匹林有什么副作用？"**

---

### 1. Context Precision（上下文精确率）

**一句话：检索回来的东西，有多少是有用的？**

🔍 举例：用户问"阿司匹林有什么副作用"，检索器返回了 5 条结果：

| # | 检索结果 | 相关？ |
|---|---------|:------:|
| 1 | 阿司匹林可能引起胃出血和胃溃疡 | ✅ |
| 2 | 阿司匹林长期服用需注意肝肾损伤 | ✅ |
| 3 | 阿司匹林最早从柳树皮中提取，历史悠久 | ❌ 废话 |
| 4 | 对乙酰氨基酚的副作用较轻，适合儿童 | ❌ 跑题 |
| 5 | 阿司匹林过敏者可能出现哮喘发作 | ✅ |

Context Precision = 3/5 = **0.6** → 5 条里有 2 条废料，及格但不好

**怎么优化**：改进检索模型，减少无关文档召回

---

### 2. Context Recall（上下文召回率）

**一句话：该找到的信息，有没有遗漏？**

🔍 举例：标准答案（GT）里提到 4 个关键副作用：胃出血、过敏反应、肝肾损伤、出血倾向。检索回来的文档只覆盖了前 3 个，"出血倾向"没有检索到。

Context Recall = 3/4 = **0.75** → 有 1 条关键信息没找到

**怎么优化**：扩大检索范围、增加 top_k、改善 query 改写

---

### 3. Context Relevancy（上下文相关性）

**一句话：检索结果中，有效内容的占比是多少？**

🔍 举例：和 Context Precision 不同，它看的是**句子级别**。检索回来一条长文档：

> "阿司匹林可能引起胃出血。这种药物最早从柳树皮提取。拜耳公司于1899年将其注册为商品名。长期服用需注意肝肾损伤风险。2015年全球阿司匹林产量达4万吨。"

5 句话里只有 2 句跟"副作用"相关（第1句、第4句）。

Context Relevancy = 2/5 = **0.4** → 虽然整篇文档算"相关"，但大半内容是废话

**怎么优化**：检索结果做精简/摘要，不要返回整篇长文档

---

### 4. Context Entity Recall（实体召回率）

**一句话：关键的人名、地名、数字、专有名词有没有检索到？**

🔍 举例：GT 中提到"拜耳公司"这个实体，但检索回来的文档都没提到拜耳公司。

GT 实体 = {阿司匹林, 胃出血, 拜耳公司, 过敏反应, 出血倾向}
检索到的实体 = {阿司匹林, 胃出血, 过敏反应, 柳树皮}

Context Entity Recall = 3/5 = **0.6** → "拜耳公司"和"出血倾向"这两个实体丢了

**和 Context Recall 的区别**：Context Recall 用语义相似度判断（"胃部出血"≈"胃出血"），Entity Recall 用精确匹配（"拜耳公司"就是"拜耳公司"，近似不算）。**法律、医疗、金融等对精确实体要求高的场景更适合用这个指标。**

---

### 5. Noise Sensitivity（噪声敏感度）

**一句话：检索结果里混了垃圾信息，模型会不会被带偏？**

🔍 举例：检索结果里有条无关文档说"对乙酰氨基酚对胃刺激小，是最安全的退烧药"。模型在回答时加了一句"阿司匹林比对乙酰氨基酚更安全"——这是错的，被无关信息误导了。

从回答中提取 5 条声明，其中 1 条是被噪声带偏产生的错误声明。

Noise Sensitivity = 1/5 = **0.2** → 还行，5 条声明里有 1 条是被垃圾信息带偏的

**越低越好**：0 = 抗干扰能力强，1 = 检索到什么就信什么

**怎么优化**：让生成模型更严格地基于相关上下文回答，减少对无关内容的注意力

---

### 6. MRR（Mean Reciprocal Rank）

**一句话：最相关的结果排第几位？**

🔍 举例：检索返回 5 条结果：

| 排名 | 内容 | 相关？ |
|------|------|:------:|
| 1 | 阿司匹林的历史... | ❌ |
| 2 | 阿司匹林可能引起胃出血 | ✅ ← 第一个相关的！ |
| 3 | ... | |
| 4 | ... | |
| 5 | ... | |

第一个相关结果排在第 2 位 → MRR = 1/2 = **0.5**

如果排在第 1 位 → MRR = 1.0（最好）
如果排在第 5 位 → MRR = 0.2（很差）
如果全都不相关 → MRR = 0.0

**生活类比**：你在百度搜索，第一个结果就是你想要的（MRR=1.0），还是翻了 5 页才找到（MRR≈0.1）？

---

### 7. NDCG（归一化折扣累积增益）

**一句话：整体排序质量如何？相关的是不是都排在前面？**

🔍 举例：和 MRR 不同，NDCG 看的是**所有结果的排序**，不只是第一个。

| 排名 | 内容 | 相关程度 |
|------|------|:--------:|
| 1 | 阿司匹林可能引起胃出血 | 高（3分）|
| 2 | 阿司匹林的历史... | 无（0分）|
| 3 | 对乙酰氨基酚副作用较轻 | 低（1分）|
| 4 | 阿司匹林过敏反应 | 高（3分）|
| 5 | 长期服用需注意肝肾损伤 | 中（2分）|

排在第 2 的是垃圾，排在第 3 的是低相关，但高相关的第 4 条被挤到后面了 → NDCG 偏低

**理想排序**应该是：高、高、中、低、无 → NDCG = 1.0

**生活类比**：你打开淘宝搜索"阿司匹林"，排在前面的全是阿司匹林（NDCG高），还是第1个是阿司匹林、第2个是感冒灵、第3个又是阿司匹林（NDCG低）？

---

### 8. Faithfulness（忠实度）

**一句话：回答里的每句话，在检索结果里都能找到依据吗？**

🔍 举例：模型回答："阿司匹林可能引起胃出血和肝肾损伤。**它不能与华法林同时服用。**"

- "引起胃出血" → 检索结果有提到 ✅
- "肝肾损伤" → 检索结果有提到 ✅
- "不能与华法林同时服用" → 检索结果**完全没提** ❌ ← 模型自己编的

Faithfulness = 2/3 = **0.67** → 有 1 条声明没有依据

**怎么优化**：在 prompt 中强调"只根据检索到的内容回答"，不要发挥

---

### 9. Hallucination（幻觉检测）

**一句话：回答有没有跟检索内容**直接冲突**？**

🔍 举例：检索结果明确说"阿司匹林建议饭后服用，可减少胃部刺激"。

模型回答："阿司匹林**空腹服用**效果更好。"

这不是"没依据"（Faithfulness 管的），而是**直接说反了**——检索说饭后，回答却说空腹。

Hallucination = 1/N（N 条检索结果中有 1 条被矛盾）→ 有矛盾

**和 Faithfulness 的区别**：
- Faithfulness：回答说"华法林不能同服"，检索没提 → **无依据但没冲突**，Faithfulness 低
- Hallucination：检索说"饭后服用"，回答却说"空腹服用" → **直接打脸**，Hallucination 高

**越低越好**：0 = 没有矛盾，1 = 全是矛盾

---

### 10. Answer Relevancy（答案相关性）

**一句话：回答有没有回答用户的问题？**

🔍 举例：用户问"阿司匹林有什么副作用？"

模型回答："阿司匹林是一种常见的非甾体抗炎药，由拜耳公司于1899年注册，广泛用于退烧和止痛。"

这个回答**关于阿司匹林**，但**没回答副作用是什么**。

Answer Relevancy 低 → 答非所问

**计算原理**：从回答反推问题——"这段回答在回答什么问题？"如果反推出来的问题是"阿司匹林是什么？"而不是"阿司匹林有什么副作用？"，说明回答跑题了。

---

### 11. Answer Correctness（答案正确性）

**一句话：回答和标准答案有多接近？**

🔍 举例：

| | 内容 |
|---|------|
| 标准答案(GT) | 阿司匹林的常见副作用包括胃出血、胃溃疡、过敏反应、肝肾损伤和出血倾向 |
| 模型回答 | 阿司匹林可能引起胃部不适、出血风险增加，少数人会出现过敏 |

语义上接近但不完全一致 → Answer Correctness ≈ **0.75**

如果模型回答和 GT 一字不差 → 1.0
如果模型回答完全不相关 → 0.0

**需要 GT**：生产环境往往没有标准答案，所以这个指标通常在离线评测集上跑。

---

### 12. Answer Similarity（答案相似度）

**一句话：两个答案在语义上有多像？**

🔍 举例：对比 A/B Test 中两个模型的回答：

- 模型A："阿司匹林可能导致胃出血和过敏"
- 模型B："阿司匹林的副作用包括胃部出血和过敏反应"

虽然用词不同，但语义几乎一样 → Answer Similarity ≈ **0.95**

**用途**：比较不同模型/不同 prompt 的输出稳定性，或者看配置变更后回答有多大变化。

---

### 13. Comprehensiveness（全面性）

**一句话：标准答案里的要点，回答有没有遗漏？**

🔍 举例：标准答案（GT）包含 5 个要点：

| # | 要点 | 回答中提到？ |
|---|------|:----------:|
| 1 | 胃出血 | ✅ |
| 2 | 胃溃疡 | ✅ |
| 3 | 过敏反应 | ✅ |
| 4 | 肝肾损伤 | ❌ 遗漏 |
| 5 | 出血倾向 | ❌ 遗漏 |

Comprehensiveness = 3/5 = **0.6** → 回答提到的都对（Faithfulness 可能很高），但漏了 2 个要点

**和 Faithfulness 的区别**：
- Faithfulness：有没有编造？ → 查"多了"
- Comprehensiveness：有没有遗漏？ → 查"少了"

**一个回答可以同时 Faithfulness=1.0（没编造）但 Comprehensiveness=0.2（大量遗漏）**。

---

### 14. Coherence（连贯性）

**一句话：回答读起来通顺吗？逻辑自洽吗？**

🔍 举例：

**高连贯性（0.9）**：
> "阿司匹林的常见副作用包括胃出血、胃溃疡和过敏反应。长期服用还可能导致肝肾损伤。如果出现黑便或胃痛，应立即停药并就医。"

**低连贯性（0.2）**：
> "副作用有胃出血。阿司匹林是1899年发明的。过敏也会发生。需要饭后服用。出血倾向。不要和华法林一起吃。"

第二个回答虽然信息可能都对，但东一榔头西一棒子，没有逻辑主线。

**不需要 GT**：只看回答本身的质量，适合在线实时监控每一条回答。

---

### 15. RAGAS Score（综合评分）

**一句话：RAG 系统整体行不行？**

4 个核心指标的**调和平均**：

```
RAGAS = 4 / (1/Precision + 1/Recall + 1/Faithfulness + 1/Relevancy)
```

🔍 举例：

| 指标 | 得分 |
|------|------|
| Context Precision | 0.9 |
| Context Recall | 0.8 |
| Faithfulness | 0.7 |
| Answer Relevancy | 0.6 |

RAGAS = 4 / (1/0.9 + 1/0.8 + 1/0.7 + 1/0.6) = 4 / (1.11 + 1.25 + 1.43 + 1.67) = 4/5.46 ≈ **0.73**

**为什么用调和平均不用算术平均？** 因为调和平均对低分惩罚更重：

- 算术平均：(0.9 + 0.8 + 0.7 + 0.6) / 4 = **0.75** → 看起来还行
- 调和平均：≈ **0.73** → 被 0.6 拉下来更多

如果任何一个是 0，调和平均直接 = 0。这迫使你**不能有短板**。

---

## 四、指标关系图

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

## 五、Faithfulness vs Hallucination 的区别

这是最容易混淆的两个指标。它们**互补但方向相反**：

| 维度 | Faithfulness | Hallucination |
|------|-------------|---------------|
| **检测方向** | 正向验证：声明是否被上下文**支持** | 反向检测：声明是否与上下文**矛盾** |
| **满分含义** | 1.0 = 所有声明都被支持（好） | 0.0 = 没有任何矛盾（好） |
| **零分含义** | 0.0 = 没有任何声明被支持（坏） | 1.0 = 所有上下文都被矛盾（坏） |
| **检测场景** | 漏了关键信息 | 编造了错误信息 |
| **举例** | 回答说"阿司匹林不能与华法林同服"（上下文没提） → Faithfulness 低 | 回答说"阿司匹林应空腹服用"（上下文说饭后服用） → Hallucination 高 |

**同时使用两个指标才能全面判断：Faithfulness 确保不遗漏，Hallucination 确保不编造。**

---

## 六、需要 Ground Truth 的指标

| 不需要 GT（生产环境可用） | 需要 GT（开发/测试环境） |
|:---|:---|
| Context Precision | Context Recall |
| Context Relevancy | Context Entity Recall 🆕 |
| MRR | Noise Sensitivity 🆕 |
| NDCG | Answer Correctness |
| Faithfulness | Answer Similarity |
| Hallucination 🆕 | Comprehensiveness 🆕 |
| Answer Relevancy | RAGAS Score |
| Coherence 🆕 | |

**生产监控**：只需要 Query + Contexts + Answer，就能得到 8 个指标。
**深度评估**：加上 Ground Truth，还能额外得到 7 个指标。

---

## 七、与开源框架的覆盖度对比

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

## 八、技术实现架构

### 8.1 评估流程

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

### 8.2 关键文件

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

### 8.3 LLM 调用次数估算

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

## 九、RAGAS Score 计算公式

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

## 十、未来扩展方向（低优先级，非 RAG 核心）

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

## 十一、验证方式

1. **API 测试**: `curl POST /api/v2/harness/rag/evaluate` 验证返回 15 个指标
2. **Dashboard 测试**: 访问 `/rag-metrics` 页面查看真实数据
3. **Evaluate 测试**: 填写表单 → 执行 → 显示完整指标
4. **Detail 测试**: 点击列表中的记录 → 查看完整指标详情
5. **仅 Query 评估**: 不填 Ground Truth → 应得到 8 个指标，GT 相关的 7 个不显示
6. **全填评估**: 填写所有字段 → 应得到全部 15 个指标
