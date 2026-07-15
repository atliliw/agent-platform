// Package service provides business logic for MCP service
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"agent-platform/pkg/config"
	"agent-platform/pkg/llm"
	mcp "agent-platform/pkg/mcp"
	commonpb "agent-platform/pkg/pb/common"
	pb "agent-platform/pkg/pb/mcp"
	"agent-platform/services/mcp-service/internal/governance"
	"agent-platform/services/mcp-service/internal/model"
	"agent-platform/services/mcp-service/internal/tools"
)

// remoteToolEntry tracks a remote tool and its owning connection.
type remoteToolEntry struct {
	Tool         model.Tool
	ConnectionID string
}

// MCPService provides MCP functionality with governance
type MCPService struct {
	pb.UnimplementedMCPServiceServer
	llmClient            llm.Client
	cfg                  *config.Config
	tools                map[string]model.Tool          // built-in tools
	toolExec             map[string]tools.Executor      // built-in tool executors
	connections          map[string]*model.Connection    // connection metadata
	mcpClients           map[string]*mcp.Client          // connectionID → MCP client
	remoteTools          map[string]remoteToolEntry      // prefixed name → entry (connID__toolName)
	governance           *governance.GovernancePipeline  // ★ 治理流水线
	callCounts           map[string]int                  // ★ 工具调用计数
	mu                   sync.RWMutex
	knowledgeServiceAddr string
	browserServiceAddr   string
}

// NewMCPService creates a new MCP service
func NewMCPService(llmClient llm.Client, cfg *config.Config) *MCPService {
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
		mcpClients:           make(map[string]*mcp.Client),
		remoteTools:          make(map[string]remoteToolEntry),
		governance:           governance.NewGovernancePipeline(),
		callCounts:           make(map[string]int),
		knowledgeServiceAddr: gatewayAddr,
		browserServiceAddr:   browserAddr,
	}

	s.setupDefaultSLOs()
	s.registerBuiltInTools()

	return s
}

// setupDefaultSLOs 设置默认 SLO
func (s *MCPService) setupDefaultSLOs() {
	s.governance.SLOManager.RegisterSLO(&governance.SLODefinition{
		ID:     "tool_success_rate",
		Name:   "工具调用成功率",
		Type:   "success_rate",
		Target: 0.95,
		Window: 24 * time.Hour,
	})

	s.governance.SLOManager.RegisterSLO(&governance.SLODefinition{
		ID:     "tool_latency",
		Name:   "工具调用延迟",
		Type:   "latency",
		Target: 5000,
		Window: 24 * time.Hour,
	})

	s.governance.SLOManager.RegisterSLO(&governance.SLODefinition{
		ID:     "browser_tool_success",
		Name:   "浏览器工具成功率",
		Type:   "success_rate",
		Target: 0.85,
		Window: 24 * time.Hour,
	})
}

