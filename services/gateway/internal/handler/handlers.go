// Package handler provides HTTP handlers for Gateway
package handler

import (
	"agent-platform/pkg/config"
	"agent-platform/pkg/client"
)

// Handlers holds all handlers
type Handlers struct {
	cfg        *config.Config
	clientPool *client.ClientPool
}

// NewHandlers creates new handlers
func NewHandlers(cfg *config.Config) (*Handlers, error) {
	pool, err := client.NewClientPool(
		cfg.Services.Chat,
		cfg.Services.Knowledge,
		cfg.Services.Memory,
		cfg.Services.A2A,
		cfg.Services.MCP,
		cfg.Services.Harness,
	)
	if err != nil {
		return nil, err
	}

	return &Handlers{
		cfg:        cfg,
		clientPool: pool,
	}, nil
}
