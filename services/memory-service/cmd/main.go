package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"agent-platform/pkg/config"
	"agent-platform/pkg/llm"
	"agent-platform/pkg/qdrant"
	pb "agent-platform/pkg/pb/memory"
	"agent-platform/services/memory-service/internal/handler"
	"agent-platform/services/memory-service/internal/repository"
	"agent-platform/services/memory-service/internal/service"

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

	// Create LLM client for embeddings
	llmClient, err := llm.NewClient(llm.Config{
		Provider:       cfg.LLM.Provider,
		APIKey:         cfg.LLM.APIKey,
		BaseURL:        cfg.LLM.BaseURL,
		Model:          cfg.LLM.Model,
		EmbeddingModel: cfg.LLM.EmbeddingModel,
	})
	if err != nil {
		log.Fatalf("Failed to create LLM client: %v", err)
	}

	// Create memory repository
	memoryRepo, err := repository.NewMemoryRepository(cfg.Database.SQLite.Path)
	if err != nil {
		log.Fatalf("Failed to create memory repository: %v", err)
	}

	// Set Qdrant client for vector search
	if cfg.Qdrant.URL != "" {
		qdrantClient := qdrant.NewClient(qdrant.Config{
			URL:        cfg.Qdrant.URL,
			Collection: cfg.Qdrant.Collection,
		})

		// Create collection if not exists (text-embedding-v3 has 1024 dimensions)
		ctx := context.Background()
		exists, err := qdrantClient.CollectionExists(ctx)
		if err != nil {
			log.Printf("Warning: Failed to check collection existence: %v", err)
		} else if !exists {
			// text-embedding-v3 from DashScope has 1024 dimensions
			if err := qdrantClient.CreateCollection(ctx, 1024); err != nil {
				log.Printf("Warning: Failed to create collection: %v", err)
			} else {
				log.Printf("Created Qdrant collection: %s", cfg.Qdrant.Collection)
			}
		}

		memoryRepo.SetQdrant(qdrantClient)
		log.Printf("Qdrant configured: %s", cfg.Qdrant.URL)
	}

	// Create memory service
	memoryService := service.NewMemoryService(llmClient, memoryRepo)

	// Create gRPC server
	server := grpc.NewServer()
	h := handler.NewMemoryHandler(memoryService)
	pb.RegisterMemoryServiceServer(server, h)

	// Start server
	addr := fmt.Sprintf(":%d", cfg.Server.GRPCPort)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	log.Printf("Memory Service starting on %s", addr)

	go func() {
		if err := server.Serve(listener); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down Memory Service...")
	server.GracefulStop()

	// Close connections
	memoryRepo.Close()

	log.Println("Memory Service stopped")
}