// Package workflow provides DAG-based workflow definitions and execution
package workflow

import (
	"time"

	"github.com/google/uuid"
)

// NodeType defines the type of workflow node
type NodeType string

const (
	NodeAgent     NodeType = "agent"
	NodeTool      NodeType = "tool"
	NodeCondition NodeType = "condition"
	NodeParallel  NodeType = "parallel"
	NodeMerge     NodeType = "merge"
)

// Node defines a workflow node
type Node struct {
	ID        string                 `json:"id" bson:"_id"`
	Type      NodeType               `json:"type" bson:"type"`
	Name      string                 `json:"name" bson:"name"`
	AgentID   string                 `json:"agent_id,omitempty" bson:"agent_id,omitempty"`
	ToolName  string                 `json:"tool_name,omitempty" bson:"tool_name,omitempty"`
	Condition string                 `json:"condition,omitempty" bson:"condition,omitempty"`
	Config    map[string]interface{} `json:"config,omitempty" bson:"config,omitempty"`
	Position  *NodePosition          `json:"position,omitempty" bson:"position,omitempty"`
}

// NodePosition defines the visual position of a node
type NodePosition struct {
	X float64 `json:"x" bson:"x"`
	Y float64 `json:"y" bson:"y"`
}

// Edge defines a workflow edge (connection between nodes)
type Edge struct {
	ID        string `json:"id" bson:"_id"`
	From      string `json:"from" bson:"from"`
	To        string `json:"to" bson:"to"`
	Condition string `json:"condition,omitempty" bson:"condition,omitempty"`
	Label     string `json:"label,omitempty" bson:"label,omitempty"`
}

// Workflow defines a directed acyclic graph workflow
type Workflow struct {
	ID          string   `json:"id" bson:"_id"`
	Name        string   `json:"name" bson:"name"`
	Description string   `json:"description,omitempty" bson:"description,omitempty"`
	Nodes       []*Node  `json:"nodes" bson:"nodes"`
	Edges       []*Edge  `json:"edges" bson:"edges"`
	EntryNodeID string   `json:"entry_node_id" bson:"entry_node_id"`
	TenantID    string   `json:"tenant_id,omitempty" bson:"tenant_id,omitempty"`
	CreatedAt   int64    `json:"created_at" bson:"created_at"`
	UpdatedAt   int64    `json:"updated_at" bson:"updated_at"`
}

// NewWorkflow creates a new workflow with defaults
func NewWorkflow(name string) *Workflow {
	now := time.Now().Unix()
	return &Workflow{
		ID:        uuid.New().String(),
		Name:      name,
		Nodes:     make([]*Node, 0),
		Edges:     make([]*Edge, 0),
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// AddNode adds a node to the workflow
func (w *Workflow) AddNode(node *Node) {
	if node.ID == "" {
		node.ID = uuid.New().String()
	}
	w.Nodes = append(w.Nodes, node)
	w.UpdatedAt = time.Now().Unix()

	// Set as entry node if first node
	if w.EntryNodeID == "" {
		w.EntryNodeID = node.ID
	}
}

// AddEdge adds an edge to the workflow
func (w *Workflow) AddEdge(edge *Edge) {
	if edge.ID == "" {
		edge.ID = uuid.New().String()
	}
	w.Edges = append(w.Edges, edge)
	w.UpdatedAt = time.Now().Unix()
}

// GetNode returns a node by ID
func (w *Workflow) GetNode(id string) *Node {
	for _, node := range w.Nodes {
		if node.ID == id {
			return node
		}
	}
	return nil
}

// GetOutgoingEdges returns all edges originating from a node
func (w *Workflow) GetOutgoingEdges(nodeID string) []*Edge {
	edges := make([]*Edge, 0)
	for _, edge := range w.Edges {
		if edge.From == nodeID {
			edges = append(edges, edge)
		}
	}
	return edges
}

// GetIncomingEdges returns all edges targeting a node
func (w *Workflow) GetIncomingEdges(nodeID string) []*Edge {
	edges := make([]*Edge, 0)
	for _, edge := range w.Edges {
		if edge.To == nodeID {
			edges = append(edges, edge)
		}
	}
	return edges
}

// Validate checks the workflow for structural errors
func (w *Workflow) Validate() error {
	if w.Name == "" {
		return ErrWorkflowNameRequired
	}

	if w.EntryNodeID == "" {
		return ErrEntryNodeRequired
	}

	if w.GetNode(w.EntryNodeID) == nil {
		return ErrEntryNodeNotFound
	}

	// Check for duplicate node IDs
	nodeIDs := make(map[string]bool)
	for _, node := range w.Nodes {
		if nodeIDs[node.ID] {
			return ErrDuplicateNodeID
		}
		nodeIDs[node.ID] = true
	}

	// Check edge references exist
	for _, edge := range w.Edges {
		if !nodeIDs[edge.From] {
			return ErrEdgeFromNotFound
		}
		if !nodeIDs[edge.To] {
			return ErrEdgeToNotFound
		}
	}

	// Check for cycles using DFS
	if w.hasCycle() {
		return ErrCycleDetected
	}

	return nil
}

// hasCycle detects cycles using DFS
func (w *Workflow) hasCycle() bool {
	const (
		white = 0 // unvisited
		gray  = 1 // in progress
		black = 2 // done
	)

	colors := make(map[string]int)
	for _, node := range w.Nodes {
		colors[node.ID] = white
	}

	// Build adjacency list
	adj := make(map[string][]string)
	for _, edge := range w.Edges {
		adj[edge.From] = append(adj[edge.From], edge.To)
	}

	var dfs func(nodeID string) bool
	dfs = func(nodeID string) bool {
		colors[nodeID] = gray
		for _, next := range adj[nodeID] {
			if colors[next] == gray {
				return true // cycle found
			}
			if colors[next] == white && dfs(next) {
				return true
			}
		}
		colors[nodeID] = black
		return false
	}

	for _, node := range w.Nodes {
		if colors[node.ID] == white && dfs(node.ID) {
			return true
		}
	}

	return false
}
