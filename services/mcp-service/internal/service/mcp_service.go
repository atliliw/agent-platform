// Package service provides business logic for MCP service
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"agent-platform/pkg/config"
	"agent-platform/pkg/llm"
	pb "agent-platform/pkg/pb/mcp"
	commonpb "agent-platform/pkg/pb/common"
	"agent-platform/services/mcp-service/internal/model"
	"agent-platform/services/mcp-service/internal/tools"
)

// MCPService provides MCP functionality
type MCPService struct {
	pb.UnimplementedMCPServiceServer
	llmClient          llm.Client
	cfg                *config.Config
	tools              map[string]model.Tool
	toolExec           map[string]tools.Executor
	connections        map[string]*model.Connection
	mu                 sync.RWMutex
	knowledgeServiceAddr string
	browserServiceAddr   string
}

// NewMCPService creates a new MCP service
func NewMCPService(llmClient llm.Client, cfg *config.Config) *MCPService {
	// Get service addresses from config
	knowledgeAddr := cfg.Services.Knowledge
	if knowledgeAddr == "" {
		knowledgeAddr = "knowledge-service:50002"
	}

	gatewayAddr := cfg.Services.Gateway
	if gatewayAddr == "" {
		gatewayAddr = "gateway:9000"
	}

	browserAddr := "browser-service:50008"

	s := &MCPService{
		llmClient:            llmClient,
		cfg:                  cfg,
		tools:                make(map[string]model.Tool),
		toolExec:             make(map[string]tools.Executor),
		connections:          make(map[string]*model.Connection),
		knowledgeServiceAddr: gatewayAddr, // Use gateway for HTTP calls
		browserServiceAddr:   browserAddr,
	}

	// Register built-in tools
	s.registerBuiltInTools()

	return s
}

