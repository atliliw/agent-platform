// Package checkpoint provides checkpoint persistence for agent execution,
// enabling save-and-resume of multi-step agent workflows.
package checkpoint

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Message represents a conversation message snapshot within a checkpoint.
// This mirrors agent.Message to avoid a circular import.
type Message struct {
	Role    string `json:"role" bson:"role"`
	Content string `json:"content" bson:"content"`
	Name    string `json:"name,omitempty" bson:"name,omitempty"`
	ToolID  string `json:"tool_id,omitempty" bson:"tool_id,omitempty"`
}

// AgentExecutionRecord records a single agent execution step within a checkpoint.
// This mirrors agent.AgentExecutionRecord to avoid a circular import.
type AgentExecutionRecord struct {
	AgentID     string    `json:"agent_id" bson:"agent_id"`
	AgentName   string    `json:"agent_name" bson:"agent_name"`
	Thought     string    `json:"thought" bson:"thought"`
	Action      string    `json:"action" bson:"action"`
	Arguments   string    `json:"arguments" bson:"arguments"`
	Result      string    `json:"result" bson:"result"`
	HandoffTo   string    `json:"handoff_to,omitempty" bson:"handoff_to,omitempty"`
	TokensUsed  int       `json:"tokens_used" bson:"tokens_used"`
	StartedAt   time.Time `json:"started_at" bson:"started_at"`
	CompletedAt time.Time `json:"completed_at" bson:"completed_at"`
	Duration    int64     `json:"duration_ms" bson:"duration_ms"`
}

// Checkpoint captures the full execution state at a given step so that
// execution can be resumed from that point later.
type Checkpoint struct {
	// ID is the unique checkpoint identifier (MongoDB ObjectID hex string).
	ID string `json:"id" bson:"_id,omitempty"`

	// SessionID links the checkpoint to an execution session.
	SessionID string `json:"session_id" bson:"session_id"`

	// Step is the zero-based step number at which this checkpoint was taken.
	Step int `json:"step" bson:"step"`

	// AgentID is the agent that was active at this step.
	AgentID string `json:"agent_id" bson:"agent_id"`

	// Messages is a snapshot of the conversation history.
	Messages []Message `json:"messages" bson:"messages"`

	// Variables is a snapshot of the context variables.
	Variables map[string]string `json:"variables" bson:"variables"`

	// ToolResults is a snapshot of tool call results keyed by tool call ID.
	ToolResults map[string]string `json:"tool_results" bson:"tool_results"`

	// AgentHistory is a snapshot of all agent execution records so far.
	AgentHistory []AgentExecutionRecord `json:"agent_history" bson:"agent_history"`

	// TotalTokens is the cumulative token count at this checkpoint.
	TotalTokens int `json:"total_tokens" bson:"total_tokens"`

	// CreatedAt is when this checkpoint was created.
	CreatedAt time.Time `json:"created_at" bson:"created_at"`
}

// CheckpointStore defines the persistence interface for checkpoints.
type CheckpointStore interface {
	// Save persists a checkpoint. If the checkpoint has no ID, a new one is
	// generated and set on the checkpoint.
	Save(ctx context.Context, cp *Checkpoint) error

	// Get retrieves a checkpoint by its ID.
	Get(ctx context.Context, id string) (*Checkpoint, error)

	// List returns all checkpoints for the given session, ordered by step.
	List(ctx context.Context, sessionID string) ([]*Checkpoint, error)

	// Delete removes a checkpoint by ID.
	Delete(ctx context.Context, id string) error
}

// ---------------------------------------------------------------------------
// MongoDBCheckpointStore
// ---------------------------------------------------------------------------

// checkpointDocument is the BSON representation stored in MongoDB.
type checkpointDocument struct {
	ID           primitive.ObjectID      `bson:"_id,omitempty"`
	SessionID    string                  `bson:"session_id"`
	Step         int                     `bson:"step"`
	AgentID      string                  `bson:"agent_id"`
	Messages     []Message               `bson:"messages"`
	Variables    map[string]string       `bson:"variables"`
	ToolResults  map[string]string       `bson:"tool_results"`
	AgentHistory []AgentExecutionRecord  `bson:"agent_history"`
	TotalTokens  int                     `bson:"total_tokens"`
	CreatedAt    time.Time               `bson:"created_at"`
}

// MongoDBCheckpointStore implements CheckpointStore using MongoDB.
type MongoDBCheckpointStore struct {
	client     *mongo.Client
	database   *mongo.Database
	collection *mongo.Collection
	mu         sync.RWMutex
}

// NewMongoDBCheckpointStore creates a new MongoDB-backed checkpoint store.
func NewMongoDBCheckpointStore(client *mongo.Client, database string) *MongoDBCheckpointStore {
	db := client.Database(database)
	collection := db.Collection("agent_checkpoints")

	return &MongoDBCheckpointStore{
		client:     client,
		database:   db,
		collection: collection,
	}
}

// CreateIndex creates the recommended indexes for the checkpoints collection.
func (s *MongoDBCheckpointStore) CreateIndex(ctx context.Context) error {
	_, err := s.collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "session_id", Value: 1}, {Key: "step", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "created_at", Value: -1}},
		},
	})
	return err
}

// Save persists a checkpoint. A new ObjectID is generated when cp.ID is empty.
func (s *MongoDBCheckpointStore) Save(ctx context.Context, cp *Checkpoint) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if cp.CreatedAt.IsZero() {
		cp.CreatedAt = time.Now()
	}

	doc := s.toDocument(cp)

	// Generate ObjectID if not present.
	if doc.ID.IsZero() {
		doc.ID = primitive.NewObjectID()
		cp.ID = doc.ID.Hex()
	}

	filter := bson.M{"_id": doc.ID}
	update := bson.M{"$set": doc}
	opts := options.Update().SetUpsert(true)

	_, err := s.collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("save checkpoint: %w", err)
	}
	return nil
}