func (s *MCPService) registerBuiltInTools() {
	// Knowledge Search Tool
	knowledgeSearchTool := model.Tool{
		Name:        "knowledge_search",
		Description: "Search the knowledge base for information. Use this to find relevant documents and information from internal sources.",
		InputSchema: `{"type":"object","properties":{"query":{"type":"string","description":"The search query"},"top_k":{"type":"integer","description":"Number of results (default: 5)"},"search_type":{"type":"string","description":"Search type: vector, bm25, or hybrid"}},"required":["query"]}`,
	}
	s.tools["knowledge_search"] = knowledgeSearchTool
	s.toolExec["knowledge_search"] = tools.NewKnowledgeSearchTool(fmt.Sprintf("http://%s", s.cfg.Services.Gateway))

	// Web Search Tool
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

	// Calculator tool
	calcTool := model.Tool{
		Name:        "calculator",
		Description: "Perform mathematical calculations. Supports basic arithmetic operations (+, -, *, /) and parentheses.",
		InputSchema: `{"type":"object","properties":{"expression":{"type":"string","description":"The mathematical expression to evaluate"}},"required":["expression"]}`,
	}
	s.tools["calculator"] = calcTool
	s.toolExec["calculator"] = tools.NewCalculatorTool()

	// Weather Tool
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

	// Browser automation tool
	browserTool := model.Tool{
		Name:        "browser_execute",
		Description: "执行浏览器自动化任务。接收自然语言描述，自动操控浏览器完成网页操作、数据采集、表单填写等任务。",
		InputSchema: `{"type":"object","properties":{"task":{"type":"string","description":"任务描述"},"max_steps":{"type":"integer","description":"最大步数"}},"required":["task"]}`,
	}
	s.tools["browser_execute"] = browserTool
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

	// Fine-grained browser primitives
	fineGrainedBrowserTools := []struct {
		name, desc, schema string
		exec               tools.Executor
	}{
		{
			name:   "browser_navigate",
			desc:   "导航到指定 URL，打开网页。同一会话内与 browser_click/type/extract 共享同一个浏览器页面（保留 Cookie/登录态）。首次导航会自动注入该域名已存储的 Cookie。",
			schema: `{"type":"object","properties":{"url":{"type":"string","description":"要导航到的完整 URL，如 https://www.baidu.com"}},"required":["url"]}`,
			exec:   tools.NewBrowserNavigateTool(),
		},
		{
			name:   "browser_click",
			desc:   "点击页面上的可交互元素。element 是 browser_extract 返回的元素列表中的索引。同一会话内共享浏览器页面。",
			schema: `{"type":"object","properties":{"element":{"type":"integer","description":"要点击的元素索引（来自 browser_extract 的元素列表）"}},"required":["element"]}`,
			exec:   tools.NewBrowserClickTool(),
		},
		{
			name:   "browser_type",
			desc:   "在指定元素中输入文本。element 是 browser_extract 返回的元素索引。同一会话内共享浏览器页面。",
			schema: `{"type":"object","properties":{"element":{"type":"integer","description":"目标输入元素索引（来自 browser_extract）"},"text":{"type":"string","description":"要输入的文本"}},"required":["element","text"]}`,
			exec:   tools.NewBrowserTypeTool(),
		},
		{
			name:   "browser_extract",
			desc:   "提取当前页面状态：URL、标题、带索引的可交互元素列表（供 browser_click/type 选择目标）、页面文本摘要。同一会话内共享浏览器页面。",
			schema: `{"type":"object","properties":{}}`,
			exec:   tools.NewBrowserExtractTool(),
		},
		{
			name:   "browser_scroll",
			desc:   "滚动页面一屏。direction 为 up 或 down，默认 down。同一会话内共享浏览器页面。",
			schema: `{"type":"object","properties":{"direction":{"type":"string","enum":["up","down"],"description":"滚动方向，默认 down"}}}`,
			exec:   tools.NewBrowserScrollTool(),
		},
		{
			name:   "browser_wait",
			desc:   "等待指定秒数，让页面加载或动态内容渲染。默认 1 秒。同一会话内共享浏览器页面。",
			schema: `{"type":"object","properties":{"seconds":{"type":"integer","description":"等待秒数，默认 1"}}}`,
			exec:   tools.NewBrowserWaitTool(),
		},
	}
	for _, t := range fineGrainedBrowserTools {
		s.tools[t.name] = model.Tool{Name: t.name, Description: t.desc, InputSchema: t.schema}
		s.toolExec[t.name] = t.exec
	}

	// XHS (Xiaohongshu) dedicated tools. All XHS logic lives in pkg/xhs; the
	// generic browser_extract no longer carries any XHS special-casing.
	xhsReadNoteTool := model.Tool{
		Name:        "xhs_read_note",
		Description: "读取小红书笔记的完整内容（标题/作者/正文/点赞/评论/标签）。传入笔记链接（含 xsec_token）或笔记ID。直接读服务端渲染的结构化数据，比通用 browser_extract 更可靠。同一会话内与 browser_* 工具共享浏览器页面（保留登录态）。",
		InputSchema: `{"type":"object","properties":{"url":{"type":"string","description":"小红书笔记链接或笔记ID，如 https://www.xiaohongshu.com/explore/<id>?xsec_token=..."}},"required":["url"]}`,
	}
	s.tools["xhs_read_note"] = xhsReadNoteTool
	s.toolExec["xhs_read_note"] = tools.NewXHSReadNoteTool()

	xhsSearchTool := model.Tool{
		Name:        "xhs_search",
		Description: "在小红书搜索笔记，返回匹配列表（标题/作者/点赞/链接，含 xsec_token，可用 xhs_read_note 读详情）。注意：小红书搜索接口有反爬，工具会尝试页面内签名 fetch 等多种方式；若被挡会返回诊断信息（HTTP状态/错误码）说明原因。同一会话内与 browser_* 工具共享浏览器页面。",
		InputSchema: `{"type":"object","properties":{"keyword":{"type":"string","description":"搜索关键词"},"page":{"type":"integer","description":"页码，默认1"},"sort":{"type":"string","description":"排序：general(综合,默认) / time_descending(最新) / popularity_descending(最热)"}},"required":["keyword"]}`,
	}
	s.tools["xhs_search"] = xhsSearchTool
	s.toolExec["xhs_search"] = tools.NewXHSSearchTool()

	// CSDN Publish Tool
	csdnPublishTool := model.Tool{
		Name:        "csdn_publish",
		Description: "【首选】在 CSDN 发布文章。直接调用 CSDN API 发布，速度快、成功率高、不会被检测。当用户要求在CSDN发表/发布/写文章时，优先使用此工具，不要使用 browser_execute。支持 Markdown 格式内容。",
		InputSchema: `{"type":"object","properties":{"title":{"type":"string","description":"文章标题"},"content":{"type":"string","description":"文章内容（支持 Markdown 格式）"},"tags":{"type":"array","items":{"type":"string"},"description":"文章标签"},"category":{"type":"string","description":"文章分类"}},"required":["title","content"]}`,
	}
	s.tools["csdn_publish"] = csdnPublishTool
	s.toolExec["csdn_publish"] = tools.NewCSDNPublishTool()

	// Quick Fetch Tool
	quickFetchTool := model.Tool{
		Name:        "quick_fetch",
		Description: "快速抓取网页内容。适用于需要登录的网站，通过注入 Cookie 获取页面内容。速度比 browser_execute 快很多。",
		InputSchema: `{"type":"object","properties":{"url":{"type":"string","description":"要抓取的网页 URL"},"selector":{"type":"string","description":"CSS 选择器，用于提取特定元素"},"cookies":{"type":"array","items":{"type":"object","properties":{"name":{"type":"string"},"value":{"type":"string"},"domain":{"type":"string"}}}}},"required":["url"]}`,
	}
	s.tools["quick_fetch"] = quickFetchTool
	s.toolExec["quick_fetch"] = tools.NewQuickFetchTool()

	// Time tool
	timeTool := model.Tool{
		Name:        "time",
		Description: "Get current time information.",
		InputSchema: `{"type":"object","properties":{"format":{"type":"string","description":"Time format: default, iso, unix"}}}`,
	}
	s.tools["time"] = timeTool
	s.toolExec["time"] = tools.NewTimeTool()

	// Data analysis tool
	dataTool := model.Tool{
		Name:        "data_analysis",
		Description: "Perform statistical analysis on data. Calculates mean, median, standard deviation, and other statistics.",
		InputSchema: `{"type":"object","properties":{"data":{"type":"array","items":{"type":"number"}},"operations":{"type":"array","items":{"type":"string"}}},"required":["data"]}`,
	}
	s.tools["data_analysis"] = dataTool
	s.toolExec["data_analysis"] = tools.NewDataAnalysisTool()

	// Visualization tool
	vizTool := model.Tool{
		Name:        "visualization",
		Description: "Generate visualization specifications for data. Returns chart configuration in JSON format.",
		InputSchema: `{"type":"object","properties":{"type":{"type":"string","description":"Chart type: bar, line, pie, scatter"},"title":{"type":"string"},"labels":{"type":"array","items":{"type":"string"}},"data":{"type":"array","items":{"type":"number"}}},"required":["type","data"]}`,
	}
	s.tools["visualization"] = vizTool
	s.toolExec["visualization"] = tools.NewVisualizationTool()

	// Code execution tool
	codeTool := model.Tool{
		Name:        "code_execute",
		Description: "Execute code in a sandboxed environment. Currently supports Python for calculations and data processing. (Note: Sandbox execution requires configuration)",
		InputSchema: `{"type":"object","properties":{"code":{"type":"string"},"language":{"type":"string","enum":["python"]}},"required":["code"]}`,
	}
	s.tools["code_execute"] = codeTool
	s.toolExec["code_execute"] = tools.NewCodeExecTool()
}

