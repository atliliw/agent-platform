package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"agent-platform/pkg/agent"
	"agent-platform/pkg/config"
	"agent-platform/pkg/llm"
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

	// Create agent registry
	registry := agent.NewRegistry()

	// Create context store
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
		mcpAddr = "mcp-service:50005"
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

	// Load default agents from config
	agentsPath := os.Getenv("AGENTS_CONFIG_PATH")
	if agentsPath == "" {
		agentsPath = "./configs/agents"
	}

	loader := agent.NewLoader(registry)
	if count, err := loader.LoadDirectoryAndRegister(agentsPath); err != nil {
		log.Printf("Warning: Failed to load agents from %s: %v", agentsPath, err)
	} else {
		log.Printf("Loaded %d agents from %s", count, agentsPath)
	}

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
	log.Printf("Registered agents: %v", registry.ListIDs())

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

	log.Println("Agent Service stopped")
}
