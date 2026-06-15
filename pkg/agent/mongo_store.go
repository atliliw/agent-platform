// Package agent provides multi-agent orchestration capabilities
package agent

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// AgentDocument represents an agent stored in MongoDB
type AgentDocument struct {
	ID           string                    `bson:"_id"`
	Name         string                    `bson:"name"`
	Description  string                    `bson:"description"`
	Instructions string                    `bson:"instructions"`
	Tools        []string                  `bson:"tools"`
	Handoffs     []string                  `bson:"handoffs"`
	Model        string                    `bson:"model,omitempty"`
	MaxTokens    int                       `bson:"max_tokens"`
	Temperature  float64                   `bson:"temperature"`
	ToolConfig   map[string]ToolSpecificConfig `bson:"tool_config,omitempty"`
	Metadata     bson.M                    `bson:"metadata,omitempty"`
	CreatedAt    time.Time                 `bson:"created_at"`
	UpdatedAt    time.Time                 `bson:"updated_at"`
}

// MongoStore implements AgentStore using MongoDB
type MongoStore struct {
	client   *mongo.Client
	database *mongo.Database
	collection *mongo.Collection
}

// NewMongoStore creates a new MongoDB-based agent store
func NewMongoStore(client *mongo.Client, database string) *MongoStore {
	db := client.Database(database)
	collection := db.Collection("agents")

	return &MongoStore{
		client:     client,
		database:   db,
		collection: collection,
	}
}

// CreateIndex creates indexes for the agents collection
func (s *MongoStore) CreateIndex(ctx context.Context) error {
	// Create unique index on _id (agent id)
	_, err := s.collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "_id", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	return err
}

// Save saves an agent to MongoDB
func (s *MongoStore) Save(ctx context.Context, agent *Agent) error {
	now := time.Now()
	if agent.CreatedAt.IsZero() {
		agent.CreatedAt = now
	}
	agent.UpdatedAt = now

	doc := s.agentToDocument(agent)

	// Use upsert to insert or update
	filter := bson.M{"_id": agent.ID}
	update := bson.M{"$set": doc}

	opts := options.Update().SetUpsert(true)
	_, err := s.collection.UpdateOne(ctx, filter, update, opts)
	return err
}

// Get retrieves an agent by ID
func (s *MongoStore) Get(ctx context.Context, id string) (*Agent, error) {
	var doc AgentDocument
	err := s.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrAgentNotFound
		}
		return nil, err
	}
	return s.documentToAgent(&doc), nil
}

// Delete removes an agent by ID
func (s *MongoStore) Delete(ctx context.Context, id string) error {
	result, err := s.collection.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}
	if result.DeletedCount == 0 {
		return ErrAgentNotFound
	}
	return nil
}

// List returns all agents
func (s *MongoStore) List(ctx context.Context) ([]*Agent, error) {
	cursor, err := s.collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var agents []*Agent
	for cursor.Next(ctx) {
		var doc AgentDocument
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		agents = append(agents, s.documentToAgent(&doc))
	}

	if err := cursor.Err(); err != nil {
		return nil, err
	}

	return agents, nil
}

// Exists checks if an agent exists
func (s *MongoStore) Exists(ctx context.Context, id string) (bool, error) {
	count, err := s.collection.CountDocuments(ctx, bson.M{"_id": id})
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// Clear removes all agents
func (s *MongoStore) Clear(ctx context.Context) error {
	_, err := s.collection.DeleteMany(ctx, bson.M{})
	return err
}

// Count returns the number of agents
func (s *MongoStore) Count(ctx context.Context) (int64, error) {
	return s.collection.CountDocuments(ctx, bson.M{})
}

// agentToDocument converts Agent to AgentDocument
func (s *MongoStore) agentToDocument(a *Agent) *AgentDocument {
	doc := &AgentDocument{
		ID:           a.ID,
		Name:         a.Name,
		Description:  a.Description,
		Instructions: a.Instructions,
		Tools:        a.Tools,
		Handoffs:     a.Handoffs,
		Model:        a.Model,
		MaxTokens:    a.MaxTokens,
		Temperature:  a.Temperature,
		ToolConfig:   a.ToolConfig,
		CreatedAt:    a.CreatedAt,
		UpdatedAt:    a.UpdatedAt,
	}

	// Convert metadata to bson.M
	if a.Metadata != nil {
		doc.Metadata = make(bson.M)
		for k, v := range a.Metadata {
			doc.Metadata[k] = v
		}
	}

	return doc
}

// documentToAgent converts AgentDocument to Agent
func (s *MongoStore) documentToAgent(doc *AgentDocument) *Agent {
	a := &Agent{
		ID:           doc.ID,
		Name:         doc.Name,
		Description:  doc.Description,
		Instructions: doc.Instructions,
		Tools:        doc.Tools,
		Handoffs:     doc.Handoffs,
		Model:        doc.Model,
		MaxTokens:    doc.MaxTokens,
		Temperature:  doc.Temperature,
		ToolConfig:   doc.ToolConfig,
		CreatedAt:    doc.CreatedAt,
		UpdatedAt:    doc.UpdatedAt,
	}

	// Convert bson.M to map[string]any
	if doc.Metadata != nil {
		a.Metadata = make(map[string]any)
		for k, v := range doc.Metadata {
			a.Metadata[k] = v
		}
	}

	return a
}

// Close closes the MongoDB connection (not needed, connection managed externally)
func (s *MongoStore) Close() error {
	// Connection is managed by the caller
	return nil
}