func (s *MCPService) registerBuiltInTools() {
	// Knowledge Search Tool - REAL implementation
	knowledgeSearchTool := model.Tool{
		Name:        "knowledge_search",
		Description: "Search the knowledge base for information. Use this to find relevant documents and information from internal sources.",
		InputSchema: `{"type":"object","properties":{"query":{"type":"string","description":"The search query"},"top_k":{"type":"integer","description":"Number of results (default: 5)"},"search_type":{"type":"string","description":"Search type: vector, bm25, or hybrid"}},"required":["query"]}`,
	}
	s.tools["knowledge_search"] = knowledgeSearchTool
	// Use the gateway HTTP endpoint for now
	s.toolExec["knowledge_search"] = tools.NewKnowledgeSearchTool(fmt.Sprintf("http://%s", s.cfg.Services.Gateway))

	// Web Search Tool - REAL implementation (uses config)
	webSearchTool := model.Tool{
		Name:        "web_search",
		Description: "Search the web for information. Use this to find current information, news, articles, and general knowledge from the internet.",
		InputSchema: `{"type":"object","properties":{"query":{"type":"string","description":"The search query"},"num_results":{"type":"integer","description":"Number of results"}},"required":["query"]}`,
	}
	s.tools["web_search"] = webSearchTool
	s.toolExec["web_search"] = tools.NewWebSearchToolWithConfig(
		s.cfg.Tools.WebSearch.APIKey,
		s.cfg.Tools.WebSearch.Provider,
	)

	// Calculator tool - works correctly
	calcTool := model.Tool{
		Name:        "calculator",
		Description: "Perform mathematical calculations. Supports basic arithmetic operations (+, -, *, /) and parentheses.",
		InputSchema: `{"type":"object","properties":{"expression":{"type":"string","description":"The mathematical expression to evaluate"}},"required":["expression"]}`,
	}
	s.tools["calculator"] = calcTool
	s.toolExec["calculator"] = tools.NewCalculatorTool()

	// Weather Tool - REAL implementation (uses config)
	weatherTool := model.Tool{
		Name:        "weather",
		Description: "Get current weather information for a location. Supports OpenWeatherMap and QWeather (和风天气).",
		InputSchema: `{"type":"object","properties":{"location":{"type":"string","description":"City name or location"},"units":{"type":"string","description":"Temperature units: celsius or fahrenheit"}},"required":["location"]}`,
	}
	s.tools["weather"] = weatherTool
	s.toolExec["weather"] = tools.NewWeatherToolWithConfig(
		s.cfg.Tools.Weather.APIKey,
		s.cfg.Tools.Weather.Provider,
	)

	// Browser automation tool - 直接调用 browseragent (使用配置)
	browserTool := model.Tool{
		Name:        "browser_execute",
		Description: "执行浏览器自动化任务。接收自然语言描述，自动操控浏览器完成网页操作、数据采集、表单填写等任务。",
		InputSchema: `{"type":"object","properties":{"task":{"type":"string","description":"任务描述"},"max_steps":{"type":"integer","description":"最大步数"}},"required":["task"]}`,
	}
	s.tools["browser_execute"] = browserTool
	// 使用 YAML 配置，如果 browser 配置不存在则 fallback 到 LLM 配置
	browserAPIKey := s.cfg.Tools.Browser.APIKey
	browserBaseURL := s.cfg.Tools.Browser.BaseURL
	browserModel := s.cfg.Tools.Browser.Model
	if browserAPIKey == "" {
		browserAPIKey = s.cfg.LLM.APIKey
	}
	if browserBaseURL == "" {
		browserBaseURL = s.cfg.LLM.BaseURL
	}
	if browserModel == "" {
		browserModel = s.cfg.LLM.Model
	}
	s.toolExec["browser_execute"] = tools.NewBrowserToolWithConfig(
		browserAPIKey,
		browserBaseURL,
		browserModel,
	)

	// Quick Fetch Tool - 快速抓取页面内容
	quickFetchTool := model.Tool{
		Name:        "quick_fetch",
		Description: "快速抓取网页内容。适用于需要登录的网站，通过注入 Cookie 获取页面内容。速度比 browser_execute 快很多。",
		InputSchema: `{"type":"object","properties":{"url":{"type":"string","description":"要抓取的网页 URL"},"selector":{"type":"string","description":"CSS 选择器，用于提取特定元素"},"cookies":{"type":"array","items":{"type":"object","properties":{"name":{"type":"string"},"value":{"type":"string"},"domain":{"type":"string"}}}}},"required":["url"]}`,
	}
	s.tools["quick_fetch"] = quickFetchTool
	s.toolExec["quick_fetch"] = tools.NewQuickFetchTool()

	// Time tool - works correctly
	timeTool := model.Tool{
		Name:        "time",
		Description: "Get current time information.",
		InputSchema: `{"type":"object","properties":{"format":{"type":"string","description":"Time format: default, iso, unix"}}}`,
	}
	s.tools["time"] = timeTool
	s.toolExec["time"] = tools.NewTimeTool()

	// Data analysis tool - works correctly
	dataTool := model.Tool{
		Name:        "data_analysis",
		Description: "Perform statistical analysis on data. Calculates mean, median, standard deviation, and other statistics.",
		InputSchema: `{"type":"object","properties":{"data":{"type":"array","items":{"type":"number"}},"operations":{"type":"array","items":{"type":"string"}}},"required":["data"]}`,
	}
	s.tools["data_analysis"] = dataTool
	s.toolExec["data_analysis"] = tools.NewDataAnalysisTool()

	// Visualization tool - works correctly
	vizTool := model.Tool{
		Name:        "visualization",
		Description: "Generate visualization specifications for data. Returns chart configuration in JSON format.",
		InputSchema: `{"type":"object","properties":{"type":{"type":"string","description":"Chart type: bar, line, pie, scatter"},"title":{"type":"string"},"labels":{"type":"array","items":{"type":"string"}},"data":{"type":"array","items":{"type":"number"}}},"required":["type","data"]}`,
	}
	s.tools["visualization"] = vizTool
	s.toolExec["visualization"] = tools.NewVisualizationTool()

	// Code execution tool - placeholder (needs sandbox)
	codeTool := model.Tool{
		Name:        "code_execute",
		Description: "Execute code in a sandboxed environment. Currently supports Python for calculations and data processing. (Note: Sandbox execution requires configuration)",
		InputSchema: `{"type":"object","properties":{"code":{"type":"string"},"language":{"type":"string","enum":["python"]}},"required":["code"]}`,
	}
	s.tools["code_execute"] = codeTool
	s.toolExec["code_execute"] = tools.NewCodeExecTool()
}

