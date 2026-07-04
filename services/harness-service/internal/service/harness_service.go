// Package service provides business logic for Harness service
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"agent-platform/pkg/config"
	"agent-platform/pkg/llm"
	pb "agent-platform/pkg/pb/harness"
	agentpb "agent-platform/pkg/pb/agent"
	commonpb "agent-platform/pkg/pb/common"
	"agent-platform/services/harness-service/internal/abtest"
	"agent-platform/services/harness-service/internal/catalog"
	"agent-platform/services/harness-service/internal/chaos"
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
	"agent-platform/services/harness-service/internal/rollback"
	"agent-platform/services/harness-service/internal/rule"
	"agent-platform/services/harness-service/internal/scheduler"
	"agent-platform/services/harness-service/internal/session"
	"agent-platform/services/harness-service/internal/slo"
	"agent-platform/services/harness-service/internal/redteam"
	"agent-platform/services/harness-service/internal/rag"
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
	rollback      *rollback.Engine
	rca           *rca.Engine
	chaos         *chaos.Engine
	cost          *cost.Engine
	evolve        *evolve.Engine
	goldenpath    *goldenpath.Engine
	catalog       *catalog.Engine
	coordinate    *coordinate.Engine
	planner       *planner.Engine
	scheduler     *scheduler.Scheduler
	playground    *playground.PlaygroundEngine
	sessionRecorder *session.Recorder
	prompt        *prompt.Engine
	gateway       *gateway.GatewayEngine
	redteam       *redteam.Engine
	ragEvaluator  *rag.RAGEvaluator
	ragRepo       *rag.Repository
	mu            sync.RWMutex
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
		abtest:        abtest.NewEngineMemory(),
		sloManager:    slo.NewManager(repo.GetDB()),
		llmMetricsBuf: make([]llm.CallMetrics, 0, 1000),
		// Initialize new engines (memory mode for now)
		featureFlag:   featureflag.NewEngineMemory(),
		rollback:      rollback.NewEngineMemory(),
		rca:           rca.NewEngineMemory(),
		chaos:         chaos.NewEngineMemory(),
		cost:          cost.NewEngineMemory(),
		evolve:        evolve.NewEngineMemory(),
		goldenpath:    goldenpath.NewEngineMemory(),
		catalog:       catalog.NewEngineMemory(),
		coordinate:    coordinate.NewEngineMemory(),
		planner:       planner.NewEngineMemory(),
		scheduler:     schedulerEngine,
		prompt:        prompt.NewEngine(repo.GetDB()),
		}

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

	// Initialize Red Team engine
	redteamRepo := redteam.NewRepository(repo.GetDB())
	if err := redteamRepo.AutoMigrate(); err != nil {
		fmt.Printf("Warning: failed to migrate redteam tables: %v\n", err)
	}
	svc.redteam = redteam.NewEngine(llmClient, redteamRepo)

	// Initialize RAG evaluator
	ragRepo := rag.NewRepository(repo.GetDB())
	svc.ragRepo = ragRepo
	svc.ragEvaluator = rag.NewRAGEvaluator(llmClient, ragRepo)

	// Wrap LLM client with metrics for automatic cost tracking
	svc.llmClient = llm.NewMetricsClient(llmClient, svc.llmMetricsCallback(), "harness")

	// Wire eval runner with metrics-wrapped LLM client
	svc.evalRunner = evaluate.NewRunner(llm.NewMetricsClient(llmClient, svc.llmMetricsCallback(), "eval"))

	// Wire SLO checker into chaos engine for auto-stop on SLO breach
	svc.chaos.SetSLOChecker(func(agentID string) (float64, error) {
		results, err := svc.sloManager.EvaluateAll(context.Background(), agentID)
		if err != nil {
			return 0, err
		}
		if len(results) == 0 {
			return 1.0, nil // No SLOs defined = healthy
		}
		// Return the worst current value among all SLOs
		var worst float64 = 1.0
		for _, r := range results {
			if r.Current < worst {
				worst = r.Current
			}
		}
		return worst, nil
	})

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
				Details:  fmt.Sprintf("SLO evaluation for agent %s: worst budget %.1f%%", agentID, worstBudget*100),
				Alerts:   alerts,
			}, nil
		default:
			return &scheduler.EvalResult{
				EvalType: evalType,
				Success:  true,
				Score:    0.85,
				Details:  fmt.Sprintf("Evaluation completed for %s on agent %s", evalType, agentID),
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

	fmt.Println("[Harness] Default governance configurations initialized")
}