// ============================================================
// Connection Management (MCP Client)
// ============================================================

// Connect connects to an external MCP server, performs handshake, and discovers tools.
func (s *MCPService) Connect(ctx context.Context, req *pb.ConnectRequest) (*pb.ConnectResponse, error) {
	connID := fmt.Sprintf("%d", time.Now().UnixNano())

	conn := &model.Connection{
		ID:      connID,
		Name:    req.Name,
		Type:    req.Type,
		Command: req.Command,
		URL:     req.Url,
		Env:     req.Env,
		Status:  "connecting",
	}

	// Store connection early so ListConnections shows "connecting"
	s.mu.Lock()
	s.connections[connID] = conn
	s.mu.Unlock()

	// Create transport based on type
	var transport mcp.Transport
	switch req.Type {
	case "stdio":
		parts := strings.Fields(req.Command)
		if len(parts) == 0 {
			s.updateConnectionError(connID, "command is required for stdio type")
			return s.connectResponse(connID), nil
		}
		command := parts[0]
		args := parts[1:]
		transport = mcp.NewStdioTransport(command, args, req.Env)

	case "streamable-http":
		if req.Url == "" {
			s.updateConnectionError(connID, "url is required for streamable-http type")
			return s.connectResponse(connID), nil
		}
		transport = mcp.NewStreamableHTTPTransport(req.Url, nil)

	default:
		s.updateConnectionError(connID, fmt.Sprintf("unsupported connection type: %s", req.Type))
		return s.connectResponse(connID), nil
	}

	// Create MCP client and perform handshake
	client := mcp.NewClient(transport)

	// Use a timeout for the handshake
	handshakeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := client.Initialize(handshakeCtx); err != nil {
		s.updateConnectionError(connID, fmt.Sprintf("handshake failed: %v", err))
		// Clean up the failed client
		_ = client.Close()
		return s.connectResponse(connID), nil
	}

	// Handshake succeeded — update connection metadata
	serverInfo := client.ServerInfo()
	remoteTools := client.ListTools()

	s.mu.Lock()
	conn.Status = "connected"
	conn.ServerName = serverInfo.Name
	conn.ServerVersion = serverInfo.Version
	conn.ToolCount = len(remoteTools)
	s.mcpClients[connID] = client

	// Register remote tools with prefixed names
	for _, rt := range remoteTools {
		prefixedName := connID + "__" + rt.Name
		schemaBytes, _ := json.Marshal(rt.InputSchema)
		s.remoteTools[prefixedName] = remoteToolEntry{
			Tool: model.Tool{
				Name:        prefixedName,
				Description: rt.Description,
				InputSchema: string(schemaBytes),
			},
			ConnectionID: connID,
		}
	}
	s.mu.Unlock()

	log.Printf("[MCP] Connected to %s (%s v%s), discovered %d tools",
		req.Name, serverInfo.Name, serverInfo.Version, len(remoteTools))

	return s.connectResponse(connID), nil
}

