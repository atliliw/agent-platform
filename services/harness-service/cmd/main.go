package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"agent-platform/pkg/config"
	"agent-platform/pkg/llm"
	pb "agent-platform/pkg/pb/harness"
	"agent-platform/services/harness-service/internal/handler"
	"agent-platform/services/harness-service/internal/repository"
	"agent-platform/services/harness-service/internal/service"

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
		Provider: cfg.LLM.Provider,
		APIKey:   cfg.LLM.APIKey,
		BaseURL:  cfg.LLM.BaseURL,
		Model:    cfg.LLM.Model,
	})
	if err != nil {
		log.Fatalf("Failed to create LLM client: %v", err)
	}

	// Create harness repository
	harnessRepo, err := repository.NewHarnessRepository(cfg.Database.SQLite.Path)
	if err != nil {
		log.Fatalf("Failed to create harness repository: %v", err)
	}

	// Create harness service
	harnessService := service.NewHarnessService(llmClient, harnessRepo, cfg)

	// Create gRPC server
	server := grpc.NewServer()
	h := handler.NewHarnessHandler(harnessService)
	pb.RegisterHarnessServiceServer(server, h)

	// Start server
	addr := fmt.Sprintf(":%d", cfg.Server.GRPCPort)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	log.Printf("Harness Service starting on %s", addr)

	go func() {
		if err := server.Serve(listener); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down Harness Service...")
	server.GracefulStop()
	harnessRepo.Close()

	log.Println("Harness Service stopped")
}