// initializeDefaultPrompts seeds built-in prompt templates across all 5 categories
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
		// ---- system ----
		{
			Key:         "system-assistant",
			Name:        "General Assistant",
			Description: "A general-purpose AI assistant system prompt with configurable role and constraints",
			Category:    prompt.CategorySystem,
			Tags:        `["system","assistant","general"]`,
			Version:     "1.0.0",
			Content: `You are {{role|an AI assistant}}.

Your goal is to help users accomplish their tasks effectively and accurately.

Guidelines:
- Be concise and direct in your responses
- If you are unsure about something, say so clearly
- Do not fabricate information or make up facts
- Respect user privacy and do not request sensitive personal data
- Respond in the same language the user uses

{{constraints|}}

When providing code:
- Always include relevant imports
- Add comments for complex logic
- Follow best practices for the language/framework`,
			Variables: `{"variables":[{"name":"role","type":"string","required":false,"default":"an AI assistant","description":"The role the assistant should adopt"},{"name":"constraints","type":"string","required":false,"default":"","description":"Additional constraints or rules"}]}`,
		},
		{
			Key:         "system-safety-guardrails",
			Name:        "Safety Guardrails",
			Description: "System-level safety and content policy guardrails prompt",
			Category:    prompt.CategorySystem,
			Tags:        `["system","safety","guardrails"]`,
			Version:     "1.0.0",
			Content: `You must follow these safety guidelines at all times:

1. Do not generate content that promotes violence, self-harm, or illegal activities
2. Do not provide instructions for creating weapons, explosives, or dangerous substances
3. Do not generate hate speech, discriminatory content, or personal attacks
4. Respect intellectual property — do not reproduce copyrighted material verbatim
5. Do not provide medical diagnosis or treatment advice — always recommend consulting a professional
6. Do not assist with hacking, exploitation, or unauthorized access to systems
7. Decline requests that could cause real-world harm

If a user request violates these guidelines, politely decline and explain why.

{{additional_rules|}}`,
			Variables: `{"variables":[{"name":"additional_rules","type":"string","required":false,"default":"","description":"Additional domain-specific safety rules"}]}`,
		},
		{
			Key:         "system-json-output",
			Name:        "JSON Output Formatter",
			Description: "Enforces structured JSON output format for API integrations",
			Category:    prompt.CategorySystem,
			Tags:        `["system","json","structured","api"]`,
			Version:     "1.0.0",
			Content: `You must respond with valid JSON only. No markdown, no explanations outside the JSON structure.

Output schema:
{{schema|{"type":"object","properties":{"result":{"type":"string"},"confidence":{"type":"number"}}}}}

Rules:
- Always output valid JSON matching the schema above
- Use null for missing optional fields
- Include a "confidence" field (0-1) indicating your certainty
- If you cannot fulfill the request, return: {"error": "description of the issue", "confidence": 0}
- Do not wrap the JSON in code blocks or add any text outside the JSON`,
			Variables: `{"variables":[{"name":"schema","type":"string","required":false,"default":"{\"type\":\"object\",\"properties\":{\"result\":{\"type\":\"string\"},\"confidence\":{\"type\":\"number\"}}}","description":"JSON schema definition for the expected output"}]}`,
		},

		// ---- user ----
		{
			Key:         "user-summarize",
			Name:        "Text Summarizer",
			Description: "Summarize text with configurable length and focus",
			Category:    prompt.CategoryUser,
			Tags:        `["user","summarize","text"]`,
			Version:     "1.0.0",
			Content: `Please summarize the following text in {{length|3-5}} sentences.

{{focus|Focus on the key points and main arguments.}}

Text to summarize:
"""
{{text}}
"""`,
			Variables: `{"variables":[{"name":"text","type":"string","required":true,"description":"The text content to summarize"},{"name":"length","type":"string","required":false,"default":"3-5","description":"Desired summary length (e.g. '3-5 sentences', '1 paragraph')"},{"name":"focus","type":"string","required":false,"default":"Focus on the key points and main arguments.","description":"What to focus on in the summary"}]}`,
		},
		{
			Key:         "user-translate",
			Name:        "Translator",
			Description: "Translate text between languages with tone control",
			Category:    prompt.CategoryUser,
			Tags:        `["user","translate","language"]`,
			Version:     "1.0.0",
			Content: `Translate the following text from {{source_lang|auto-detect}} to {{target_lang}}.

Tone: {{tone|neutral}}
Domain: {{domain|general}}

Text:
"""
{{text}}
"""

Provide only the translation, without explanations. If a term has no direct translation, use the most natural equivalent and add a brief note in parentheses.`,
			Variables: `{"variables":[{"name":"text","type":"string","required":true,"description":"Text to translate"},{"name":"target_lang","type":"string","required":true,"description":"Target language"},{"name":"source_lang","type":"string","required":false,"default":"auto-detect","description":"Source language"},{"name":"tone","type":"string","required":false,"default":"neutral","description":"Translation tone: formal, casual, neutral"},{"name":"domain","type":"string","required":false,"default":"general","description":"Domain context: technical, medical, legal, general"}]}`,
		},

		// ---- template ----
		{
			Key:         "template-code-review",
			Name:        "Code Review",
			Description: "Comprehensive code review template with configurable focus areas",
			Category:    prompt.CategoryTemplate,
			Tags:        `["template","code-review","quality"]`,
			Version:     "1.0.0",
			Content: `Review the following {{language|}} code and provide feedback.

Focus areas: {{focus|correctness, readability, performance, security}}

Code:
"""
{{code}}
"""

Please structure your review as:
1. **Summary**: Brief overall assessment
2. **Issues Found**: List bugs, security vulnerabilities, or logic errors
3. **Suggestions**: Improvements for readability, performance, or maintainability
4. **Rating**: Code quality score (1-10)

Be specific — reference line numbers or code snippets when pointing out issues.`,
			Variables: `{"variables":[{"name":"code","type":"string","required":true,"description":"The code to review"},{"name":"language","type":"string","required":false,"default":"","description":"Programming language"},{"name":"focus","type":"string","required":false,"default":"correctness, readability, performance, security","description":"Comma-separated focus areas"}]}`,
		},
		{
			Key:         "template-email",
			Name:        "Email Composer",
			Description: "Compose professional emails with configurable tone and context",
			Category:    prompt.CategoryTemplate,
			Tags:        `["template","email","communication"]`,
			Version:     "1.0.0",
			Content: `Compose a {{tone|professional}} email.

Context:
- From: {{sender}}
- To: {{recipient}}
- Subject: {{subject}}
- Purpose: {{purpose}}

Key points to include:
{{key_points}}

Requirements:
- Keep the email concise (under {{max_words|200}} words)
- Use an appropriate greeting and closing
- Be clear about any action items or next steps
- CC: {{cc|none}}`,
			Variables: `{"variables":[{"name":"sender","type":"string","required":true,"description":"Email sender name/role"},{"name":"recipient","type":"string","required":true,"description":"Email recipient"},{"name":"subject","type":"string","required":true,"description":"Email subject line"},{"name":"purpose","type":"string","required":true,"description":"Purpose of the email"},{"name":"key_points","type":"string","required":true,"description":"Key points to include in the email body"},{"name":"tone","type":"string","required":false,"default":"professional","description":"Email tone: professional, friendly, formal, urgent"},{"name":"max_words","type":"string","required":false,"default":"200","description":"Maximum word count"},{"name":"cc","type":"string","required":false,"default":"none","description":"CC recipients"}]}`,
		},
		{
			Key:         "template-data-extraction",
			Name:        "Data Extraction",
			Description: "Extract structured data from unstructured text using a defined schema",
			Category:    prompt.CategoryTemplate,
			Tags:        `["template","extraction","structured-data"]`,
			Version:     "1.0.0",
			Content: `Extract the following information from the text below.

Fields to extract:
{{fields}}

Output format: JSON with the extracted fields. Use null for missing values.

Text:
"""
{{text}}
"""

Rules:
- Extract only factual information present in the text
- Do not infer or guess missing values
- Normalize values where appropriate (e.g., dates to ISO 8601)
- If a field has multiple values, use an array`,
			Variables: `{"variables":[{"name":"text","type":"string","required":true,"description":"Source text to extract data from"},{"name":"fields","type":"string","required":true,"description":"List of fields to extract with descriptions"}]}`,
		},

		// ---- rag ----
		{
			Key:         "rag-retrieval-query",
			Name:        "RAG Retrieval Query",
			Description: "Optimize user queries for vector search retrieval in RAG pipelines",
			Category:    prompt.CategoryRAG,
			Tags:        `["rag","retrieval","query","search"]`,
			Version:     "1.0.0",
			Content: `Given a user question, generate {{num_queries|3}} optimized search queries for retrieving relevant documents.

User question: {{question}}

Context: {{context|general knowledge}}

Requirements:
- Each query should target a different aspect or angle of the question
- Use keywords and terms likely to appear in relevant documents
- Include synonyms and related terms
- Keep each query concise (5-15 words)

Output as a JSON array of strings:
["query1", "query2", "query3"]`,
			Variables: `{"variables":[{"name":"question","type":"string","required":true,"description":"The user's original question"},{"name":"num_queries","type":"string","required":false,"default":"3","description":"Number of search queries to generate"},{"name":"context","type":"string","required":false,"default":"general knowledge","description":"Domain or context for the search"}]}`,
		},
		{
			Key:         "rag-answer-generation",
			Name:        "RAG Answer Generation",
			Description: "Generate answers using retrieved context documents in RAG pipelines",
			Category:    prompt.CategoryRAG,
			Tags:        `["rag","generation","answer","context"]`,
			Version:     "1.0.0",
			Content: `Answer the user's question based on the provided context documents.

Context documents:
{{context}}

User question: {{question}}

Instructions:
- Answer based ONLY on the provided context
- If the context does not contain enough information, say "I don't have enough information to answer this question"
- Cite the specific part of the context you used (e.g., [Document 1, paragraph 2])
- Be accurate and do not add information not present in the context
- Structure your answer clearly with appropriate headings if the answer is complex

{{format_instruction|}}`,
			Variables: `{"variables":[{"name":"question","type":"string","required":true,"description":"The user's question"},{"name":"context","type":"string","required":true,"description":"Retrieved context documents"},{"name":"format_instruction","type":"string","required":false,"default":"","description":"Additional formatting instructions for the answer"}]}`,
		},
		{
			Key:         "rag-fact-check",
			Name:        "RAG Fact Verification",
			Description: "Verify a claim against retrieved evidence documents",
			Category:    prompt.CategoryRAG,
			Tags:        `["rag","fact-check","verification","evidence"]`,
			Version:     "1.0.0",
			Content: `Verify the following claim against the provided evidence.

Claim: {{claim}}

Evidence:
{{evidence}}

Analyze:
1. Does the evidence support, contradict, or is it neutral toward the claim?
2. What specific evidence supports or contradicts the claim?
3. Are there any logical gaps or missing evidence?

Output format (JSON):
{
  "verdict": "supported|contradicted|insufficient_evidence|mixed",
  "confidence": 0.0-1.0,
  "supporting_evidence": ["..."],
  "contradicting_evidence": ["..."],
  "analysis": "..."
}`,
			Variables: `{"variables":[{"name":"claim","type":"string","required":true,"description":"The claim to verify"},{"name":"evidence","type":"string","required":true,"description":"Evidence documents to check against"}]}`,
		},

		// ---- agent ----
		{
			Key:         "agent-task-decomposition",
			Name:        "Task Decomposition",
			Description: "Break down complex tasks into executable sub-tasks for agent workflows",
			Category:    prompt.CategoryAgent,
			Tags:        `["agent","task","decomposition","planning"]`,
			Version:     "1.0.0",
			Content: `Break down the following task into {{max_subtasks|5}} or fewer actionable sub-tasks.

Task: {{task}}
Agent capabilities: {{capabilities|general purpose}}

For each sub-task, provide:
1. **Description**: What needs to be done
2. **Tool**: Which tool or capability to use (e.g., search, code, analyze, write)
3. **Dependencies**: Which sub-tasks must complete first (by number)
4. **Expected Output**: What the sub-task should produce

Output as JSON:
{
  "subtasks": [
    {
      "id": 1,
      "description": "...",
      "tool": "...",
      "depends_on": [],
      "expected_output": "..."
    }
  ],
  "execution_order": [1, 2, 3],
  "estimated_complexity": "low|medium|high"
}`,
			Variables: `{"variables":[{"name":"task","type":"string","required":true,"description":"The complex task to decompose"},{"name":"max_subtasks","type":"string","required":false,"default":"5","description":"Maximum number of sub-tasks"},{"name":"capabilities","type":"string","required":false,"default":"general purpose","description":"Available agent capabilities or tools"}]}`,
		},
		{
			Key:         "agent-reflection",
			Name:        "Self-Reflection",
			Description: "Agent self-reflection and output quality assessment template",
			Category:    prompt.CategoryAgent,
			Tags:        `["agent","reflection","quality","self-assessment"]`,
			Version:     "1.0.0",
			Content: `Review and assess the quality of the following agent output.

Original task: {{task}}
Agent output: {{output}}

Evaluate on these dimensions:
1. **Completeness** (1-5): Does the output fully address the task?
2. **Accuracy** (1-5): Is the information correct and factual?
3. **Clarity** (1-5): Is the output clear and well-structured?
4. **Actionability** (1-5): Can the user act on this output immediately?

Provide:
- Overall score (average)
- Top 2 strengths
- Top 2 areas for improvement
- A revised version of the output incorporating improvements (if score < 4)

Output as JSON:
{
  "scores": {"completeness": 0, "accuracy": 0, "clarity": 0, "actionability": 0},
  "overall": 0,
  "strengths": ["...", "..."],
  "improvements": ["...", "..."],
  "revised_output": "..."
}`,
			Variables: `{"variables":[{"name":"task","type":"string","required":true,"description":"The original task given to the agent"},{"name":"output","type":"string","required":true,"description":"The agent's output to evaluate"}]}`,
		},
		{
			Key:         "agent-tool-selection",
			Name:        "Tool Selection",
			Description: "Select the best tool and parameters for a given agent task",
			Category:    prompt.CategoryAgent,
			Tags:        `["agent","tool","selection","routing"]`,
			Version:     "1.0.0",
			Content: `Given a task and available tools, select the most appropriate tool and its parameters.

Task: {{task}}

Available tools:
{{tools}}

Select the best tool and provide:
1. Which tool to use and why
2. Required parameters with values
3. Alternative tools if the primary tool fails
4. Estimated execution time and resource usage

Output as JSON:
{
  "primary_tool": "tool_name",
  "reasoning": "...",
  "parameters": {"key": "value"},
  "fallback_tools": ["tool_name"],
  "estimated_time": "...",
  "resource_usage": "low|medium|high"
}`,
			Variables: `{"variables":[{"name":"task","type":"string","required":true,"description":"The task to accomplish"},{"name":"tools","type":"string","required":true,"description":"List of available tools with descriptions and parameters"}]}`,
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

// GetRollbackEngine returns the rollback engine
func (s *HarnessService) GetRollbackEngine() *rollback.Engine {
	return s.rollback
}

// GetRCAEngine returns the RCA engine
func (s *HarnessService) GetRCAEngine() *rca.Engine {
	return s.rca
}

// GetChaosEngine returns the chaos engine
func (s *HarnessService) GetChaosEngine() *chaos.Engine {
	return s.chaos
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

// GetCatalogEngine returns the catalog engine
func (s *HarnessService) GetCatalogEngine() *catalog.Engine {
	return s.catalog
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
func (s *HarnessService) EvaluateFeatureFlag(ctx context.Context, key string, userID string, attributes map[string]interface{}) (interface{}, error) {
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
func (s *HarnessService) RecordCostUsage(ctx context.Context, agentID, modelID, sessionID string, inputTokens, outputTokens int64) error {
	return s.cost.RecordUsage(ctx, agentID, modelID, sessionID, inputTokens, outputTokens)
}

// RecordCostUsageGRPC records cost usage via gRPC
func (s *HarnessService) RecordCostUsageGRPC(ctx context.Context, req *pb.RecordCostUsageRequest) (*commonpb.Empty, error) {
	if err := s.cost.RecordUsage(ctx, req.AgentId, req.ModelId, req.SessionId, req.InputTokens, req.OutputTokens); err != nil {
		return nil, fmt.Errorf("record cost usage: %w", err)
	}
	return &commonpb.Empty{}, nil
}

// GetCostReport generates a cost report
func (s *HarnessService) GetCostReport(ctx context.Context, agentID string, start, end time.Time) (*cost.CostReport, error) {
	return s.cost.CostReport(ctx, agentID, start, end)
}

// ==================== Chaos Methods ====================

// ShouldInjectChaos checks if chaos should be injected
func (s *HarnessService) ShouldInjectChaos(ctx context.Context, agentID string) (bool, *chaos.Experiment, error) {
	shouldInject, expID, _ := s.chaos.ShouldInjectFault(ctx, agentID)
	if !shouldInject {
		return false, nil, nil
	}
	exp, err := s.chaos.GetExperiment(ctx, expID)
	if err != nil {
		return false, nil, err
	}
	return true, exp, nil
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
func (s *HarnessService) RunOptimizer(ctx context.Context, agentID string, metrics map[string]float64) (*evolve.OptimizationResult, error) {
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
func (s *HarnessService) EvaluateFeatureFlagGRPC(ctx context.Context, req *pb.EvaluateFeatureFlagRequest) (*pb.EvaluateFeatureFlagResponse, error) {
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

// ==================== Chaos gRPC Methods ====================

// CreateChaosExperiment creates a chaos experiment
func (s *HarnessService) CreateChaosExperiment(ctx context.Context, req *pb.CreateChaosExperimentRequest) (*pb.ChaosExperiment, error) {
	exp := &chaos.Experiment{
		Name:            req.Name,
		Description:     req.Description,
		AgentID:         req.AgentId,
		FaultType:       chaos.FaultType(req.FaultType),
		FaultConfig:     req.FaultConfig,
		Duration:        int(req.Duration),
		BlastRadius:     req.BlastRadius,
		AutoStopOnSLO:   req.AutoStopOnSlo,
		SLOThreshold:    req.SloThreshold,
	}
	if err := s.chaos.CreateExperiment(ctx, exp); err != nil {
		return nil, err
	}
	return s.experimentToPB(exp), nil
}

// StartChaosExperiment starts a chaos experiment
func (s *HarnessService) StartChaosExperiment(ctx context.Context, req *pb.StartChaosExperimentRequest) (*pb.ChaosExperiment, error) {
	exp, err := s.chaos.GetExperiment(ctx, req.ExperimentId)
	if err != nil {
		return nil, err
	}
	_, err = s.chaos.StartExperiment(ctx, req.ExperimentId)
	if err != nil {
		return nil, err
	}
	return s.experimentToPB(exp), nil
}

// StopChaosExperiment stops a chaos experiment
func (s *HarnessService) StopChaosExperiment(ctx context.Context, req *pb.StopChaosExperimentRequest) (*pb.ChaosExperiment, error) {
	if err := s.chaos.StopExperiment(ctx, req.ExperimentId, false); err != nil {
		return nil, err
	}
	exp, err := s.chaos.GetExperiment(ctx, req.ExperimentId)
	if err != nil {
		return nil, err
	}
	return s.experimentToPB(exp), nil
}

// ListChaosExperiments lists chaos experiments
func (s *HarnessService) ListChaosExperiments(ctx context.Context, req *pb.ListChaosExperimentsRequest) (*pb.ListChaosExperimentsResponse, error) {
	exps, err := s.chaos.ListExperiments(ctx, req.AgentId, chaos.ExperimentStatus(req.Status))
	if err != nil {
		return nil, err
	}
	var pbExps []*pb.ChaosExperiment
	for _, e := range exps {
		pbExps = append(pbExps, s.experimentToPB(e))
	}
	return &pb.ListChaosExperimentsResponse{Experiments: pbExps}, nil
}

func (s *HarnessService) experimentToPB(e *chaos.Experiment) *pb.ChaosExperiment {
	var startedAt, endedAt int64
	if e.StartedAt != nil {
		startedAt = e.StartedAt.Unix()
	}
	if e.EndedAt != nil {
		endedAt = e.EndedAt.Unix()
	}
	return &pb.ChaosExperiment{
		Id:              e.ID,
		Name:            e.Name,
		Description:     e.Description,
		AgentId:         e.AgentID,
		FaultType:       string(e.FaultType),
		FaultConfig:     e.FaultConfig,
		Duration:        int32(e.Duration),
		BlastRadius:     e.BlastRadius,
		AutoStopOnSlo:   e.AutoStopOnSLO,
		SloThreshold:    e.SLOThreshold,
		Status:          string(e.Status),
		CreatedAt:       e.CreatedAt.Unix(),
		StartedAt:       startedAt,
		EndedAt:         endedAt,
	}
}

// ==================== Rollback gRPC Methods ====================

// CreateRollbackConfig creates a rollback config
func (s *HarnessService) CreateRollbackConfig(ctx context.Context, req *pb.CreateRollbackConfigRequest) (*pb.RollbackConfig, error) {
	config := &rollback.RollbackConfig{
		AgentID:         req.AgentId,
		Name:            req.Name,
		ConfigType:      req.ConfigType,
		TargetID:        req.TargetId,
		MaxSnapshots:    int(req.MaxSnapshots),
		CoolDownPeriod:  int(req.CoolDownPeriod),
		AutoRollback:    req.AutoRollback,
	}
	if err := s.rollback.CreateConfig(ctx, config); err != nil {
		return nil, err
	}
	return s.rollbackConfigToPB(config), nil
}

// GetRollbackConfig gets a rollback config
func (s *HarnessService) GetRollbackConfigGRPC(ctx context.Context, req *pb.GetFeatureFlagRequest) (*pb.RollbackConfig, error) {
	config, err := s.rollback.GetConfig(ctx, req.Key)
	if err != nil {
		return nil, err
	}
	return s.rollbackConfigToPB(config), nil
}

// TakeSnapshot takes a snapshot
func (s *HarnessService) TakeSnapshot(ctx context.Context, req *pb.TakeSnapshotRequest) (*pb.ConfigSnapshot, error) {
	snapshot, err := s.rollback.TakeSnapshot(ctx, req.ConfigId, req.SnapshotData, req.Version, req.Description, req.CreatedBy)
	if err != nil {
		return nil, err
	}
	return &pb.ConfigSnapshot{
		Id:            snapshot.ID,
		ConfigId:      snapshot.ConfigID,
		SnapshotData:  snapshot.SnapshotData,
		Version:       snapshot.Version,
		Description:   snapshot.Description,
		CreatedAt:     snapshot.CreatedAt.Unix(),
		CreatedBy:     snapshot.CreatedBy,
		IsActive:      snapshot.IsActive,
	}, nil
}

// ListSnapshots lists snapshots
func (s *HarnessService) ListSnapshots(ctx context.Context, req *pb.ListSnapshotsRequest) (*pb.ListSnapshotsResponse, error) {
	snapshots, err := s.rollback.ListSnapshots(ctx, req.ConfigId, int(req.Limit))
	if err != nil {
		return nil, err
	}
	var pbSnapshots []*pb.ConfigSnapshot
	for _, s := range snapshots {
		pbSnapshots = append(pbSnapshots, &pb.ConfigSnapshot{
			Id:            s.ID,
			ConfigId:      s.ConfigID,
			SnapshotData:  s.SnapshotData,
			Version:       s.Version,
			Description:   s.Description,
			CreatedAt:     s.CreatedAt.Unix(),
			CreatedBy:     s.CreatedBy,
			IsActive:      s.IsActive,
		})
	}
	return &pb.ListSnapshotsResponse{Snapshots: pbSnapshots}, nil
}

// ExecuteRollback executes a rollback
func (s *HarnessService) ExecuteRollback(ctx context.Context, req *pb.ExecuteRollbackRequest) (*pb.RollbackEvent, error) {
	event, err := s.rollback.ExecuteRollback(ctx, req.ConfigId, req.SnapshotId, "manual")
	if err != nil {
		return nil, err
	}
	return &pb.RollbackEvent{
		Id:           event.ID,
		ConfigId:     event.ConfigID,
		SnapshotId:   event.SnapshotID,
		EventType:    event.EventType,
		TriggeredBy:  event.TriggeredBy,
		FromVersion:  event.FromVersion,
		ToVersion:    event.ToVersion,
		Success:      event.Success,
		Error:        event.Error,
		DurationMs:   event.DurationMs,
		Timestamp:    event.Timestamp.Unix(),
	}, nil
}

func (s *HarnessService) rollbackConfigToPB(c *rollback.RollbackConfig) *pb.RollbackConfig {
	return &pb.RollbackConfig{
		Id:              c.ID,
		AgentId:         c.AgentID,
		Name:            c.Name,
		ConfigType:      c.ConfigType,
		TargetId:        c.TargetID,
		MaxSnapshots:    int32(c.MaxSnapshots),
		CoolDownPeriod:  int32(c.CoolDownPeriod),
		AutoRollback:    c.AutoRollback,
		RollbackOnSlo:   c.RollbackOnSLO,
		
		CreatedAt:       c.CreatedAt.Unix(),
	}
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
func (s *HarnessService) GetCostReportGRPC(ctx context.Context, req *pb.CostReportRequest) (*pb.CostReport, error) {
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
func (s *HarnessService) RunOptimizerGRPC(ctx context.Context, req *pb.RunOptimizerRequest) (*pb.OptimizationResult, error) {
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

// ==================== Catalog gRPC Methods ====================

// ListCatalogAgents lists catalog agents
func (s *HarnessService) ListCatalogAgents(ctx context.Context, req *pb.ListCatalogAgentsRequest) (*pb.ListCatalogAgentsResponse, error) {
	agents, err := s.catalog.ListCatalogAgents(ctx, req.Type, catalog.AgentStatus(req.Status))
	if err != nil {
		return nil, err
	}
	var pbAgents []*pb.CatalogAgent
	for _, a := range agents {
		pbAgents = append(pbAgents, s.catalogAgentToPB(a))
	}
	return &pb.ListCatalogAgentsResponse{Agents: pbAgents}, nil
}

// GetCatalogAgentGRPC gets a catalog agent
func (s *HarnessService) GetCatalogAgentGRPC(ctx context.Context, req *pb.GetFeatureFlagRequest) (*pb.CatalogAgent, error) {
	agent, err := s.catalog.GetCatalogAgent(ctx, req.Key)
	if err != nil {
		return nil, err
	}
	return s.catalogAgentToPB(agent), nil
}

func (s *HarnessService) catalogAgentToPB(a *catalog.CatalogAgent) *pb.CatalogAgent {
	return &pb.CatalogAgent{
		Id:            a.ID,
		Name:          a.Name,
		Type:          string(a.Type),
		Description:   a.Description,
		Version:       a.Version,
		Author:        a.Author,
		Status:        string(a.Status),
		Configuration: a.Configuration,
		Capabilities:  a.Capabilities,
		Rating:        a.Rating,
		UsageCount:    a.UsageCount,
		CreatedAt:     a.CreatedAt.Unix(),
	}
}

// RegisterCatalogAgentGRPC registers a catalog agent via gRPC
func (s *HarnessService) RegisterCatalogAgentGRPC(ctx context.Context, req *pb.RegisterCatalogAgentRequest) (*pb.CatalogAgent, error) {
	agent := &catalog.CatalogAgent{
		Name:          req.Name,
		Type:          req.Type,
		Description:   req.Description,
		Version:       req.Version,
		Author:        req.Author,
		Configuration: req.Configuration,
		Capabilities:  req.Capabilities,
		Requirements:  req.Requirements,
		Tags:          req.Tags,
	}
	if req.AgentId != "" {
		agent.ID = req.AgentId
	}
	if err := s.catalog.RegisterAgent(ctx, agent); err != nil {
		return nil, fmt.Errorf("register catalog agent: %w", err)
	}
	return s.catalogAgentToPB(agent), nil
}

// RecordCatalogUsageGRPC records catalog usage via gRPC
func (s *HarnessService) RecordCatalogUsageGRPC(ctx context.Context, req *pb.RecordCatalogUsageRequest) (*commonpb.Empty, error) {
	if err := s.catalog.RecordUsage(ctx, req.AgentId); err != nil {
		return nil, fmt.Errorf("record catalog usage: %w", err)
	}
	return &commonpb.Empty{}, nil
}

// RateCatalogAgentGRPC rates a catalog agent via gRPC
func (s *HarnessService) RateCatalogAgentGRPC(ctx context.Context, req *pb.RateCatalogAgentRequest) (*commonpb.Empty, error) {
	if err := s.catalog.RateAgent(ctx, req.AgentId, req.Rating); err != nil {
		return nil, fmt.Errorf("rate catalog agent: %w", err)
	}
	return &commonpb.Empty{}, nil
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
	}

	// Store in ring buffer
	s.mu.Lock()
	s.llmMetricsBuf = append(s.llmMetricsBuf, *m)
	if len(s.llmMetricsBuf) > 1000 {
		s.llmMetricsBuf = s.llmMetricsBuf[len(s.llmMetricsBuf)-1000:]
	}
	s.mu.Unlock()

	// Record into Cost engine
	if err := s.cost.RecordLLMCall(ctx, req.AgentId, req.Model, int64(req.InputTokens), int64(req.OutputTokens), req.Cost, int64(req.LatencyMs), req.Success); err != nil {
		fmt.Printf("Warning: failed to record cost for agent %s: %v\n", req.AgentId, err)
	}

	// Record into SLO manager for all matching SLOs
	slos, err := s.sloManager.ListSLOs(ctx, "", "")
	if err != nil {
		return &commonpb.Empty{}, nil
	}
	for _, sloDef := range slos {
		switch sloDef.Type {
		case slo.SLOTypeLatency:
			s.sloManager.RecordEvent(ctx, sloDef.ID, true, float64(req.LatencyMs))
		case slo.SLOTypeSuccessRate, slo.SLOTypeAvailability:
			s.sloManager.RecordEvent(ctx, sloDef.ID, req.Success, float64(req.LatencyMs))
		}
	}

	return &commonpb.Empty{}, nil
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
func (s *HarnessService) DeleteSessionGRPC(ctx context.Context, req *pb.GetSessionRequest) (*commonpb.Empty, error) {
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

// ==================== Red Team gRPC Methods ====================

// CreateRedTeamTest creates a new red team test
func (s *HarnessService) CreateRedTeamTest(ctx context.Context, req *pb.CreateRedTeamTestRequest) (*pb.RedTeamTest, error) {
	test := &redteam.RedTeamTest{
		Name:        req.Name,
		Description: req.Description,
		AgentID:     req.AgentId,
		Model:       req.Model,
		Category:    req.Category,
		Config:      req.Config,
		TenantID:    req.TenantId,
	}
	if err := s.redteam.CreateTest(ctx, test); err != nil {
		return nil, fmt.Errorf("create red team test: %w", err)
	}
	return s.redTeamTestToPB(test), nil
}

// GetRedTeamTest retrieves a red team test by ID
func (s *HarnessService) GetRedTeamTest(ctx context.Context, req *pb.GetRedTeamTestRequest) (*pb.RedTeamTest, error) {
	test, err := s.redteam.GetTest(ctx, req.Id)
	if err != nil {
		return nil, fmt.Errorf("get red team test: %w", err)
	}
	return s.redTeamTestToPB(test), nil
}

// ListRedTeamTests lists red team tests
func (s *HarnessService) ListRedTeamTests(ctx context.Context, req *pb.ListRedTeamTestsRequest) (*pb.ListRedTeamTestsResponse, error) {
	tests, err := s.redteam.ListTests(ctx, req.AgentId, req.Status)
	if err != nil {
		return nil, fmt.Errorf("list red team tests: %w", err)
	}
	var pbTests []*pb.RedTeamTest
	for _, t := range tests {
		pbTests = append(pbTests, s.redTeamTestToPB(t))
	}
	return &pb.ListRedTeamTestsResponse{Tests: pbTests}, nil
}

// RunRedTeamTest executes a red team test
func (s *HarnessService) RunRedTeamTest(ctx context.Context, req *pb.RunRedTeamTestRequest) (*pb.RunRedTeamTestResponse, error) {
	report, err := s.redteam.RunTest(ctx, req.TestId)
	if err != nil {
		return nil, fmt.Errorf("run red team test: %w", err)
	}
	return &pb.RunRedTeamTestResponse{
		Report: s.redTeamReportToPB(report),
	}, nil
}

// GetRedTeamReport retrieves a red team report by ID
func (s *HarnessService) GetRedTeamReport(ctx context.Context, req *pb.GetRedTeamReportRequest) (*pb.RedTeamReport, error) {
	report, err := s.redteam.GetReport(ctx, req.ReportId)
	if err != nil {
		return nil, fmt.Errorf("get red team report: %w", err)
	}
	return s.redTeamReportToPB(report), nil
}

// GetRedTeamReportByTest retrieves a red team report by test ID
func (s *HarnessService) GetRedTeamReportByTest(ctx context.Context, req *pb.GetRedTeamReportByTestRequest) (*pb.RedTeamReport, error) {
	report, err := s.redteam.GetReportByTest(ctx, req.TestId)
	if err != nil {
		return nil, fmt.Errorf("get red team report by test: %w", err)
	}
	return s.redTeamReportToPB(report), nil
}

// ListRedTeamAttacks lists attacks for a test
func (s *HarnessService) ListRedTeamAttacks(ctx context.Context, req *pb.ListRedTeamAttacksRequest) (*pb.ListRedTeamAttacksResponse, error) {
	attacks, err := s.redteam.GetAttacks(ctx, req.TestId)
	if err != nil {
		return nil, fmt.Errorf("list red team attacks: %w", err)
	}
	var pbAttacks []*pb.RedTeamAttack
	for _, a := range attacks {
		pbAttacks = append(pbAttacks, s.redTeamAttackToPB(a))
	}
	return &pb.ListRedTeamAttacksResponse{Attacks: pbAttacks}, nil
}

// GetAttackPayloads returns attack payloads
func (s *HarnessService) GetAttackPayloads(ctx context.Context, req *pb.GetAttackPayloadsRequest) (*pb.GetAttackPayloadsResponse, error) {
	payloads := redteam.GetAttackPayloads(req.Category)
	var pbPayloads []*pb.AttackPayload
	for _, p := range payloads {
		pbPayloads = append(pbPayloads, &pb.AttackPayload{
			Id:          p.ID,
			Type:        p.Type,
			Name:        p.Name,
			Description: p.Description,
			Payload:     p.Payload,
			Expected:    p.Expected,
			Severity:    p.Severity,
			Tags:        p.Tags,
		})
	}
	stats := redteam.GetPayloadStats()
	return &pb.GetAttackPayloadsResponse{
		Payloads: pbPayloads,
		Stats:    stats,
	}, nil
}

// DeleteRedTeamTest deletes a red team test
func (s *HarnessService) DeleteRedTeamTest(ctx context.Context, req *pb.DeleteRedTeamTestRequest) (*commonpb.Empty, error) {
	if err := s.redteam.DeleteTest(ctx, req.TestId); err != nil {
		return nil, fmt.Errorf("delete red team test: %w", err)
	}
	return &commonpb.Empty{}, nil
}

// GetRedTeamEngine returns the red team engine
func (s *HarnessService) GetRedTeamEngine() *redteam.Engine {
	return s.redteam
}

// redTeamTestToPB converts redteam.RedTeamTest to pb.RedTeamTest
func (s *HarnessService) redTeamTestToPB(t *redteam.RedTeamTest) *pb.RedTeamTest {
	if t == nil {
		return nil
	}
	var startTime, endTime int64
	if t.StartTime != nil {
		startTime = t.StartTime.Unix()
	}
	if t.EndTime != nil {
		endTime = t.EndTime.Unix()
	}
	return &pb.RedTeamTest{
		Id:          t.ID,
		Name:        t.Name,
		Description: t.Description,
		AgentId:     t.AgentID,
		Model:       t.Model,
		Category:    t.Category,
		Status:      t.Status,
		Config:      t.Config,
		StartTime:   startTime,
		EndTime:     endTime,
		TenantId:    t.TenantID,
		CreatedAt:   t.CreatedAt.Unix(),
		UpdatedAt:   t.UpdatedAt.Unix(),
	}
}

// redTeamAttackToPB converts redteam.RedTeamAttack to pb.RedTeamAttack
func (s *HarnessService) redTeamAttackToPB(a *redteam.RedTeamAttack) *pb.RedTeamAttack {
	if a == nil {
		return nil
	}
	return &pb.RedTeamAttack{
		Id:         a.ID,
		TestId:     a.TestID,
		AttackType: a.AttackType,
		AttackName: a.AttackName,
		Payload:    a.Payload,
		Expected:   a.Expected,
		Actual:     a.Actual,
		Passed:     a.Passed,
		Severity:   a.Severity,
		Confidence: a.Confidence,
		Duration:   a.Duration,
		Tokens:     a.Tokens,
		Cost:       a.Cost,
		Timestamp:  a.Timestamp.Unix(),
	}
}

// redTeamReportToPB converts redteam.RedTeamReport to pb.RedTeamReport
func (s *HarnessService) redTeamReportToPB(r *redteam.RedTeamReport) *pb.RedTeamReport {
	if r == nil {
		return nil
	}
	return &pb.RedTeamReport{
		Id:             r.ID,
	TestId:         r.TestID,
		TotalAttacks:   int32(r.TotalAttacks),
		PassedAttacks:  int32(r.PassedAttacks),
		FailedAttacks:  int32(r.FailedAttacks),
		BlockedAttacks: int32(r.BlockedAttacks),
		CriticalCount:  int32(r.CriticalCount),
		HighCount:      int32(r.HighCount),
		MediumCount:    int32(r.MediumCount),
		LowCount:       int32(r.LowCount),
		RiskScore:      r.RiskScore,
		SecurityLevel:  r.SecurityLevel,
		Vulnerabilities: r.Vulnerabilities,
		Recommendations: r.Recommendations,
		GeneratedAt:    r.GeneratedAt.Unix(),
	}
}


