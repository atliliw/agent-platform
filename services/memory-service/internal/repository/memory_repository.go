// Package repository provides data access for memory service
package repository

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"agent-platform/pkg/qdrant"
)

// Memory represents a memory entry
type Memory struct {
	ID         string    `gorm:"primaryKey"`
	SessionID  string    `gorm:"index"`
	AgentID    string    `gorm:"index"`
	Type       string
	Content    string
	Importance float64
	Vector     []float64 `gorm:"-"`
	TenantID   string    `gorm:"index"`
	CreatedAt  time.Time
}

// MemoryRepository manages memories
type MemoryRepository struct {
	db     *gorm.DB
	qdrant *qdrant.Client
	mu     sync.RWMutex
}

// NewMemoryRepository creates a new memory repository
func NewMemoryRepository(dbPath string) (*MemoryRepository, error) {
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := db.AutoMigrate(&Memory{}); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &MemoryRepository{db: db}, nil
}

// SetQdrant sets the qdrant client
func (r *MemoryRepository) SetQdrant(client *qdrant.Client) {
	r.qdrant = client
}

// Save saves a memory
func (r *MemoryRepository) Save(ctx context.Context, memory *Memory) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if memory.ID == "" {
		memory.ID = uuid.New().String()
	}
	memory.CreatedAt = time.Now()

	// Save to SQLite
	if err := r.db.WithContext(ctx).Create(memory).Error; err != nil {
		return err
	}

	// Save to Qdrant
	if r.qdrant != nil && memory.Vector != nil {
		point := qdrant.Point{
			ID:     memory.ID,
			Vector: memory.Vector,
			Payload: map[string]interface{}{
				"session_id":  memory.SessionID,
				"agent_id":    memory.AgentID,
				"type":        memory.Type,
				"content":     memory.Content,
				"importance":  memory.Importance,
				"tenant_id":   memory.TenantID,
				"created_at":  memory.CreatedAt.Unix(),
			},
		}
		if err := r.qdrant.Upsert(ctx, []qdrant.Point{point}); err != nil {
			// Log the error but don't fail - vector search will be disabled
			// but memory is still saved to SQLite
			fmt.Printf("Warning: failed to save to Qdrant: %v\n", err)
		}
	} else if r.qdrant == nil {
		fmt.Println("Warning: Qdrant client is nil, skipping vector storage")
	} else if memory.Vector == nil {
		fmt.Println("Warning: Vector is nil, skipping vector storage")
	}

	return nil
}

// GetBySession gets memories by session
func (r *MemoryRepository) GetBySession(ctx context.Context, sessionID, tenantID string) ([]*Memory, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var memories []*Memory
	if err := r.db.WithContext(ctx).
		Where("session_id = ? AND tenant_id = ?", sessionID, tenantID).
		Order("created_at DESC").
		Find(&memories).Error; err != nil {
		return nil, err
	}

	return memories, nil
}

// Search searches memories by vector similarity
func (r *MemoryRepository) Search(ctx context.Context, vector []float64, sessionID, agentID, tenantID string, topK int) ([]*Memory, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.qdrant == nil {
		return nil, fmt.Errorf("qdrant not configured")
	}

	// Build filter conditions
	var mustConditions []interface{}
	if tenantID != "" {
		mustConditions = append(mustConditions, map[string]interface{}{
			"key":   "tenant_id",
			"match": map[string]interface{}{"value": tenantID},
		})
	}
	if sessionID != "" {
		mustConditions = append(mustConditions, map[string]interface{}{
			"key":   "session_id",
			"match": map[string]interface{}{"value": sessionID},
		})
	}
	if agentID != "" {
		mustConditions = append(mustConditions, map[string]interface{}{
			"key":   "agent_id",
			"match": map[string]interface{}{"value": agentID},
		})
	}

	req := &qdrant.SearchRequest{
		Vector:      vector,
		Limit:       topK,
		WithPayload: true,
	}

	if len(mustConditions) > 0 {
		req.Filter = map[string]interface{}{
			"must": mustConditions,
		}
	}

	results, err := r.qdrant.Search(ctx, req)
	if err != nil {
		return nil, err
	}

	var memories []*Memory
	for _, r := range results {
		memories = append(memories, &Memory{
			ID:         r.ID,
			SessionID:  getStringPayload(r.Payload, "session_id"),
			AgentID:    getStringPayload(r.Payload, "agent_id"),
			Type:       getStringPayload(r.Payload, "type"),
			Content:    getStringPayload(r.Payload, "content"),
			Importance: getFloatPayload(r.Payload, "importance"),
			CreatedAt:  time.Unix(getInt64Payload(r.Payload, "created_at"), 0),
		})
	}

	return memories, nil
}

// DeleteBySession deletes memories by session
func (r *MemoryRepository) DeleteBySession(ctx context.Context, sessionID, tenantID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Get memory IDs
	var ids []string
	if err := r.db.WithContext(ctx).
		Model(&Memory{}).
		Where("session_id = ? AND tenant_id = ?", sessionID, tenantID).
		Pluck("id", &ids).Error; err != nil {
		return err
	}

	// Delete from Qdrant
	if r.qdrant != nil && len(ids) > 0 {
		if err := r.qdrant.Delete(ctx, ids); err != nil {
			return fmt.Errorf("delete from qdrant: %w", err)
		}
	}

	// Delete from SQLite
	return r.db.WithContext(ctx).
		Where("session_id = ? AND tenant_id = ?", sessionID, tenantID).
		Delete(&Memory{}).Error
}

// GetAll gets all memories for a tenant
func (r *MemoryRepository) GetAll(ctx context.Context, tenantID string) ([]*Memory, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var memories []*Memory
	query := r.db.WithContext(ctx).Order("created_at DESC")
	if tenantID != "" {
		query = query.Where("tenant_id = ?", tenantID)
	}
	if err := query.Find(&memories).Error; err != nil {
		return nil, err
	}
	return memories, nil
}

// Delete deletes a memory by ID
func (r *MemoryRepository) Delete(ctx context.Context, id, tenantID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Delete from Qdrant
	if r.qdrant != nil {
		if err := r.qdrant.Delete(ctx, []string{id}); err != nil {
			// Log but don't fail
		}
	}

	// Delete from SQLite
	query := r.db.WithContext(ctx)
	if tenantID != "" {
		query = query.Where("tenant_id = ?", tenantID)
	}
	return query.Delete(&Memory{}, id).Error
}

// Close closes the database connection
func (r *MemoryRepository) Close() error {
	sqlDB, err := r.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func getStringPayload(payload map[string]interface{}, key string) string {
	if v, ok := payload[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getFloatPayload(payload map[string]interface{}, key string) float64 {
	if v, ok := payload[key]; ok {
		switch val := v.(type) {
		case float64:
			return val
		case float32:
			return float64(val)
		}
	}
	return 0
}

func getInt64Payload(payload map[string]interface{}, key string) int64 {
	if v, ok := payload[key]; ok {
		switch val := v.(type) {
		case int64:
			return val
		case float64:
			return int64(val)
		}
	}
	return 0
}