// Disconnect disconnects from an external MCP server and cleans up.
func (s *MCPService) Disconnect(ctx context.Context, req *pb.DisconnectRequest) (*commonpb.Empty, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	connID := req.ConnectionId

	// Close the MCP client (which closes the transport)
	if client, ok := s.mcpClients[connID]; ok {
		_ = client.Close()
		delete(s.mcpClients, connID)
	}

	// Remove all remote tools belonging to this connection
	for prefixedName, entry := range s.remoteTools {
		if entry.ConnectionID == connID {
			delete(s.remoteTools, prefixedName)
		}
	}

	// Update connection status
	if conn, ok := s.connections[connID]; ok {
		conn.Status = "disconnected"
	}

	// Remove from connections map
	delete(s.connections, connID)

	log.Printf("[MCP] Disconnected connection %s", connID)

	return &commonpb.Empty{}, nil
}

// ListConnections lists all MCP connections.
func (s *MCPService) ListConnections(ctx context.Context, req *pb.ListConnectionsRequest) (*pb.ListConnectionsResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Check transport health and update status
	for connID, client := range s.mcpClients {
		if !client.IsAlive() {
			if conn, ok := s.connections[connID]; ok && conn.Status == "connected" {
				conn.Status = "disconnected"
				conn.ErrorMsg = "transport disconnected"
			}
		}
	}

	var connections []*pb.Connection
	for _, conn := range s.connections {
		pbConn := &pb.Connection{
			Id:            conn.ID,
			Name:          conn.Name,
			Type:          conn.Type,
			Status:        conn.Status,
			Command:       conn.Command,
			Url:           conn.URL,
			Env:           conn.Env,
			ServerName:    conn.ServerName,
			ServerVersion: conn.ServerVersion,
			ToolCount:     int32(conn.ToolCount),
			ErrorMsg:      conn.ErrorMsg,
		}
		connections = append(connections, pbConn)
	}

	return &pb.ListConnectionsResponse{Connections: connections}, nil
}

