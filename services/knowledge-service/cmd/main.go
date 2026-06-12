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
	"agent-platform/pkg/mongodb"
	"agent-platform/pkg/qdrant"
	pb "agent-platform/pkg/pb/knowledge"
	"agent-platform/services/knowledge-service/internal/handler"
	"agent-platform/services/knowledge-service/internal/repository"
	"agent-platform/services/knowledge-service/internal/service"

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

	// Create Qdrant client (optional)
	var qdrantClient *qdrant.Client
	if cfg.Qdrant.URL != "" && cfg.Qdrant.URL != "http://localhost:6333" {
		qdrantClient = qdrant.NewClient(qdrant.Config{
			URL:        cfg.Qdrant.URL,
			Collection: cfg.Qdrant.Collection,
		})
	} else {
		log.Println("Qdrant not configured, using in-memory storage")
	}

	// Create MongoDB client (optional)
	var mongoClient *mongodb.Client
	if cfg.MongoDB.URL != "" && cfg.MongoDB.URL != "mongodb://localhost:27017" {
		mongoClient, err = mongodb.NewClient(context.Background(), mongodb.Config{
			URI:      cfg.MongoDB.URL,
			Database: cfg.MongoDB.Database,
		})
		if err != nil {
			log.Printf("Warning: Failed to create MongoDB client: %v", err)
			log.Println("Continuing without MongoDB - knowledge storage will be limited")
			mongoClient = nil
		}
	} else {
		log.Println("MongoDB not configured, using SQLite for document storage")
	}

	// Create document repository
	docRepo := repository.NewDocumentRepository(mongoClient, qdrantClient)

	// Create knowledge service
	knowledgeService := service.NewKnowledgeService(llmClient, docRepo, cfg)

	// Create gRPC server
	server := grpc.NewServer()
	h := handler.NewKnowledgeHandler(knowledgeService)
	pb.RegisterKnowledgeServiceServer(server, h)

	// Start server
	addr := fmt.Sprintf(":%d", cfg.Server.GRPCPort)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	log.Printf("Knowledge Service starting on %s", addr)

	go func() {
		if err := server.Serve(listener); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down Knowledge Service...")
	server.GracefulStop()

	// Close connections
	if mongoClient != nil {
		mongoClient.Close(context.Background())
	}

	log.Println("Knowledge Service stopped")
}