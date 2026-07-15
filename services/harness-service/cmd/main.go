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
	"syscall"
	"time"

	"agent-platform/pkg/config"
	"agent-platform/pkg/llm"
	"agent-platform/pkg/observability"
	pb "agent-platform/pkg/pb/harness"
	agentpb "agent-platform/pkg/pb/agent"
	"agent-platform/services/harness-service/internal/handler"
	"agent-platform/services/harness-service/internal/repository"
	"agent-platform/services/harness-service/internal/service"

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
	tp, err := observability.InitServiceTracing("harness-service")
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

	// Connect to agent-service (optional — graceful if unavailable)
	var agentClient agentpb.AgentServiceClient
	if addr := cfg.Services.Agent; addr != "" {
		conn, err := grpc.Dial(addr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()),
		)
		if err != nil {
			log.Printf("Warning: could not connect to agent-service at %s: %v", addr, err)
		} else {
			agentClient = agentpb.NewAgentServiceClient(conn)
			log.Printf("Connected to agent-service at %s", addr)
		}
	} else {
		log.Printf("No agent-service address configured; proposal execution will be best-effort")
	}

	// Create harness service
	harnessService := service.NewHarnessService(llmClient, harnessRepo, cfg, agentClient)

	// Create internal HTTP server for receiving metrics from other services
	internalHTTP := http.NewServeMux()
	internalHTTP.HandleFunc("/internal/metrics/llm", func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			AgentID    string  `json:"agent_id"`
			Model      string  `json:"model"`
			LatencyMs  int64   `json:"latency_ms"`
			TotalTokens int64  `json:"total_tokens"`
			Cost       float64 `json:"cost"`
			Success    bool    `json:"success"`
			Caller     string  `json:"caller"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}

		// Record this metric into SLO manager
		// Get all SLOs for this agent and record events
		slos, _ := harnessService.GetSLOManager().ListSLOs(r.Context(), payload.AgentID, "")
		for _, sloDef := range slos {
			switch sloDef.Type {
			case "latency":
				harnessService.GetSLOManager().RecordEvent(r.Context(), sloDef.ID, true, float64(payload.LatencyMs))
			case "success_rate", "availability":
				harnessService.GetSLOManager().RecordEvent(r.Context(), sloDef.ID, payload.Success, float64(payload.LatencyMs))
			}
		}

		// Also record cost usage
		if payload.TotalTokens > 0 {
			inputTokens := payload.TotalTokens * 7 / 10 // approximate split
			outputTokens := payload.TotalTokens * 3 / 10
			harnessService.RecordCostUsageInternal(r.Context(), payload.AgentID, payload.Model, "", inputTokens, outputTokens)
		}

		w.WriteHeader(200)
		w.Write([]byte(`{"recorded":true}`))
	})

	go func() {
		httpAddr := fmt.Sprintf(":%d", cfg.Server.GRPCPort+1) // Use next port for internal HTTP
		log.Printf("Internal HTTP server for metrics on %s", httpAddr)
		if err := http.ListenAndServe(httpAddr, internalHTTP); err != nil {
			log.Printf("Internal HTTP server error: %v", err)
		}
	}()

	// Create gRPC server with OTel interceptor
	server := grpc.NewServer(
		grpc.UnaryInterceptor(otelgrpc.UnaryServerInterceptor()),
		grpc.StreamInterceptor(otelgrpc.StreamServerInterceptor()),
	)
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
