// Package service provides business logic for Harness service
package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"agent-platform/pkg/config"
	"agent-platform/pkg/llm"
	pb "agent-platform/pkg/pb/harness"
	commonpb "agent-platform/pkg/pb/common"
	"agent-platform/services/harness-service/internal/abtest"
	"agent-platform/services/harness-service/internal/catalog"
	"agent-platform/services/harness-service/internal/chaos"
	"agent-platform/services/harness-service/internal/coordinate"
	"agent-platform/services/harness-service/internal/cost"
	"agent-platform/services/harness-service/internal/evaluate"
	"agent-platform/services/harness-service/internal/evolve"
	"agent-platform/services/harness-service/internal/featureflag"
	"agent-platform/services/harness-service/internal/goldenpath"
	"agent-platform/services/harness-service/internal/planner"
	"agent-platform/services/harness-service/internal/rca"
	"agent-platform/services/harness-service/internal/repository"
	"agent-platform/services/harness-service/internal/rollback"
	"agent-platform/services/harness-service/internal/rule"
	"agent-platform/services/harness-service/internal/slo"
)

// HarnessService provides harness functionality
type HarnessService struct {
	pb.UnimplementedHarnessServiceServer
	llmClient   llm.Client
	repo        *repository.HarnessRepository
	cfg         *config.Config
	ruleEngine  *rule.Engine
	guardrail   *rule.Guardrail
	permissions *rule.PermissionMatrix
	evalRunner  *evaluate.Runner
	abtest      *abtest.Engine
	sloManager  *slo.Manager
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
	mu            sync.RWMutex
}

// NewHarnessService creates a new harness service
func NewHarnessService(llmClient llm.Client, repo *repository.HarnessRepository, cfg *config.Config) *HarnessService {
	return &HarnessService{
		llmClient:   llmClient,
		repo:        repo,
		cfg:         cfg,
		ruleEngine:  rule.NewEngine(),
		guardrail:   rule.NewGuardrail(),
		permissions: rule.NewPermissionMatrix(),
		evalRunner:  evaluate.NewRunner(llmClient),
		abtest:      abtest.NewEngineMemory(),
		sloManager:  slo.NewManagerMemory(),
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
		Name:         req.Name,
		ControlModel: req.ControlModel,
		VariantModel: req.VariantModel,
		TrafficSplit: req.TrafficSplit,
		AgentID:      req.AgentId,
		TenantID:     req.TenantId,
		Status:       "running",
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
	}, nil
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
		AgentID: req.AgentId,
		Name:    req.Name,
		Target:  req.Target,
		Type:    slo.SLOType(req.Type),
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

	// Record metrics for SLO
	// TODO: Record latency and success metrics

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

// GetCostReport generates a cost report
func (s *HarnessService) GetCostReport(ctx context.Context, agentID string, start, end time.Time) (*cost.CostReport, error) {
	return s.cost.CostReport(ctx, agentID, start, end)
}

// ==================== Chaos Methods ====================

// ShouldInjectChaos checks if chaos should be injected
func (s *HarnessService) ShouldInjectChaos(ctx context.Context, agentID string) (bool, *chaos.Experiment, error) {
	return s.chaos.ShouldInjectFault(ctx, agentID)
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
