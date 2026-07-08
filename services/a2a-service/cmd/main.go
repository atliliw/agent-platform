package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"agent-platform/pkg/config"
	pb "agent-platform/pkg/pb/a2a"
	agentpb "agent-platform/pkg/pb/agent"
	"agent-platform/services/a2a-service/internal/handler"
	"agent-platform/services/a2a-service/internal/repository"
	"agent-platform/services/a2a-service/internal/service"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// AgentClientImpl implements AgentClient by calling Agent Service via gRPC
type AgentClientImpl struct {
	client agentpb.AgentServiceClient
}

// Execute calls the Agent Service to execute a message
func (c *AgentClientImpl) Execute(ctx context.Context, sessionID, message string) (string, error) {
	resp, err := c.client.Execute(ctx, &agentpb.ExecuteRequest{
		SessionId: sessionID,
		Message:   message,
	})
	if err != nil {
		return "", err
	}
	return resp.Response, nil
}

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

	// Create A2A repository
	a2aRepo, err := repository.NewA2ARepository(cfg.Database.SQLite.Path)
	if err != nil {
		log.Fatalf("Failed to create A2A repository: %v", err)
	}

	// Seed default A2A agents if database is empty
	if n, err := a2aRepo.SeedDefaultAgents(context.Background()); err != nil {
		log.Printf("Warning: failed to seed default A2A agents: %v", err)
	} else if n > 0 {
		log.Printf("Seeded %d default A2A agents", n)
	}

	// Connect to Agent Service (optional - for task execution)
	var agentClient service.AgentClient
	agentAddr := os.Getenv("AGENT_SERVICE_ADDR")
	if agentAddr != "" {
		conn, err := grpc.Dial(agentAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Printf("Warning: Failed to connect to Agent Service: %v", err)
		} else {
			agentClient = &AgentClientImpl{
				client: agentpb.NewAgentServiceClient(conn),
			}
			log.Printf("Connected to Agent Service at %s", agentAddr)
		}
	}

	// Create A2A service
	a2aService := service.NewA2AService(a2aRepo, agentClient)

	// Create gRPC server
	server := grpc.NewServer()
	h := handler.NewA2AHandler(a2aService)
	pb.RegisterA2AServiceServer(server, h)

	// Start gRPC server
	grpcAddr := fmt.Sprintf(":%d", cfg.Server.GRPCPort)
	listener, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	log.Printf("A2A Service (gRPC) starting on %s", grpcAddr)

	go func() {
		if err := server.Serve(listener); err != nil {
			log.Fatalf("Failed to serve gRPC: %v", err)
		}
	}()

	// Start HTTP server for A2A protocol endpoints
	httpAddr := fmt.Sprintf(":%d", cfg.Server.HttpPort)
	httpMux := http.NewServeMux()

	// Agent card endpoint
	httpMux.HandleFunc("/.well-known/agent.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		card := a2aService.GetLocalAgentCard()
		w.Write([]byte(card))
	})

	// A2A task endpoints
	httpMux.HandleFunc("/api/v2/a2a/tasks/send", a2aService.HandleSendTask)
	httpMux.HandleFunc("/api/v2/a2a/tasks/", a2aService.HandleGetTask)

	httpServer := &http.Server{
		Addr:    httpAddr,
		Handler: httpMux,
	}

	log.Printf("A2A Service (HTTP) starting on %s", httpAddr)

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to serve HTTP: %v", err)
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down A2A Service...")
	server.GracefulStop()
	httpServer.Shutdown(context.Background())
	a2aRepo.Close()

	log.Println("A2A Service stopped")
}