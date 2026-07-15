package agent

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// SkillDocument represents a skill stored in MongoDB.
type SkillDocument struct {
	ID           string    `bson:"_id"`
	Name         string    `bson:"name"`
	Description  string    `bson:"description"`
	Instructions string    `bson:"instructions"`
	Tools        []string  `bson:"tools,omitempty"`
	Tags         []string  `bson:"tags,omitempty"`
	Status       string    `bson:"status"`
	Version      int       `bson:"version"`
	CreatedAt    time.Time `bson:"created_at"`
	UpdatedAt    time.Time `bson:"updated_at"`
}

// MongoSkillStore implements SkillStore using MongoDB. Mirrors MongoStore.
type MongoSkillStore struct {
	client     *mongo.Client
	database   *mongo.Database
	collection *mongo.Collection
}

// NewMongoSkillStore creates a new MongoDB-based skill store.
func NewMongoSkillStore(client *mongo.Client, database string) *MongoSkillStore {
	db := client.Database(database)
	collection := db.Collection("skills")
	return &MongoSkillStore{
		client:     client,
		database:   db,
		collection: collection,
	}
}

// CreateIndex creates indexes for the skills collection. A unique index on
// name enforces skill-name uniqueness (load_skill resolves by name).
func (s *MongoSkillStore) CreateIndex(ctx context.Context) error {
	_, err := s.collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "name", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	return err
}

// SaveSkill inserts or updates a skill (upsert by ID).
func (s *MongoSkillStore) SaveSkill(ctx context.Context, skill *Skill) error {
	now := time.Now()
	if skill.CreatedAt.IsZero() {
		skill.CreatedAt = now
	}
	skill.UpdatedAt = now

	doc := skillToDocument(skill)
	filter := bson.M{"_id": skill.ID}
	update := bson.M{"$set": doc}
	opts := options.Update().SetUpsert(true)
	_, err := s.collection.UpdateOne(ctx, filter, update, opts)
	return err
}

// GetSkill retrieves a skill by ID.
func (s *MongoSkillStore) GetSkill(ctx context.Context, id string) (*Skill, error) {
	var doc SkillDocument
	err := s.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrSkillNotFound
		}
		return nil, err
	}
	return documentToSkill(&doc), nil
}

// GetSkillByName retrieves a skill by its unique Name.
func (s *MongoSkillStore) GetSkillByName(ctx context.Context, name string) (*Skill, error) {
	var doc SkillDocument
	err := s.collection.FindOne(ctx, bson.M{"name": name}).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrSkillNotFound
		}
		return nil, err
	}
	return documentToSkill(&doc), nil
}

// DeleteSkill removes a skill by ID.
func (s *MongoSkillStore) DeleteSkill(ctx context.Context, id string) error {
	result, err := s.collection.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}
	if result.DeletedCount == 0 {
		return ErrSkillNotFound
	}
	return nil
}

// ListSkills returns all skills.
func (s *MongoSkillStore) ListSkills(ctx context.Context) ([]*Skill, error) {
	cursor, err := s.collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var skills []*Skill
	for cursor.Next(ctx) {
		var doc SkillDocument
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		skills = append(skills, documentToSkill(&doc))
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	return skills, nil
}

// GetSkillsByIDs returns the skills matching the given IDs. Missing IDs are
// skipped (the engine tolerates dangling skill references on agents).
func (s *MongoSkillStore) GetSkillsByIDs(ctx context.Context, ids []string) ([]*Skill, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	filter := bson.M{"_id": bson.M{"$in": ids}}
	cursor, err := s.collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var skills []*Skill
	for cursor.Next(ctx) {
		var doc SkillDocument
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		skills = append(skills, documentToSkill(&doc))
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	return skills, nil
}

// skillToDocument converts Skill to SkillDocument.
func skillToDocument(s *Skill) *SkillDocument {
	return &SkillDocument{
		ID:           s.ID,
		Name:         s.Name,
		Description:  s.Description,
		Instructions: s.Instructions,
		Tools:        s.Tools,
		Tags:         s.Tags,
		Status:       string(s.Status),
		Version:      s.Version,
		CreatedAt:    s.CreatedAt,
		UpdatedAt:    s.UpdatedAt,
	}
}

// documentToSkill converts SkillDocument to Skill.
func documentToSkill(doc *SkillDocument) *Skill {
	return &Skill{
		ID:           doc.ID,
		Name:         doc.Name,
		Description:  doc.Description,
		Instructions: doc.Instructions,
		Tools:        doc.Tools,
		Tags:         doc.Tags,
		Status:       SkillStatus(doc.Status),
		Version:      doc.Version,
		CreatedAt:    doc.CreatedAt,
		UpdatedAt:    doc.UpdatedAt,
	}
}
