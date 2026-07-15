// Package handler provides gRPC handlers for chat service
package handler

import (
	"context"

	pb "agent-platform/pkg/pb/chat"
	commonpb "agent-platform/pkg/pb/common"
	"agent-platform/services/chat-service/internal/service"
)

// ChatHandler implements ChatServiceServer
type ChatHandler struct {
	pb.UnimplementedChatServiceServer
	chatService *service.ChatService
}

// NewChatHandler creates a new chat handler
func NewChatHandler(chatService *service.ChatService) *ChatHandler {
	return &ChatHandler{
		chatService: chatService,
	}
}

// Chat handles a chat request
func (h *ChatHandler) Chat(ctx context.Context, req *pb.ChatRequest) (*pb.ChatResponse, error) {
	return h.chatService.Chat(ctx, req)
}

// ChatStream handles a streaming chat request
func (h *ChatHandler) ChatStream(req *pb.ChatRequest, stream pb.ChatService_ChatStreamServer) error {
	return h.chatService.ChatStream(req, stream)
}

// MultiAgentChat handles a multi-agent chat request
func (h *ChatHandler) MultiAgentChat(ctx context.Context, req *pb.MultiAgentRequest) (*pb.MultiAgentResponse, error) {
	return h.chatService.MultiAgentChat(ctx, req)
}

// CreateSession creates a new session
func (h *ChatHandler) CreateSession(ctx context.Context, req *pb.CreateSessionRequest) (*pb.Session, error) {
	return h.chatService.CreateSession(ctx, req)
}

// GetSession gets a session
func (h *ChatHandler) GetSession(ctx context.Context, req *pb.GetSessionRequest) (*pb.Session, error) {
	return h.chatService.GetSession(ctx, req)
}

// ListSessions lists sessions
func (h *ChatHandler) ListSessions(ctx context.Context, req *pb.ListSessionsRequest) (*pb.ListSessionsResponse, error) {
	return h.chatService.ListSessions(ctx, req)
}

// DeleteSession deletes a session
func (h *ChatHandler) DeleteSession(ctx context.Context, req *pb.DeleteSessionRequest) (*commonpb.Empty, error) {
	return h.chatService.DeleteSession(ctx, req)
}