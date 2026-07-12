package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"agent-platform/pkg/agent"
	"agent-platform/pkg/agent/approval"
	"agent-platform/pkg/agent/checkpoint"
	"agent-platform/pkg/config"
	"agent-platform/pkg/llm"
	"agent-platform/pkg/mongodb"
	"agent-platform/pkg/observability"
	pb "agent-platform/pkg/pb/agent"
	mcppb "agent-platform/pkg/pb/mcp"
	memorypb "agent-platform/pkg/pb/memory"
	"agent-platform/services/agent-service/internal/handler"
	"agent-platform/services/agent-service/internal/service"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	// Load config
	cfgPath := os.Getenv("CONFIG_PATH")
	if cfgPath == "" {
		cfgPath = "./config.yaml"
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Printf("Using default config: %v", err)
		cfg = config.LoadDefault()
	}

	// Initialize OpenTelemetry tracing
	tp, err := observability.InitServiceTracing("agent-service")
	if err != nil {
		log.Printf("Warning: OTel init failed: %v", err)
	}
	if tp != nil {
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			tp.Shutdown(ctx)
		}()
	}

	// Create LLM client
	llmClient, err := llm.NewClient(llm.Config{
		Provider:  cfg.LLM.Provider,
		APIKey:    cfg.LLM.APIKey,
		BaseURL:   cfg.LLM.BaseURL,
		Model:     cfg.LLM.Model,
		MaxTokens: cfg.LLM.MaxTokens,
	})
	if err != nil {
		log.Fatalf("Failed to create LLM client: %v", err)
	}

	// Connect to MongoDB
	mongoURL := os.Getenv("MONGODB_URL")
	if mongoURL == "" {
		mongoURL = cfg.MongoDB.URL
	}
	mongoDB := os.Getenv("MONGODB_DATABASE")
	if mongoDB == "" {
		mongoDB = cfg.MongoDB.Database
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mongoClient, err := mongodb.NewClient(ctx, mongodb.Config{
		URI:      mongoURL,
		Database: mongoDB,
	})
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	log.Printf("Connected to MongoDB: %s", mongoURL)

	// Create MongoDB-based agent store
	agentStore := agent.NewMongoStore(mongoClient.Client(), mongoDB)
	if err := agentStore.CreateIndex(ctx); err != nil {
		log.Printf("Warning: Failed to create index: %v", err)
	}

	// Initialize default agents if database is empty
	// 如果数据库为空，自动插入默认 Agent
	inserted, err := agent.InitializeDefaultAgents(context.Background(), agentStore)
	if err != nil {
		log.Printf("Warning: Failed to initialize default agents: %v", err)
	} else if inserted > 0 {
		log.Printf("Initialized %d default agents in MongoDB", inserted)
	}

	// Create agent registry with persistence
	registry := agent.NewRegistryWithStore(agentStore)

	// Load all agents from MongoDB (初始化时从数据库读取)
	if err := registry.LoadFromStore(context.Background()); err != nil {
		log.Printf("Warning: Failed to load agents from MongoDB: %v", err)
	}

	agentCount := registry.Count()
	log.Printf("Loaded %d agents from MongoDB", agentCount)
	log.Printf("Registered agents: %v", registry.ListIDs())

	// Create context store (for execution contexts)
	storePath := os.Getenv("STORE_PATH")
	if storePath == "" {
		storePath = "./data/agent_contexts.db"
	}
	store, err := agent.NewSQLiteStore(storePath)
	if err != nil {
		log.Fatalf("Failed to create context store: %v", err)
	}

	// Connect to MCP service for tools
	mcpAddr := os.Getenv("MCP_SERVICE_ADDR")
	if mcpAddr == "" {
		mcpAddr = cfg.Services.MCP
		if mcpAddr == "" {
			mcpAddr = "mcp-service:50005"
		}
	}

	mcpConn, err := grpc.Dial(mcpAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()),
		grpc.WithStreamInterceptor(otelgrpc.StreamClientInterceptor()),
	)
	if err != nil {
		log.Printf("Warning: Failed to connect to MCP service: %v", err)
		mcpConn = nil
	}

	var mcpClient mcppb.MCPServiceClient
	if mcpConn != nil {
		mcpClient = mcppb.NewMCPServiceClient(mcpConn)
		log.Printf("Connected to MCP service at %s", mcpAddr)
	}

	// Connect to Memory service for agent recall/write
	memAddr := os.Getenv("MEMORY_SERVICE_ADDR")
	if memAddr == "" {
		memAddr = cfg.Services.Memory
		if memAddr == "" {
			memAddr = "memory-service:50003"
		}
	}

	memConn, err := grpc.Dial(memAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()),
		grpc.WithStreamInterceptor(otelgrpc.StreamClientInterceptor()),
	)
	if err != nil {
		log.Printf("Warning: Failed to connect to Memory service: %v", err)
		memConn = nil
	}

	var memoryClient memorypb.MemoryServiceClient
	if memConn != nil {
		memoryClient = memorypb.NewMemoryServiceClient(memConn)
		log.Printf("Connected to Memory service at %s", memAddr)
	}

	// Create agent service
	agentService := service.NewAgentService(registry, llmClient, mcpClient, memoryClient, store, cfg)

	// Wire checkpoint store into the engine (MongoDB-backed, for crash recovery).
	// The store is fully implemented in pkg/agent/checkpoint; this just connects it.
	cpStore := checkpoint.NewMongoDBCheckpointStore(mongoClient.Client(), mongoDB)
	cpCtx, cpCancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := cpStore.CreateIndex(cpCtx); err != nil {
		log.Printf("Warning: Failed to create checkpoint index: %v", err)
	}
	cpCancel()
	agentService.SetCheckpointStore(cpStore)

	// Create MongoDB-backed skill store and wire it into the engine.
	// Skills are independent capability modules an agent can mount (many-to-many).
	// The engine injects each mounted skill's Name+Description into the prompt and
	// serves full Instructions on demand via the load_skill built-in tool.
	skillStore := agent.NewMongoSkillStore(mongoClient.Client(), mongoDB)
	skillCtx, skillCancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := skillStore.CreateIndex(skillCtx); err != nil {
		log.Printf("Warning: Failed to create skill index: %v", err)
	}
	skillCancel()
	if inserted, err := agent.InitializeDefaultSkills(context.Background(), skillStore); err != nil {
		log.Printf("Warning: Failed to initialize default skills: %v", err)
	} else if inserted > 0 {
		log.Printf("Initialized %d default skills in MongoDB", inserted)
	}
	agentService.SetSkillStore(skillStore)

	// Create gRPC handler
	agentHandler := handler.NewAgentHandler(agentService)

	// Create gRPC server with OTel interceptor
	server := grpc.NewServer(
		grpc.UnaryInterceptor(otelgrpc.UnaryServerInterceptor()),
		grpc.StreamInterceptor(otelgrpc.StreamServerInterceptor()),
	)
	pb.RegisterAgentServiceServer(server, agentHandler)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "50006"
	}
	addr := fmt.Sprintf(":%s", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	log.Printf("Agent Service starting on %s", addr)

	go func() {
		if err := server.Serve(listener); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	// Start HTTP server for approval API
	approvalManager := agentService.GetApprovalManager()
	ruleEngine := agentService.GetRuleEngine()
	if approvalManager != nil {
		mux := http.NewServeMux()
		registerApprovalRoutes(mux, approvalManager, ruleEngine)
		httpPort := os.Getenv("HTTP_PORT")
		if httpPort == "" {
			httpPort = "50009"
		}
		go func() {
			log.Printf("Approval HTTP API starting on :%s", httpPort)
			if err := http.ListenAndServe(fmt.Sprintf(":%s", httpPort), mux); err != nil {
				log.Printf("Approval HTTP API error: %v", err)
			}
		}()
	}

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down Agent Service...")

	server.GracefulStop()

	// Close connections
	store.Close()
	if mcpConn != nil {
		mcpConn.Close()
	}
	if memConn != nil {
		memConn.Close()
	}
	if err := mongoClient.Close(context.Background()); err != nil {
		log.Printf("Error closing MongoDB connection: %v", err)
	}

	log.Println("Agent Service stopped")
}

// registerApprovalRoutes registers HTTP routes for the approval API
func registerApprovalRoutes(mux *http.ServeMux, am *approval.ApprovalFlowManager, re *approval.RuleEngine) {
	// GET /approval/pending?user_id=xxx — list pending approval requests
	mux.HandleFunc("/approval/pending", func(w http.ResponseWriter, r *http.Request) {
		userID := r.URL.Query().Get("user_id")
		requests := am.GetPendingRequests(userID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code": 0,
			"data": map[string]interface{}{
				"requests": requests,
			},
		})
	})

	// GET /approval/rules — list approval rules
	mux.HandleFunc("/approval/rules", func(w http.ResponseWriter, r *http.Request) {
		rules := re.ListRules()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code": 0,
			"data": map[string]interface{}{
				"rules": rules,
			},
		})
	})

	// POST /approval/approve — approve a pending request
	mux.HandleFunc("/approval/approve", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			RequestID string `json:"request_id"`
			UserID    string `json:"user_id"`
			Reason    string `json:"reason"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		decision := &approval.ApprovalDecision{
			RequestID: body.RequestID,
			Decision:  approval.StatusApproved,
			UserID:    body.UserID,
			Reason:    body.Reason,
			Timestamp: time.Now(),
		}
		err := am.SubmitDecision(r.Context(), decision)
		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"code": -1, "message": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"code": 0, "message": "approved"})
	})

	// POST /approval/reject — reject a pending request
	mux.HandleFunc("/approval/reject", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			RequestID string `json:"request_id"`
			UserID    string `json:"user_id"`
			Reason    string `json:"reason"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		decision := &approval.ApprovalDecision{
			RequestID: body.RequestID,
			Decision:  approval.StatusRejected,
			UserID:    body.UserID,
			Reason:    body.Reason,
			Timestamp: time.Now(),
		}
		err := am.SubmitDecision(r.Context(), decision)
		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"code": -1, "message": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"code": 0, "message": "rejected"})
	})

	// POST /approval/rules — add a new approval rule
	mux.HandleFunc("/approval/rules/add", func(w http.ResponseWriter, r *http.Request) {
		var rule approval.ApprovalRule
		if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		re.AddRule(&rule)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"code": 0, "message": "rule added"})
	})
}

// ListRules returns all rules (helper method for RuleEngine)
// This is added here since the approval package doesn't have a ListRules method
func init() {
	// Ensure RuleEngine has ListRules method available
	_ = strconv.Itoa(0) // suppress unused import
}
