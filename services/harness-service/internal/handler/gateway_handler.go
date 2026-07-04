// Gateway gRPC handlers for grpc_handler.go

package handler

import (
	"context"

	pb "agent-platform/pkg/pb/harness"
	commonpb "agent-platform/pkg/pb/common"
)

// ==================== Gateway Methods ====================

func (h *HarnessHandler) GatewayChat(ctx context.Context, req *pb.GatewayChatRequest) (*pb.GatewayChatResponse, error) {
	return h.service.GatewayChat(ctx, req)
}

func (h *HarnessHandler) GatewayChatStream(req *pb.GatewayChatRequest, stream pb.HarnessService_GatewayChatStreamServer) error {
	return h.service.GatewayChatStream(req, stream)
}

func (h *HarnessHandler) CreateGatewayConfig(ctx context.Context, req *pb.CreateGatewayConfigRequest) (*pb.GatewayConfig, error) {
	return h.service.CreateGatewayConfig(ctx, req)
}

func (h *HarnessHandler) ListGatewayConfigs(ctx context.Context, req *pb.ListGatewayConfigsRequest) (*pb.ListGatewayConfigsResponse, error) {
	return h.service.ListGatewayConfigs(ctx, req)
}

func (h *HarnessHandler) GetGatewayConfig(ctx context.Context, req *pb.GetGatewayConfigRequest) (*pb.GatewayConfig, error) {
	return h.service.GetGatewayConfig(ctx, req)
}

func (h *HarnessHandler) UpdateGatewayConfig(ctx context.Context, req *pb.UpdateGatewayConfigRequest) (*pb.GatewayConfig, error) {
	return h.service.UpdateGatewayConfig(ctx, req)
}

func (h *HarnessHandler) DeleteGatewayConfig(ctx context.Context, req *pb.DeleteGatewayConfigRequest) (*commonpb.Empty, error) {
	return h.service.DeleteGatewayConfig(ctx, req)
}

func (h *HarnessHandler) CreateGatewayRoute(ctx context.Context, req *pb.CreateGatewayRouteRequest) (*pb.GatewayRoute, error) {
	return h.service.CreateGatewayRoute(ctx, req)
}

func (h *HarnessHandler) ListGatewayRoutes(ctx context.Context, req *pb.ListGatewayRoutesRequest) (*pb.ListGatewayRoutesResponse, error) {
	return h.service.ListGatewayRoutes(ctx, req)
}

func (h *HarnessHandler) DeleteGatewayRoute(ctx context.Context, req *pb.DeleteGatewayRouteRequest) (*pb.DeleteGatewayRouteResponse, error) {
	return h.service.DeleteGatewayRoute(ctx, req)
}

func (h *HarnessHandler) GetGatewayStats(ctx context.Context, req *commonpb.Empty) (*pb.GatewayStatsResponse, error) {
	return h.service.GetGatewayStats(ctx, req)
}

func (h *HarnessHandler) SetLoadBalanceStrategy(ctx context.Context, req *pb.SetLoadBalanceStrategyRequest) (*commonpb.Empty, error) {
	return h.service.SetLoadBalanceStrategy(ctx, req)
}