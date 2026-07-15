// Package service provides business logic for A2A service
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	pb "agent-platform/pkg/pb/a2a"
	commonpb "agent-platform/pkg/pb/common"
	"agent-platform/services/a2a-service/internal/model"
	"agent-platform/services/a2a-service/internal/repository"
)

// A2AService provides A2A functionality
type A2AService struct {
	pb.UnimplementedA2AServiceServer
	repo        *repository.A2ARepository
	httpClient  *http.Client
	localCard   *model.AgentCard
	agentClient AgentClient // Client to call local Agent Service
}

// AgentClient is the interface for calling Agent Service
type AgentClient interface {
	Execute(ctx context.Context, sessionID, message string) (string, error)
}

// NewA2AService creates a new A2A service
func NewA2AService(repo *repository.A2ARepository, agentClient AgentClient) *A2AService {
	// Create local agent card
	localCard := &model.AgentCard{
		ID:           "local-agent-platform",
		Name:         "Local Agent Platform",
		Description:  "A multi-agent platform with RAG and tool calling capabilities",
		Capabilities: []string{"chat", "search", "multi_agent", "tool_calling"},
		InputModes:   []string{"text", "json"},
		OutputModes:  []string{"text", "json"},
		URL:          "http://localhost:8080",
	}

	return &A2AService{
		repo:        repo,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		localCard:   localCard,
		agentClient: agentClient,
	}
}

// Discover discovers a remote agent
func (s *A2AService) Discover(ctx context.Context, req *pb.DiscoverRequest) (*pb.DiscoverResponse, error) {
	url := strings.TrimSuffix(req.AgentUrl, "/") + "/.well-known/agent.json"

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("discover agent: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("discover failed: status %d", resp.StatusCode)
	}

	var card model.AgentCard
	if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
		return nil, fmt.Errorf("parse agent card: %w", err)
	}

	// Register the discovered agent
	if err := s.repo.RegisterAgent(ctx, &card, req.TenantId); err != nil {
		return nil, fmt.Errorf("register agent: %w", err)
	}

	return &pb.DiscoverResponse{
		Card: &pb.AgentCard{
			Id:           card.ID,
			Name:         card.Name,
			Description:  card.Description,
			Capabilities: card.Capabilities,
			InputModes:   card.InputModes,
			OutputModes:  card.OutputModes,
			Url:          card.URL,
		},
	}, nil
}

// RegisterAgent registers an agent
func (s *A2AService) RegisterAgent(ctx context.Context, req *pb.RegisterAgentRequest) (*commonpb.Empty, error) {
	card := &model.AgentCard{
		ID:           req.Card.Id,
		Name:         req.Card.Name,
		Description:  req.Card.Description,
		Capabilities: req.Card.Capabilities,
		InputModes:   req.Card.InputModes,
		OutputModes:  req.Card.OutputModes,
		URL:          req.Card.Url,
	}

	if err := s.repo.RegisterAgent(ctx, card, req.TenantId); err != nil {
		return nil, err
	}

	return &commonpb.Empty{}, nil
}

// UnregisterAgent unregisters an agent
func (s *A2AService) UnregisterAgent(ctx context.Context, req *pb.UnregisterAgentRequest) (*commonpb.Empty, error) {
	if err := s.repo.UnregisterAgent(ctx, req.AgentId, req.TenantId); err != nil {
		return nil, err
	}
	return &commonpb.Empty{}, nil
}

// ListAgents lists agents
func (s *A2AService) ListAgents(ctx context.Context, req *pb.ListAgentsRequest) (*pb.ListAgentsResponse, error) {
	agents, err := s.repo.ListAgents(ctx, req.TenantId)
	if err != nil {
		return nil, err
	}

	var pbAgents []*pb.AgentCard
	for _, a := range agents {
		pbAgents = append(pbAgents, &pb.AgentCard{
			Id:           a.ID,
			Name:         a.Name,
			Description:  a.Description,
			Capabilities: a.Capabilities,
			InputModes:   a.InputModes,
			OutputModes:  a.OutputModes,
			Url:          a.URL,
		})
	}

	return &pb.ListAgentsResponse{Agents: pbAgents}, nil
}