// ============================================================
// Tool Operations
// ============================================================

// ListTools lists all available tools (built-in + remote).
func (s *MCPService) ListTools(ctx context.Context, req *pb.ListToolsRequest) (*pb.ListToolsResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var toolList []*pb.Tool

	// If connection_id is specified, only return tools for that connection
	if req.ConnectionId != "" {
		if client, ok := s.mcpClients[req.ConnectionId]; ok {
			remoteTools := client.ListTools()
			for _, rt := range remoteTools {
				schemaBytes, _ := json.Marshal(rt.InputSchema)
				toolList = append(toolList, &pb.Tool{
					Name:        rt.Name,
					Description: rt.Description,
					InputSchema: string(schemaBytes),
				})
			}
		}
		return &pb.ListToolsResponse{Tools: toolList}, nil
	}

	// Built-in tools
	for _, tool := range s.tools {
		toolList = append(toolList, &pb.Tool{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: tool.InputSchema,
		})
	}

	// Remote tools (with prefixed names)
	for _, entry := range s.remoteTools {
		toolList = append(toolList, &pb.Tool{
			Name:        entry.Tool.Name,
			Description: entry.Tool.Description,
			InputSchema: entry.Tool.InputSchema,
		})
	}

	return &pb.ListToolsResponse{Tools: toolList}, nil
}

