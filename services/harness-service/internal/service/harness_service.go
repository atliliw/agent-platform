// Package service provides business logic for Harness service
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"agent-platform/pkg/agent/checkpoint"
	"agent-platform/pkg/config"
	"agent-platform/pkg/llm"
	pb "agent-platform/pkg/pb/harness"
	agentpb "agent-platform/pkg/pb/agent"
	commonpb "agent-platform/pkg/pb/common"
	"agent-platform/services/harness-service/internal/abtest"
	"agent-platform/services/harness-service/internal/coordinate"
	"agent-platform/services/harness-service/internal/cost"
	"agent-platform/services/harness-service/internal/evaluate"
	"agent-platform/services/harness-service/internal/evolve"
	"agent-platform/services/harness-service/internal/featureflag"
	"agent-platform/services/harness-service/internal/gateway"
	"agent-platform/services/harness-service/internal/goldenpath"
	"agent-platform/services/harness-service/internal/planner"
	"agent-platform/services/harness-service/internal/playground"
	"agent-platform/services/harness-service/internal/prompt"
	"agent-platform/services/harness-service/internal/rca"
	"agent-platform/services/harness-service/internal/repository"
	"agent-platform/services/harness-service/internal/rule"
	"agent-platform/services/harness-service/internal/scheduler"
	"agent-platform/services/harness-service/internal/session"
	"agent-platform/services/harness-service/internal/slo"
	"agent-platform/services/harness-service/internal/rag"
	wfengine "agent-platform/services/harness-service/internal/workflow"
)

// HarnessService provides harness functionality
type HarnessService struct {
	pb.UnimplementedHarnessServiceServer
	llmClient      llm.Client
	repo           *repository.HarnessRepository
	cfg            *config.Config
	ruleEngine     *rule.Engine
	guardrail      *rule.Guardrail
	permissions    *rule.PermissionMatrix
	evalRunner     *evaluate.Runner
	abtest         *abtest.Engine
	sloManager     *slo.Manager
	llmMetricsBuf  []llm.CallMetrics // recent LLM call metrics (ring buffer)
	// New engines
	featureFlag   *featureflag.Engine
	rca           *rca.Engine
	cost          *cost.Engine
	evolve        *evolve.Engine
	goldenpath    *goldenpath.Engine
	coordinate    *coordinate.Engine
	planner       *planner.Engine
	scheduler     *scheduler.Scheduler
	playground    *playground.PlaygroundEngine
	sessionRecorder *session.Recorder
	prompt        *prompt.Engine
	gateway       *gateway.GatewayEngine
	ragEvaluator  *rag.RAGEvaluator
	ragRepo       *rag.Repository
	checkpointStore checkpoint.CheckpointStore
	agentClient     agentpb.AgentServiceClient
	workflowRepo    *repository.WorkflowRepository
	workflowEngine  *wfengine.Engine
	mu              sync.RWMutex
}

// NewHarnessService creates a new harness service
func NewHarnessService(llmClient llm.Client, repo *repository.HarnessRepository, cfg *config.Config, agentClient agentpb.AgentServiceClient) *HarnessService {
	schedulerEngine := scheduler.NewSchedulerMemory()

	svc := &HarnessService{
		repo:          repo,
		cfg:           cfg,
		ruleEngine:    rule.NewEngine(),
		guardrail:     rule.NewGuardrail(),
		permissions:   rule.NewPermissionMatrix(),
		abtest:        abtest.NewEngine(repo.GetDB()),
		sloManager:    slo.NewManager(repo.GetDB()),
		llmMetricsBuf: make([]llm.CallMetrics, 0, 1000),
		// Initialize engines with DB persistence
		featureFlag:   featureflag.NewEngine(repo.GetDB()),
		rca:           rca.NewEngine(repo.GetDB()),
		cost:          cost.NewEngine(repo.GetDB()),
		evolve:        evolve.NewEngine(repo.GetDB()),
		goldenpath:    goldenpath.NewEngine(repo.GetDB()),
		coordinate:    coordinate.NewEngine(repo.GetDB()),
		planner:       planner.NewEngine(repo.GetDB()),
		scheduler:     schedulerEngine,
		prompt:        prompt.NewEngine(repo.GetDB()),
		}

	svc.agentClient = agentClient

	// Initialize Playground engine
	playgroundRecorder := playground.NewRecorder()
	svc.playground = playground.NewPlaygroundEngine(llmClient, playgroundRecorder)

	// Initialize Session recorder
	sessionRepo := session.NewRepository(repo.GetDB())
	if err := sessionRepo.AutoMigrate(); err != nil {
		fmt.Printf("Warning: failed to migrate session tables: %v\n", err)
	}
		// Initialize Gateway engine
	svc.sessionRecorder = session.NewRecorder(sessionRepo)

	// Migrate prompt tables (now using DB mode)
	if err := svc.prompt.AutoMigrate(); err != nil {
		fmt.Printf("Warning: failed to migrate prompt tables: %v\n", err)
	}

	// Initialize Gateway engine
		gatewayRepo := gateway.NewRepository(repo.GetDB())
		if err := gatewayRepo.AutoMigrate(); err != nil {
			fmt.Printf("Warning: failed to migrate gateway tables: %v\n", err)
		}
		svc.gateway = gateway.NewGatewayEngine(gatewayRepo)


	// Initialize RAG evaluator
	ragRepo := rag.NewRepository(repo.GetDB())
	svc.ragRepo = ragRepo
	svc.ragEvaluator = rag.NewRAGEvaluator(llmClient, ragRepo, svc.prompt)

	// Initialize checkpoint store (in-memory; replace with MongoDB for production)
	svc.checkpointStore = checkpoint.NewMemoryCheckpointStore()

	// Initialize workflow repository
	svc.workflowRepo = repository.NewWorkflowRepositoryWithDB(repo.GetDB())

	// Initialize workflow execution engine
	wfExecRepo := wfengine.NewExecutionRepositoryWithDB(repo.GetDB())
	svc.workflowEngine = wfengine.NewEngine(svc.workflowRepo, wfExecRepo, llmClient, agentClient, func(ctx context.Context, m *llm.CallMetrics) {
		inputTokens := int64(float64(m.TotalTokens) * 0.6)
		outputTokens := int64(m.TotalTokens) - inputTokens
		svc.recordLLMMetric(ctx, m, inputTokens, outputTokens)
	})

	// Wrap LLM client with metrics for automatic cost tracking
	svc.llmClient = llm.NewMetricsClient(llmClient, svc.llmMetricsCallback(), "harness")

	// Wire eval runner with metrics-wrapped LLM client
	svc.evalRunner = evaluate.NewRunner(llm.NewMetricsClient(llmClient, svc.llmMetricsCallback(), "eval"))

	// Wire AgentUpdater callback for proposal execution
	if agentClient != nil {
		svc.evolve.SetAgentUpdater(func(ctx context.Context, agentID string, updates map[string]interface{}) error {
			agentResp, err := agentClient.GetAgent(ctx, &agentpb.GetAgentRequest{AgentId: agentID})
			if err != nil {
				return fmt.Errorf("get agent %s: %w", agentID, err)
			}
			ag := agentResp.Agent
			if m, ok := updates["model"]; ok {
				if s, ok := m.(string); ok {
					ag.Model = s
				}
			}
			if t, ok := updates["temperature"]; ok {
				if f, ok := t.(float64); ok {
					ag.Temperature = f
				}
			}
			if mt, ok := updates["max_tokens"]; ok {
				if f, ok := mt.(float64); ok {
					ag.MaxTokens = int32(f)
				}
			}
			_, err = agentClient.RegisterAgent(ctx, &agentpb.RegisterAgentRequest{Agent: ag})
			if err != nil {
				return fmt.Errorf("update agent %s: %w", agentID, err)
			}
			fmt.Printf("AgentUpdater: updated agent %s with %v\n", agentID, updates)
			return nil
		})
	}

	// Wire SLO alert callback with AutoTune trigger
	var autoTuneLastRun sync.Map
	svc.sloManager.SetAlertCallback(func(alert slo.BurnRateAlert) {
		sloDef, err := svc.sloManager.GetSLO(context.Background(), alert.SLOID)
		if err != nil {
			return
		}
		fmt.Printf("SLO Alert: %s burn rate %.4f exceeds threshold %.4f (agent: %s, status: %s)\n",
			alert.Name, alert.BurnRate, alert.Threshold, sloDef.AgentID, alert.Status)

		// AutoTune: auto-generate proposal when SLO is breached
		if sloDef.AgentID == "" {
			return
		}
		// Cooldown: 1 hour per agent
		if last, ok := autoTuneLastRun.Load(sloDef.AgentID); ok {
			if time.Since(last.(time.Time)) < time.Hour {
				return
			}
		}
		metrics := svc.computeMetricsFromBuffer(sloDef.AgentID)
		proposal, err := svc.evolve.AutoTune(context.Background(), sloDef.AgentID, metrics)
		if err != nil {
			fmt.Printf("AutoTune failed for agent %s: %v\n", sloDef.AgentID, err)
			return
		}
		autoTuneLastRun.Store(sloDef.AgentID, time.Now())
		fmt.Printf("AutoTune: generated proposal %s for agent %s (type: %s)\n", proposal.ID, sloDef.AgentID, proposal.Type)
	})

	// Wire scheduler eval runner with SLO evaluation
	schedulerEngine.SetEvalRunner(func(ctx context.Context, evalType scheduler.EvalType, agentID string) (*scheduler.EvalResult, error) {
		switch evalType {
		case scheduler.EvalTypeSLO, scheduler.EvalTypeAll:
			results, err := svc.sloManager.EvaluateAll(ctx, agentID)
			if err != nil {
				return nil, err
			}
			var worstBudget float64 = 1.0
			var alerts []string
			for _, r := range results {
				if r.ErrorBudget < worstBudget {
					worstBudget = r.ErrorBudget
				}
				if r.Status != slo.StatusHealthy {
					alerts = append(alerts, fmt.Sprintf("%s: %s (budget: %.1f%%)", r.Name, r.Status, r.ErrorBudget*100))
				}
			}
			return &scheduler.EvalResult{
				EvalType: evalType,
				Success:  worstBudget > 0.1,
				Score:    worstBudget,
				Details:  fmt.Sprintf("SLO evaluation: %d SLOs checked, worst budget %.1f%%", len(results), worstBudget*100),
				Alerts:   alerts,
			}, nil

		case scheduler.EvalTypeABTest:
			experiments, err := svc.abtest.ListExperiments(ctx, agentID, "", abtest.StatusRunning)
			if err != nil {
				return nil, fmt.Errorf("list ab tests: %w", err)
			}
			if len(experiments) == 0 {
				return &scheduler.EvalResult{
					EvalType: evalType,
					Success:  true,
					Score:    1.0,
					Details:  "No active A/B tests to evaluate",
				}, nil
			}
			var significantCount int
			var alerts []string
			for _, exp := range experiments {
				stats, err := svc.abtest.Evaluate(ctx, exp.ID)
				if err != nil {
					alerts = append(alerts, fmt.Sprintf("%s: evaluation error: %v", exp.Name, err))
					continue
				}
				if stats.Significant {
					significantCount++
					alerts = append(alerts, fmt.Sprintf("%s: significant (p=%.4f, action=%s, delta=%.1f%%)", exp.Name, stats.PValue, stats.RecommendedAction, stats.DeltaPercent))
				}
			}
			return &scheduler.EvalResult{
				EvalType: evalType,
				Success:  true,
				Score:    float64(significantCount) / float64(len(experiments)),
				Details:  fmt.Sprintf("A/B test evaluation: %d active, %d reached significance", len(experiments), significantCount),
				Alerts:   alerts,
			}, nil

		case scheduler.EvalTypeFeatureFlag:
			staleFlags, err := svc.featureFlag.DetectStaleFlags(ctx, 30*24*time.Hour)
			if err != nil {
				return nil, fmt.Errorf("detect stale flags: %w", err)
			}
			allFlags, _ := svc.featureFlag.ListFlags(ctx, "", featureflag.FlagStatusActive)
			totalFlags := len(allFlags)
			if totalFlags == 0 {
				totalFlags = 1
			}
			var alerts []string
			for _, f := range staleFlags {
				alerts = append(alerts, fmt.Sprintf("Stale flag: %s (last used: %s)", f.Key, f.LastUsed.Format("2006-01-02")))
			}
			return &scheduler.EvalResult{
				EvalType: evalType,
				Success:  len(staleFlags) == 0,
				Score:    1.0 - float64(len(staleFlags))/float64(totalFlags),
				Details:  fmt.Sprintf("Feature flag scan: %d total, %d stale (>30 days unused)", totalFlags, len(staleFlags)),
				Alerts:   alerts,
			}, nil

		case scheduler.EvalTypeCost:
			now := time.Now()
			start := now.AddDate(0, 0, -7)
			report, err := svc.cost.CostReport(ctx, agentID, start, now)
			if err != nil {
				return nil, fmt.Errorf("cost report: %w", err)
			}
			recommendations, _ := svc.cost.Recommendations(ctx)
			var alerts []string
			for _, rec := range recommendations {
				alerts = append(alerts, fmt.Sprintf("%s: %s (saving: $%.2f)", rec.Type, rec.Title, rec.PotentialSavings))
			}
			return &scheduler.EvalResult{
				EvalType: evalType,
				Success:  len(recommendations) == 0,
				Score:    1.0 - report.TotalCost/100.0,
				Details:  fmt.Sprintf("Cost analysis: 7-day total $%.4f, %d recommendations", report.TotalCost, len(recommendations)),
				Alerts:   alerts,
			}, nil

		default:
			return &scheduler.EvalResult{
				EvalType: evalType,
				Success:  true,
				Score:    0.85,
				Details:  fmt.Sprintf("Evaluation type %s not yet implemented", evalType),
			}, nil
		}
	})

	// Initialize default data (SLOs, Feature Flags, Schedules)
	svc.initializeDefaults(context.Background())

	return svc
}

// initializeDefaults creates default SLOs, Feature Flags, and Schedules on startup
func (s *HarnessService) initializeDefaults(ctx context.Context) {
	fmt.Println("[Harness] Initializing default governance configurations...")

	// 1. Create default SLOs for main-agent
	// Note: For Latency SLO, Target is in milliseconds (e.g., 2000ms = 2 seconds)
	defaultSLOs := []struct {
		Name    string
		AgentID string
		Target  float64
		Type    slo.SLOType
	}{
		{"Latency P95 < 2s", "main-agent", 2000, slo.SLOTypeLatency}, // Target: 2000ms
		{"Success Rate > 99%", "main-agent", 0.99, slo.SLOTypeSuccessRate},
		{"Availability > 99.9%", "main-agent", 0.999, slo.SLOTypeAvailability},
	}

	for _, sloDef := range defaultSLOs {
		existing, _ := s.sloManager.ListSLOs(ctx, sloDef.AgentID, "")
		found := false
		for _, e := range existing {
			if e.Name == sloDef.Name {
				found = true
				break
			}
		}
		if !found {
			if err := s.sloManager.CreateSLO(ctx, &slo.SLODefinition{
				AgentID: sloDef.AgentID,
				Name:    sloDef.Name,
				Target:  sloDef.Target,
				Type:    sloDef.Type,
			}); err == nil {
				// Format target display based on SLO type
				if sloDef.Type == slo.SLOTypeLatency {
					fmt.Printf("[Harness] Created default SLO: %s (agent: %s, target: %.0fms)\n", sloDef.Name, sloDef.AgentID, sloDef.Target)
				} else {
					fmt.Printf("[Harness] Created default SLO: %s (agent: %s, target: %.1f%%)\n", sloDef.Name, sloDef.AgentID, sloDef.Target*100)
				}
			}
		}
	}

	// 2. Create default Feature Flags
	defaultFlags := []struct {
		Key         string
		Name        string
		Description string
		Type        string
		Value       string
	}{
		{"enable_streaming", "Enable Streaming Response", "Enable streaming response for chat", "boolean", "true"},
		{"enable_multimodal", "Enable Multimodal Input", "Enable image and file input", "boolean", "true"},
		{"max_context_tokens", "Max Context Tokens", "Maximum context window size", "number", "4096"},
	}

	for _, flag := range defaultFlags {
		if _, err := s.featureFlag.GetFlag(ctx, flag.Key); err != nil {
			if err := s.featureFlag.CreateFlag(ctx, &featureflag.FeatureFlag{
				Key:         flag.Key,
				Name:        flag.Name,
				Description: flag.Description,
				Type:        flag.Type,
				Value:       flag.Value,
				Status:      featureflag.FlagStatusActive,
			}); err == nil {
				fmt.Printf("[Harness] Created default Feature Flag: %s\n", flag.Key)
			}
		}
	}

	// 3. Create default SLO evaluation schedule
	schedules, _ := s.scheduler.ListSchedules(ctx, "", "")
	if len(schedules) == 0 {
		if err := s.scheduler.SetEvalSchedule(ctx, &scheduler.EvalSchedule{
			ID:           "slo-monitor-default",
			Name:         "SLO Monitoring - Every 5 Minutes",
			Type:         scheduler.ScheduleTypeInterval,
			EvalType:     scheduler.EvalTypeSLO,
			AgentID:      "", // All agents
			ScheduleExpr: "5m",
			Enabled:      true,
		}); err == nil {
			fmt.Println("[Harness] Created default SLO evaluation schedule (every 5 minutes)")
		}
	}

	// 4. Start the scheduler automatically
	if err := s.scheduler.Start(ctx); err == nil {
		fmt.Println("[Harness] Scheduler started automatically")
	}

	// 5. Seed default prompt templates
	s.initializeDefaultPrompts(ctx)

		// 6. Seed default Gateway providers and routes
		s.initializeDefaultGateway(ctx)

	fmt.Println("[Harness] Default governance configurations initialized")
}

