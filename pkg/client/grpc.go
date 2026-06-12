// Package client provides gRPC client factories
package client

import (
	"crypto/tls"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// Config holds gRPC client configuration
type Config struct {
	Address         string
	UseTLS          bool
	Timeout         time.Duration
	MaxRecvMsgSize  int
	MaxSendMsgSize  int
}

// DefaultConfig returns default configuration
func DefaultConfig(address string) Config {
	return Config{
		Address:        address,
		UseTLS:         false,
		Timeout:        30 * time.Second,
		MaxRecvMsgSize: 10 * 1024 * 1024, // 10MB
		MaxSendMsgSize: 10 * 1024 * 1024, // 10MB
	}
}

// NewConn creates a new gRPC connection
func NewConn(cfg Config) (*grpc.ClientConn, error) {
	var opts []grpc.DialOption

	if cfg.UseTLS {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: false,
		}
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	if cfg.MaxRecvMsgSize > 0 {
		opts = append(opts, grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(cfg.MaxRecvMsgSize),
		))
	}

	if cfg.MaxSendMsgSize > 0 {
		opts = append(opts, grpc.WithDefaultCallOptions(
			grpc.MaxCallSendMsgSize(cfg.MaxSendMsgSize),
		))
	}

	return grpc.Dial(cfg.Address, opts...)
}

// ClientPool holds all gRPC clients
type ClientPool struct {
	ChatConn     *grpc.ClientConn
	KnowledgeConn *grpc.ClientConn
	MemoryConn   *grpc.ClientConn
	A2AConn      *grpc.ClientConn
	MCPConn      *grpc.ClientConn
	HarnessConn  *grpc.ClientConn
}

// NewClientPool creates a new client pool
func NewClientPool(chatAddr, knowledgeAddr, memoryAddr, a2aAddr, mcpAddr, harnessAddr string) (*ClientPool, error) {
	pool := &ClientPool{}
	var err error

	if chatAddr != "" {
		pool.ChatConn, err = NewConn(DefaultConfig(chatAddr))
		if err != nil {
			return nil, err
		}
	}

	if knowledgeAddr != "" {
		pool.KnowledgeConn, err = NewConn(DefaultConfig(knowledgeAddr))
		if err != nil {
			return nil, err
		}
	}

	if memoryAddr != "" {
		pool.MemoryConn, err = NewConn(DefaultConfig(memoryAddr))
		if err != nil {
			return nil, err
		}
	}

	if a2aAddr != "" {
		pool.A2AConn, err = NewConn(DefaultConfig(a2aAddr))
		if err != nil {
			return nil, err
		}
	}

	if mcpAddr != "" {
		pool.MCPConn, err = NewConn(DefaultConfig(mcpAddr))
		if err != nil {
			return nil, err
		}
	}

	if harnessAddr != "" {
		pool.HarnessConn, err = NewConn(DefaultConfig(harnessAddr))
		if err != nil {
			return nil, err
		}
	}

	return pool, nil
}

// Close closes all connections
func (p *ClientPool) Close() error {
	if p.ChatConn != nil {
		p.ChatConn.Close()
	}
	if p.KnowledgeConn != nil {
		p.KnowledgeConn.Close()
	}
	if p.MemoryConn != nil {
		p.MemoryConn.Close()
	}
	if p.A2AConn != nil {
		p.A2AConn.Close()
	}
	if p.MCPConn != nil {
		p.MCPConn.Close()
	}
	if p.HarnessConn != nil {
		p.HarnessConn.Close()
	}
	return nil
}
