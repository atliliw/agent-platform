package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"agent-platform/pkg/config"
	"agent-platform/pkg/redis"
	"agent-platform/services/gateway/internal/middleware"
	"agent-platform/services/gateway/internal/router"

	"github.com/gin-gonic/gin"
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

	// Create Redis client (optional)
	var redisClient *redis.Client
	if cfg.Redis.URL != "" {
		redisClient, err = redis.NewClient(redis.Config{URL: cfg.Redis.URL})
		if err != nil {
			log.Printf("Warning: Redis not available: %v", err)
		}
	}

	// Setup Gin
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()

	// Middleware
	engine.Use(gin.Recovery())
	engine.Use(middleware.Logger())
	engine.Use(middleware.CORS())

	if redisClient != nil {
		engine.Use(middleware.RateLimit(redisClient, 100, time.Minute))
	}

	// Setup routes
	router.Setup(engine, cfg)

	// Start HTTP server
	addr := fmt.Sprintf(":%d", cfg.Server.HttpPort)
	srv := &http.Server{
		Addr:    addr,
		Handler: engine,
	}

	log.Printf("Gateway Service starting on %s", addr)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down Gateway Service...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	if redisClient != nil {
		redisClient.Close()
	}

	log.Println("Gateway Service stopped")
}