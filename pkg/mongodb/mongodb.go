// Package mongodb provides MongoDB client
package mongodb

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Config holds MongoDB client configuration
type Config struct {
	URI      string
	Database string
}

// Client is the MongoDB client wrapper
type Client struct {
	client   *mongo.Client
	database *mongo.Database
}

// Document represents a document
type Document struct {
	ID        string    `bson:"_id" json:"id"`
	Title     string    `bson:"title" json:"title"`
	Content   string    `bson:"content" json:"content"`
	Metadata  bson.M    `bson:"metadata,omitempty" json:"metadata,omitempty"`
	CreatedAt time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time `bson:"updated_at" json:"updated_at"`
}

// Chunk represents a document chunk
type Chunk struct {
	ID         string    `bson:"_id" json:"id"`
	DocID      string    `bson:"doc_id" json:"doc_id"`
	Content    string    `bson:"content" json:"content"`
	ChunkIndex int       `bson:"chunk_index" json:"chunk_index"`
	Metadata   bson.M    `bson:"metadata,omitempty" json:"metadata,omitempty"`
	CreatedAt  time.Time `bson:"created_at" json:"created_at"`
}

// NewClient creates a new MongoDB client
func NewClient(ctx context.Context, cfg Config) (*Client, error) {
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.URI))
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx, nil); err != nil {
		return nil, err
	}

	return &Client{
		client:   client,
		database: client.Database(cfg.Database),
	}, nil
}

// Close closes the connection
func (c *Client) Close(ctx context.Context) error {
	return c.client.Disconnect(ctx)
}

// Client returns the underlying mongo.Client
func (c *Client) Client() *mongo.Client {
	return c.client
}

// Database returns the database
func (c *Client) Database() *mongo.Database {
	return c.database
}

// InsertDocument inserts a document
func (c *Client) InsertDocument(ctx context.Context, doc *Document) error {
	collection := c.database.Collection("documents")
	if doc.CreatedAt.IsZero() {
		doc.CreatedAt = time.Now()
	}
	doc.UpdatedAt = doc.CreatedAt
	_, err := collection.InsertOne(ctx, doc)
	return err
}

// GetDocument gets a document by ID
func (c *Client) GetDocument(ctx context.Context, id string) (*Document, error) {
	collection := c.database.Collection("documents")
	var doc Document
	err := collection.FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

// ListDocuments lists documents
func (c *Client) ListDocuments(ctx context.Context, limit, offset int64) ([]Document, error) {
	collection := c.database.Collection("documents")
	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(limit).
		SetSkip(offset)

	cursor, err := collection.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var docs []Document
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, err
	}
	return docs, nil
}

// DeleteDocument deletes a document and its chunks
func (c *Client) DeleteDocument(ctx context.Context, id string) error {
	collection := c.database.Collection("documents")
	_, err := collection.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}

	chunkCollection := c.database.Collection("chunks")
	_, err = chunkCollection.DeleteMany(ctx, bson.M{"doc_id": id})
	return err
}

// InsertChunk inserts a chunk
func (c *Client) InsertChunk(ctx context.Context, chunk *Chunk) error {
	collection := c.database.Collection("chunks")
	if chunk.CreatedAt.IsZero() {
		chunk.CreatedAt = time.Now()
	}
	_, err := collection.InsertOne(ctx, chunk)
	return err
}

// InsertChunks inserts multiple chunks
func (c *Client) InsertChunks(ctx context.Context, chunks []Chunk) error {
	collection := c.database.Collection("chunks")
	interfaces := make([]interface{}, len(chunks))
	for i := range chunks {
		if chunks[i].CreatedAt.IsZero() {
			chunks[i].CreatedAt = time.Now()
		}
		interfaces[i] = chunks[i]
	}
	_, err := collection.InsertMany(ctx, interfaces)
	return err
}

// GetChunksByDocID gets chunks by document ID
func (c *Client) GetChunksByDocID(ctx context.Context, docID string) ([]Chunk, error) {
	collection := c.database.Collection("chunks")
	cursor, err := collection.Find(ctx, bson.M{"doc_id": docID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var chunks []Chunk
	if err := cursor.All(ctx, &chunks); err != nil {
		return nil, err
	}
	return chunks, nil
}

// DeleteChunksByDocID deletes chunks by document ID
func (c *Client) DeleteChunksByDocID(ctx context.Context, docID string) error {
	collection := c.database.Collection("chunks")
	_, err := collection.DeleteMany(ctx, bson.M{"doc_id": docID})
	return err
}

// SearchBM25 performs BM25 text search
func (c *Client) SearchBM25(ctx context.Context, query string, topK int) ([]Chunk, error) {
	collection := c.database.Collection("chunks")

	// Use text search if text index exists
	filter := bson.M{
		"$text": bson.M{
			"$search": query,
		},
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "score", Value: bson.M{"$meta": "textScore"}}}).
		SetLimit(int64(topK))

	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		// Fallback to regex search if text index doesn't exist
		filter = bson.M{
			"content": bson.M{
				"$regex":   query,
				"$options": "i",
			},
		}
		cursor, err = collection.Find(ctx, filter, options.Find().SetLimit(int64(topK)))
		if err != nil {
			return nil, err
		}
	}
	defer cursor.Close(ctx)

	var chunks []Chunk
	if err := cursor.All(ctx, &chunks); err != nil {
		return nil, err
	}
	return chunks, nil
}

// CountDocuments counts documents
func (c *Client) CountDocuments(ctx context.Context) (int64, error) {
	collection := c.database.Collection("documents")
	return collection.CountDocuments(ctx, bson.M{})
}

// CountChunks counts chunks
func (c *Client) CountChunks(ctx context.Context) (int64, error) {
	collection := c.database.Collection("chunks")
	return collection.CountDocuments(ctx, bson.M{})
}

// CreateTextIndex creates a text index on content field
func (c *Client) CreateTextIndex(ctx context.Context) error {
	collection := c.database.Collection("chunks")
	indexModel := mongo.IndexModel{
		Keys: bson.D{{Key: "content", Value: "text"}},
	}
	_, err := collection.Indexes().CreateOne(ctx, indexModel)
	return err
}

// GetAllChunks gets all chunks (for BM25 index building)
func (c *Client) GetAllChunks(ctx context.Context) ([]Chunk, error) {
	collection := c.database.Collection("chunks")
	cursor, err := collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var chunks []Chunk
	if err := cursor.All(ctx, &chunks); err != nil {
		return nil, err
	}
	return chunks, nil
}