// initializeDefaultPrompts seeds real prompt templates from the system's existing codebase
func (s *HarnessService) initializeDefaultPrompts(ctx context.Context) {
	type templateDef struct {
		Key         string
		Name        string
		Description string
		Category    prompt.PromptCategory
		Tags        string
		Version     string
		Content     string
		Variables   string
	}

	defaultTemplates := []templateDef{
		// ---- system: Agent 系统指令 (from pkg/agent/defaults.go) ----
		{
			Key:         "agent-main-dispatch",
			Name:        "主调度 Agent",
			Description: "主调度 Agent 系统指令，负责理解用户意图并分配任务给专业 Agent",
			Category:    prompt.CategorySystem,
			Tags:        `["system","agent","调度","中文"]`,
			Version:     "1.0.0",
			Content: `你是一个智能调度助手。根据用户请求，决定应该交给哪个专业 Agent 处理：

- Researcher Agent: 研究类任务，如搜索信息、查找资料
- Coder Agent: 编程类任务，如写代码、调试程序
- Analyst Agent: 分析类任务，如数据分析、生成报告
{{extra_agents|}}

分析用户请求，选择最合适的 Agent 进行交接。`,
			Variables: `{"variables":[{"name":"extra_agents","type":"string","required":false,"default":"","description":"额外可调度的 Agent 列表"}]}`,
		},
		{
			Key:         "agent-researcher",
			Name:        "研究 Agent",
			Description: "研究 Agent 系统指令，负责信息搜索和知识检索",
			Category:    prompt.CategorySystem,
			Tags:        `["system","agent","研究","搜索"]`,
			Version:     "1.0.0",
			Content: `你是一个专业的研究助手。你可以：
1. 使用 web_search 工具搜索网络信息
2. 使用 knowledge_search 工具检索知识库
{{extra_tools|}}

根据用户的问题，进行全面的信息收集，并整理成清晰的答案。`,
			Variables: `{"variables":[{"name":"extra_tools","type":"string","required":false,"default":"","description":"额外可用工具描述"}]}`,
		},
		{
			Key:         "agent-coder",
			Name:        "编程 Agent",
			Description: "编程 Agent 系统指令，负责代码编写和执行",
			Category:    prompt.CategorySystem,
			Tags:        `["system","agent","编程","代码"]`,
			Version:     "1.0.0",
			Content: `你是一个专业的编程助手。你可以：
1. 使用 code_execute 工具执行代码
2. 使用 file_read/file_write 工具读写文件
{{extra_tools|}}

根据用户的需求，编写、调试或优化代码。`,
			Variables: `{"variables":[{"name":"extra_tools","type":"string","required":false,"default":"","description":"额外可用工具描述"}]}`,
		},
		{
			Key:         "agent-analyst",
			Name:        "分析 Agent",
			Description: "分析 Agent 系统指令，负责数据分析和可视化",
			Category:    prompt.CategorySystem,
			Tags:        `["system","agent","分析","数据"]`,
			Version:     "1.0.0",
			Content: `你是一个专业的数据分析助手。你可以：
1. 使用 data_analysis 工具进行数据分析
2. 使用 visualization 工具生成图表
{{extra_tools|}}

根据用户提供的数据，进行深入分析并生成洞察报告。`,
			Variables: `{"variables":[{"name":"extra_tools","type":"string","required":false,"default":"","description":"额外可用工具描述"}]}`,
		},

		// ---- system: 浏览器自动化 (from pkg/browseragent/agent.go) ----
		{
			Key:         "agent-browser",
			Name:        "浏览器自动化 Agent",
			Description: "浏览器自动化 Agent 系统指令，负责网页操作和数据采集",
			Category:    prompt.CategorySystem,
			Tags:        `["system","agent","浏览器","自动化"]`,
			Version:     "1.0.0",
			Content: `你是一个浏览器自动化助手。你的任务是操控浏览器完成用户请求。

## 你可以执行的动作

1. navigate(url) - 导航到指定 URL
   示例: {"action": "navigate", "url": "https://www.baidu.com"}

2. click(element_index) - 点击元素（根据元素列表中的索引）
   示例: {"action": "click", "element": 5}

3. type(element_index, text) - 在元素中输入文本
   示例: {"action": "type", "element": 3, "text": "{{search_text|关键词}}"}

4. scroll(direction) - 滚动页面 (up/down)
   示例: {"action": "scroll", "direction": "down"}

5. wait(seconds) - 等待页面加载
   示例: {"action": "wait", "seconds": 2}

6. execute_js(javascript) - 执行JavaScript代码
   示例: {"action": "execute_js", "javascript": "document.querySelector('#txtTitle').value='标题'"}

7. done(result) - 任务完成，返回结果
   示例: {"action": "done", "result": "完成"}

## 重要规则

1. 每次只执行一个动作，以 JSON 格式回复
2. 根据页面上的可交互元素列表，选择正确的元素索引
3. 如果看到弹窗或新页面，继续操作直到完成任务
4. 不要重复执行相同操作
5. 任务完成后调用 done 返回结果`,
			Variables: `{"variables":[{"name":"search_text","type":"string","required":false,"default":"关键词","description":"默认搜索关键词示例"}]}`,
		},

		// ---- system: 反思系统 (from pkg/agent/reflection/loop.go) ----
		{
			Key:         "agent-reflection",
			Name:        "Agent 自我反思",
			Description: "Agent 执行过程自我反思模板，用于行动前后及任务完成时分析",
			Category:    prompt.CategorySystem,
			Tags:        `["system","agent","反思","质量"]`,
			Version:     "1.0.0",
			Content: `你是一个 AI Agent 自我反思专家。请对以下执行过程进行深入反思和分析。

## 反思阶段：{{phase|任务完成反思}}

## 任务信息
- 任务: {{task}}
- 目标: {{goal}}
- 执行成功: {{success|true}}

## 执行指标
- Token使用: {{token_usage|0}}
- 执行时间: {{elapsed_ms|0}}ms

请以 JSON 格式输出反思结果：
{
  "score": 0.85,
  "strengths": ["优点1", "优点2"],
  "weaknesses": ["缺点1", "缺点2"],
  "suggestions": ["建议1", "建议2"],
  "lessons_learned": ["经验1", "经验2"],
  "alternative_actions": ["替代方案1", "替代方案2"],
  "confidence": 0.9,
  "analysis": "详细分析..."
}`,
			Variables: `{"variables":[{"name":"phase","type":"string","required":false,"default":"任务完成反思","description":"反思阶段：行动前反思/行动后反思/任务完成反思/错误反思"},{"name":"task","type":"string","required":true,"description":"任务描述"},{"name":"goal","type":"string","required":true,"description":"任务目标"},{"name":"success","type":"string","required":false,"default":"true","description":"是否执行成功"},{"name":"token_usage","type":"string","required":false,"default":"0","description":"Token使用量"},{"name":"elapsed_ms","type":"string","required":false,"default":"0","description":"执行时间(ms)"}]}`,
		},

		// ---- system: 评估打分 (from services/harness-service/internal/evaluate/evaluate.go) ----
		{
			Key:         "eval-quality-score",
			Name:        "回答质量评估",
			Description: "AI 回答质量评估模板，从忠实度、相关性、精确度、幻觉四个维度打分",
			Category:    prompt.CategorySystem,
			Tags:        `["system","评估","打分","质量"]`,
			Version:     "1.0.0",
			Content: `你是一个严格的 AI 回答质量评估专家。请对以下回答进行评分。

用户问题: {{input}}

AI 回答: {{output}}

参考答案: {{expected}}

请从以下四个维度分别打分（0-1的浮点数），并以 JSON 格式输出：
{"faithfulness": 0.8, "relevancy": 0.9, "precision": 0.7, "hallucination": 0.1}

评分标准：
- faithfulness: 回答与参考答案的一致性（关键信息是否匹配）
- relevancy: 回答与问题的相关性（是否偏题）
- precision: 回答的精确度和完整性（是否遗漏关键点）
- hallucination: 回答中包含参考答案之外的错误信息的程度（0=无幻觉，1=严重幻觉）

只输出 JSON，不要其他解释。`,
			Variables: `{"variables":[{"name":"input","type":"string","required":true,"description":"用户问题"},{"name":"output","type":"string","required":true,"description":"AI回答"},{"name":"expected","type":"string","required":true,"description":"参考答案"}]}`,
		},

		// ---- agent: Prompt 优化 (from pkg/agent/optimization/prompt_gen.go) ----
		{
			Key:         "agent-prompt-task",
			Name:        "任务型 Prompt 生成",
			Description: "根据角色和任务生成结构化的任务型 Prompt 模板",
			Category:    prompt.CategoryAgent,
			Tags:        `["agent","prompt","优化","任务"]`,
			Version:     "1.0.0",
			Content: `你是一个专业的 {{role|AI 助手}}。

任务：{{task}}

请按照以下步骤完成任务：
1. 分析任务需求
2. 制定执行计划
3. 执行并验证结果

输出格式：
{{output_format|文本格式}}`,
			Variables: `{"variables":[{"name":"role","type":"string","required":false,"default":"AI 助手","description":"Agent角色"},{"name":"task","type":"string","required":true,"description":"任务描述"},{"name":"output_format","type":"string","required":false,"default":"文本格式","description":"输出格式要求"}]}`,
		},
		{
			Key:         "agent-prompt-reasoning",
			Name:        "推理型 Prompt 生成",
			Description: "用于需要逐步推理和分析的问题",
			Category:    prompt.CategoryAgent,
			Tags:        `["agent","prompt","优化","推理"]`,
			Version:     "1.0.0",
			Content: `请分析以下问题：

{{problem}}

分析步骤：
1. 理解问题
2. 识别关键因素
3. 推理过程
4. 得出结论

请逐步思考并给出推理过程。`,
			Variables: `{"variables":[{"name":"problem","type":"string","required":true,"description":"需要推理分析的问题"}]}`,
		},

		// ---- rag: RAG 评估 (from services/harness-service/internal/rag/evaluator.go) ----
		{
			Key:         "rag-relevance-check",
			Name:        "RAG 相关性判断",
			Description: "判断检索到的上下文是否与查询相关",
			Category:    prompt.CategoryRAG,
			Tags:        `["rag","相关性","检索","评估"]`,
			Version:     "1.0.0",
			Content: `You are a relevance evaluator. Determine if the given context is relevant to the query.
Answer only "yes" or "no".

Query: {{query}}

Context: {{context}}

Is this context relevant to the query? Answer only "yes" or "no".`,
			Variables: `{"variables":[{"name":"query","type":"string","required":true,"description":"用户查询"},{"name":"context","type":"string","required":true,"description":"检索到的上下文"}]}`,
		},
		{
			Key:         "rag-fact-verify",
			Name:        "RAG 事实核查",
			Description: "验证声明是否被上下文支持",
			Category:    prompt.CategoryRAG,
			Tags:        `["rag","事实","核查","验证"]`,
			Version:     "1.0.0",
			Content: `You are a fact checker. Determine if the claim is supported by the provided context.
Answer only "yes" or "no".

Claim: {{claim}}

Context: {{context}}

Is this claim supported by the context? Answer only "yes" or "no".`,
			Variables: `{"variables":[{"name":"claim","type":"string","required":true,"description":"需要验证的声明"},{"name":"context","type":"string","required":true,"description":"参考上下文"}]}`,
		},
		{
			Key:         "rag-question-generate",
			Name:        "RAG 问题生成",
			Description: "根据答案反向生成可能的问题，用于评估检索质量",
			Category:    prompt.CategoryRAG,
			Tags:        `["rag","问题生成","评估"]`,
			Version:     "1.0.0",
			Content: `You are a question generator. Given an answer, generate {{num_questions|3}} potential questions that this answer could address.
Return each question on a separate line, numbered 1-{{num_questions|3}}. Be specific and relevant.

Answer: {{answer}}`,
			Variables: `{"variables":[{"name":"answer","type":"string","required":true,"description":"参考答案"},{"name":"num_questions","type":"string","required":false,"default":"3","description":"生成问题数量"}]}`,
		},

		// ---- rag: RAG 回答生成 ----
		{
			Key:         "rag-answer-generate",
			Name:        "RAG 回答生成",
			Description: "基于检索到的上下文文档生成回答",
			Category:    prompt.CategoryRAG,
			Tags:        `["rag","生成","回答","中文"]`,
			Version:     "1.0.0",
			Content: `请根据以下上下文信息回答用户问题。

上下文文档：
{{context}}

用户问题：{{question}}

要求：
- 仅根据提供的上下文回答问题
- 如果上下文中没有足够信息，请说明"根据现有信息无法回答该问题"
- 标注引用的具体来源
- 回答要准确、简洁

{{format_instruction|}}`,
			Variables: `{"variables":[{"name":"question","type":"string","required":true,"description":"用户问题"},{"name":"context","type":"string","required":true,"description":"检索到的上下文文档"},{"name":"format_instruction","type":"string","required":false,"default":"","description":"额外的格式要求"}]}`,
		},

		// ---- template: 安全护栏 ----
		{
			Key:         "template-safety",
			Name:        "安全护栏",
			Description: "系统级安全策略和内容审核指令",
			Category:    prompt.CategoryTemplate,
			Tags:        `["template","安全","护栏","审核"]`,
			Version:     "1.0.0",
			Content: `你必须始终遵守以下安全准则：

1. 不生成宣扬暴力、自残或违法活动的内容
2. 不提供制造武器、爆炸物或危险物质的指导
3. 不生成仇恨言论、歧视性内容或人身攻击
4. 尊重知识产权——不逐字复制受版权保护的材料
5. 不提供医疗诊断或治疗建议——始终建议咨询专业人士
6. 不协助黑客攻击、漏洞利用或未经授权访问系统
7. 拒绝可能造成现实伤害的请求

如果用户请求违反上述准则，请礼貌拒绝并说明原因。

{{additional_rules|}}`,
			Variables: `{"variables":[{"name":"additional_rules","type":"string","required":false,"default":"","description":"额外安全规则"}]}`,
		},
	}

	seeded := 0
	for _, tmpl := range defaultTemplates {
		// Check if prompt already exists (skip on restart)
		if _, err := s.prompt.GetPrompt(ctx, tmpl.Key); err == nil {
			continue
		}

		// Create the prompt
		p := &prompt.Prompt{
			Key:         tmpl.Key,
			Name:        tmpl.Name,
			Description: tmpl.Description,
			Category:    tmpl.Category,
			Tags:        tmpl.Tags,
			CreatedBy:   "system",
		}
		if err := s.prompt.CreatePrompt(ctx, p); err != nil {
			fmt.Printf("[Harness] Failed to seed prompt %s: %v\n", tmpl.Key, err)
			continue
		}

		// Create the initial active version
		v := &prompt.PromptVersion{
			PromptID:  p.ID,
			Version:   tmpl.Version,
			Content:   tmpl.Content,
			Variables: tmpl.Variables,
			Status:    prompt.VersionStatusActive,
			IsActive:  true,
			CreatedBy: "system",
		}
		if err := s.prompt.CreateVersion(ctx, v); err != nil {
			fmt.Printf("[Harness] Failed to seed version for %s: %v\n", tmpl.Key, err)
			continue
		}

		// Activate the version
		if err := s.prompt.ActivateVersion(ctx, v.ID); err != nil {
			fmt.Printf("[Harness] Failed to activate version for %s: %v\n", tmpl.Key, err)
		}

		seeded++
	}

	if seeded > 0 {
		fmt.Printf("[Harness] Seeded %d default prompt templates\n", seeded)
	}
}

// CreateRule creates a rule
func (s *HarnessService) CreateRule(ctx context.Context, req *pb.CreateRuleRequest) (*pb.Rule, error) {
	r := &repository.Rule{
		AgentID:  req.AgentId,
		Name:     req.Name,
		Type:     req.Type,
		Config:   req.Config,
		Enabled:  req.Enabled,
		TenantID: req.TenantId,
	}

	if err := s.repo.CreateRule(ctx, r); err != nil {
		return nil, err
	}

	return &pb.Rule{
		Id:        r.ID,
		AgentId:   r.AgentID,
		Name:      r.Name,
		Type:      r.Type,
		Config:    r.Config,
		Enabled:   r.Enabled,
		CreatedAt: r.CreatedAt.Unix(),
	}, nil
}

// ListRules lists rules
func (s *HarnessService) ListRules(ctx context.Context, req *pb.ListRulesRequest) (*pb.ListRulesResponse, error) {
	rules, err := s.repo.ListRules(ctx, req.AgentId, req.TenantId)
	if err != nil {
		return nil, err
	}

	var pbRules []*pb.Rule
	for _, r := range rules {
		pbRules = append(pbRules, &pb.Rule{
			Id:        r.ID,
			AgentId:   r.AgentID,
			Name:      r.Name,
			Type:      r.Type,
			Config:    r.Config,
			Enabled:   r.Enabled,
			CreatedAt: r.CreatedAt.Unix(),
		})
	}

	return &pb.ListRulesResponse{Rules: pbRules}, nil
}

// UpdateRule updates a rule
func (s *HarnessService) UpdateRule(ctx context.Context, req *pb.UpdateRuleRequest) (*pb.Rule, error) {
	r := &repository.Rule{
		ID:       req.Id,
		AgentID:  req.AgentId,
		Name:     req.Name,
		Type:     req.Type,
		Config:   req.Config,
		Enabled:  req.Enabled,
		TenantID: req.TenantId,
	}

	if err := s.repo.UpdateRule(ctx, r); err != nil {
		return nil, err
	}

	return &pb.Rule{
		Id:        r.ID,
		AgentId:   r.AgentID,
		Name:      r.Name,
		Type:      r.Type,
		Config:    r.Config,
		Enabled:   r.Enabled,
	}, nil
}

// DeleteRule deletes a rule
func (s *HarnessService) DeleteRule(ctx context.Context, req *pb.DeleteRuleRequest) (*commonpb.Empty, error) {
	if err := s.repo.DeleteRule(ctx, req.Id, req.TenantId); err != nil {
		return nil, err
	}
	return &commonpb.Empty{}, nil
}

// CheckGuardrail checks guardrail
func (s *HarnessService) CheckGuardrail(ctx context.Context, req *pb.GuardrailCheckRequest) (*pb.GuardrailCheckResponse, error) {
	violations := s.guardrail.Check(req.Content, req.Type)
	return &pb.GuardrailCheckResponse{
		Passed:     len(violations) == 0,
		Violations: violations,
	}, nil
}

