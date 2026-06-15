package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"agent-platform/pkg/agent"
	"agent-platform/pkg/config"
	"agent-platform/pkg/llm"
	"agent-platform/pkg/mongodb"
	pb "agent-platform/pkg/pb/agent"
	mcppb "agent-platform/pkg/pb/mcp"
	"agent-platform/services/agent-service/internal/handler"
	"agent-platform/services/agent-service/internal/service"

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

	mcpConn, err := grpc.Dial(mcpAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Printf("Warning: Failed to connect to MCP service: %v", err)
		mcpConn = nil
	}

	var mcpClient mcppb.MCPServiceClient
	if mcpConn != nil {
		mcpClient = mcppb.NewMCPServiceClient(mcpConn)
		log.Printf("Connected to MCP service at %s", mcpAddr)
	}

	// Create agent service
	agentService := service.NewAgentService(registry, llmClient, mcpClient, store, cfg)

	// Create gRPC handler
	agentHandler := handler.NewAgentHandler(agentService)

	// Create gRPC server
	server := grpc.NewServer()
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
	if err := mongoClient.Close(context.Background()); err != nil {
		log.Printf("Error closing MongoDB connection: %v", err)
	}

	log.Println("Agent Service stopped")
}
