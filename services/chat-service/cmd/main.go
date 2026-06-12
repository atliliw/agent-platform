package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"agent-platform/pkg/config"
	"agent-platform/pkg/client"
	"agent-platform/pkg/llm"
	"agent-platform/pkg/qdrant"
	pb "agent-platform/pkg/pb/chat"
	"agent-platform/services/chat-service/internal/handler"
	"agent-platform/services/chat-service/internal/repository"
	"agent-platform/services/chat-service/internal/service"

	"google.golang.org/grpc"
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

	// Create Qdrant client
	qdrantClient := qdrant.NewClient(qdrant.Config{
		URL:        cfg.Qdrant.URL,
		Collection: cfg.Qdrant.Collection,
	})

	// Create session repository
	sessionRepo, err := repository.NewSessionRepository(cfg.Database.SQLite.Path)
	if err != nil {
		log.Fatalf("Failed to create session repository: %v", err)
	}

	// Create MCP client for tool calling
	mcpAddr := os.Getenv("MCP_SERVICE_ADDR")
	if mcpAddr == "" {
		mcpAddr = "mcp-service:50005"
	}
	mcpClient, err := client.NewMCPClient(mcpAddr)
	if err != nil {
		log.Printf("Warning: Failed to create MCP client: %v (continuing without tools)", err)
		mcpClient = nil
	} else {
		log.Printf("Connected to MCP service at %s", mcpAddr)
	}

	// Create chat service with Agent capabilities
	chatService := service.NewChatService(llmClient, qdrantClient, sessionRepo, mcpClient, cfg)

	// Create gRPC server
	server := grpc.NewServer()
	pb.RegisterChatServiceServer(server, handler.NewChatHandler(chatService))

	// Start server
	addr := fmt.Sprintf(":%d", cfg.Server.GRPCPort)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	log.Printf("Chat Service (Agent-enabled) starting on %s", addr)

	go func() {
		if err := server.Serve(listener); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down Chat Service...")

	server.GracefulStop()

	// Close connections
	sessionRepo.Close()
	if mcpClient != nil {
		mcpClient.Close()
	}

	log.Println("Chat Service stopped")
}