// CreateEvalSuite creates an eval suite
func (s *HarnessService) CreateEvalSuite(ctx context.Context, req *pb.CreateEvalSuiteRequest) (*pb.EvalSuite, error) {
	suite := &repository.EvalSuite{
		Name:        req.Name,
		Description: req.Description,
		TenantID:    req.TenantId,
	}

	if err := s.repo.CreateEvalSuite(ctx, suite); err != nil {
		return nil, err
	}

	return &pb.EvalSuite{
		Id:          suite.ID,
		Name:        suite.Name,
		Description: suite.Description,
		CreatedAt:   suite.CreatedAt.Unix(),
	}, nil
}

// RunEval runs evaluation
func (s *HarnessService) RunEval(ctx context.Context, req *pb.RunEvalRequest) (*pb.RunEvalResponse, error) {
	results, avgScore, err := s.evalRunner.Run(ctx, req.SuiteId, req.Model)
	if err != nil {
		return nil, err
	}

	var pbResults []*pb.EvalResult
	for _, r := range results {
		pbResults = append(pbResults, &pb.EvalResult{
			CaseId: r.CaseID,
			Actual: r.Actual,
			Score:  r.Score,
			Passed: r.Passed,
		})
	}

	// Record SLO metrics for eval
	return &pb.RunEvalResponse{
		RunId:              fmt.Sprintf("%d", time.Now().UnixNano()),
		Results:            pbResults,
		AvgScore:           avgScore,
		RegressionDetected: avgScore < 0.7,
	}, nil
}

// GetEvalResults gets eval results
func (s *HarnessService) GetEvalResults(ctx context.Context, req *pb.GetEvalResultsRequest) (*pb.RunEvalResponse, error) {
	return &pb.RunEvalResponse{}, nil
}

// CreateABTest creates an A/B test
func (s *HarnessService) CreateABTest(ctx context.Context, req *pb.CreateABTestRequest) (*pb.ABTest, error) {
	test := &repository.ABTest{
		Name:          req.Name,
		ControlModel:  req.ControlModel,
		VariantModel:  req.VariantModel,
		TrafficSplit:  req.TrafficSplit,
		AgentID:       req.AgentId,
		TenantID:      req.TenantId,
		Status:        "running",
		Type:          req.Type,
		ControlConfig: req.ControlConfig,
		VariantConfig: req.VariantConfig,
	}

	if err := s.repo.CreateABTest(ctx, test); err != nil {
		return nil, err
	}

	return &pb.ABTest{
		Id:            test.ID,
		Name:          test.Name,
		ControlModel:  test.ControlModel,
		VariantModel:  test.VariantModel,
		TrafficSplit:  test.TrafficSplit,
		Status:        test.Status,
		CreatedAt:     test.CreatedAt.Unix(),
		Type:          test.Type,
		ControlConfig: test.ControlConfig,
		VariantConfig: test.VariantConfig,
		AgentId:       test.AgentID,
	}, nil
}

// ListABTests lists A/B tests
func (s *HarnessService) ListABTests(ctx context.Context, req *pb.ListABTestsRequest) (*pb.ListABTestsResponse, error) {
	tests, err := s.repo.ListABTests(ctx, req.AgentId, req.TenantId, req.Status)
	if err != nil {
		return nil, err
	}

	var pbTests []*pb.ABTest
	for _, t := range tests {
		pbTests = append(pbTests, &pb.ABTest{
			Id:            t.ID,
			Name:          t.Name,
			ControlModel:  t.ControlModel,
			VariantModel:  t.VariantModel,
			TrafficSplit:  t.TrafficSplit,
			Status:        t.Status,
			CreatedAt:     t.CreatedAt.Unix(),
			Type:          t.Type,
			ControlConfig: t.ControlConfig,
			VariantConfig: t.VariantConfig,
			AgentId:       t.AgentID,
		})
	}

	return &pb.ListABTestsResponse{Tests: pbTests}, nil
}

// ShouldUseVariant determines if a request should use variant
func (s *HarnessService) ShouldUseVariant(ctx context.Context, req *pb.ShouldUseVariantRequest) (*pb.ShouldUseVariantResponse, error) {
	isVariant, err := s.abtest.ShouldUseVariant(ctx, req.ExperimentId, req.SessionId)
	if err != nil {
		return &pb.ShouldUseVariantResponse{IsVariant: false}, nil
	}
	return &pb.ShouldUseVariantResponse{IsVariant: isVariant}, nil
}

// RecordABTestResult records A/B test result
func (s *HarnessService) RecordABTestResult(ctx context.Context, req *pb.RecordABTestResultRequest) (*commonpb.Empty, error) {
	err := s.abtest.RecordResult(ctx, req.ExperimentId, req.SessionId, req.IsVariant, req.Score, req.LatencyMs, req.Success)
	if err != nil {
		fmt.Printf("Warning: failed to record A/B test result: %v\n", err)
	}
	return &commonpb.Empty{}, nil
}

// GetABTestResult gets A/B test result
func (s *HarnessService) GetABTestResult(ctx context.Context, req *pb.GetABTestResultRequest) (*pb.ABTestResult, error) {
	stats, err := s.abtest.Evaluate(ctx, req.TestId)
	if err != nil {
		return nil, err
	}
	return &pb.ABTestResult{
		ControlScore: stats.ControlMean,
		VariantScore: stats.VariantMean,
		Delta:        stats.Delta,
		PValue:       stats.PValue,
		Significant:  stats.Significant,
		Recommended:  stats.RecommendedAction,
	}, nil
}

// DeleteABTest deletes an A/B test
func (s *HarnessService) DeleteABTest(ctx context.Context, req *pb.PromoteVariantRequest) (*commonpb.Empty, error) {
	// Delete from repository (SQLite)
	if err := s.repo.DeleteABTest(ctx, req.TestId, req.TenantId); err != nil {
		return nil, fmt.Errorf("delete ab test from repository: %w", err)
	}
	// Delete from abtest engine (in-memory + DB if present)
	if err := s.abtest.Delete(ctx, req.TestId); err != nil {
		// Log but dont fail - the engine might not have this experiment
		fmt.Printf("Warning: failed to delete ab test from engine: %v\n", err)
	}
	return &commonpb.Empty{}, nil
}

// PromoteVariant promotes variant
func (s *HarnessService) PromoteVariant(ctx context.Context, req *pb.PromoteVariantRequest) (*commonpb.Empty, error) {
	if err := s.abtest.Promote(ctx, req.TestId); err != nil {
		return nil, err
	}
	return &commonpb.Empty{}, nil
}

// CreateSLO creates an SLO
func (s *HarnessService) CreateSLO(ctx context.Context, req *pb.CreateSLORequest) (*pb.SLO, error) {
	sloDef := &slo.SLODefinition{
		AgentID:  req.AgentId,
		Name:     req.Name,
		Target:   req.Target,
		Type:     slo.SLOType(req.Type),
		TenantID: req.TenantId,
	}

	if err := s.sloManager.CreateSLO(ctx, sloDef); err != nil {
		return nil, err
	}

	return &pb.SLO{
		Id:        sloDef.ID,
		AgentId:   sloDef.AgentID,
		Name:      sloDef.Name,
		Target:    sloDef.Target,
		Type:      string(sloDef.Type),
		CreatedAt: time.Now().Unix(),
	}, nil
}

// GetSLOStatus gets SLO status
func (s *HarnessService) GetSLOStatus(ctx context.Context, req *pb.GetSLOStatusRequest) (*pb.GetSLOStatusResponse, error) {
	results, err := s.sloManager.EvaluateAll(ctx, req.AgentId)
	if err != nil {
		return nil, err
	}

	var statuses []*pb.SLOStatus
	for _, r := range results {
		statuses = append(statuses, &pb.SLOStatus{
			Name:            r.Name,
			Current:         r.Current,
			Target:          r.Target,
			BudgetRemaining: r.ErrorBudget,
			Status:          string(r.Status),
		})
	}

	return &pb.GetSLOStatusResponse{Statuses: statuses}, nil
}

// Chat handles harness chat with full governance pipeline
func (s *HarnessService) Chat(ctx context.Context, req *pb.HarnessChatRequest) (*pb.HarnessChatResponse, error) {
	resp := &pb.HarnessChatResponse{}

	// Gate 1: Input guardrail - check for prompt injection
	inputViolations := s.guardrail.Check(req.Message, "input")
	resp.InputGuard = &pb.GuardCheckResult{
		Passed:     len(inputViolations) == 0,
		Violations: inputViolations,
	}
	if len(inputViolations) > 0 {
		resp.Content = "Input blocked by guardrail: potential prompt injection detected"
		resp.Error = "guardrail_blocked"
		return resp, nil
	}

	// Gate 2: Permission check - verify agent permissions
	// Note: Tool name can be passed via metadata or message parsing
	// For now, skip tool-specific permission check in chat

	// Gate 3: Rule check - custom rules
	ruleResult := s.ruleEngine.Check(req.AgentId, req.Message)
	resp.RuleCheck = &pb.RuleCheckResult{
		Passed:     ruleResult.Passed,
		Violations: ruleResult.Violations,
	}
	if !ruleResult.Passed {
		resp.Content = "Request blocked by rules"
		resp.Error = "rule_violation"
		return resp, nil
	}

	// Call LLM
	model := req.Model
	if model == "" {
		model = s.cfg.LLM.Model
	}

	llmResp, err := s.llmClient.Chat(ctx, &llm.ChatRequest{
		Messages:     []llm.Message{{Role: "user", Content: req.Message}},
		Model:        model,
		SystemPrompt: req.SystemPrompt,
	})
	if err != nil {
		resp.Error = err.Error()
		resp.Content = fmt.Sprintf("LLM error: %v", err)
		return resp, nil
	}

	// Gate 4: Output guardrail - check for sensitive information
	outputViolations := s.guardrail.Check(llmResp.Content, "output")
	resp.OutputGuard = &pb.GuardCheckResult{
		Passed:     len(outputViolations) == 0,
		Violations: outputViolations,
	}

	// If output guardrail failed, sanitize or block
	if len(outputViolations) > 0 {
		resp.Content = "[Response sanitized - sensitive information detected]"
		resp.Error = "output_sanitized"
	} else {
		resp.Content = llmResp.Content
	}

	resp.Tokens = int32(llmResp.TotalTokens)
	resp.Cost = llmResp.Cost
	resp.TraceId = fmt.Sprintf("%d", time.Now().UnixNano())

	// Note: LLM call metrics are recorded by the llm.MetricsClient decorator,
	// not here. The decorator wraps every Chat() call to the LLM automatically.

	return resp, nil
}

// ChatStream handles streaming harness chat
func (s *HarnessService) ChatStream(req *pb.HarnessChatRequest, stream pb.HarnessService_ChatStreamServer) error {
	resp, err := s.Chat(stream.Context(), req)
	if err != nil {
		return err
	}
	return stream.Send(resp)
}

// CheckToolPermission checks if an agent can use a tool
func (s *HarnessService) CheckToolPermission(agentType, toolName string, callCount int) error {
	return s.permissions.Check(agentType, toolName, callCount)
}

// RecordABTestMetric records a metric for A/B testing
func (s *HarnessService) RecordABTestMetric(testID string, isVariant bool, score float64, latencyMs float64) {
	s.abtest.RecordResult(context.Background(), testID, "session", isVariant, score, latencyMs, true)
}

// AssignABTestVariant assigns a request to control or variant
func (s *HarnessService) AssignABTestVariant(testID string, splitRatio float64) bool {
	result, _ := s.abtest.ShouldUseVariant(context.Background(), testID, "session")
	return result
}

// RecordSLOMetric records a metric for SLO tracking
func (s *HarnessService) RecordSLOMetric(sloID string, latencyMs float64, success bool) {
	s.sloManager.RecordEvent(context.Background(), sloID, success, latencyMs)
}

// ==================== Engine Accessors ====================

// GetABTestEngine returns the A/B test engine
func (s *HarnessService) GetABTestEngine() *abtest.Engine {
	return s.abtest
}

// GetSLOManager returns the SLO manager
func (s *HarnessService) GetSLOManager() *slo.Manager {
	return s.sloManager
}

// GetFeatureFlagEngine returns the feature flag engine
func (s *HarnessService) GetFeatureFlagEngine() *featureflag.Engine {
	return s.featureFlag
}


// GetRCAEngine returns the RCA engine
func (s *HarnessService) GetRCAEngine() *rca.Engine {
	return s.rca
}


// GetCostEngine returns the cost engine
func (s *HarnessService) GetCostEngine() *cost.Engine {
	return s.cost
}

// GetEvolveEngine returns the evolve engine
func (s *HarnessService) GetEvolveEngine() *evolve.Engine {
	return s.evolve
}

// GetGoldenPathEngine returns the golden path engine
func (s *HarnessService) GetGoldenPathEngine() *goldenpath.Engine {
	return s.goldenpath
}


// GetCoordinateEngine returns the coordinate engine
func (s *HarnessService) GetCoordinateEngine() *coordinate.Engine {
	return s.coordinate
}

// GetPlannerEngine returns the planner engine
func (s *HarnessService) GetPlannerEngine() *planner.Engine {
	return s.planner
}

// GetScheduler returns the scheduler engine
func (s *HarnessService) GetScheduler() *scheduler.Scheduler {
	return s.scheduler
}

// ==================== Feature Flag Methods ====================

// EvaluateFeatureFlag evaluates a feature flag
func (s *HarnessService) evaluateFeatureFlagInternal(ctx context.Context, key string, userID string, attributes map[string]interface{}) (interface{}, error) {
	evalCtx := &featureflag.EvaluationContext{
		UserID:     userID,
		Attributes: attributes,
	}
	result, err := s.featureFlag.Evaluate(ctx, key, evalCtx)
	if err != nil {
		return nil, err
	}
	return result.Value, nil
}

// ==================== Cost Methods ====================

// RecordCostUsage records cost usage
func (s *HarnessService) RecordCostUsageInternal(ctx context.Context, agentID, modelID, sessionID string, inputTokens, outputTokens int64) error {
	return s.cost.RecordUsage(ctx, agentID, modelID, sessionID, inputTokens, outputTokens)
}

// RecordCostUsageGRPC records cost usage via gRPC
func (s *HarnessService) RecordCostUsage(ctx context.Context, req *pb.RecordCostUsageRequest) (*commonpb.Empty, error) {
	if err := s.cost.RecordUsage(ctx, req.AgentId, req.ModelId, req.SessionId, req.InputTokens, req.OutputTokens); err != nil {
		return nil, fmt.Errorf("record cost usage: %w", err)
	}
	return &commonpb.Empty{}, nil
}

// GetCostReport generates a cost report
func (s *HarnessService) getCostReportInternal(ctx context.Context, agentID string, start, end time.Time) (*cost.CostReport, error) {
	return s.cost.CostReport(ctx, agentID, start, end)
}

// ==================== RCA Methods ====================

// RecordRCAChange records a change event for RCA
func (s *HarnessService) RecordRCAChange(ctx context.Context, change *rca.ChangeEvent) error {
	return s.rca.RecordChange(ctx, change)
}

// AnalyzeRootCause performs RCA analysis
func (s *HarnessService) AnalyzeRootCause(ctx context.Context, incidentID string) (*rca.AnalysisReport, error) {
	return s.rca.Analyze(ctx, incidentID)
}

// ==================== Evolution Methods ====================

// CreateEvolutionProposal creates a new evolution proposal
func (s *HarnessService) CreateEvolutionProposal(ctx context.Context, proposal *evolve.Proposal) error {
	return s.evolve.CreateProposal(ctx, proposal)
}

// RunOptimizer runs the optimizer
func (s *HarnessService) runOptimizerInternal(ctx context.Context, agentID string, metrics map[string]float64) (*evolve.OptimizationResult, error) {
	return s.evolve.RunOptimizer(ctx, agentID, metrics)
}

// ==================== Feature Flag gRPC Methods ====================

// CreateFeatureFlag creates a feature flag
func (s *HarnessService) CreateFeatureFlag(ctx context.Context, req *pb.CreateFeatureFlagRequest) (*pb.FeatureFlag, error) {
	flag := &featureflag.FeatureFlag{
		Key:         req.Key,
		Name:        req.Name,
		Description: req.Description,
		Type:        req.Type,
		Value:       req.Value,
		Rules:       req.Rules,
		Rollout:     req.Rollout,
		TenantID:    req.TenantId,
		Status:      featureflag.FlagStatusActive,
	}
	if err := s.featureFlag.CreateFlag(ctx, flag); err != nil {
		return nil, err
	}
	return &pb.FeatureFlag{
		Id:          flag.ID,
		Key:         flag.Key,
		Name:        flag.Name,
		Description: flag.Description,
		Type:        string(flag.Type),
		Value:       flag.Value,
		Status:      string(flag.Status),
		Rules:       flag.Rules,
		Rollout:     flag.Rollout,
		CreatedAt:   flag.CreatedAt.Unix(),
		UpdatedAt:   flag.UpdatedAt.Unix(),
	}, nil
}

// ListFeatureFlags lists feature flags
func (s *HarnessService) ListFeatureFlags(ctx context.Context, req *pb.ListFeatureFlagsRequest) (*pb.ListFeatureFlagsResponse, error) {
	flags, err := s.featureFlag.ListFlags(ctx, req.TenantId, featureflag.FlagStatus(req.Status))
	if err != nil {
		return nil, err
	}
	var pbFlags []*pb.FeatureFlag
	for _, f := range flags {
		pbFlags = append(pbFlags, &pb.FeatureFlag{
			Id:          f.ID,
			Key:         f.Key,
			Name:        f.Name,
			Description: f.Description,
			Type:        string(f.Type),
			Value:       f.Value,
			Status:      string(f.Status),
			Rules:       f.Rules,
			Rollout:     f.Rollout,
			CreatedAt:   f.CreatedAt.Unix(),
			UpdatedAt:   f.UpdatedAt.Unix(),
		})
	}
	return &pb.ListFeatureFlagsResponse{Flags: pbFlags}, nil
}

