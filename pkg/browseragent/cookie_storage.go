// Package browseragent provides cookie storage for browser automation
package browseragent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// CookieStorage defines the interface for cookie storage
type CookieStorage interface {
	// Save saves cookies for a user/domain
	Save(ctx context.Context, userID, tenantID string, cookies []Cookie) error

	// Get retrieves cookies by domain
	Get(ctx context.Context, userID, tenantID, domain string) ([]Cookie, error)

	// GetAll retrieves all cookies for a user
	GetAll(ctx context.Context, userID, tenantID string) ([]StoredCookie, error)

	// Delete deletes cookies by domain
	Delete(ctx context.Context, userID, tenantID, domain string) error

	// DeleteExpired deletes all expired cookies
	DeleteExpired(ctx context.Context) error
}

// StoredCookie represents a cookie stored in database
type StoredCookie struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	UserID    string             `bson:"user_id" json:"user_id"`
	TenantID  string             `bson:"tenant_id" json:"tenant_id"`
	Domain    string             `bson:"domain" json:"domain"`
	Name      string             `bson:"name" json:"name"`
	Value     string             `bson:"value" json:"value"`
	Path      string             `bson:"path,omitempty" json:"path,omitempty"`
	ExpiresAt *time.Time         `bson:"expires_at,omitempty" json:"expires_at,omitempty"`
	HTTPOnly  bool               `bson:"http_only" json:"http_only"`
	Secure    bool               `bson:"secure" json:"secure"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time          `bson:"updated_at" json:"updated_at"`
}

// MongoCookieStorage implements CookieStorage using MongoDB
type MongoCookieStorage struct {
	collection *mongo.Collection
}

// NewMongoCookieStorage creates a new MongoDB cookie storage
func NewMongoCookieStorage(db *mongo.Database) *MongoCookieStorage {
	collection := db.Collection("browser_cookies")

	// Create indexes
	indexModel := mongo.IndexModel{
		Keys: bson.D{
			{Key: "user_id", Value: 1},
			{Key: "tenant_id", Value: 1},
			{Key: "domain", Value: 1},
			{Key: "name", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}

	// TTL index for expired cookies
	ttlIndex := mongo.IndexModel{
		Keys:    bson.D{{Key: "expires_at", Value: 1}},
		Options: options.Index().SetExpireAfterSeconds(0),
	}

	// Create indexes in background
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		collection.Indexes().CreateOne(ctx, indexModel)
		collection.Indexes().CreateOne(ctx, ttlIndex)
	}()

	return &MongoCookieStorage{collection: collection}
}

// Save saves cookies for a user/domain
func (s *MongoCookieStorage) Save(ctx context.Context, userID, tenantID string, cookies []Cookie) error {
	now := time.Now()

	for _, c := range cookies {
		filter := bson.M{
			"user_id":   userID,
			"tenant_id": tenantID,
			"domain":    c.Domain,
			"name":      c.Name,
		}

		update := bson.M{
			"$set": bson.M{
				"value":      c.Value,
				"path":       c.Path,
				"http_only":  c.HTTPOnly,
				"secure":     c.Secure,
				"updated_at": now,
			},
			"$setOnInsert": bson.M{
				"created_at": now,
			},
		}

		// Handle expiration
		if c.Expires > 0 {
			expiresAt := time.Unix(c.Expires, 0)
			update["$set"].(bson.M)["expires_at"] = expiresAt
		}

		opts := options.Update().SetUpsert(true)
		_, err := s.collection.UpdateOne(ctx, filter, update, opts)
		if err != nil {
			return fmt.Errorf("save cookie %s: %w", c.Name, err)
		}
	}

	return nil
}

// Get retrieves cookies by domain
func (s *MongoCookieStorage) Get(ctx context.Context, userID, tenantID, domain string) ([]Cookie, error) {
	filter := bson.M{
		"user_id":   userID,
		"tenant_id": tenantID,
		"domain":    domain,
		"$or": []bson.M{
			{"expires_at": bson.M{"$exists": false}}, // No expiration
			{"expires_at": bson.M{"$gt": time.Now()}}, // Not expired
		},
	}

	cursor, err := s.collection.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("find cookies: %w", err)
	}
	defer cursor.Close(ctx)

	var stored []StoredCookie
	if err := cursor.All(ctx, &stored); err != nil {
		return nil, fmt.Errorf("decode cookies: %w", err)
	}

	cookies := make([]Cookie, len(stored))
	for i, sc := range stored {
		cookies[i] = Cookie{
			Name:     sc.Name,
			Value:    sc.Value,
			Domain:   sc.Domain,
			Path:     sc.Path,
			HTTPOnly: sc.HTTPOnly,
			Secure:   sc.Secure,
		}
		if sc.ExpiresAt != nil {
			cookies[i].Expires = sc.ExpiresAt.Unix()
		}
	}

	return cookies, nil
}

// GetAll retrieves all cookies for a user
func (s *MongoCookieStorage) GetAll(ctx context.Context, userID, tenantID string) ([]StoredCookie, error) {
	filter := bson.M{
		"user_id":   userID,
		"tenant_id": tenantID,
		"$or": []bson.M{
			{"expires_at": bson.M{"$exists": false}},
			{"expires_at": bson.M{"$gt": time.Now()}},
		},
	}

	cursor, err := s.collection.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("find cookies: %w", err)
	}
	defer cursor.Close(ctx)

	var cookies []StoredCookie
	if err := cursor.All(ctx, &cookies); err != nil {
		return nil, fmt.Errorf("decode cookies: %w", err)
	}

	return cookies, nil
}

// Delete deletes cookies by domain
func (s *MongoCookieStorage) Delete(ctx context.Context, userID, tenantID, domain string) error {
	filter := bson.M{
		"user_id":   userID,
		"tenant_id": tenantID,
		"domain":    domain,
	}

	_, err := s.collection.DeleteMany(ctx, filter)
	return err
}

// DeleteExpired deletes all expired cookies
func (s *MongoCookieStorage) DeleteExpired(ctx context.Context) error {
	filter := bson.M{
		"expires_at": bson.M{"$lt": time.Now()},
	}

	_, err := s.collection.DeleteMany(ctx, filter)
	return err
}

// ============================================================
// In-Memory Cookie Storage (for testing)
// ============================================================

// MemoryCookieStorage implements CookieStorage using in-memory storage
type MemoryCookieStorage struct {
	cookies map[string][]StoredCookie // key: userID:tenantID:domain
}

// NewMemoryCookieStorage creates a new in-memory cookie storage
func NewMemoryCookieStorage() *MemoryCookieStorage {
	return &MemoryCookieStorage{
		cookies: make(map[string][]StoredCookie),
	}
}

func (s *MemoryCookieStorage) key(userID, tenantID, domain string) string {
	return fmt.Sprintf("%s:%s:%s", userID, tenantID, domain)
}

// Save saves cookies for a user/domain
func (s *MemoryCookieStorage) Save(ctx context.Context, userID, tenantID string, cookies []Cookie) error {
	now := time.Now()

	for _, c := range cookies {
		key := s.key(userID, tenantID, c.Domain)

		var expiresAt *time.Time
		if c.Expires > 0 {
			t := time.Unix(c.Expires, 0)
			expiresAt = &t
		}

		stored := StoredCookie{
			ID:        primitive.NewObjectID(),
			UserID:    userID,
			TenantID:  tenantID,
			Domain:    c.Domain,
			Name:      c.Name,
			Value:     c.Value,
			Path:      c.Path,
			ExpiresAt: expiresAt,
			HTTPOnly:  c.HTTPOnly,
			Secure:    c.Secure,
			CreatedAt: now,
			UpdatedAt: now,
		}

		// Find and update or append
		found := false
		for i, existing := range s.cookies[key] {
			if existing.Name == c.Name {
				s.cookies[key][i] = stored
				found = true
				break
			}
		}
		if !found {
			s.cookies[key] = append(s.cookies[key], stored)
		}
	}

	return nil
}

// Get retrieves cookies by domain
func (s *MemoryCookieStorage) Get(ctx context.Context, userID, tenantID, domain string) ([]Cookie, error) {
	key := s.key(userID, tenantID, domain)
	stored := s.cookies[key]

	var cookies []Cookie
	now := time.Now()

	for _, sc := range stored {
		// Skip expired
		if sc.ExpiresAt != nil && sc.ExpiresAt.Before(now) {
			continue
		}

		c := Cookie{
			Name:     sc.Name,
			Value:    sc.Value,
			Domain:   sc.Domain,
			Path:     sc.Path,
			HTTPOnly: sc.HTTPOnly,
			Secure:   sc.Secure,
		}
		if sc.ExpiresAt != nil {
			c.Expires = sc.ExpiresAt.Unix()
		}
		cookies = append(cookies, c)
	}

	return cookies, nil
}

// GetAll retrieves all cookies for a user
func (s *MemoryCookieStorage) GetAll(ctx context.Context, userID, tenantID string) ([]StoredCookie, error) {
	var all []StoredCookie
	now := time.Now()

	for key, cookies := range s.cookies {
		parts := strings.SplitN(key, ":", 3)
		if len(parts) != 3 {
			continue
		}
		if parts[0] != userID || parts[1] != tenantID {
			continue
		}

		for _, sc := range cookies {
			// Skip expired
			if sc.ExpiresAt != nil && sc.ExpiresAt.Before(now) {
				continue
			}
			all = append(all, sc)
		}
	}

	return all, nil
}

// Delete deletes cookies by domain
func (s *MemoryCookieStorage) Delete(ctx context.Context, userID, tenantID, domain string) error {
	key := s.key(userID, tenantID, domain)
	delete(s.cookies, key)
	return nil
}

// DeleteExpired deletes all expired cookies
func (s *MemoryCookieStorage) DeleteExpired(ctx context.Context) error {
	// In-memory doesn't need explicit cleanup
	return nil
}