// SendTask sends a task to an agent
func (s *A2AService) SendTask(ctx context.Context, req *pb.SendTaskRequest) (*pb.SendTaskResponse, error) {
	// Get agent
	agent, err := s.repo.GetAgent(ctx, req.AgentId, req.TenantId)
	if err != nil {
		return nil, fmt.Errorf("agent not found: %w", err)
	}

	// Create task
	task := &model.Task{
		AgentID:   req.AgentId,
		Status:    model.TaskStatusSubmitted,
		Messages:  []model.Message{*modelFromPBMessage(req.Message)},
		Metadata:  req.Metadata,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.repo.CreateTask(ctx, task, req.TenantId); err != nil {
		return nil, err
	}

	// Send to remote agent
	taskURL := strings.TrimSuffix(agent.URL, "/") + "/api/v2/a2a/tasks/send"
	taskReq := map[string]interface{}{
		"id":      task.ID,
		"message": req.Message,
	}

	body, err := json.Marshal(taskReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", taskURL, strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		task.Status = model.TaskStatusFailed
		s.repo.UpdateTask(ctx, task, req.TenantId)
		return nil, fmt.Errorf("send task: %w", err)
	}
	defer resp.Body.Close()

	// Update task status
	task.Status = model.TaskStatusWorking
	s.repo.UpdateTask(ctx, task, req.TenantId)

	return &pb.SendTaskResponse{
		Task: pbFromModelTask(task),
	}, nil
}

// GetTask gets a task
func (s *A2AService) GetTask(ctx context.Context, req *pb.GetTaskRequest) (*pb.GetTaskResponse, error) {
	task, err := s.repo.GetTask(ctx, req.TaskId, req.TenantId)
	if err != nil {
		return nil, err
	}

	return &pb.GetTaskResponse{
		Task: pbFromModelTask(task),
	}, nil
}

// CancelTask cancels a task
func (s *A2AService) CancelTask(ctx context.Context, req *pb.CancelTaskRequest) (*commonpb.Empty, error) {
	task, err := s.repo.GetTask(ctx, req.TaskId, req.TenantId)
	if err != nil {
		return nil, err
	}

	task.Status = model.TaskStatusCancelled
	if err := s.repo.UpdateTask(ctx, task, req.TenantId); err != nil {
		return nil, err
	}

	return &commonpb.Empty{}, nil
}

// ListTasks lists tasks
func (s *A2AService) ListTasks(ctx context.Context, req *pb.ListTasksRequest) (*pb.ListTasksResponse, error) {
	page := int(req.Pagination.GetPage())
	pageSize := int(req.Pagination.GetPageSize())
	if page == 0 {
		page = 1
	}
	if pageSize == 0 {
		pageSize = 20
	}

	tasks, total, err := s.repo.ListTasks(ctx, req.AgentId, req.Status.String(), req.TenantId, page, pageSize)
	if err != nil {
		return nil, err
	}

	var pbTasks []*pb.A2ATask
	for _, t := range tasks {
		pbTasks = append(pbTasks, pbFromModelTask(t))
	}

	return &pb.ListTasksResponse{
		Tasks: pbTasks,
		Pagination: &commonpb.PaginationResponse{
			Total:    int32(total),
			Page:     int32(page),
			PageSize: int32(pageSize),
		},
	}, nil
}

// GetLocalAgentCard returns the local agent card as JSON
func (s *A2AService) GetLocalAgentCard() string {
	card := map[string]interface{}{
		"id":           s.localCard.ID,
		"name":         s.localCard.Name,
		"description":  s.localCard.Description,
		"capabilities": s.localCard.Capabilities,
		"input_modes":  s.localCard.InputModes,
		"output_modes": s.localCard.OutputModes,
		"url":          s.localCard.URL,
	}
	data, _ := json.Marshal(card)
	return string(data)
}

// HandleSendTask handles HTTP task send - executes via local Agent Service
func (s *A2AService) HandleSendTask(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID        string                 `json:"id"`
		SessionID string                 `json:"session_id"`
		Message   map[string]interface{} `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	// Create task
	task := &model.Task{
		ID:        req.ID,
		Status:    model.TaskStatusWorking,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	content, _ := req.Message["content"].(string)
	task.Messages = append(task.Messages, model.Message{
		Role:    "user",
		Content: content,
	})

	// Execute via local Agent Service if available
	var result string
	var execErr error

	if s.agentClient != nil {
		// Call Agent Service to execute the task
		result, execErr = s.agentClient.Execute(r.Context(), req.SessionID, content)
	} else {
		// Fallback: use internal agent processing
		result, execErr = s.executeLocalAgent(r.Context(), content)
	}

	if execErr != nil {
		task.Status = model.TaskStatusFailed
		task.Result = fmt.Sprintf("Error: %v", execErr)
	} else {
		task.Status = model.TaskStatusCompleted
		task.Result = result
	}

	task.UpdatedAt = time.Now()

	// Save task to repository
	s.repo.CreateTask(r.Context(), task, "")

	// Add result message
	task.Messages = append(task.Messages, model.Message{
		Role:    "assistant",
		Content: task.Result,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":         task.ID,
		"status":     task.Status,
		"result":     task.Result,
		"messages":   task.Messages,
		"created_at": task.CreatedAt,
		"updated_at": task.UpdatedAt,
	})
}

// executeLocalAgent executes a task using local agent logic
func (s *A2AService) executeLocalAgent(ctx context.Context, content string) (string, error) {
	// Simple agent execution - in production, this should call Agent Service
	// For now, return a helpful response

	// Check if it's a search query
	if strings.Contains(strings.ToLower(content), "search") ||
		strings.Contains(strings.ToLower(content), "find") ||
		strings.Contains(strings.ToLower(content), "查找") ||
		strings.Contains(strings.ToLower(content), "搜索") {
		return "I can help you search. Please use the knowledge_search tool for internal documents or web_search for internet content.", nil
	}

	// Check if it's a question
	if strings.Contains(content, "?") || strings.Contains(content, "？") {
		return fmt.Sprintf("I received your question: %s\n\nTo get the best answer, please ensure the relevant knowledge documents are uploaded to the knowledge base.", content), nil
	}

	// Default response
	return fmt.Sprintf("Task received: %s\n\nI am the local agent platform. I can help with:\n- Knowledge search (RAG)\n- Web search\n- Multi-agent orchestration\n- Tool calling\n\nPlease specify what you'd like me to do.", content), nil
}

// HandleGetTask handles HTTP task get
func (s *A2AService) HandleGetTask(w http.ResponseWriter, r *http.Request) {
	// Extract task ID from path
	path := strings.TrimPrefix(r.URL.Path, "/api/v2/a2a/tasks/")
	taskID := strings.TrimSuffix(path, "/")

	// Get task from repo
	task, err := s.repo.GetTask(r.Context(), taskID, "")
	if err != nil {
		http.Error(w, "Task not found", 404)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":         task.ID,
		"status":     task.Status,
		"messages":   task.Messages,
		"result":     task.Result,
		"created_at": task.CreatedAt,
		"updated_at": task.UpdatedAt,
	})
}

func modelFromPBMessage(msg *pb.A2AMessage) *model.Message {
	return &model.Message{
		Role:     msg.Role,
		Content:  msg.Content,
		Metadata: msg.Metadata,
	}
}

func pbFromModelTask(t *model.Task) *pb.A2ATask {
	var messages []*pb.A2AMessage
	for _, m := range t.Messages {
		messages = append(messages, &pb.A2AMessage{
			Role:     m.Role,
			Content:  m.Content,
			Metadata: m.Metadata,
		})
	}

	return &pb.A2ATask{
		Id:         t.ID,
		AgentId:    t.AgentID,
		Status:     pb.TaskStatus(pb.TaskStatus_value[string(t.Status)]),
		Messages:   messages,
		Result:     t.Result,
		Metadata:   t.Metadata,
		CreatedAt:  t.CreatedAt.Unix(),
		UpdatedAt:  t.UpdatedAt.Unix(),
	}
}

// Helper for reading response body
func readBody(r *http.Response) string {
	defer r.Body.Close()
	body, _ := io.ReadAll(r.Body)
	return string(body)
}