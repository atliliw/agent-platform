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
	"agent-platform/services/harness-service/internal/evaluate"
	"agent-platform/services/harness-service/internal/repository"
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
	mu          sync.RWMutex
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
		abtest:      abtest.NewEngine(),
		sloManager:  slo.NewManager(),
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
	return s.abtest.GetResult(req.TestId)
}

// PromoteVariant promotes variant
func (s *HarnessService) PromoteVariant(ctx context.Context, req *pb.PromoteVariantRequest) (*commonpb.Empty, error) {
	return &commonpb.Empty{}, nil
}

// CreateSLO creates an SLO
func (s *HarnessService) CreateSLO(ctx context.Context, req *pb.CreateSLORequest) (*pb.SLO, error) {
	sloRecord := &repository.SLO{
		AgentID:  req.AgentId,
		Name:     req.Name,
		Target:   req.Target,
		Type:     req.Type,
		TenantID: req.TenantId,
	}

	if err := s.repo.CreateSLO(ctx, sloRecord); err != nil {
		return nil, err
	}

	// Register in SLO manager
	s.sloManager.RegisterSLO(&slo.SLODefinition{
		ID:      sloRecord.ID,
		AgentID: sloRecord.AgentID,
		Name:    sloRecord.Name,
		Type:    sloRecord.Type,
		Target:  sloRecord.Target,
	})

	return &pb.SLO{
		Id:        sloRecord.ID,
		AgentId:   sloRecord.AgentID,
		Name:      sloRecord.Name,
		Target:    sloRecord.Target,
		Type:      sloRecord.Type,
		CreatedAt: sloRecord.CreatedAt.Unix(),
	}, nil
}

// GetSLOStatus gets SLO status
func (s *HarnessService) GetSLOStatus(ctx context.Context, req *pb.GetSLOStatusRequest) (*pb.GetSLOStatusResponse, error) {
	return s.sloManager.GetStatus(req.AgentId)
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
	s.abtest.RecordScore(testID, isVariant, score)
	s.abtest.RecordLatency(testID, isVariant, latencyMs)
}

// AssignABTestVariant assigns a request to control or variant
func (s *HarnessService) AssignABTestVariant(testID string, splitRatio float64) bool {
	return s.abtest.AssignVariant(testID, splitRatio)
}

// RecordSLOMetric records a metric for SLO tracking
func (s *HarnessService) RecordSLOMetric(sloID string, latencyMs float64, success bool) {
	s.sloManager.RecordLatency(sloID, latencyMs)
	s.sloManager.RecordSuccess(sloID, success)
}