// CallTool calls a tool (built-in or remote) with governance checks.
func (s *MCPService) CallTool(ctx context.Context, req *pb.CallToolRequest) (*pb.CallToolResponse, error) {
	startTime := time.Now()
	fmt.Printf("[Governance] CallTool: %s\n", req.Name)

	// ★ Gate 1: 输入护栏检查
	inputContent := req.Arguments
	inputViolations := s.governance.Guardrail.CheckInput(inputContent)
	fmt.Printf("[Governance] Gate 1 - Input check: violations=%d\n", len(inputViolations))
	for _, v := range inputViolations {
		if v.Severity == "high" {
			fmt.Printf("[Governance] BLOCKED by input guardrail: %s\n", v.Description)
			return &pb.CallToolResponse{
				IsError: true,
				Content: fmt.Sprintf("输入被护栏拦截: %s", v.Description),
			}, nil
		}
	}

	// ★ 获取调用计数
	s.mu.Lock()
	callKey := fmt.Sprintf("general:%s", req.Name)
	s.callCounts[callKey]++
	callCount := s.callCounts[callKey]
	s.mu.Unlock()

	// ★ Gate 2-3: 工具权限和规则检查
	govReq := &governance.GovernanceRequest{
		AgentType:    "general",
		ToolName:     req.Name,
		InputContent: inputContent,
		CallCount:    callCount,
	}

	toolCheck := s.governance.CheckTool(govReq)
	fmt.Printf("[Governance] Gate 2-3 - Tool check: blocked=%v\n", toolCheck.Blocked)
	if toolCheck.Blocked {
		fmt.Printf("[Governance] BLOCKED by tool check: %s\n", toolCheck.BlockReason)
		return &pb.CallToolResponse{
			IsError: true,
			Content: fmt.Sprintf("工具调用被拦截: %s", toolCheck.BlockReason),
		}, nil
	}

	// --- Route: built-in first, then remote ---

	// 1. Try built-in tool
	s.mu.RLock()
	executor, isBuiltIn := s.toolExec[req.Name]
	s.mu.RUnlock()

	if isBuiltIn {
		return s.executeBuiltinTool(ctx, req, executor, startTime)
	}

	// 2. Try remote tool (by prefixed name or by connection_id + tool name)
	return s.executeRemoteTool(ctx, req, startTime)
}

// executeBuiltinTool runs a built-in tool executor.
func (s *MCPService) executeBuiltinTool(ctx context.Context, req *pb.CallToolRequest, executor tools.Executor, startTime time.Time) (*pb.CallToolResponse, error) {
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(req.Arguments), &args); err != nil {
		return &pb.CallToolResponse{
			IsError: true,
			Content: fmt.Sprintf("Invalid arguments: %v", err),
		}, nil
	}

	var toolConfig map[string]interface{}
	if req.ToolConfig != "" {
		if err := json.Unmarshal([]byte(req.ToolConfig), &toolConfig); err != nil {
			toolConfig = nil
		}
	}

	result, err := executor.ExecuteWithConfig(ctx, args, toolConfig)

	latencyMs := float64(time.Since(startTime).Milliseconds())
	success := err == nil
	s.governance.RecordMetrics("tool_success_rate", latencyMs, success)
	s.governance.RecordMetrics("tool_latency", latencyMs, success)

	if req.Name == "browser_execute" {
		s.governance.RecordMetrics("browser_tool_success", latencyMs, success)
	}

	if err != nil {
		return &pb.CallToolResponse{
			IsError: true,
			Content: err.Error(),
		}, nil
	}

	outputViolations := s.governance.Guardrail.CheckOutput(result)
	if len(outputViolations) > 0 {
		result = s.governance.Guardrail.SanitizeOutput(result)
	}

	return &pb.CallToolResponse{
		IsError: false,
		Content: result,
	}, nil
}

