// Gateway gRPC methods for harness_service.go
// Append these methods to the HarnessService

package service

import (
	"context"
	"fmt"

	pb "agent-platform/pkg/pb/harness"
	commonpb "agent-platform/pkg/pb/common"
	"agent-platform/services/harness-service/internal/gateway"
)

// ==================== Gateway Methods ====================

// GatewayChat handles gateway chat requests
func (s *HarnessService) GatewayChat(ctx context.Context, req *pb.GatewayChatRequest) (*pb.GatewayChatResponse, error) {
	// Convert messages
	var messages []gateway.Message
	for _, m := range req.Messages {
		messages = append(messages, gateway.Message{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	// Build gateway request
	gwReq := &gateway.ChatRequest{
		Provider:    req.Provider,
		Model:       req.Model,
		Messages:    messages,
		Temperature: req.Temperature,
		MaxTokens:   int(req.MaxTokens),
		Parameters:  req.Parameters,
		TenantID:    req.TenantId,
	}

	// Execute through gateway
	resp, err := s.gateway.Chat(ctx, gwReq)
	if err != nil {
		return nil, fmt.Errorf("gateway chat: %w", err)
	}

	return &pb.GatewayChatResponse{
		Content:       resp.Content,
		Model:         resp.Model,
		Provider:      resp.Provider,
		TotalTokens:   resp.TotalTokens,
		Cost:          resp.Cost,
		Latency:       resp.Latency,
		UsedFallback:  resp.UsedFallback,
		OriginalModel: resp.OriginalModel,
		Error:         resp.Error,
	}, nil
}

// GatewayChatStream handles streaming gateway chat requests
func (s *HarnessService) GatewayChatStream(req *pb.GatewayChatRequest, stream pb.HarnessService_GatewayChatStreamServer) error {
	// Convert messages
	var messages []gateway.Message
	for _, m := range req.Messages {
		messages = append(messages, gateway.Message{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	// Build gateway request
	gwReq := &gateway.ChatRequest{
		Provider:    req.Provider,
		Model:       req.Model,
		Messages:    messages,
		Temperature: req.Temperature,
		MaxTokens:   int(req.MaxTokens),
		Parameters:  req.Parameters,
		TenantID:    req.TenantId,
	}

	// Execute streaming through gateway
	ch, err := s.gateway.ChatStream(stream.Context(), gwReq)
	if err != nil {
		return fmt.Errorf("gateway chat stream: %w", err)
	}

	// Stream responses
	for chunk := range ch {
		if chunk.Error != nil {
			return chunk.Error
		}

		if err := stream.Send(&pb.GatewayChatResponse{
			Content:       chunk.Content,
			Model:         chunk.Model,
			Provider:      chunk.Provider,
			UsedFallback:  chunk.UsedFallback,
			OriginalModel: chunk.OriginalModel,
		}); err != nil {
			return err
		}

		if chunk.Done {
			break
		}
	}

	return nil
}

// CreateGatewayConfig creates a gateway configuration
func (s *HarnessService) CreateGatewayConfig(ctx context.Context, req *pb.CreateGatewayConfigRequest) (*pb.GatewayConfig, error) {
	cfg := &gateway.GatewayConfig{
		Name:        req.Name,
		Description: req.Description,
		Provider:    req.Provider,
		APIKey:      req.ApiKey,
		BaseURL:     req.BaseUrl,
		Models:      req.Models,
		RateLimit:   int(req.RateLimit),
		Timeout:     int(req.Timeout),
		RetryCount:  int(req.RetryCount),
		Priority:    int(req.Priority),
		Enabled:     req.Enabled,
		TenantID:    req.TenantId,
	}

	if err := s.gateway.AddConfig(ctx, cfg); err != nil {
		return nil, fmt.Errorf("create gateway config: %w", err)
	}

	return s.gatewayConfigToPB(cfg), nil
}

// ListGatewayConfigs lists gateway configurations
func (s *HarnessService) ListGatewayConfigs(ctx context.Context, req *pb.ListGatewayConfigsRequest) (*pb.ListGatewayConfigsResponse, error) {
	configs, err := s.gateway.ListConfigs(ctx, req.TenantId)
	if err != nil {
		return nil, fmt.Errorf("list gateway configs: %w", err)
	}

	var pbConfigs []*pb.GatewayConfig
	for _, cfg := range configs {
		pbConfigs = append(pbConfigs, s.gatewayConfigToPB(cfg))
	}

	return &pb.ListGatewayConfigsResponse{Configs: pbConfigs}, nil
}

// GetGatewayConfig gets a gateway configuration
func (s *HarnessService) GetGatewayConfig(ctx context.Context, req *pb.GetGatewayConfigRequest) (*pb.GatewayConfig, error) {
	cfg, err := s.gateway.GetConfig(ctx, req.Id)
	if err != nil {
		return nil, fmt.Errorf("get gateway config: %w", err)
	}

	return s.gatewayConfigToPB(cfg), nil
}

// UpdateGatewayConfig updates a gateway configuration
func (s *HarnessService) UpdateGatewayConfig(ctx context.Context, req *pb.UpdateGatewayConfigRequest) (*pb.GatewayConfig, error) {
	cfg := &gateway.GatewayConfig{
		ID:          req.Id,
		Name:        req.Name,
		Description: req.Description,
		APIKey:      req.ApiKey,
		BaseURL:     req.BaseUrl,
		Models:      req.Models,
		RateLimit:   int(req.RateLimit),
		Timeout:     int(req.Timeout),
		RetryCount:  int(req.RetryCount),
		Priority:    int(req.Priority),
		Enabled:     req.Enabled,
	}

	if err := s.gateway.UpdateConfig(ctx, cfg); err != nil {
		return nil, fmt.Errorf("update gateway config: %w", err)
	}

	updatedCfg, err := s.gateway.GetConfig(ctx, req.Id)
	if err != nil {
		return nil, fmt.Errorf("get updated config: %w", err)
	}

	return s.gatewayConfigToPB(updatedCfg), nil
}

// DeleteGatewayConfig deletes a gateway configuration
func (s *HarnessService) DeleteGatewayConfig(ctx context.Context, req *pb.DeleteGatewayConfigRequest) (*commonpb.Empty, error) {
	if err := s.gateway.DeleteConfig(ctx, req.Id); err != nil {
		return nil, fmt.Errorf("delete gateway config: %w", err)
	}

	return &commonpb.Empty{}, nil
}

// CreateGatewayRoute creates a gateway route
func (s *HarnessService) CreateGatewayRoute(ctx context.Context, req *pb.CreateGatewayRouteRequest) (*pb.GatewayRoute, error) {
	route := &gateway.GatewayRoute{
		Name:      req.Name,
		Pattern:   req.Pattern,
		ModelID:   req.ModelId,
		Fallbacks: req.Fallbacks,
		Enabled:   req.Enabled,
		TenantID:  req.TenantId,
	}

	if err := s.gateway.AddRoute(ctx, route); err != nil {
		return nil, fmt.Errorf("create gateway route: %w", err)
	}

	return s.gatewayRouteToPB(route), nil
}

// ListGatewayRoutes lists gateway routes
func (s *HarnessService) ListGatewayRoutes(ctx context.Context, req *pb.ListGatewayRoutesRequest) (*pb.ListGatewayRoutesResponse, error) {
	routes, err := s.gateway.ListRoutes(ctx, req.TenantId)
	if err != nil {
		return nil, fmt.Errorf("list gateway routes: %w", err)
	}

	var pbRoutes []*pb.GatewayRoute
	for _, route := range routes {
		pbRoutes = append(pbRoutes, s.gatewayRouteToPB(route))
	}

	return &pb.ListGatewayRoutesResponse{Routes: pbRoutes}, nil
}

// DeleteGatewayRoute deletes a gateway route
func (s *HarnessService) DeleteGatewayRoute(ctx context.Context, req *pb.DeleteGatewayRouteRequest) (*pb.DeleteGatewayRouteResponse, error) {
	if err := s.gateway.DeleteRoute(ctx, req.Id); err != nil {
		return nil, fmt.Errorf("delete gateway route: %w", err)
	}

	return &pb.DeleteGatewayRouteResponse{Success: true}, nil
}

// GetGatewayStats gets gateway statistics
func (s *HarnessService) GetGatewayStats(ctx context.Context, req *commonpb.Empty) (*pb.GatewayStatsResponse, error) {
	stats, err := s.gateway.GetStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("get gateway stats: %w", err)
	}

	var pbStats []*pb.GatewayStats
	for provider, s := range stats {
		pbStats = append(pbStats, &pb.GatewayStats{
			Provider:       provider,
			TotalRequests:  s.TotalRequests,
			SuccessCount:   s.SuccessCount,
			ErrorCount:     s.ErrorCount,
			AvgLatency:     s.AvgLatency,
			TotalTokens:    s.TotalTokens,
			TotalCost:      s.TotalCost,
			LastActiveTime: s.LastActiveTime.Unix(),
		})
	}

	return &pb.GatewayStatsResponse{Stats: pbStats}, nil
}

// SetLoadBalanceStrategy sets the load balancing strategy
func (s *HarnessService) SetLoadBalanceStrategy(ctx context.Context, req *pb.SetLoadBalanceStrategyRequest) (*commonpb.Empty, error) {
	strategy := gateway.LoadBalanceStrategy(req.Strategy)
	s.gateway.SetLoadBalanceStrategy(strategy)
	return &commonpb.Empty{}, nil
}

// Helper conversion functions

func (s *HarnessService) gatewayConfigToPB(cfg *gateway.GatewayConfig) *pb.GatewayConfig {
	return &pb.GatewayConfig{
		Id:          cfg.ID,
		Name:        cfg.Name,
		Description: cfg.Description,
		Provider:    cfg.Provider,
		ApiKey:      cfg.APIKey,
		BaseUrl:     cfg.BaseURL,
		Models:      cfg.Models,
		RateLimit:   int32(cfg.RateLimit),
		Timeout:     int32(cfg.Timeout),
		RetryCount:  int32(cfg.RetryCount),
		Priority:    int32(cfg.Priority),
		Enabled:     cfg.Enabled,
		TenantId:    cfg.TenantID,
		CreatedAt:   cfg.CreatedAt.Unix(),
		UpdatedAt:   cfg.UpdatedAt.Unix(),
	}
}

func (s *HarnessService) gatewayRouteToPB(route *gateway.GatewayRoute) *pb.GatewayRoute {
	return &pb.GatewayRoute{
		Id:        route.ID,
		Name:      route.Name,
		Pattern:   route.Pattern,
		ModelId:   route.ModelID,
		Fallbacks: route.Fallbacks,
		TenantId:  route.TenantID,
		Enabled:   route.Enabled,
		CreatedAt: route.CreatedAt.Unix(),
		UpdatedAt: route.UpdatedAt.Unix(),
	}
}