// GetFeatureFlag gets a feature flag by key
func (s *HarnessService) GetFeatureFlag(ctx context.Context, req *pb.GetFeatureFlagRequest) (*pb.FeatureFlag, error) {
	flag, err := s.featureFlag.GetFlag(ctx, req.Key)
	if err != nil {
		return nil, err
	}
	return &pb.FeatureFlag{
		Id:          flag.ID,
		Key:         flag.Key,
		Name:        flag.Name,
		Description: flag.Description,
		Type:        string(flag.Type),
		Value:       flag.Value,
		Status:      string(flag.Status),
		Rules:       flag.Rules,
		Rollout:     flag.Rollout,
		CreatedAt:   flag.CreatedAt.Unix(),
		UpdatedAt:   flag.UpdatedAt.Unix(),
	}, nil
}

// ToggleFeatureFlag toggles a feature flag
func (s *HarnessService) ToggleFeatureFlag(ctx context.Context, req *pb.ToggleFeatureFlagRequest) (*pb.FeatureFlag, error) {
	if err := s.featureFlag.Toggle(ctx, req.Key, req.Enabled); err != nil {
		return nil, err
	}
	flag, err := s.featureFlag.GetFlag(ctx, req.Key)
	if err != nil {
		return nil, err
	}
	return &pb.FeatureFlag{
		Id:          flag.ID,
		Key:         flag.Key,
		Name:        flag.Name,
		Description: flag.Description,
		Type:        string(flag.Type),
		Value:       flag.Value,
		Status:      string(flag.Status),
		Rules:       flag.Rules,
		Rollout:     flag.Rollout,
		CreatedAt:   flag.CreatedAt.Unix(),
		UpdatedAt:   flag.UpdatedAt.Unix(),
	}, nil
}

// DeleteFeatureFlag deletes a feature flag
func (s *HarnessService) DeleteFeatureFlag(ctx context.Context, req *pb.GetFeatureFlagRequest) (*commonpb.Empty, error) {
	if err := s.featureFlag.DeleteFlag(ctx, req.Key); err != nil {
		return nil, err
	}
	return &commonpb.Empty{}, nil
}

// EvaluateFeatureFlag evaluates a feature flag
func (s *HarnessService) EvaluateFeatureFlag(ctx context.Context, req *pb.EvaluateFeatureFlagRequest) (*pb.EvaluateFeatureFlagResponse, error) {
	// Convert map[string]string to map[string]interface{}
	attributes := make(map[string]interface{})
	for k, v := range req.Attributes {
		attributes[k] = v
	}
	evalCtx := &featureflag.EvaluationContext{
		UserID:     req.UserId,
		Attributes: attributes,
	}
	result, err := s.featureFlag.Evaluate(ctx, req.Key, evalCtx)
	if err != nil {
		return nil, err
	}
	return &pb.EvaluateFeatureFlagResponse{
		Key:    result.Key,
		Value:  fmt.Sprintf("%v", result.Value),
		Reason: result.Reason,
	}, nil
}

// ==================== RCA gRPC Methods ====================

// RecordChange records a change event
func (s *HarnessService) RecordChange(ctx context.Context, req *pb.RecordChangeRequest) (*pb.ChangeEvent, error) {
	change := &rca.ChangeEvent{
		AgentID:       req.AgentId,
		ChangeType:    rca.ChangeType(req.ChangeType),
		ResourceID:    req.ResourceId,
		ResourceType:  req.ResourceType,
		Description:   req.Description,
		OldValue:      req.OldValue,
		NewValue:      req.NewValue,
		User:          req.User,
		Source:        req.Source,
	}
	if err := s.rca.RecordChange(ctx, change); err != nil {
		return nil, err
	}
	return &pb.ChangeEvent{
		Id:            change.ID,
		AgentId:       change.AgentID,
		ChangeType:    string(change.ChangeType),
		ResourceId:    change.ResourceID,
		ResourceType:  change.ResourceType,
		Description:   change.Description,
		OldValue:      change.OldValue,
		NewValue:      change.NewValue,
		Timestamp:     change.Timestamp.Unix(),
		User:          change.User,
		Source:        change.Source,
	}, nil
}

// Analyze performs RCA analysis
func (s *HarnessService) Analyze(ctx context.Context, req *pb.AnalyzeRequest) (*pb.AnalysisReport, error) {
	report, err := s.rca.Analyze(ctx, req.IncidentId)
	if err != nil {
		return nil, err
	}
	return s.analysisReportToPB(report), nil
}

func (s *HarnessService) analysisReportToPB(r *rca.AnalysisReport) *pb.AnalysisReport {
	var rootCauses []*pb.RootCause
	for _, rc := range r.SuspectedRootCauses {
		rootCauses = append(rootCauses, &pb.RootCause{
			Correlation: rc.Correlation,
			Reason:      rc.Reason,
			Evidence:    rc.Evidence,
			IsLikely:    rc.IsLikely,
		})
	}
	var changes []*pb.ChangeEvent
	for _, c := range r.RelatedChanges {
		changes = append(changes, &pb.ChangeEvent{
			Id:            c.ID,
			AgentId:       c.AgentID,
			ChangeType:    string(c.ChangeType),
			ResourceId:    c.ResourceID,
			ResourceType:  c.ResourceType,
			Description:   c.Description,
			OldValue:      c.OldValue,
			NewValue:      c.NewValue,
			Timestamp:     c.Timestamp.Unix(),
			User:          c.User,
			Source:        c.Source,
		})
	}
	return &pb.AnalysisReport{
		Id:                   r.ID,
		IncidentId:           r.IncidentID,
		GeneratedAt:          r.GeneratedAt.Unix(),
		SuspectedRootCauses:  rootCauses,
		RelatedChanges:       changes,
		Recommendations:      r.Recommendations,
		Confidence:           r.Confidence,
	}
}

// ==================== Cost gRPC Methods ====================

// SetModelPricing sets model pricing
func (s *HarnessService) SetModelPricing(ctx context.Context, req *pb.SetModelPricingRequest) (*pb.ModelPricing, error) {
	pricing := &cost.ModelPricing{
		ModelID:           req.ModelId,
		ModelName:         req.ModelName,
		Provider:          req.Provider,
		InputPricePer1M:   req.InputPricePer_1M,
		OutputPricePer1M:  req.OutputPricePer_1M,
		Currency:          req.Currency,
	}
	if err := s.cost.SetModelPricing(ctx, pricing); err != nil {
		return nil, err
	}
	return &pb.ModelPricing{
		Id:                pricing.ID,
		ModelId:           pricing.ModelID,
		ModelName:         pricing.ModelName,
		Provider:          pricing.Provider,
		InputPricePer_1M:   pricing.InputPricePer1M,
		OutputPricePer_1M:  pricing.OutputPricePer1M,
		Currency:          pricing.Currency,
	}, nil
}

// ListModelPricing lists model pricing
func (s *HarnessService) ListModelPricing(ctx context.Context, req *commonpb.Empty) (*pb.ListModelPricingResponse, error) {
	pricings, err := s.cost.ListModelPricing(ctx)
	if err != nil {
		return nil, err
	}
	var pbPricings []*pb.ModelPricing
	for _, p := range pricings {
		pbPricings = append(pbPricings, &pb.ModelPricing{
			Id:                p.ID,
			ModelId:           p.ModelID,
			ModelName:         p.ModelName,
			Provider:          p.Provider,
			InputPricePer_1M:   p.InputPricePer1M,
			OutputPricePer_1M:  p.OutputPricePer1M,
			Currency:          p.Currency,
		})
	}
	return &pb.ListModelPricingResponse{Pricings: pbPricings}, nil
}

// GetCostReport gets a cost report
func (s *HarnessService) GetCostReport(ctx context.Context, req *pb.CostReportRequest) (*pb.CostReport, error) {
	start := time.Unix(req.StartTime, 0)
	end := time.Unix(req.EndTime, 0)
	report, err := s.cost.CostReport(ctx, req.AgentId, start, end)
	if err != nil {
		return nil, err
	}
	return s.costReportToPB(report), nil
}

// GetCostRecommendations gets cost recommendations
func (s *HarnessService) GetCostRecommendations(ctx context.Context, req *commonpb.Empty) (*pb.ListCostRecommendationsResponse, error) {
	recs, err := s.cost.Recommendations(ctx)
	if err != nil {
		return nil, err
	}
	var pbRecs []*pb.CostRecommendation
	for _, r := range recs {
		pbRecs = append(pbRecs, &pb.CostRecommendation{
			Type:             r.Type,
			Priority:         r.Priority,
			Title:            r.Title,
			Description:      r.Description,
			PotentialSavings: r.PotentialSavings,
			AgentId:          r.AgentID,
		})
	}
	return &pb.ListCostRecommendationsResponse{Recommendations: pbRecs}, nil
}

func (s *HarnessService) costReportToPB(r *cost.CostReport) *pb.CostReport {
	var byAgent []*pb.AgentCost
	for _, a := range r.ByAgent {
		byAgent = append(byAgent, &pb.AgentCost{
			AgentId:       a.AgentID,
			TotalCost:     a.TotalCost,
			InputTokens:   a.InputTokens,
			OutputTokens:  a.OutputTokens,
			RequestCount:  a.RequestCount,
		})
	}
	return &pb.CostReport{
		PeriodStart:      r.PeriodStart.Unix(),
		PeriodEnd:        r.PeriodEnd.Unix(),
		TotalCost:        r.TotalCost,
		TotalInputTokens: r.TotalInputTokens,
		TotalOutputTokens: r.TotalOutputTokens,
		RequestCount:     r.RequestCount,
		ByAgent:          byAgent,
		Currency:         r.Currency,
	}
}

// ==================== Evolve gRPC Methods ====================

// CreateProposal creates a proposal
func (s *HarnessService) CreateProposal(ctx context.Context, req *pb.CreateProposalRequest) (*pb.Proposal, error) {
	proposal := &evolve.Proposal{
		AgentID:         req.AgentId,
		Type:           evolve.ProposalType(req.Type),
		Title:          req.Title,
		Description:    req.Description,
		CurrentState:   req.CurrentState,
		ProposedState:  req.ProposedState,
		ExpectedBenefit: req.ExpectedBenefit,
		RiskLevel:      req.RiskLevel,
	}
	if err := s.evolve.CreateProposal(ctx, proposal); err != nil {
		return nil, err
	}
	return s.proposalToPB(proposal), nil
}

// ListProposals lists proposals
func (s *HarnessService) ListProposals(ctx context.Context, req *pb.ListProposalsRequest) (*pb.ListProposalsResponse, error) {
	proposals, err := s.evolve.ListProposals(ctx, req.AgentId, evolve.ProposalStatus(req.Status))
	if err != nil {
		return nil, err
	}
	var pbProposals []*pb.Proposal
	for _, p := range proposals {
		pbProposals = append(pbProposals, s.proposalToPB(p))
	}
	return &pb.ListProposalsResponse{Proposals: pbProposals}, nil
}

// ApproveProposal approves a proposal
func (s *HarnessService) ApproveProposal(ctx context.Context, req *pb.ApproveProposalRequest) (*pb.Proposal, error) {
	if err := s.evolve.ApproveProposal(ctx, req.ProposalId, req.ApprovedBy); err != nil {
		return nil, err
	}
	proposals, _ := s.evolve.ListProposals(ctx, "", "")
	for _, p := range proposals {
		if p.ID == req.ProposalId {
			return s.proposalToPB(p), nil
		}
	}
	return nil, fmt.Errorf("proposal not found")
}

// RejectProposal rejects a proposal
func (s *HarnessService) RejectProposal(ctx context.Context, req *pb.RejectProposalRequest) (*pb.Proposal, error) {
	if err := s.evolve.RejectProposal(ctx, req.ProposalId, req.Reason); err != nil {
		return nil, err
	}
	proposals, _ := s.evolve.ListProposals(ctx, "", "")
	for _, p := range proposals {
		if p.ID == req.ProposalId {
			return s.proposalToPB(p), nil
		}
	}
	return nil, fmt.Errorf("proposal not found")
}

// RunOptimizerGRPC runs the optimizer
func (s *HarnessService) RunOptimizer(ctx context.Context, req *pb.RunOptimizerRequest) (*pb.OptimizationResult, error) {
	result, err := s.evolve.RunOptimizer(ctx, req.AgentId, req.Metrics)
	if err != nil {
		return nil, err
	}
	return &pb.OptimizationResult{
		AgentId:         result.AgentID,
		Type:           string(result.Type),
		CurrentValue:   result.CurrentValue,
		OptimizedValue: result.OptimizedValue,
		Improvement:    result.Improvement,
		Config:         fmt.Sprintf("%v", result.Config),
		Confidence:     result.Confidence,
	}, nil
}

// ExecuteProposal executes an approved proposal
func (s *HarnessService) ExecuteProposal(ctx context.Context, req *pb.ApproveProposalRequest) (*pb.Proposal, error) {
	if err := s.evolve.ExecuteProposal(ctx, req.ProposalId); err != nil {
		return nil, fmt.Errorf("execute proposal: %w", err)
	}
	proposal, err := s.evolve.GetProposal(ctx, req.ProposalId)
	if err != nil {
		return nil, fmt.Errorf("get proposal after execution: %w", err)
	}
	return s.proposalToPB(proposal), nil
}

func (s *HarnessService) proposalToPB(p *evolve.Proposal) *pb.Proposal {
	var approvedAt int64
	if p.ApprovedAt != nil {
		approvedAt = p.ApprovedAt.Unix()
	}
	var executedAt int64
	if p.ExecutedAt != nil {
		executedAt = p.ExecutedAt.Unix()
	}
	return &pb.Proposal{
		Id:              p.ID,
		AgentId:         p.AgentID,
		Type:           string(p.Type),
		Title:          p.Title,
		Description:    p.Description,
		CurrentState:   p.CurrentState,
		ProposedState:  p.ProposedState,
		ExpectedBenefit: p.ExpectedBenefit,
		RiskLevel:      string(p.RiskLevel),
		Status:         string(p.Status),
		ApprovedBy:     p.ApprovedBy,
		ApprovedAt:     approvedAt,
		CreatedAt:      p.CreatedAt.Unix(),
		Result:         p.Result,
		ExecutedAt:     executedAt,
	}
}

// computeMetricsFromBuffer calculates agent metrics from the LLM metrics buffer
func (s *HarnessService) computeMetricsFromBuffer(agentID string) map[string]float64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	var successCount, totalCount int
	var totalLatency float64
	var totalCost float64

	for _, m := range s.llmMetricsBuf {
		totalCount++
		if m.Success {
			successCount++
		}
		totalLatency += float64(m.LatencyMs)
		totalCost += m.Cost
	}

	if totalCount == 0 {
		return map[string]float64{
			"success_rate": 1.0,
			"latency":      0,
			"cost":         0,
		}
	}

	return map[string]float64{
		"success_rate": float64(successCount) / float64(totalCount),
		"latency":      totalLatency / float64(totalCount),
		"cost":         totalCost,
	}
}

// ==================== Golden Path gRPC Methods ====================

// CreateGoldenPathTemplate creates a golden path template
func (s *HarnessService) CreateGoldenPathTemplate(ctx context.Context, req *pb.CreateGoldenPathTemplateRequest) (*pb.GoldenPathTemplate, error) {
	template := &goldenpath.Template{
		Name:        req.Name,
		Type:        goldenpath.TemplateType(req.Type),
		Description: req.Description,
		Category:    req.Category,
		Template:    req.Template,
		Variables:   req.Variables,
		Tags:        req.Tags,
		Author:      req.Author,
		IsPublic:    req.IsPublic,
	}
	if err := s.goldenpath.CreateTemplate(ctx, template); err != nil {
		return nil, err
	}
	return s.goldenPathTemplateToPB(template), nil
}

// ListGoldenPathTemplates lists golden path templates
func (s *HarnessService) ListGoldenPathTemplates(ctx context.Context, req *pb.ListGoldenPathTemplatesRequest) (*pb.ListGoldenPathTemplatesResponse, error) {
	templates, err := s.goldenpath.ListTemplates(ctx, goldenpath.TemplateType(req.Type), req.Category)
	if err != nil {
		return nil, err
	}
	var pbTemplates []*pb.GoldenPathTemplate
	for _, t := range templates {
		pbTemplates = append(pbTemplates, s.goldenPathTemplateToPB(t))
	}
	return &pb.ListGoldenPathTemplatesResponse{Templates: pbTemplates}, nil
}

// InstantiateTemplate instantiates a template
func (s *HarnessService) InstantiateTemplate(ctx context.Context, req *pb.InstantiateTemplateRequest) (*commonpb.Empty, error) {
	var variables map[string]interface{}
	if req.Variables != "" {
		json.Unmarshal([]byte(req.Variables), &variables)
	}
	_, err := s.goldenpath.InstantiateTemplate(ctx, req.TemplateId, req.Name, variables)
	if err != nil {
		return nil, err
	}
	return &commonpb.Empty{}, nil
}

func (s *HarnessService) goldenPathTemplateToPB(t *goldenpath.Template) *pb.GoldenPathTemplate {
	return &pb.GoldenPathTemplate{
		Id:          t.ID,
		Name:        t.Name,
		Type:        string(t.Type),
		Description: t.Description,
		Category:    t.Category,
		Version:     t.Version,
		Template:    t.Template,
		Variables:   t.Variables,
		Tags:        t.Tags,
		Author:      t.Author,
		IsPublic:    t.IsPublic,
		UsageCount:  t.UsageCount,
		CreatedAt:   t.CreatedAt.Unix(),
	}
}

// ==================== Scheduler Methods ====================

// SetEvalSchedule creates or updates an evaluation schedule
func (s *HarnessService) SetEvalSchedule(ctx context.Context, req *pb.SetEvalScheduleRequest) (*pb.EvalSchedule, error) {
	schedule := &scheduler.EvalSchedule{
		ID:           req.Id,
		Name:         req.Name,
		Type:         scheduler.ScheduleType(req.Type),
		EvalType:     scheduler.EvalType(req.EvalType),
		AgentID:      req.AgentId,
		ScheduleExpr: req.ScheduleExpr,
		Status:       scheduler.ScheduleStatusActive,
		Enabled:      req.Enabled,
		Metadata:     req.Metadata,
	}
	if err := s.scheduler.SetEvalSchedule(ctx, schedule); err != nil {
		return nil, fmt.Errorf("set eval schedule: %w", err)
	}
	return s.evalScheduleToPB(schedule), nil
}

