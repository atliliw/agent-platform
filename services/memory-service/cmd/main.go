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

	"agent-platform/pkg/config"
	"agent-platform/pkg/llm"
	"agent-platform/pkg/observability"
	"agent-platform/pkg/qdrant"
	pb "agent-platform/pkg/pb/memory"
	"agent-platform/services/memory-service/internal/cases"
	"agent-platform/services/memory-service/internal/episodic"
	"agent-platform/services/memory-service/internal/handler"
	"agent-platform/services/memory-service/internal/repository"
	"agent-platform/services/memory-service/internal/semantic"
	"agent-platform/services/memory-service/internal/service"
	"agent-platform/services/memory-service/internal/working"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
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

	// Initialize OpenTelemetry tracing
	tp, err := observability.InitServiceTracing("memory-service")
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
	var qdrantClient *qdrant.Client
	if cfg.Qdrant.URL != "" {
		qdrantClient = qdrant.NewClient(qdrant.Config{
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

	// Create memory service with forgetting (replaces basic MemoryService)
	forgettingConfig := service.DefaultForgettingConfig()
	memoryService := service.NewMemoryServiceWithForgetting(llmClient, qdrantClient, memoryRepo, forgettingConfig)
	log.Printf("Memory service initialized with forgetting (decay=%.2f, threshold=%.2f, maxAge=%dh)",
		forgettingConfig.TimeDecayRate, forgettingConfig.ImportanceThreshold, forgettingConfig.MaxAgeHours)

	// Initialize episodic memory subsystem
	episodicMemory := episodic.NewEpisodicMemory(10000)
	log.Printf("Episodic memory initialized (capacity=10000)")

	// Initialize semantic memory subsystem
	semanticMemory := semantic.NewSemanticMemory(5000)
	log.Printf("Semantic memory initialized (capacity=5000)")

	// Initialize working memory subsystem
	workingMemory := working.NewWorkingMemory(8000, 100)
	log.Printf("Working memory initialized (maxTokens=8000, maxMessages=100)")

	// Initialize case-based reasoning subsystem
	caseLibrary := cases.NewCaseLibrary(10000)
	caseRetriever := cases.NewCaseRetriever(caseLibrary)
	log.Printf("Case-based reasoning initialized (capacity=10000)")

	// Create gRPC handler with all subsystems
	h := handler.NewMemoryHandler(memoryService, episodicMemory, semanticMemory, workingMemory, caseLibrary, caseRetriever)

	// Create gRPC server with OTel interceptor
	server := grpc.NewServer(
		grpc.UnaryInterceptor(otelgrpc.UnaryServerInterceptor()),
		grpc.StreamInterceptor(otelgrpc.StreamServerInterceptor()),
	)
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
