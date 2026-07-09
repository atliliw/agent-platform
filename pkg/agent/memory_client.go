package agent

import "context"

// MemoryClient is the interface between the agent engine and the memory service.
// The concrete gRPC implementation is injected by agent-service via SetMemoryClient.
//
// This interface lives in pkg/agent (not the memory service) to avoid a reverse
// dependency on the memory proto package. When memoryClient is nil, the engine
// degrades gracefully: no recall, no write — the agent still runs, just without
// memory. This keeps the loop robust to memory-service outages.
type MemoryClient interface {
	// Recall retrieves memories relevant to the given query before a decision step.
	// Returns formatted memory text to inject into the agent's context, or "" if none.
	Recall(ctx context.Context, sessionID, tenantID, query string) (string, error)

	// Write stores a memory entry derived from a step's outcome (called after tool
	// execution). importance is a hint in [0,1]; higher means more memorable.
	Write(ctx context.Context, sessionID, tenantID, agentID, content string, importance float64) error
}