// Connect connects to an MCP server
func (s *MCPService) Connect(ctx context.Context, req *pb.ConnectRequest) (*pb.ConnectResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	connID := fmt.Sprintf("%d", time.Now().UnixNano())
	conn := &model.Connection{
		ID:      connID,
		Name:    req.Name,
		Type:    req.Type,
		Command: req.Command,
		URL:     req.Url,
		Env:     req.Env,
		Status:  "connected",
	}

	s.connections[connID] = conn

	return &pb.ConnectResponse{
		Connection: &pb.Connection{
			Id:      conn.ID,
			Name:    conn.Name,
			Type:    conn.Type,
			Status:  conn.Status,
			Command: conn.Command,
			Url:     conn.URL,
			Env:     conn.Env,
		},
	}, nil
}

// Disconnect disconnects from an MCP server
func (s *MCPService) Disconnect(ctx context.Context, req *pb.DisconnectRequest) (*commonpb.Empty, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.connections, req.ConnectionId)

	return &commonpb.Empty{}, nil
}

// ListConnections lists connections
func (s *MCPService) ListConnections(ctx context.Context, req *pb.ListConnectionsRequest) (*pb.ListConnectionsResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var connections []*pb.Connection
	for _, conn := range s.connections {
		connections = append(connections, &pb.Connection{
			Id:      conn.ID,
			Name:    conn.Name,
			Type:    conn.Type,
			Status:  conn.Status,
			Command: conn.Command,
			Url:     conn.URL,
		})
	}

	return &pb.ListConnectionsResponse{Connections: connections}, nil
}

// ListTools lists available tools
func (s *MCPService) ListTools(ctx context.Context, req *pb.ListToolsRequest) (*pb.ListToolsResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var toolList []*pb.Tool
	for _, tool := range s.tools {
		toolList = append(toolList, &pb.Tool{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: tool.InputSchema,
		})
	}

	return &pb.ListToolsResponse{Tools: toolList}, nil
}

// CallTool calls a tool
func (s *MCPService) CallTool(ctx context.Context, req *pb.CallToolRequest) (*pb.CallToolResponse, error) {
	s.mu.RLock()
	executor, exists := s.toolExec[req.Name]
	s.mu.RUnlock()

	if !exists {
		return &pb.CallToolResponse{
			IsError: true,
			Content: fmt.Sprintf("Tool not found: %s", req.Name),
		}, nil
	}

	// Parse arguments
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(req.Arguments), &args); err != nil {
		return &pb.CallToolResponse{
			IsError: true,
			Content: fmt.Sprintf("Invalid arguments: %v", err),
		}, nil
	}

	// Parse tool config if provided
	var toolConfig map[string]interface{}
	if req.ToolConfig != "" {
		if err := json.Unmarshal([]byte(req.ToolConfig), &toolConfig); err != nil {
			// Log warning but continue
			toolConfig = nil
		}
	}

	// Execute tool with config
	result, err := executor.ExecuteWithConfig(ctx, args, toolConfig)
	if err != nil {
		return &pb.CallToolResponse{
			IsError: true,
			Content: err.Error(),
		}, nil
	}

	return &pb.CallToolResponse{
		IsError: false,
		Content: result,
	}, nil
}

// ListResources lists resources
func (s *MCPService) ListResources(ctx context.Context, req *pb.ListResourcesRequest) (*pb.ListResourcesResponse, error) {
	return &pb.ListResourcesResponse{}, nil
}

// ReadResource reads a resource
func (s *MCPService) ReadResource(ctx context.Context, req *pb.ReadResourceRequest) (*pb.ReadResourceResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

// ListPrompts lists prompts
func (s *MCPService) ListPrompts(ctx context.Context, req *pb.ListPromptsRequest) (*pb.ListPromptsResponse, error) {
	prompts := []*pb.Prompt{
		{
			Name:        "search-expert",
			Description: "Expert search assistant",
			Arguments: []*pb.PromptArgument{
				{Name: "topic", Description: "The topic to search for", Required: true},
			},
		},
	}

	return &pb.ListPromptsResponse{Prompts: prompts}, nil
}

// GetPrompt gets a prompt
func (s *MCPService) GetPrompt(ctx context.Context, req *pb.GetPromptRequest) (*pb.GetPromptResponse, error) {
	topic := ""
	if v, ok := req.Arguments["topic"]; ok {
		topic = v
	}

	return &pb.GetPromptResponse{
		Description: "Search expert prompt",
		Messages: []*pb.PromptMessage{
			{Role: "system", Content: "You are an expert search assistant."},
			{Role: "user", Content: fmt.Sprintf("Search for: %s", topic)},
		},
	}, nil
}