// GetEvalSchedule gets an evaluation schedule by ID
func (s *HarnessService) GetEvalSchedule(ctx context.Context, req *pb.GetEvalScheduleRequest) (*pb.EvalSchedule, error) {
	schedule, err := s.scheduler.GetSchedule(ctx, req.Id)
	if err != nil {
		return nil, fmt.Errorf("get eval schedule: %w", err)
	}
	return s.evalScheduleToPB(schedule), nil
}

// ListEvalSchedules lists evaluation schedules
func (s *HarnessService) ListEvalSchedules(ctx context.Context, req *pb.ListEvalSchedulesRequest) (*pb.ListEvalSchedulesResponse, error) {
	schedules, err := s.scheduler.ListSchedules(ctx, req.AgentId, scheduler.ScheduleStatus(req.Status))
	if err != nil {
		return nil, fmt.Errorf("list eval schedules: %w", err)
	}
	var pbSchedules []*pb.EvalSchedule
	for _, sch := range schedules {
		pbSchedules = append(pbSchedules, s.evalScheduleToPB(sch))
	}
	return &pb.ListEvalSchedulesResponse{Schedules: pbSchedules}, nil
}

// PauseEvalSchedule pauses an evaluation schedule
func (s *HarnessService) PauseEvalSchedule(ctx context.Context, req *pb.PauseScheduleRequest) (*pb.EvalSchedule, error) {
	if err := s.scheduler.PauseSchedule(ctx, req.Id); err != nil {
		return nil, fmt.Errorf("pause eval schedule: %w", err)
	}
	schedule, err := s.scheduler.GetSchedule(ctx, req.Id)
	if err != nil {
		return nil, fmt.Errorf("get eval schedule after pause: %w", err)
	}
	return s.evalScheduleToPB(schedule), nil
}

// ResumeEvalSchedule resumes a paused evaluation schedule
func (s *HarnessService) ResumeEvalSchedule(ctx context.Context, req *pb.ResumeScheduleRequest) (*pb.EvalSchedule, error) {
	if err := s.scheduler.ResumeSchedule(ctx, req.Id); err != nil {
		return nil, fmt.Errorf("resume eval schedule: %w", err)
	}
	schedule, err := s.scheduler.GetSchedule(ctx, req.Id)
	if err != nil {
		return nil, fmt.Errorf("get eval schedule after resume: %w", err)
	}
	return s.evalScheduleToPB(schedule), nil
}

// DeleteEvalSchedule deletes an evaluation schedule
func (s *HarnessService) DeleteEvalSchedule(ctx context.Context, req *pb.GetEvalScheduleRequest) (*commonpb.Empty, error) {
	if err := s.scheduler.DeleteSchedule(ctx, req.Id); err != nil {
		return nil, fmt.Errorf("delete eval schedule: %w", err)
	}
	return &commonpb.Empty{}, nil
}

// RunEvalScheduleNow manually triggers a schedule run
func (s *HarnessService) RunEvalScheduleNow(ctx context.Context, req *pb.RunScheduleNowRequest) (*pb.ScheduledEvalResult, error) {
	result, err := s.scheduler.RunNow(ctx, req.Id)
	if err != nil {
		return nil, fmt.Errorf("run eval schedule now: %w", err)
	}
	return s.evalResultToPB(result), nil
}

// GetEvalScheduleResults gets results for an evaluation schedule
func (s *HarnessService) GetEvalScheduleResults(ctx context.Context, req *pb.GetScheduleResultsRequest) (*pb.GetScheduleResultsResponse, error) {
	results, err := s.scheduler.GetResults(ctx, req.ScheduleId, int(req.Limit))
	if err != nil {
		return nil, fmt.Errorf("get eval schedule results: %w", err)
	}
	var pbResults []*pb.ScheduledEvalResult
	for _, r := range results {
		pbResults = append(pbResults, s.evalResultToPB(r))
	}
	return &pb.GetScheduleResultsResponse{Results: pbResults}, nil
}

// GetSchedulerStatus returns the scheduler status
func (s *HarnessService) GetSchedulerStatus(ctx context.Context, req *commonpb.Empty) (*pb.SchedulerStatus, error) {
	status, err := s.scheduler.SchedulerStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("get scheduler status: %w", err)
	}
	return s.schedulerStatusToPB(status), nil
}

// SchedulerControl controls the scheduler (start/stop)
func (s *HarnessService) SchedulerControl(ctx context.Context, req *pb.SchedulerControlRequest) (*pb.SchedulerStatus, error) {
	switch req.Action {
	case "start":
		if err := s.scheduler.Start(ctx); err != nil {
			return nil, fmt.Errorf("start scheduler: %w", err)
		}
	case "stop":
		if err := s.scheduler.Stop(ctx); err != nil {
			return nil, fmt.Errorf("stop scheduler: %w", err)
		}
	default:
		return nil, fmt.Errorf("unknown scheduler action: %s", req.Action)
	}
	status, err := s.scheduler.SchedulerStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("get scheduler status after control: %w", err)
	}
	return s.schedulerStatusToPB(status), nil
}

// GetSchedulerStats returns scheduler statistics
func (s *HarnessService) GetSchedulerStats(ctx context.Context, req *commonpb.Empty) (*pb.SchedulerStatsResponse, error) {
	stats := s.scheduler.GetStats(ctx)

	var totalSchedules, activeSchedules, pausedSchedules, stoppedSchedules, totalResults int64
	var running bool

	if v, ok := stats["total_schedules"]; ok {
		totalSchedules = int64(v.(int))
	}
	if v, ok := stats["active_schedules"]; ok {
		activeSchedules = int64(v.(int))
	}
	if v, ok := stats["paused_schedules"]; ok {
		pausedSchedules = int64(v.(int))
	}
	if v, ok := stats["stopped_schedules"]; ok {
		stoppedSchedules = int64(v.(int))
	}
	if v, ok := stats["total_results"]; ok {
		totalResults = int64(v.(int))
	}
	if v, ok := stats["running"]; ok {
		running = v.(bool)
	}

	return &pb.SchedulerStatsResponse{
		TotalSchedules:   totalSchedules,
		ActiveSchedules:  activeSchedules,
		PausedSchedules:  pausedSchedules,
		StoppedSchedules: stoppedSchedules,
		TotalResults:     totalResults,
		Running:          running,
	}, nil
}

// evalScheduleToPB converts a scheduler.EvalSchedule to pb.EvalSchedule
func (s *HarnessService) evalScheduleToPB(sc *scheduler.EvalSchedule) *pb.EvalSchedule {
	var lastRunAt, nextRunAt, createdAt, updatedAt int64
	if sc.LastRunAt != nil {
		lastRunAt = sc.LastRunAt.Unix()
	}
	if sc.NextRunAt != nil {
		nextRunAt = sc.NextRunAt.Unix()
	}
	if !sc.CreatedAt.IsZero() {
		createdAt = sc.CreatedAt.Unix()
	}
	if !sc.UpdatedAt.IsZero() {
		updatedAt = sc.UpdatedAt.Unix()
	}

	var lastResult *pb.EvalResult
	if sc.LastResult != nil {
		lastResult = &pb.EvalResult{
			Score: sc.LastResult.Score,
		}
	}

	return &pb.EvalSchedule{
		Id:           sc.ID,
		Name:         sc.Name,
		Type:         string(sc.Type),
		EvalType:     string(sc.EvalType),
		AgentId:      sc.AgentID,
		ScheduleExpr: sc.ScheduleExpr,
		Status:       string(sc.Status),
		LastRunAt:    lastRunAt,
		NextRunAt:    nextRunAt,
		RunCount:     sc.RunCount,
		LastResult:   lastResult,
		Enabled:      sc.Enabled,
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
		Metadata:     sc.Metadata,
	}
}

// evalResultToPB converts a scheduler.EvalResult to pb.ScheduledEvalResult
func (s *HarnessService) evalResultToPB(r *scheduler.EvalResult) *pb.ScheduledEvalResult {
	return &pb.ScheduledEvalResult{
		Id:         r.ID,
		ScheduleId: r.ScheduleID,
		EvalType:   string(r.EvalType),
		Success:    r.Success,
		Score:      r.Score,
		Details:    r.Details,
		Alerts:     r.Alerts,
		DurationMs: r.DurationMs,
		Timestamp:  r.Timestamp.Unix(),
	}
}

// schedulerStatusToPB converts a scheduler.SchedulerStatus to pb.SchedulerStatus
func (s *HarnessService) schedulerStatusToPB(st *scheduler.SchedulerStatus) *pb.SchedulerStatus {
	var lastRunAt, nextScheduledRun int64
	if st.LastRunAt != nil {
		lastRunAt = st.LastRunAt.Unix()
	}
	if st.NextScheduledRun != nil {
		nextScheduledRun = st.NextScheduledRun.Unix()
	}
	return &pb.SchedulerStatus{
		Running:          st.Running,
		ActiveSchedules:  int32(st.ActiveSchedules),
		TotalRuns:        st.TotalRuns,
		LastRunAt:        lastRunAt,
		NextScheduledRun: nextScheduledRun,
		UptimeSeconds:    st.UptimeSeconds,
	}
}

// llmMetricsCallback returns a metrics callback that logs, stores, and records SLO metrics + Cost
func (s *HarnessService) llmMetricsCallback() llm.MetricsCallback {
	return func(ctx context.Context, m *llm.CallMetrics) {
		status := "success"
		if !m.Success {
			status = "error"
		}
		fmt.Printf("[LLM Metrics] caller=%s model=%s latency=%dms tokens=%d cost=%.6f status=%s\n",
			m.Caller, m.Model, m.LatencyMs, m.TotalTokens, m.Cost, status)

		// Store in ring buffer (keep last 1000 entries)
		s.mu.Lock()
		s.llmMetricsBuf = append(s.llmMetricsBuf, *m)
		if len(s.llmMetricsBuf) > 1000 {
			s.llmMetricsBuf = s.llmMetricsBuf[len(s.llmMetricsBuf)-1000:]
		}
		s.mu.Unlock()

		// Record into Cost engine for real-time cost tracking
		// Estimate input/output tokens (rough split)
		inputTokens := int64(m.TotalTokens * 6 / 10)  // ~60% input
		outputTokens := int64(m.TotalTokens * 4 / 10) // ~40% output
		agentID := m.Caller // Use caller as agent ID (eval, chat, reflection, etc.)

		fmt.Printf("[Cost] Recording LLM call: agent=%s model=%s tokens=%d cost=%.6f\n", agentID, m.Model, m.TotalTokens, m.Cost)

		if err := s.cost.RecordLLMCall(ctx, agentID, m.Model, inputTokens, outputTokens, m.Cost, m.LatencyMs, m.Success); err != nil {
			fmt.Printf("Warning: failed to record cost for agent %s: %v\n", agentID, err)
		} else {
			fmt.Printf("[Cost] Successfully recorded cost for agent %s\n", agentID)
		}

		// Record into SLO manager for all matching SLOs
		slos, err := s.sloManager.ListSLOs(ctx, "", "")
		if err != nil {
			return
		}
		for _, sloDef := range slos {
			switch sloDef.Type {
			case slo.SLOTypeLatency:
				s.sloManager.RecordEvent(ctx, sloDef.ID, true, float64(m.LatencyMs))
			case slo.SLOTypeSuccessRate, slo.SLOTypeAvailability:
				s.sloManager.RecordEvent(ctx, sloDef.ID, m.Success, float64(m.LatencyMs))
			}
		}
	}
}

// RecordLLMMetrics records LLM call metrics from external services (chat-service, agent-service, etc.)
func (s *HarnessService) RecordLLMMetrics(ctx context.Context, req *pb.RecordLLMMetricsRequest) (*commonpb.Empty, error) {
	fmt.Printf("[Harness] Recording LLM metrics from %s: model=%s latency=%dms success=%v\n",
		req.AgentId, req.Model, req.LatencyMs, req.Success)

	// Create CallMetrics from request
	m := &llm.CallMetrics{
		Caller:      req.AgentId,
		Model:       req.Model,
		LatencyMs:   int64(req.LatencyMs),
		TotalTokens: int(req.InputTokens + req.OutputTokens),
		Cost:        req.Cost,
		Success:     req.Success,
		Timestamp:   time.Now(),
	}

	s.recordLLMMetric(ctx, m, int64(req.InputTokens), int64(req.OutputTokens))

	return &commonpb.Empty{}, nil
}

// recordLLMMetric records a single LLM call metric to the ring buffer, cost engine,
// and SLO manager. Shared by gRPC RecordLLMMetrics and the workflow engine's direct LLM calls.
func (s *HarnessService) recordLLMMetric(ctx context.Context, m *llm.CallMetrics, inputTokens, outputTokens int64) {
	// Store in ring buffer
	s.mu.Lock()
	s.llmMetricsBuf = append(s.llmMetricsBuf, *m)
	if len(s.llmMetricsBuf) > 1000 {
		s.llmMetricsBuf = s.llmMetricsBuf[len(s.llmMetricsBuf)-1000:]
	}
	s.mu.Unlock()

	// Record into Cost engine
	if err := s.cost.RecordLLMCall(ctx, m.Caller, m.Model, inputTokens, outputTokens, m.Cost, m.LatencyMs, m.Success); err != nil {
		fmt.Printf("Warning: failed to record cost for agent %s: %v\n", m.Caller, err)
	}

	// Record into SLO manager for all matching SLOs
	slos, err := s.sloManager.ListSLOs(ctx, "", "")
	if err != nil {
		return
	}
	for _, sloDef := range slos {
		switch sloDef.Type {
		case slo.SLOTypeLatency:
			s.sloManager.RecordEvent(ctx, sloDef.ID, true, float64(m.LatencyMs))
		case slo.SLOTypeSuccessRate, slo.SLOTypeAvailability:
			s.sloManager.RecordEvent(ctx, sloDef.ID, m.Success, float64(m.LatencyMs))
		}
	}
}

// GetLLMMetrics returns recent LLM call metrics
func (s *HarnessService) GetLLMMetrics(ctx context.Context, limit int) []llm.CallMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 || limit > len(s.llmMetricsBuf) {
		limit = len(s.llmMetricsBuf)
	}
	start := len(s.llmMetricsBuf) - limit
	result := make([]llm.CallMetrics, limit)
	copy(result, s.llmMetricsBuf[start:])
	return result
}

// GetLLMMetricsPB returns LLM metrics summary for gRPC
func (s *HarnessService) GetLLMMetricsPB(ctx context.Context, req *pb.GetLLMMetricsRequest) (*pb.LLMMetricsSummary, error) {
	s.mu.RLock()
	totalCalls := len(s.llmMetricsBuf)
	var successCalls int
	var totalLatency int64
	var totalCost float64
	for _, m := range s.llmMetricsBuf {
		if m.Success {
			successCalls++
		}
		totalLatency += int64(m.LatencyMs)
		totalCost += m.Cost
	}

	// Build detail metrics for trace viewer (recent 100 calls)
	metricLimit := 100
	if metricLimit > len(s.llmMetricsBuf) {
		metricLimit = len(s.llmMetricsBuf)
	}
	startIdx := len(s.llmMetricsBuf) - metricLimit
	pbMetrics := make([]*pb.LLMCallMetric, 0, metricLimit)
	for _, m := range s.llmMetricsBuf[startIdx:] {
		pbMetrics = append(pbMetrics, &pb.LLMCallMetric{
			Model:       m.Model,
			TotalTokens: int64(m.TotalTokens),
			Cost:        m.Cost,
			LatencyMs:   m.LatencyMs,
			Success:     m.Success,
			Error:       m.Error,
			Caller:      m.Caller,
			Timestamp:   m.Timestamp.Unix(),
		})
	}
	s.mu.RUnlock()

	successRate := 0.0
	avgLatency := 0.0
	if totalCalls > 0 {
		successRate = float64(successCalls) / float64(totalCalls)
		avgLatency = float64(totalLatency) / float64(totalCalls)
	}

	// Get SLO statuses
	sloStatuses, err := s.sloManager.EvaluateAll(ctx, req.AgentId)
	if err != nil {
		sloStatuses = nil
	}

	var pbSloStatuses []*pb.SLOStatus
	for _, ss := range sloStatuses {
		pbSloStatuses = append(pbSloStatuses, &pb.SLOStatus{
			Name:            ss.Name,
			Current:         ss.Current,
			Target:          ss.Target,
			BudgetRemaining: ss.ErrorBudget,
			Status:          string(ss.Status),
		})
	}

	return &pb.LLMMetricsSummary{
		TotalCalls:    int64(totalCalls),
		SuccessCalls:  int64(successCalls),
		SuccessRate:   successRate,
		AvgLatency:    avgLatency,
		TotalCost:     totalCost,
		SloStatuses:   pbSloStatuses,
		Metrics:       pbMetrics,
	}, nil
}

// ==================== AnalyzeAndPropose Methods ====================