// executeRemoteTool routes a tool call to the appropriate remote MCP server.
func (s *MCPService) executeRemoteTool(ctx context.Context, req *pb.CallToolRequest, startTime time.Time) (*pb.CallToolResponse, error) {
	// Look up the remote client + tool name under the read lock, then release
	// before the (potentially slow) remote RPC so concurrent Connect/Disconnect
	// (which take the write lock) are not blocked for the whole call duration.
	s.mu.RLock()

	var client *mcp.Client
	var remoteToolName string

	// Strategy 1: req.Name is already a prefixed name (connID__toolName)
	if entry, ok := s.remoteTools[req.Name]; ok {
		if c, ok := s.mcpClients[entry.ConnectionID]; ok {
			client = c
			// Strip the prefix to get the original tool name
			parts := strings.SplitN(req.Name, "__", 2)
			if len(parts) == 2 {
				remoteToolName = parts[1]
			} else {
				remoteToolName = req.Name
			}
		}
	}

	// Strategy 2: connection_id specified in request + tool name
	if client == nil && req.ConnectionId != "" {
		if c, ok := s.mcpClients[req.ConnectionId]; ok {
			client = c
			remoteToolName = req.Name
		}
	}
	s.mu.RUnlock()

	if client == nil {
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

	// Call remote tool
	callResult, err := client.CallTool(ctx, remoteToolName, args)

	latencyMs := float64(time.Since(startTime).Milliseconds())
	success := err == nil
	s.governance.RecordMetrics("tool_success_rate", latencyMs, success)
	s.governance.RecordMetrics("tool_latency", latencyMs, success)

	if err != nil {
		return &pb.CallToolResponse{
			IsError: true,
			Content: fmt.Sprintf("Remote tool call failed: %v", err),
		}, nil
	}

	// Convert ToolCallResult to string content
	var contentParts []string
	for _, c := range callResult.Content {
		if c.Text != "" {
			contentParts = append(contentParts, c.Text)
		}
	}
	content := strings.Join(contentParts, "\n")

	if callResult.IsError {
		return &pb.CallToolResponse{
			IsError: true,
			Content: content,
		}, nil
	}

	outputViolations := s.governance.Guardrail.CheckOutput(content)
	if len(outputViolations) > 0 {
		content = s.governance.Guardrail.SanitizeOutput(content)
	}

	return &pb.CallToolResponse{
		IsError: false,
		Content: content,
	}, nil
}

// ============================================================
// Resource Operations (route to remote)
// ============================================================

// ListResources lists resources from a remote MCP server.
func (s *MCPService) ListResources(ctx context.Context, req *pb.ListResourcesRequest) (*pb.ListResourcesResponse, error) {
	if req.ConnectionId == "" {
		return &pb.ListResourcesResponse{}, nil
	}

	s.mu.RLock()
	client, ok := s.mcpClients[req.ConnectionId]
	s.mu.RUnlock()

	if !ok {
		return &pb.ListResourcesResponse{}, nil
	}

	result, err := client.ListResources(ctx)
	if err != nil {
		log.Printf("[MCP] ListResources failed for connection %s: %v", req.ConnectionId, err)
		return &pb.ListResourcesResponse{}, nil
	}

	var resources []*pb.Resource
	for _, r := range result.Resources {
		resources = append(resources, &pb.Resource{
			Uri:         r.URI,
			Name:        r.Name,
			Description: r.Description,
			MimeType:    r.MimeType,
		})
	}

	return &pb.ListResourcesResponse{Resources: resources}, nil
}

// ReadResource reads a resource from a remote MCP server.
func (s *MCPService) ReadResource(ctx context.Context, req *pb.ReadResourceRequest) (*pb.ReadResourceResponse, error) {
	if req.ConnectionId == "" {
		return nil, fmt.Errorf("connection_id is required")
	}

	s.mu.RLock()
	client, ok := s.mcpClients[req.ConnectionId]
	s.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("connection not found: %s", req.ConnectionId)
	}

	result, err := client.ReadResource(ctx, req.Uri)
	if err != nil {
		return nil, fmt.Errorf("read resource failed: %w", err)
	}

	if len(result.Contents) == 0 {
		return &pb.ReadResourceResponse{}, nil
	}

	// Return the first content item
	c := result.Contents[0]
	return &pb.ReadResourceResponse{
		Content:  c.Text,
		MimeType: c.MimeType,
	}, nil
}

// ============================================================
// Prompt Operations (route to remote)
// ============================================================

