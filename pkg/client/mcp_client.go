// Package client provides gRPC clients for inter-service communication
package client

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "agent-platform/pkg/pb/mcp"
)

// MCPClient provides client for MCP service
type MCPClient interface {
	ListTools(ctx context.Context, req *pb.ListToolsRequest) (*pb.ListToolsResponse, error)
	CallTool(ctx context.Context, req *pb.CallToolRequest) (*pb.CallToolResponse, error)
	Close() error
}

// mcpClientImpl implements MCPClient
type mcpClientImpl struct {
	conn   *grpc.ClientConn
	client pb.MCPServiceClient
}

// NewMCPClient creates a new MCP client
func NewMCPClient(addr string) (MCPClient, error) {
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithTimeout(5*time.Second),
	)
	if err != nil {
		return nil, err
	}

	return &mcpClientImpl{
		conn:   conn,
		client: pb.NewMCPServiceClient(conn),
	}, nil
}

// ListTools lists available tools
func (c *mcpClientImpl) ListTools(ctx context.Context, req *pb.ListToolsRequest) (*pb.ListToolsResponse, error) {
	return c.client.ListTools(ctx, req)
}

// CallTool calls a tool
func (c *mcpClientImpl) CallTool(ctx context.Context, req *pb.CallToolRequest) (*pb.CallToolResponse, error) {
	return c.client.CallTool(ctx, req)
}

// Close closes the connection
func (c *mcpClientImpl) Close() error {
	return c.conn.Close()
}