// AnalyzeAndPropose analyzes cost/SLO data and generates proposals automatically
func (s *HarnessService) AnalyzeAndPropose(ctx context.Context, req *pb.AnalyzeAndProposeRequest) (*pb.AnalyzeAndProposeResponse, error) {
	// Gather cost data
	start := time.Now().AddDate(0, 0, -30) // Last 30 days
	end := time.Now()
	costReport, err := s.cost.CostReport(ctx, req.AgentId, start, end)
	if err != nil {
		fmt.Printf("AnalyzeAndPropose: cost report error: %v\n", err)
	}

	// Gather SLO data
	sloResults, err := s.sloManager.EvaluateAll(ctx, req.AgentId)
	if err != nil {
		fmt.Printf("AnalyzeAndPropose: SLO evaluate error: %v\n", err)
	}

	// Build analysis data
	analysisData := &evolve.AnalysisData{}

	// Cost data
	if costReport != nil {
		avgCostPerRequest := 0.0
		if costReport.RequestCount > 0 {
			avgCostPerRequest = costReport.TotalCost / float64(costReport.RequestCount)
		}
		var modelCosts []evolve.ModelCostData
		for modelID, mc := range costReport.ByModel {
			modelCosts = append(modelCosts, evolve.ModelCostData{
				ModelID:     modelID,
				ModelName:   mc.ModelName,
				Cost:        mc.TotalCost,
				RequestCount: mc.RequestCount,
				InputPrice:  mc.AvgCostPerRequest * 1000, // rough estimate
				OutputPrice: mc.AvgCostPerRequest * 500,
			})
		}
		analysisData.CostData = &evolve.CostAnalysisData{
			TotalCost:        costReport.TotalCost,
			ForecastCost:     costReport.TotalCost * 1.5, // Simple forecast
			RequestCount:     costReport.RequestCount,
			InputTokens:      costReport.TotalInputTokens,
			OutputTokens:     costReport.TotalOutputTokens,
			AvgCostPerRequest: avgCostPerRequest,
			ByModel:          modelCosts,
		}
	}

	// SLO data
	if len(sloResults) > 0 {
		var sloData []evolve.SLOData
		for _, r := range sloResults {
			sloData = append(sloData, evolve.SLOData{
				ID:          "",
				Name:        r.Name,
				Target:      r.Target,
				Current:     r.Current,
				Status:      string(r.Status),
				ErrorBudget: r.ErrorBudget,
				BurnRate:    r.BurnRate,
				AgentID:     req.AgentId,
			})
		}
		analysisData.SLOData = &evolve.SLOAnalysisData{
			SLOs: sloData,
		}
	}

	// Model alternatives (hardcoded for demo, could be from catalog)
	if costReport != nil && len(costReport.ByModel) > 0 {
		// Find current most used model
		var currentModel string
		var currentCost float64
		for modelID, mc := range costReport.ByModel {
			if mc.TotalCost > currentCost {
				currentCost = mc.TotalCost
				currentModel = modelID
			}
		}
		// Provide alternatives based on common model pricing
		alternatives := []evolve.AlternativeModel{
			{ModelID: "gpt-4o-mini", ModelName: "GPT-4o Mini", InputPrice: 0.15, OutputPrice: 0.60, QualityScore: 0.85},
			{ModelID: "claude-3-haiku", ModelName: "Claude 3 Haiku", InputPrice: 0.25, OutputPrice: 1.25, QualityScore: 0.90},
			{ModelID: "claude-sonnet-4", ModelName: "Claude Sonnet 4", InputPrice: 3.0, OutputPrice: 15.0, QualityScore: 0.95},
		}
		analysisData.ModelData = &evolve.ModelAnalysisData{
			CurrentModel:   currentModel,
			CurrentCost:    currentCost,
			Alternatives:   alternatives,
		}
	}

	// Run analysis
	proposals, err := s.evolve.AnalyzeAndPropose(ctx, analysisData)
	if err != nil {
		return nil, fmt.Errorf("analyze and propose: %w", err)
	}

	var pbProposals []*pb.Proposal
	for _, p := range proposals {
		pbProposals = append(pbProposals, s.proposalToPB(p))
	}

	return &pb.AnalyzeAndProposeResponse{
		Proposals:     pbProposals,
		AnalysisSummary: fmt.Sprintf("Analyzed %d SLO results with %.2f total cost, generated %d proposals", len(sloResults), costReport.TotalCost, len(proposals)),
	}, nil
}

// RunPeriodicAnalysis runs periodic analysis for all agents (called by scheduler)
func (s *HarnessService) RunPeriodicAnalysis(ctx context.Context) error {
	// Get all agents from catalog or buffer
	agentIDs := s.getAgentIDsFromMetrics()

	for _, agentID := range agentIDs {
		_, err := s.AnalyzeAndPropose(ctx, &pb.AnalyzeAndProposeRequest{AgentId: agentID})
		if err != nil {
			fmt.Printf("RunPeriodicAnalysis: failed for agent %s: %v\n", agentID, err)
			continue
		}
		fmt.Printf("RunPeriodicAnalysis: completed for agent %s\n", agentID)
	}

	return nil
}

// getAgentIDsFromMetrics extracts unique agent IDs from metrics buffer
func (s *HarnessService) getAgentIDsFromMetrics() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	agentSet := make(map[string]bool)
	for _ = range s.llmMetricsBuf {
		// AgentID might be in metadata, for now return default
		agentSet["default"] = true
	}

	var agentIDs []string
	for id := range agentSet {
		agentIDs = append(agentIDs, id)
	}
	return agentIDs
}

// ==================== Playground gRPC Methods ====================