// Get retrieves a checkpoint by ID.
func (s *MongoDBCheckpointStore) Get(ctx context.Context, id string) (*Checkpoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, fmt.Errorf("invalid checkpoint id %q: %w", id, err)
	}

	var doc checkpointDocument
	if err := s.collection.FindOne(ctx, bson.M{"_id": objID}).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrCheckpointNotFound
		}
		return nil, fmt.Errorf("get checkpoint: %w", err)
	}

	return s.fromDocument(&doc), nil
}

// List returns all checkpoints for the given session, ordered by step.
func (s *MongoDBCheckpointStore) List(ctx context.Context, sessionID string) ([]*Checkpoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	opts := options.Find().SetSort(bson.D{{Key: "step", Value: 1}})
	cursor, err := s.collection.Find(ctx, bson.M{"session_id": sessionID}, opts)
	if err != nil {
		return nil, fmt.Errorf("list checkpoints: %w", err)
	}
	defer cursor.Close(ctx)

	var checkpoints []*Checkpoint
	for cursor.Next(ctx) {
		var doc checkpointDocument
		if err := cursor.Decode(&doc); err != nil {
			return nil, fmt.Errorf("decode checkpoint: %w", err)
		}
		checkpoints = append(checkpoints, s.fromDocument(&doc))
	}

	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("cursor error: %w", err)
	}

	return checkpoints, nil
}

// Delete removes a checkpoint by ID.
func (s *MongoDBCheckpointStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return fmt.Errorf("invalid checkpoint id %q: %w", id, err)
	}

	result, err := s.collection.DeleteOne(ctx, bson.M{"_id": objID})
	if err != nil {
		return fmt.Errorf("delete checkpoint: %w", err)
	}
	if result.DeletedCount == 0 {
		return ErrCheckpointNotFound
	}
	return nil
}

// ---------------------------------------------------------------------------
// Conversion helpers
// ---------------------------------------------------------------------------

func (s *MongoDBCheckpointStore) toDocument(cp *Checkpoint) *checkpointDocument {
	doc := &checkpointDocument{
		SessionID:    cp.SessionID,
		Step:         cp.Step,
		AgentID:      cp.AgentID,
		Messages:     cp.Messages,
		Variables:    cp.Variables,
		ToolResults:  cp.ToolResults,
		AgentHistory: cp.AgentHistory,
		TotalTokens:  cp.TotalTokens,
		CreatedAt:    cp.CreatedAt,
	}

	if cp.ID != "" {
		objID, _ := primitive.ObjectIDFromHex(cp.ID)
		doc.ID = objID
	}

	return doc
}

func (s *MongoDBCheckpointStore) fromDocument(doc *checkpointDocument) *Checkpoint {
	return &Checkpoint{
		ID:           doc.ID.Hex(),
		SessionID:    doc.SessionID,
		Step:         doc.Step,
		AgentID:      doc.AgentID,
		Messages:     doc.Messages,
		Variables:    doc.Variables,
		ToolResults:  doc.ToolResults,
		AgentHistory: doc.AgentHistory,
		TotalTokens:  doc.TotalTokens,
		CreatedAt:    doc.CreatedAt,
	}
}

// ---------------------------------------------------------------------------
// MemoryCheckpointStore (for testing)
// ---------------------------------------------------------------------------

// MemoryCheckpointStore is an in-memory CheckpointStore for tests.
type MemoryCheckpointStore struct {
	mu          sync.RWMutex
	checkpoints map[string]*Checkpoint
}

// NewMemoryCheckpointStore creates a new in-memory checkpoint store.
func NewMemoryCheckpointStore() *MemoryCheckpointStore {
	return &MemoryCheckpointStore{
		checkpoints: make(map[string]*Checkpoint),
	}
}

// Save persists a checkpoint in memory.
func (s *MemoryCheckpointStore) Save(ctx context.Context, cp *Checkpoint) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if cp.ID == "" {
		cp.ID = primitive.NewObjectID().Hex()
	}
	if cp.CreatedAt.IsZero() {
		cp.CreatedAt = time.Now()
	}
	s.checkpoints[cp.ID] = cp
	return nil
}

// Get retrieves a checkpoint by ID from memory.
func (s *MemoryCheckpointStore) Get(ctx context.Context, id string) (*Checkpoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cp, ok := s.checkpoints[id]
	if !ok {
		return nil, ErrCheckpointNotFound
	}
	return cp, nil
}

// List returns checkpoints for a session from memory.
func (s *MemoryCheckpointStore) List(ctx context.Context, sessionID string) ([]*Checkpoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Checkpoint
	for _, cp := range s.checkpoints {
		if cp.SessionID == sessionID {
			result = append(result, cp)
		}
	}
	return result, nil
}

// Delete removes a checkpoint from memory.
func (s *MemoryCheckpointStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.checkpoints, id)
	return nil
}

// ---------------------------------------------------------------------------
// JSON helpers for serialization across service boundaries
// ---------------------------------------------------------------------------

// ToJSON serializes the checkpoint to JSON bytes.
func (cp *Checkpoint) ToJSON() ([]byte, error) {
	return json.Marshal(cp)
}

// CheckpointFromJSON deserializes a checkpoint from JSON bytes.
func CheckpointFromJSON(data []byte) (*Checkpoint, error) {
	var cp Checkpoint
	if err := json.Unmarshal(data, &cp); err != nil {
		return nil, fmt.Errorf("unmarshal checkpoint: %w", err)
	}
	return &cp, nil
}
