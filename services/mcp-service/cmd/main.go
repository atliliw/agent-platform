package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"agent-platform/pkg/computeruse"
	"agent-platform/pkg/config"
	"agent-platform/pkg/llm"
	pb "agent-platform/pkg/pb/mcp"
	"agent-platform/services/mcp-service/internal/handler"
	"agent-platform/services/mcp-service/internal/service"

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

	// Computer Use: if a desktop sidecar URL is configured, drive that container
	// over HTTP instead of local xdotool (which needs an X server in this
	// process, usually absent). When unset, computer_use falls back to local
	// xdotool and will only work if an X server is available.
	if desktopURL := os.Getenv("DESKTOP_URL"); desktopURL != "" {
		computeruse.SetDesktopPoolFactory(func() (computeruse.Desktop, error) {
			return computeruse.NewHTTPDesktop(desktopURL), nil
		})
		log.Printf("Computer Use desktop sidecar: %s", desktopURL)
	}

	// Create LLM client
	llmClient, err := llm.NewClient(llm.Config{
		Provider: cfg.LLM.Provider,
		APIKey:   cfg.LLM.APIKey,
		BaseURL:  cfg.LLM.BaseURL,
		Model:    cfg.LLM.Model,
	})
	if err != nil {
		log.Fatalf("Failed to create LLM client: %v", err)
	}

	// Create MCP service
	mcpService := service.NewMCPService(llmClient, cfg)

	// Create gRPC server
	server := grpc.NewServer()
	h := handler.NewMCPHandler(mcpService)
	pb.RegisterMCPServiceServer(server, h)

	// Start server
	addr := fmt.Sprintf(":%d", cfg.Server.GRPCPort)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	log.Printf("MCP Service starting on %s", addr)

	go func() {
		if err := server.Serve(listener); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down MCP Service...")
	server.GracefulStop()

	log.Println("MCP Service stopped")
}