// ExecutePlayground executes a single prompt in playground
func (s *HarnessService) ExecutePlayground(ctx context.Context, req *pb.PlaygroundRequest) (*pb.PlaygroundResult, error) {
	// Convert proto messages to llm.Message
	messages := make([]llm.Message, 0, len(req.Messages))
	for _, m := range req.Messages {
		messages = append(messages, llm.Message{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	// Convert parameters
	parameters := make(map[string]interface{})
	for k, v := range req.Parameters {
		parameters[k] = v
	}

	playgroundReq := &playground.PlaygroundRequest{
		Model:       req.Model,
		Messages:    messages,
		Temperature: req.Temperature,
		MaxTokens:   int(req.MaxTokens),
		TopP:        req.TopP,
		Parameters:  parameters,
		TenantID:    req.TenantId,
		UserID:      req.UserId,
		SessionID:   req.SessionId,
	}

	result, err := s.playground.Execute(ctx, playgroundReq)
	if err != nil {
		return nil, fmt.Errorf("execute playground: %w", err)
	}

	return s.playgroundResultToPB(result), nil
}

// CompareModels compares multiple models in parallel
func (s *HarnessService) CompareModels(ctx context.Context, req *pb.CompareModelsRequest) (*pb.CompareModelsResponse, error) {
	// Convert proto messages to llm.Message
	messages := make([]llm.Message, 0, len(req.Messages))
	for _, m := range req.Messages {
		messages = append(messages, llm.Message{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	compareReq := &playground.CompareModelsRequest{
		Models:      req.Models,
		Messages:    messages,
		Temperature: req.Temperature,
		MaxTokens:   int(req.MaxTokens),
		TopP:        req.TopP,
		TenantID:    req.TenantId,
		UserID:      req.UserId,
	}

	response, err := s.playground.CompareModels(ctx, compareReq)
	if err != nil {
		return nil, fmt.Errorf("compare models: %w", err)
	}

	return s.compareModelsResponseToPB(response), nil
}

// StreamPlayground executes a prompt with streaming response
func (s *HarnessService) StreamPlayground(req *pb.PlaygroundRequest, stream pb.HarnessService_StreamPlaygroundServer) error {
	// Convert proto messages to llm.Message
	messages := make([]llm.Message, 0, len(req.Messages))
	for _, m := range req.Messages {
		messages = append(messages, llm.Message{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	// Convert parameters
	parameters := make(map[string]interface{})
	for k, v := range req.Parameters {
		parameters[k] = v
	}

	playgroundReq := &playground.PlaygroundRequest{
		Model:       req.Model,
		Messages:    messages,
		Temperature: req.Temperature,
		MaxTokens:   int(req.MaxTokens),
		TopP:        req.TopP,
		Parameters:  parameters,
		TenantID:    req.TenantId,
		UserID:      req.UserId,
		SessionID:   req.SessionId,
	}

	// Get stream channel
	ch, err := s.playground.StreamExecute(stream.Context(), playgroundReq)
	if err != nil {
		return fmt.Errorf("stream execute: %w", err)
	}

	// Forward chunks to gRPC stream
	for chunk := range ch {
		pbChunk := &pb.PlaygroundStreamChunk{
			Content:   chunk.Content,
			Done:      chunk.Done,
			LogId:     chunk.LogID,
			CreatedAt: chunk.CreatedAt.Unix(),
		}

		if chunk.Error != nil {
			pbChunk.Error = chunk.Error.Error()
		}

		if err := stream.Send(pbChunk); err != nil {
			return fmt.Errorf("send stream chunk: %w", err)
		}

		if chunk.Done {
			break
		}
	}

	return nil
}

// GetPlaygroundHistory retrieves playground execution history
func (s *HarnessService) GetPlaygroundHistory(ctx context.Context, req *pb.GetPlaygroundHistoryRequest) (*pb.GetPlaygroundHistoryResponse, error) {
	histories, err := s.playground.GetHistory(ctx, req.TenantId, req.UserId, int(req.Limit))
	if err != nil {
		return nil, fmt.Errorf("get playground history: %w", err)
	}

	var pbHistories []*pb.PlaygroundHistory
	for _, h := range histories {
		pbHistories = append(pbHistories, s.playgroundHistoryToPB(h))
	}

	return &pb.GetPlaygroundHistoryResponse{Histories: pbHistories}, nil
}

// DeletePlaygroundHistory deletes a playground history record
func (s *HarnessService) DeletePlaygroundHistory(ctx context.Context, req *pb.DeletePlaygroundHistoryRequest) (*commonpb.Empty, error) {
	if err := s.playground.DeleteHistory(ctx, req.HistoryId); err != nil {
		return nil, fmt.Errorf("delete playground history: %w", err)
	}
	return &commonpb.Empty{}, nil
}

// GetPlaygroundStats returns playground usage statistics
func (s *HarnessService) GetPlaygroundStats(ctx context.Context, req *pb.GetPlaygroundStatsRequest) (*pb.PlaygroundStats, error) {
	// Get recorder from playground engine
	recorder := s.playground.GetRecorder()
	if recorder == nil {
		return &pb.PlaygroundStats{}, nil
	}

	stats := recorder.GetStats(ctx, req.TenantId)

	modelCounts := make(map[string]int64)
	for k, v := range stats.ModelCounts {
		modelCounts[k] = int64(v)
	}

	return &pb.PlaygroundStats{
		TenantId:             stats.TenantID,
		TotalExecutions:      int64(stats.TotalExecutions),
		StreamedExecutions:   int64(stats.StreamedExecutions),
		ComparisonExecutions: int64(stats.ComparisonExecutions),
		TotalTokens:          stats.TotalTokens,
		TotalCost:            stats.TotalCost,
		TotalLatency:         stats.TotalLatency,
		AvgLatency:           stats.AvgLatency,
		AvgCost:              stats.AvgCost,
		AvgTokens:            stats.AvgTokens,
		ModelCounts:          modelCounts,
	}, nil
}

// playgroundResultToPB converts playground.PlaygroundResult to pb.PlaygroundResult
func (s *HarnessService) playgroundResultToPB(r *playground.PlaygroundResult) *pb.PlaygroundResult {
	if r == nil {
		return nil
	}
	return &pb.PlaygroundResult{
		Content:      r.Content,
		TotalTokens:  r.TotalTokens,
		InputTokens:  r.InputTokens,
		OutputTokens: r.OutputTokens,
		Cost:         r.Cost,
		Latency:      r.Latency,
		Model:        r.Model,
		FinishReason: r.FinishReason,
		LogId:        r.LogID,
		CreatedAt:    r.CreatedAt.Unix(),
	}
}

// compareModelsResponseToPB converts playground.CompareModelsResponse to pb.CompareModelsResponse
func (s *HarnessService) compareModelsResponseToPB(r *playground.CompareModelsResponse) *pb.CompareModelsResponse {
	if r == nil {
		return nil
	}

	var pbResults []*pb.PlaygroundResult
	for _, res := range r.Results {
		pbResults = append(pbResults, s.playgroundResultToPB(res))
	}

	var comparison *pb.ModelComparison
	if r.Comparison != nil {
		comparison = &pb.ModelComparison{
			BestModel:     r.Comparison.BestModel,
			FastestModel:  r.Comparison.FastestModel,
			CheapestModel: r.Comparison.CheapestModel,
			AvgLatency:    r.Comparison.AvgLatency,
			AvgCost:       r.Comparison.AvgCost,
			AvgTokens:     r.Comparison.AvgTokens,
		}
	}

	return &pb.CompareModelsResponse{
		Results:    pbResults,
		Comparison: comparison,
		CreatedAt:  r.CreatedAt.Unix(),
	}
}

// playgroundHistoryToPB converts playground.PlaygroundHistory to pb.PlaygroundHistory
func (s *HarnessService) playgroundHistoryToPB(h *playground.PlaygroundHistory) *pb.PlaygroundHistory {
	if h == nil {
		return nil
	}

	var messages []*pb.PlaygroundMessage
	for _, m := range h.Messages {
		messages = append(messages, &pb.PlaygroundMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	parameters := make(map[string]string)
	for k, v := range h.Parameters {
		parameters[k] = fmt.Sprintf("%v", v)
	}

	return &pb.PlaygroundHistory{
		Id:          h.ID,
		TenantId:    h.TenantID,
		UserId:      h.UserID,
		SessionId:   h.SessionID,
		Model:       h.Model,
		Messages:    messages,
		Result:      s.playgroundResultToPB(h.Result),
		Comparison:  s.compareModelsResponseToPB(h.Comparison),
		Temperature: h.Temperature,
		MaxTokens:   int32(h.MaxTokens),
		TopP:        h.TopP,
		Parameters:  parameters,
		Streamed:    h.Streamed,
		CreatedAt:   h.CreatedAt.Unix(),
	}
}

// GetPlaygroundEngine returns the playground engine
func (s *HarnessService) GetPlaygroundEngine() *playground.PlaygroundEngine {
	return s.playground
}

// ==================== Session Replay gRPC Methods ====================

// CreateSession creates a new session for recording
func (s *HarnessService) CreateSession(ctx context.Context, req *pb.CreateSessionRequest) (*pb.CreateSessionResponse, error) {
	session, err := s.sessionRecorder.CreateSession(ctx, req.AgentId, req.TraceId, req.Model, req.Metadata, req.TenantId)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	return &pb.CreateSessionResponse{
		Session: s.sessionToPB(session),
	}, nil
}

// GetSession retrieves a session with its details
func (s *HarnessService) GetSession(ctx context.Context, req *pb.GetSessionRequest) (*pb.SessionDetail, error) {
	detail, err := s.sessionRecorder.GetSession(ctx, req.SessionId)
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}

	return s.sessionDetailToPB(detail), nil
}

// ListSessions lists sessions with filters
func (s *HarnessService) ListSessions(ctx context.Context, req *pb.ListSessionsRequest) (*pb.ListSessionsResponse, error) {
	filter := &session.ListSessionsFilter{
		AgentID:   req.AgentId,
		Status:    req.Status,
		StartTime: req.StartTime,
		EndTime:   req.EndTime,
		Page:      req.Page,
		PageSize:  req.PageSize,
		TenantID:  req.TenantId,
	}

	sessions, total, err := s.sessionRecorder.ListSessions(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}

	var pbSessions []*pb.Session
	for _, sess := range sessions {
		pbSessions = append(pbSessions, s.sessionToPB(sess))
	}

	return &pb.ListSessionsResponse{
		Sessions: pbSessions,
		Total:    total,
	}, nil
}

// RecordStep records a step in a session
func (s *HarnessService) RecordStep(ctx context.Context, req *pb.RecordStepRequest) (*pb.RecordStepResponse, error) {
	step, err := s.sessionRecorder.RecordStep(
		ctx,
		req.SessionId,
		session.StepType(req.StepType),
		req.ParentStepId,
		req.Input,
		req.Output,
		nil, // metadata parsed from request if needed
		req.Duration,
	)
	if err != nil {
		return nil, fmt.Errorf("record step: %w", err)
	}

	return &pb.RecordStepResponse{
		Step: s.sessionStepToPB(step),
	}, nil
}

// EndSession ends a session
func (s *HarnessService) EndSession(ctx context.Context, req *pb.EndSessionRequest) (*pb.EndSessionResponse, error) {
	sess, err := s.sessionRecorder.EndSession(ctx, req.SessionId, session.SessionStatus(req.Status))
	if err != nil {
		return nil, fmt.Errorf("end session: %w", err)
	}

	return &pb.EndSessionResponse{
		Session: s.sessionToPB(sess),
	}, nil
}

// ReplaySession replays a session and compares outputs
func (s *HarnessService) ReplaySession(ctx context.Context, req *pb.ReplaySessionRequest) (*pb.ReplaySessionResponse, error) {
	// Create executor that re-executes LLM call steps for real replay comparison
	executor := func(ctx context.Context, step session.SessionStep) (string, error) {
		if step.StepType == session.StepTypeLLMCall {
			var input map[string]interface{}
			if err := json.Unmarshal([]byte(step.Input), &input); err == nil {
				prompt, _ := input["prompt"].(string)
				model, _ := input["model"].(string)
				if prompt != "" {
					if model == "" {
						model = "qwen-plus"
					}
					resp, err := s.llmClient.Chat(ctx, &llm.ChatRequest{
						Messages: []llm.Message{{Role: "user", Content: prompt}},
						Model:    model,
					})
					if err != nil {
						return "", fmt.Errorf("replay LLM call: %w", err)
					}
					return resp.Content, nil
				}
			}
		}
		// For non-LLM steps (tool calls, observations, etc.) return original output
		return step.Output, nil
	}

	replay, err := s.sessionRecorder.ReplaySession(
		ctx,
		req.SessionId,
		req.FromStep,
		req.ToStep,
		req.DryRun,
		executor,
	)
	if err != nil {
		return nil, fmt.Errorf("replay session: %w", err)
	}

	var pbDiffs []*pb.ReplayDiff
	for _, diff := range replay.Diffs {
		pbDiffs = append(pbDiffs, &pb.ReplayDiff{
			StepId:          diff.StepID,
			StepNumber:      diff.StepNumber,
			OriginalOutput:  diff.OriginalOutput,
			ReplayOutput:    diff.ReplayOutput,
			Matches:         diff.Matches,
		})
	}

	return &pb.ReplaySessionResponse{
		ReplayId: replay.ID,
		Diffs:    pbDiffs,
		Success:  replay.Success,
		Error:    replay.Error,
	}, nil
}

// GetSessionGraph retrieves the execution graph for a session
func (s *HarnessService) GetSessionGraph(ctx context.Context, req *pb.GetSessionGraphRequest) (*pb.SessionGraph, error) {
	detail, err := s.sessionRecorder.GetSession(ctx, req.SessionId)
	if err != nil {
		return nil, fmt.Errorf("get session graph: %w", err)
	}

	return s.sessionGraphToPB(&detail.Graph), nil
}

// ExportSession exports a session to a specified format
func (s *HarnessService) ExportSession(ctx context.Context, req *pb.ExportSessionRequest) (*pb.ExportSessionResponse, error) {
	content, err := s.sessionRecorder.ExportSession(ctx, req.SessionId, req.Format)
	if err != nil {
		return nil, fmt.Errorf("export session: %w", err)
	}

	return &pb.ExportSessionResponse{
		Content: content,
		Format:  req.Format,
	}, nil
}

// DeleteSessionGRPC deletes a session
func (s *HarnessService) DeleteSession(ctx context.Context, req *pb.GetSessionRequest) (*commonpb.Empty, error) {
	if err := s.sessionRecorder.DeleteSession(ctx, req.SessionId); err != nil {
		return nil, fmt.Errorf("delete session: %w", err)
	}
	return &commonpb.Empty{}, nil
}

// GetSessionRecorder returns the session recorder
func (s *HarnessService) GetSessionRecorder() *session.Recorder {
	return s.sessionRecorder
}

// sessionToPB converts session.Session to pb.Session
func (s *HarnessService) sessionToPB(sess *session.Session) *pb.Session {
	if sess == nil {
		return nil
	}

	var endTime int64
	if sess.EndTime != nil {
		endTime = sess.EndTime.Unix()
	}

	return &pb.Session{
		Id:          sess.ID,
		AgentId:     sess.AgentID,
		TraceId:     sess.TraceID,
		Status:      string(sess.Status),
		StartTime:   sess.StartTime.Unix(),
		EndTime:     endTime,
		Duration:    sess.Duration,
		TotalTokens: sess.TotalTokens,
		TotalCost:   sess.TotalCost,
		Model:       sess.Model,
		Metadata:    sess.Metadata,
		CreatedAt:   sess.CreatedAt.Unix(),
	}
}

// sessionStepToPB converts session.SessionStep to pb.SessionStep
func (s *HarnessService) sessionStepToPB(step *session.SessionStep) *pb.SessionStep {
	if step == nil {
		return nil
	}

	return &pb.SessionStep{
		Id:           step.ID,
		SessionId:    step.SessionID,
		StepNumber:   step.StepNumber,
		StepType:     string(step.StepType),
		ParentStepId: step.ParentStepID,
		Input:        step.Input,
		Output:       step.Output,
		Metadata:     step.Metadata,
		Duration:     step.Duration,
		Status:       string(step.Status),
		Timestamp:    step.Timestamp.Unix(),
	}
}

// sessionDetailToPB converts session.SessionDetail to pb.SessionDetail
func (s *HarnessService) sessionDetailToPB(detail *session.SessionDetail) *pb.SessionDetail {
	if detail == nil {
		return nil
	}

	var pbSteps []*pb.SessionStep
	for _, step := range detail.Steps {
		pbSteps = append(pbSteps, s.sessionStepToPB(&step))
	}

	return &pb.SessionDetail{
		Session: s.sessionToPB(&detail.Session),
		Steps:   pbSteps,
		Graph:   s.sessionGraphToPB(&detail.Graph),
	}
}

// sessionGraphToPB converts session.SessionGraph to pb.SessionGraph
func (s *HarnessService) sessionGraphToPB(graph *session.SessionGraph) *pb.SessionGraph {
	if graph == nil {
		return nil
	}

	var pbNodes []*pb.GraphNode
	for _, node := range graph.Nodes {
		pbNodes = append(pbNodes, &pb.GraphNode{
			Id:       node.ID,
			Type:     node.Type,
			Label:    node.Label,
			Duration: node.Duration,
			Status:   node.Status,
			Metadata: node.Metadata,
		})
	}

	var pbEdges []*pb.GraphEdge
	for _, edge := range graph.Edges {
		pbEdges = append(pbEdges, &pb.GraphEdge{
			From:  edge.From,
			To:    edge.To,
			Label: edge.Label,
		})
	}

	return &pb.SessionGraph{
		Nodes: pbNodes,
		Edges: pbEdges,
	}
}

// ==================== Prompt Management Methods ====================

// CreatePrompt creates a new prompt
func (s *HarnessService) CreatePrompt(ctx context.Context, req *pb.CreatePromptRequest) (*pb.Prompt, error) {
	p := &prompt.Prompt{
		Key:         req.Key,
		Name:        req.Name,
		Description: req.Description,
		Category:    prompt.PromptCategory(req.Category),
		Tags:        req.Tags,
		TenantID:    req.TenantId,
		CreatedBy:   req.CreatedBy,
	}
	if err := s.prompt.CreatePrompt(ctx, p); err != nil {
		return nil, fmt.Errorf("create prompt: %w", err)
	}
	return s.promptToPB(p), nil
}

// GetPrompt retrieves a prompt by key
func (s *HarnessService) GetPrompt(ctx context.Context, req *pb.GetPromptRequest) (*pb.Prompt, error) {
	p, err := s.prompt.GetPrompt(ctx, req.Key)
	if err != nil {
		return nil, fmt.Errorf("get prompt: %w", err)
	}
	return s.promptToPB(p), nil
}

// ListPrompts lists all prompts
func (s *HarnessService) ListPrompts(ctx context.Context, req *pb.ListPromptsRequest) (*pb.ListPromptsResponse, error) {
	prompts, err := s.prompt.ListPrompts(ctx, req.TenantId, prompt.PromptCategory(req.Category))
	if err != nil {
		return nil, fmt.Errorf("list prompts: %w", err)
	}
	var pbPrompts []*pb.Prompt
	for _, p := range prompts {
		pbPrompts = append(pbPrompts, s.promptToPB(p))
	}
	return &pb.ListPromptsResponse{Prompts: pbPrompts}, nil
}

// DeletePrompt deletes a prompt
func (s *HarnessService) DeletePrompt(ctx context.Context, req *pb.GetPromptRequest) (*commonpb.Empty, error) {
	if err := s.prompt.DeletePrompt(ctx, req.Key); err != nil {
		return nil, fmt.Errorf("delete prompt: %w", err)
	}
	return &commonpb.Empty{}, nil
}

// CreatePromptVersion creates a new version of a prompt
func (s *HarnessService) CreatePromptVersion(ctx context.Context, req *pb.CreatePromptVersionRequest) (*pb.PromptVersion, error) {
	// Get prompt by key to get the ID
	p, err := s.prompt.GetPrompt(ctx, req.PromptKey)
	if err != nil {
		return nil, fmt.Errorf("get prompt: %w", err)
	}

	v := &prompt.PromptVersion{
		PromptID:  p.ID,
		Version:   req.Version,
		Content:   req.Content,
		Variables: req.Variables,
		Metadata:  req.Metadata,
		CreatedBy: req.CreatedBy,
	}
	if err := s.prompt.CreateVersion(ctx, v); err != nil {
		return nil, fmt.Errorf("create version: %w", err)
	}

	// Activate if requested
	if req.Activate {
		if err := s.prompt.ActivateVersion(ctx, v.ID); err != nil {
			return nil, fmt.Errorf("activate version: %w", err)
		}
	}

	return s.promptVersionToPB(v), nil
}

// GetPromptVersion retrieves a specific version
func (s *HarnessService) GetPromptVersion(ctx context.Context, req *pb.GetPromptVersionRequest) (*pb.PromptVersion, error) {
	v, err := s.prompt.GetVersion(ctx, req.VersionId)
	if err != nil {
		return nil, fmt.Errorf("get version: %w", err)
	}
	return s.promptVersionToPB(v), nil
}

// GetActivePromptVersion retrieves the active version for a prompt
func (s *HarnessService) GetActivePromptVersion(ctx context.Context, req *pb.GetActivePromptVersionRequest) (*pb.PromptVersion, error) {
	v, err := s.prompt.GetActiveVersion(ctx, req.PromptKey)
	if err != nil {
		return nil, fmt.Errorf("get active version: %w", err)
	}
	return s.promptVersionToPB(v), nil
}

// ListPromptVersions lists all versions of a prompt
func (s *HarnessService) ListPromptVersions(ctx context.Context, req *pb.ListPromptVersionsRequest) (*pb.ListPromptVersionsResponse, error) {
	versions, err := s.prompt.ListVersions(ctx, req.PromptKey)
	if err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}
	var pbVersions []*pb.PromptVersion
	for _, v := range versions {
		pbVersions = append(pbVersions, s.promptVersionToPB(&v))
	}
	return &pb.ListPromptVersionsResponse{Versions: pbVersions}, nil
}

// ActivatePromptVersion activates a specific version
func (s *HarnessService) ActivatePromptVersion(ctx context.Context, req *pb.ActivatePromptVersionRequest) (*pb.PromptVersion, error) {
	if err := s.prompt.ActivateVersion(ctx, req.VersionId); err != nil {
		return nil, fmt.Errorf("activate version: %w", err)
	}
	v, err := s.prompt.GetVersion(ctx, req.VersionId)
	if err != nil {
		return nil, fmt.Errorf("get version: %w", err)
	}
	return s.promptVersionToPB(v), nil
}

// ArchivePromptVersion archives a specific version
func (s *HarnessService) ArchivePromptVersion(ctx context.Context, req *pb.ArchivePromptVersionRequest) (*pb.PromptVersion, error) {
	if err := s.prompt.ArchiveVersion(ctx, req.VersionId); err != nil {
		return nil, fmt.Errorf("archive version: %w", err)
	}
	v, err := s.prompt.GetVersion(ctx, req.VersionId)
	if err != nil {
		return nil, fmt.Errorf("get version: %w", err)
	}
	return s.promptVersionToPB(v), nil
}

// RollbackPromptVersion reverts to a previous version
func (s *HarnessService) RollbackPromptVersion(ctx context.Context, req *pb.ActivatePromptVersionRequest) (*pb.PromptVersion, error) {
	if err := s.prompt.RollbackVersion(ctx, req.VersionId); err != nil {
		return nil, fmt.Errorf("rollback version: %w", err)
	}
	v, err := s.prompt.GetVersion(ctx, req.VersionId)
	if err != nil {
		return nil, fmt.Errorf("get version: %w", err)
	}
	return s.promptVersionToPB(v), nil
}

// ComparePromptVersions compares two versions
func (s *HarnessService) ComparePromptVersions(ctx context.Context, req *pb.ComparePromptVersionsRequest) (*pb.PromptVersionDiff, error) {
	diff, err := s.prompt.CompareVersions(ctx, req.Version1Id, req.Version2Id)
	if err != nil {
		return nil, fmt.Errorf("compare versions: %w", err)
	}
	return s.promptVersionDiffToPB(diff), nil
}

// RenderPrompt renders a prompt with variables
func (s *HarnessService) RenderPrompt(ctx context.Context, req *pb.RenderPromptRequest) (*pb.RenderPromptResponse, error) {
	var vars map[string]interface{}
	if req.Variables != "" {
		if err := json.Unmarshal([]byte(req.Variables), &vars); err != nil {
			return nil, fmt.Errorf("parse variables: %w", err)
		}
	}

	renderCtx := s.prompt.GetRenderer().BuildContext(req.UserId, req.SessionId, req.AgentId, req.Model, vars, nil)
	content, warnings, err := s.prompt.RenderPromptWithValidation(ctx, req.PromptKey, vars)
	_ = renderCtx // context for potential future use
	if err != nil {
		return nil, fmt.Errorf("render prompt: %w", err)
	}

	return &pb.RenderPromptResponse{
		Content:  content,
		Warnings: warnings,
	}, nil
}

// RecordPromptUsage records usage for performance tracking
func (s *HarnessService) RecordPromptUsage(ctx context.Context, req *pb.RecordPromptUsageRequest) (*commonpb.Empty, error) {
	if err := s.prompt.RecordUsage(ctx, req.VersionId, req.SessionId, req.Success, req.LatencyMs, req.InputTokens, req.OutputTokens, req.Cost, req.UserRating); err != nil {
		return nil, fmt.Errorf("record usage: %w", err)
	}
	return &commonpb.Empty{}, nil
}

// GetPromptPerformance gets performance metrics for a version
func (s *HarnessService) GetPromptPerformance(ctx context.Context, req *pb.GetPromptPerformanceRequest) (*pb.PromptPerformance, error) {
	periodStart := time.Unix(req.PeriodStart, 0)
	periodEnd := time.Unix(req.PeriodEnd, 0)
	perf, err := s.prompt.GetPerformance(ctx, req.VersionId, periodStart, periodEnd)
	if err != nil {
		return nil, fmt.Errorf("get performance: %w", err)
	}
	return s.promptPerformanceToPB(perf), nil
}

// GetPromptPerformanceTrend gets performance trend for a version
func (s *HarnessService) GetPromptPerformanceTrend(ctx context.Context, req *pb.GetPromptPerformanceTrendRequest) (*pb.PromptPerformanceTrend, error) {
	trend, err := s.prompt.GetPerformanceTrend(ctx, req.VersionId, int(req.Days))
	if err != nil {
		return nil, fmt.Errorf("get performance trend: %w", err)
	}
	return s.promptPerformanceTrendToPB(trend), nil
}

// GetPromptEngine returns the prompt engine
func (s *HarnessService) GetPromptEngine() *prompt.Engine {
	return s.prompt
}

// Helper methods for Prompt

func (s *HarnessService) promptToPB(p *prompt.Prompt) *pb.Prompt {
	return &pb.Prompt{
		Id:          p.ID,
		Key:         p.Key,
		Name:        p.Name,
		Description: p.Description,
		Category:    string(p.Category),
		Tags:        p.Tags,
		TenantId:    p.TenantID,
		CreatedAt:   p.CreatedAt.Unix(),
		UpdatedAt:   p.UpdatedAt.Unix(),
		CreatedBy:   p.CreatedBy,
	}
}

func (s *HarnessService) promptVersionToPB(v *prompt.PromptVersion) *pb.PromptVersion {
	return &pb.PromptVersion{
		Id:        v.ID,
		PromptId:  v.PromptID,
		Version:   v.Version,
		Content:   v.Content,
		Variables: v.Variables,
		Metadata:  v.Metadata,
		Status:    string(v.Status),
		IsActive:  v.IsActive,
		CreatedAt: v.CreatedAt.Unix(),
		CreatedBy: v.CreatedBy,
	}
}

func (s *HarnessService) promptPerformanceToPB(p *prompt.PromptPerformance) *pb.PromptPerformance {
	return &pb.PromptPerformance{
		Id:              p.ID,
		VersionId:       p.VersionID,
		TotalCalls:      p.TotalCalls,
		SuccessCalls:    p.SuccessCalls,
		SuccessRate:     p.SuccessRate,
		AvgLatency:      p.AvgLatency,
		AvgInputTokens:  p.AvgInputTokens,
		AvgOutputTokens: p.AvgOutputTokens,
		AvgTotalTokens:  p.AvgTotalTokens,
		AvgCost:         p.AvgCost,
		UserRating:      p.UserRating,
		FeedbackCount:   p.FeedbackCount,
		PeriodStart:     p.PeriodStart.Unix(),
		PeriodEnd:       p.PeriodEnd.Unix(),
	}
}

func (s *HarnessService) promptVersionDiffToPB(d *prompt.VersionDiff) *pb.PromptVersionDiff {
	var contentDiff []*pb.VersionDiffLine
	for _, line := range d.ContentDiff {
		contentDiff = append(contentDiff, &pb.VersionDiffLine{
			Type:    line.Type,
			Content: line.Content,
		})
	}

	var varDiff []*pb.VariableDiff
	for _, v := range d.VarDiff {
		var oldValue, newValue string
		if v.OldValue != nil {
			oldBytes, _ := json.Marshal(v.OldValue)
			oldValue = string(oldBytes)
		}
		if v.NewValue != nil {
			newBytes, _ := json.Marshal(v.NewValue)
			newValue = string(newBytes)
		}
		varDiff = append(varDiff, &pb.VariableDiff{
			Name:     v.Name,
			Type:     v.Type,
			OldValue: oldValue,
			NewValue: newValue,
		})
	}

	return &pb.PromptVersionDiff{
		Version1Id:  d.Version1,
		Version2Id:  d.Version2,
		ContentDiff: contentDiff,
		VarDiff:     varDiff,
		Summary:     d.Summary,
	}
}

func (s *HarnessService) promptPerformanceTrendToPB(t *prompt.PerformanceTrend) *pb.PromptPerformanceTrend {
	var dataPoints []*pb.PerformanceDataPoint
	for _, dp := range t.DataPoints {
		dataPoints = append(dataPoints, &pb.PerformanceDataPoint{
			Timestamp:   dp.Timestamp.Unix(),
			SuccessRate: dp.SuccessRate,
			AvgLatency:  dp.AvgLatency,
			AvgCost:     dp.AvgCost,
			UserRating:  dp.UserRating,
			CallCount:   dp.CallCount,
		})
	}
	return &pb.PromptPerformanceTrend{
		VersionId:   t.VersionID,
		DataPoints:  dataPoints,
		Trend:       t.Trend,
		ChangeRate:  t.ChangeRate,
	}
}


// ==================== Checkpoint ====================

// ListCheckpoints lists checkpoints for a session
func (s *HarnessService) ListCheckpoints(ctx context.Context, req *pb.ListCheckpointsRequest) (*pb.ListCheckpointsResponse, error) {
	checkpoints, err := s.checkpointStore.List(ctx, req.SessionId)
	if err != nil {
		return nil, fmt.Errorf("list checkpoints: %w", err)
	}

	var pbCheckpoints []*pb.Checkpoint
	for _, cp := range checkpoints {
		pbCheckpoints = append(pbCheckpoints, &pb.Checkpoint{
			Id:          cp.ID,
			SessionId:   cp.SessionID,
			Step:        int32(cp.Step),
			AgentId:     cp.AgentID,
			TotalTokens: int32(cp.TotalTokens),
			CreatedAt:   cp.CreatedAt.Unix(),
		})
	}

	return &pb.ListCheckpointsResponse{Checkpoints: pbCheckpoints}, nil
}

// GetCheckpoint gets a specific checkpoint
func (s *HarnessService) GetCheckpoint(ctx context.Context, req *pb.GetCheckpointRequest) (*pb.GetCheckpointResponse, error) {
	cp, err := s.checkpointStore.Get(ctx, req.CheckpointId)
	if err != nil {
		return nil, fmt.Errorf("get checkpoint: %w", err)
	}

	messagesJSON, _ := json.Marshal(cp.Messages)
	variablesJSON, _ := json.Marshal(cp.Variables)
	toolResultsJSON, _ := json.Marshal(cp.ToolResults)
	agentHistoryJSON, _ := json.Marshal(cp.AgentHistory)

	pbCheckpoint := &pb.Checkpoint{
		Id:          cp.ID,
		SessionId:   cp.SessionID,
		Step:        int32(cp.Step),
		AgentId:     cp.AgentID,
		TotalTokens: int32(cp.TotalTokens),
		CreatedAt:   cp.CreatedAt.Unix(),
	}

	return &pb.GetCheckpointResponse{
		Checkpoint:    pbCheckpoint,
		Messages:      string(messagesJSON),
		Variables:     string(variablesJSON),
		ToolResults:   string(toolResultsJSON),
		AgentHistory:  string(agentHistoryJSON),
	}, nil
}

// ResumeFromCheckpoint resumes execution from a checkpoint
func (s *HarnessService) ResumeFromCheckpoint(ctx context.Context, req *pb.ResumeFromCheckpointRequest) (*pb.ResumeFromCheckpointResponse, error) {
	// Delegate to the agent service, which loads the checkpoint from its
	// (MongoDB) checkpoint store and re-enters the engine loop. The verifier
	// and reflection gate completion the same way as a fresh execution.
	if s.agentClient == nil {
		return nil, fmt.Errorf("agent-service client not configured; resume unavailable")
	}

	resp, err := s.agentClient.Resume(ctx, &agentpb.ResumeRequest{CheckpointId: req.CheckpointId})
	if err != nil {
		return nil, fmt.Errorf("agent resume: %w", err)
	}

	agentHistoryJSON, _ := json.Marshal(resp.AgentHistory)
	return &pb.ResumeFromCheckpointResponse{
		ContextId:    resp.ContextId,
		SessionId:    resp.SessionId,
		Response:     resp.Response,
		TotalTokens:  resp.TotalTokens,
		TotalCost:    resp.TotalCost,
		Status:       resp.Status,
		AgentHistory: string(agentHistoryJSON),
		Error:        resp.Error,
	}, nil
}

// ==================== Workflow ====================

// CreateWorkflow creates a new workflow
func (s *HarnessService) CreateWorkflow(ctx context.Context, req *pb.CreateWorkflowRequest) (*pb.Workflow, error) {
	// Validate the workflow DAG before saving
	if s.workflowEngine != nil && req.Nodes != "" && req.Edges != "" {
		if err := s.workflowEngine.ValidateWorkflow(req.Nodes, req.Edges, req.EntryNodeId); err != nil {
			return nil, fmt.Errorf("workflow validation: %w", err)
		}
	}

	wf := &repository.WorkflowModel{
		Name:        req.Name,
		Description: req.Description,
		Nodes:       req.Nodes,
		Edges:       req.Edges,
		EntryNodeID: req.EntryNodeId,
		TenantID:    req.TenantId,
	}

	if err := s.workflowRepo.Save(ctx, wf); err != nil {
		return nil, fmt.Errorf("save workflow: %w", err)
	}

	return &pb.Workflow{
		Id:           wf.ID,
		Name:         wf.Name,
		Description:  wf.Description,
		Nodes:        wf.Nodes,
		Edges:        wf.Edges,
		EntryNodeId:  wf.EntryNodeID,
		TenantId:     wf.TenantID,
		CreatedAt:    wf.CreatedAt.Unix(),
		UpdatedAt:    wf.UpdatedAt.Unix(),
	}, nil
}

// GetWorkflow retrieves a workflow by ID
func (s *HarnessService) GetWorkflow(ctx context.Context, req *pb.GetWorkflowRequest) (*pb.Workflow, error) {
	wf, err := s.workflowRepo.Get(ctx, req.Id)
	if err != nil {
		return nil, fmt.Errorf("get workflow: %w", err)
	}

	return &pb.Workflow{
		Id:           wf.ID,
		Name:         wf.Name,
		Description:  wf.Description,
		Nodes:        wf.Nodes,
		Edges:        wf.Edges,
		EntryNodeId:  wf.EntryNodeID,
		TenantId:     wf.TenantID,
		CreatedAt:    wf.CreatedAt.Unix(),
		UpdatedAt:    wf.UpdatedAt.Unix(),
	}, nil
}

// ListWorkflows lists all workflows
func (s *HarnessService) ListWorkflows(ctx context.Context, req *pb.ListWorkflowsRequest) (*pb.ListWorkflowsResponse, error) {
	workflows, err := s.workflowRepo.List(ctx, req.TenantId)
	if err != nil {
		return nil, fmt.Errorf("list workflows: %w", err)
	}

	var pbWorkflows []*pb.Workflow
	for _, wf := range workflows {
		pbWorkflows = append(pbWorkflows, &pb.Workflow{
			Id:           wf.ID,
			Name:         wf.Name,
			Description:  wf.Description,
			Nodes:        wf.Nodes,
			Edges:        wf.Edges,
			EntryNodeId:  wf.EntryNodeID,
			TenantId:     wf.TenantID,
			CreatedAt:    wf.CreatedAt.Unix(),
			UpdatedAt:    wf.UpdatedAt.Unix(),
		})
	}

	return &pb.ListWorkflowsResponse{Workflows: pbWorkflows}, nil
}

// DeleteWorkflow deletes a workflow
func (s *HarnessService) DeleteWorkflow(ctx context.Context, req *pb.DeleteWorkflowRequest) (*commonpb.Empty, error) {
	if err := s.workflowRepo.Delete(ctx, req.Id); err != nil {
		return nil, fmt.Errorf("delete workflow: %w", err)
	}
	return &commonpb.Empty{}, nil
}

// ExecuteWorkflow executes a workflow using the workflow engine
func (s *HarnessService) ExecuteWorkflow(ctx context.Context, req *pb.ExecuteWorkflowRequest) (*pb.ExecuteWorkflowResponse, error) {
	if s.workflowEngine == nil {
		return nil, fmt.Errorf("workflow engine not initialized")
	}

	result, execID, err := s.workflowEngine.Execute(ctx, req.Id, req.Input, "", 0)

	resp := &pb.ExecuteWorkflowResponse{
		WorkflowId:  req.Id,
		ExecutionId: execID,
	}

	if result != nil {
		resp.FinalOutput = result.FinalOutput
		for _, nr := range result.Nodes {
			resp.Nodes = append(resp.Nodes, &pb.WorkflowNodeResult{
				NodeId: nr.NodeID,
				Output: nr.Output,
				Error:  nr.Error,
			})
		}
		if result.Error != "" {
			resp.Error = result.Error
		}
		if err != nil {
			resp.Status = "failed"
		} else {
			resp.Status = "completed"
		}
	} else if err != nil {
		resp.Error = err.Error()
		resp.Status = "failed"
	}

	return resp, nil
}

// UpdateWorkflow updates an existing workflow
func (s *HarnessService) UpdateWorkflow(ctx context.Context, req *pb.UpdateWorkflowRequest) (*pb.Workflow, error) {
	if req.Nodes != "" && req.Edges != "" {
		if err := s.workflowEngine.ValidateWorkflow(req.Nodes, req.Edges, req.EntryNodeId); err != nil {
			return nil, fmt.Errorf("workflow validation: %w", err)
		}
	}

	wf, err := s.workflowRepo.Get(ctx, req.Id)
	if err != nil {
		return nil, fmt.Errorf("get workflow: %w", err)
	}

	if req.Name != "" {
		wf.Name = req.Name
	}
	if req.Description != "" {
		wf.Description = req.Description
	}
	if req.Nodes != "" {
		wf.Nodes = req.Nodes
	}
	if req.Edges != "" {
		wf.Edges = req.Edges
	}
	if req.EntryNodeId != "" {
		wf.EntryNodeID = req.EntryNodeId
	}

	if err := s.workflowRepo.Save(ctx, wf); err != nil {
		return nil, fmt.Errorf("save workflow: %w", err)
	}

	return &pb.Workflow{
		Id:          wf.ID,
		Name:        wf.Name,
		Description: wf.Description,
		Nodes:       wf.Nodes,
		Edges:       wf.Edges,
		EntryNodeId: wf.EntryNodeID,
		TenantId:    wf.TenantID,
		CreatedAt:   wf.CreatedAt.Unix(),
		UpdatedAt:   wf.UpdatedAt.Unix(),
	}, nil
}

// GetWorkflowExecution retrieves a workflow execution by ID
func (s *HarnessService) GetWorkflowExecution(ctx context.Context, req *pb.GetWorkflowExecutionRequest) (*pb.WorkflowExecution, error) {
	if s.workflowEngine == nil {
		return nil, fmt.Errorf("workflow engine not initialized")
	}

	exec, err := s.workflowEngine.GetExecution(ctx, req.ExecutionId)
	if err != nil {
		return nil, fmt.Errorf("get execution: %w", err)
	}

	return executionToPB(exec), nil
}

// ListWorkflowExecutions lists executions for a workflow
func (s *HarnessService) ListWorkflowExecutions(ctx context.Context, req *pb.ListWorkflowExecutionsRequest) (*pb.ListWorkflowExecutionsResponse, error) {
	if s.workflowEngine == nil {
		return nil, fmt.Errorf("workflow engine not initialized")
	}

	limit := int(req.Limit)
	if limit <= 0 {
		limit = 20
	}

	executions, err := s.workflowEngine.ListExecutions(ctx, req.WorkflowId, limit)
	if err != nil {
		return nil, fmt.Errorf("list executions: %w", err)
	}

	var pbExecs []*pb.WorkflowExecution
	for _, exec := range executions {
		pbExecs = append(pbExecs, executionToPB(exec))
	}

	return &pb.ListWorkflowExecutionsResponse{Executions: pbExecs}, nil
}

// CancelWorkflowExecution cancels a running workflow execution
func (s *HarnessService) CancelWorkflowExecution(ctx context.Context, req *pb.CancelWorkflowExecutionRequest) (*commonpb.Empty, error) {
	if s.workflowEngine == nil {
		return nil, fmt.Errorf("workflow engine not initialized")
	}

	if err := s.workflowEngine.CancelExecution(ctx, req.ExecutionId); err != nil {
		return nil, fmt.Errorf("cancel execution: %w", err)
	}

	return &commonpb.Empty{}, nil
}

// ValidateWorkflow validates a workflow's DAG structure
func (s *HarnessService) ValidateWorkflow(ctx context.Context, req *pb.ValidateWorkflowRequest) (*pb.ValidateWorkflowResponse, error) {
	err := s.workflowEngine.ValidateWorkflow(req.Nodes, req.Edges, req.EntryNodeId)
	if err != nil {
		return &pb.ValidateWorkflowResponse{
			Valid:  false,
			Errors: []string{err.Error()},
		}, nil
	}
	return &pb.ValidateWorkflowResponse{Valid: true}, nil
}

// executionToPB converts an ExecutionRecord to a protobuf WorkflowExecution
func executionToPB(exec *wfengine.ExecutionRecord) *pb.WorkflowExecution {
	pbExec := &pb.WorkflowExecution{
		Id:          exec.ID,
		WorkflowId:  exec.WorkflowID,
		Status:      exec.Status,
		Input:       exec.Input,
		FinalOutput: exec.FinalOutput,
		Error:       exec.Error,
		StartedAt:   exec.StartedAt.Unix(),
		DurationMs:  exec.Duration,
	}

	if exec.CompletedAt != nil {
		pbExec.CompletedAt = exec.CompletedAt.Unix()
	}

	if exec.NodeResults != "" {
		var nodeResults []wfengine.NodeResultDetail
		if err := json.Unmarshal([]byte(exec.NodeResults), &nodeResults); err == nil {
			for _, nr := range nodeResults {
				pbExec.NodeResults = append(pbExec.NodeResults, &pb.WorkflowNodeResult{
					NodeId:   nr.NodeID,
					Output:   nr.Output,
					Error:    nr.Error,
					NodeType: nr.NodeType,
				})
			}
		}
	}

	return pbExec
}

// initializeDefaultGateway seeds default LLM Gateway providers and routing rules
func (s *HarnessService) initializeDefaultGateway(ctx context.Context) {
	if s.gateway == nil {
		return
	}

	// Seed default provider configurations (disabled by default — user enables after adding API key)
	defaultProviders := []*gateway.GatewayConfig{
		{
			Name:        "OpenAI",
			Description: "OpenAI GPT models via official API",
			Provider:    string(gateway.ProviderOpenAI),
			APIKey:      "", // User must provide their own key
			BaseURL:     "https://api.openai.com/v1",
			Models:      `[{"model_id":"gpt-4o","model_name":"GPT-4o","max_tokens":128000,"input_price":2.50,"output_price":10.00},{"model_id":"gpt-4o-mini","model_name":"GPT-4o Mini","max_tokens":128000,"input_price":0.15,"output_price":0.60},{"model_id":"gpt-4-turbo","model_name":"GPT-4 Turbo","max_tokens":128000,"input_price":10.00,"output_price":30.00},{"model_id":"gpt-3.5-turbo","model_name":"GPT-3.5 Turbo","max_tokens":16384,"input_price":0.50,"output_price":1.50}]`,
			RateLimit:   100,
			Timeout:     30,
			RetryCount:  3,
			Priority:    1,
			Enabled:     false,
		},
		{
			Name:        "DashScope (Qwen)",
			Description: "Alibaba Cloud DashScope Qwen models via OpenAI-compatible API",
			Provider:    string(gateway.ProviderDashScope),
			APIKey:      "", // User must provide their own key
			BaseURL:     "https://dashscope.aliyuncs.com/compatible-mode/v1",
			Models:      `[{"model_id":"qwen-turbo","model_name":"Qwen Turbo","max_tokens":131072,"input_price":0.30,"output_price":0.60},{"model_id":"qwen-plus","model_name":"Qwen Plus","max_tokens":131072,"input_price":0.80,"output_price":2.00},{"model_id":"qwen-max","model_name":"Qwen Max","max_tokens":32768,"input_price":2.40,"output_price":9.60},{"model_id":"qwen-max-longcontext","model_name":"Qwen Max LongContext","max_tokens":131072,"input_price":2.40,"output_price":9.60}]`,
			RateLimit:   100,
			Timeout:     30,
			RetryCount:  3,
			Priority:    2,
			Enabled:     false,
		},
		{
			Name:        "Anthropic",
			Description: "Anthropic Claude models via official API",
			Provider:    string(gateway.ProviderAnthropic),
			APIKey:      "", // User must provide their own key
			BaseURL:     "https://api.anthropic.com",
			Models:      `[{"model_id":"claude-sonnet-4-20250514","model_name":"Claude Sonnet 4","max_tokens":200000,"input_price":3.00,"output_price":15.00},{"model_id":"claude-3-5-haiku-20241022","model_name":"Claude 3.5 Haiku","max_tokens":200000,"input_price":0.80,"output_price":4.00},{"model_id":"claude-opus-4-20250514","model_name":"Claude Opus 4","max_tokens":200000,"input_price":15.00,"output_price":75.00}]`,
			RateLimit:   50,
			Timeout:     60,
			RetryCount:  3,
			Priority:    3,
			Enabled:     false,
		},
	}

	// Check existing configs to avoid duplicates
	existingConfigs, _ := s.gateway.ListConfigs(ctx, "")
	existingNames := make(map[string]bool)
	for _, c := range existingConfigs {
		existingNames[c.Name] = true
	}

	seeded := 0
	for _, cfg := range defaultProviders {
		if existingNames[cfg.Name] {
			continue
		}
		if err := s.gateway.AddConfig(ctx, cfg); err != nil {
			fmt.Printf("[Harness/Gateway] Failed to seed provider %s: %v\n", cfg.Name, err)
			continue
		}
		seeded++
		fmt.Printf("[Harness/Gateway] Seeded default provider: %s (disabled — add API key to enable)\n", cfg.Name)
	}

	// Seed default routing rules
	defaultRoutes := []*gateway.GatewayRoute{
		{
			Name:      "High-Quality Route",
			Pattern:   "quality-sensitive",
			ModelID:   "gpt-4o",
			Fallbacks: `["qwen-max","claude-sonnet-4-20250514"]`,
			Enabled:   true,
		},
		{
			Name:      "Cost-Effective Route",
			Pattern:   "cost-sensitive",
			ModelID:   "qwen-turbo",
			Fallbacks: `["gpt-4o-mini","gpt-3.5-turbo"]`,
			Enabled:   true,
		},
		{
			Name:      "Default Route",
			Pattern:   "default",
			ModelID:   "qwen-plus",
			Fallbacks: `["gpt-4o-mini","qwen-turbo"]`,
			Enabled:   true,
		},
	}

	existingRoutes, _ := s.gateway.ListRoutes(ctx, "")
	existingRouteNames := make(map[string]bool)
	for _, r := range existingRoutes {
		existingRouteNames[r.Name] = true
	}

	for _, route := range defaultRoutes {
		if existingRouteNames[route.Name] {
			continue
		}
		if err := s.gateway.AddRoute(ctx, route); err != nil {
			fmt.Printf("[Harness/Gateway] Failed to seed route %s: %v\n", route.Name, err)
			continue
		}
		seeded++
		fmt.Printf("[Harness/Gateway] Seeded default route: %s\n", route.Name)
	}

	if seeded > 0 {
		fmt.Printf("[Harness/Gateway] Seeded %d default gateway configurations\n", seeded)
	}
}
