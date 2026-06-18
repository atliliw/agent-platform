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

func (s *HarnessService) proposalToPB(p *evolve.Proposal) *pb.Proposal {
	var approvedAt int64
	if p.ApprovedAt != nil {
		approvedAt = p.ApprovedAt.Unix()
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