// ListPrompts lists prompts from a remote MCP server or returns built-in prompts.
func (s *MCPService) ListPrompts(ctx context.Context, req *pb.ListPromptsRequest) (*pb.ListPromptsResponse, error) {
	// If connection_id specified, route to remote
	if req.ConnectionId != "" {
		s.mu.RLock()
		client, ok := s.mcpClients[req.ConnectionId]
		s.mu.RUnlock()

		if !ok {
			return &pb.ListPromptsResponse{}, nil
		}

		result, err := client.ListPrompts(ctx)
		if err != nil {
			log.Printf("[MCP] ListPrompts failed for connection %s: %v", req.ConnectionId, err)
			return &pb.ListPromptsResponse{}, nil
		}

		var prompts []*pb.Prompt
		for _, p := range result.Prompts {
			pbPrompt := &pb.Prompt{
				Name:        p.Name,
				Description: p.Description,
			}
			for _, a := range p.Arguments {
				pbPrompt.Arguments = append(pbPrompt.Arguments, &pb.PromptArgument{
					Name:        a.Name,
					Description: a.Description,
					Required:    a.Required,
				})
			}
			prompts = append(prompts, pbPrompt)
		}

		return &pb.ListPromptsResponse{Prompts: prompts}, nil
	}

	// Default: return built-in prompts
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

// GetPrompt gets a prompt from a remote MCP server or returns built-in.
func (s *MCPService) GetPrompt(ctx context.Context, req *pb.GetPromptRequest) (*pb.GetPromptResponse, error) {
	// If connection_id specified, route to remote
	if req.ConnectionId != "" {
		s.mu.RLock()
		client, ok := s.mcpClients[req.ConnectionId]
		s.mu.RUnlock()

		if !ok {
			return nil, fmt.Errorf("connection not found: %s", req.ConnectionId)
		}

		result, err := client.GetPrompt(ctx, req.Name, req.Arguments)
		if err != nil {
			return nil, fmt.Errorf("get prompt failed: %w", err)
		}

		var messages []*pb.PromptMessage
		for _, m := range result.Messages {
			contentStr := fmt.Sprintf("%v", m.Content)
			messages = append(messages, &pb.PromptMessage{
				Role:    m.Role,
				Content: contentStr,
			})
		}

		return &pb.GetPromptResponse{
			Description: result.Description,
			Messages:    messages,
		}, nil
	}

	// Default: built-in prompt
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

// ============================================================
// Helpers
// ============================================================

// updateConnectionError sets a connection's status to "error" with a message.
func (s *MCPService) updateConnectionError(connID, errMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if conn, ok := s.connections[connID]; ok {
		conn.Status = "error"
		conn.ErrorMsg = errMsg
	}

	log.Printf("[MCP] Connection %s error: %s", connID, errMsg)
}

// connectResponse builds a ConnectResponse from the current connection state.
func (s *MCPService) connectResponse(connID string) *pb.ConnectResponse {
	s.mu.RLock()
	conn, ok := s.connections[connID]
	s.mu.RUnlock()

	if !ok {
		return &pb.ConnectResponse{Connection: &pb.Connection{Id: connID, Status: "error", ErrorMsg: "connection not found"}}
	}

	return &pb.ConnectResponse{
		Connection: &pb.Connection{
			Id:            conn.ID,
			Name:          conn.Name,
			Type:          conn.Type,
			Status:        conn.Status,
			Command:       conn.Command,
			Url:           conn.URL,
			Env:           conn.Env,
			ServerName:    conn.ServerName,
			ServerVersion: conn.ServerVersion,
			ToolCount:     int32(conn.ToolCount),
			ErrorMsg:      conn.ErrorMsg,
		},
	}
}

// ============================================================
// Governance API Methods
// ============================================================

// GetSLOStatus 获取 SLO 状态
func (s *MCPService) GetSLOStatus(sloID string) *governance.SLOStatus {
	return s.governance.GetSLOStatus(sloID)
}

// GetABTestResult 获取 A/B 测试结果
func (s *MCPService) GetABTestResult(testID string) *governance.ABTestResult {
	return s.governance.GetABResult(testID)
}

// CreateABTest 创建 A/B 测试
func (s *MCPService) CreateABTest(def *governance.ABTestDefinition) {
	s.governance.ABEngine.CreateTest(def)
}

// GetAlertStatus 获取告警状态
func (s *MCPService) GetAlertStatus() map[string]string {
	return s.governance.SLOManager.GetAlertStatus()
}

// AddRule 添加规则
func (s *MCPService) AddRule(agentType string, rule governance.ToolRule) {
	s.governance.RuleEngine.AddRule(agentType, rule)
}

// SetPermission 设置权限
func (s *MCPService) SetPermission(perm *governance.Permission) {
	s.governance.Permission.SetPermission(perm)
}
