package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// ContextStore is the interface for storing execution contexts
type ContextStore interface {
	// Save saves an execution context
	Save(ctx context.Context, execCtx *ExecutionContext) error

	// Get retrieves an execution context by ID
	Get(ctx context.Context, id string) (*ExecutionContext, error)

	// Delete deletes an execution context
	Delete(ctx context.Context, id string) error

	// ListBySession lists contexts for a session
	ListBySession(ctx context.Context, sessionID string) ([]*ExecutionContext, error)

	// ListRecent lists recent contexts
	ListRecent(ctx context.Context, limit int) ([]*ExecutionContext, error)

	// CleanupOld deletes contexts older than duration
	CleanupOld(ctx context.Context, olderThan time.Duration) error
}

// StoredContext is the database model for ExecutionContext
type StoredContext struct {
	ID           string     `gorm:"primaryKey"`
	SessionID    string     `gorm:"index"`
	TenantID     string     `gorm:"index"`
	UserID       string
	EntryAgent   string
	CurrentAgent string
	Status       string
	TotalTokens  int
	TotalCost    float64
	Error        string
	StepCount    int
	Data         string     `gorm:"type:text"` // JSON serialized ExecutionContext
	CreatedAt    time.Time  `gorm:"index"`
	UpdatedAt    time.Time
	CompletedAt  *time.Time
}

// TableName returns the table name
func (StoredContext) TableName() string {
	return "agent_execution_contexts"
}

// SQLiteStore implements ContextStore using SQLite
type SQLiteStore struct {
	db   *gorm.DB
	mu   sync.RWMutex
}

// NewSQLiteStore creates a new SQLite context store
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Auto migrate
	if err := db.AutoMigrate(&StoredContext{}); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &SQLiteStore{db: db}, nil
}

// Save saves an execution context
func (s *SQLiteStore) Save(ctx context.Context, execCtx *ExecutionContext) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Serialize context to JSON
	data, err := execCtx.ToJSON()
	if err != nil {
		return fmt.Errorf("serialize context: %w", err)
	}

	// Create stored context
	stored := &StoredContext{
		ID:           execCtx.ID,
		SessionID:    execCtx.SessionID,
		TenantID:     execCtx.TenantID,
		UserID:       execCtx.UserID,
		EntryAgent:   execCtx.EntryAgent,
		CurrentAgent: execCtx.CurrentAgent,
		Status:       string(execCtx.Status),
		TotalTokens:  execCtx.TotalTokens,
		TotalCost:    execCtx.TotalCost,
		Error:        execCtx.Error,
		StepCount:    execCtx.StepCount,
		Data:         string(data),
		CreatedAt:    execCtx.StartedAt,
		UpdatedAt:    time.Now(),
		CompletedAt:  execCtx.CompletedAt,
	}

	// Use transaction
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Check if exists
		var existing StoredContext
		if err := tx.Where("id = ?", execCtx.ID).First(&existing).Error; err == nil {
			// Update existing
			return tx.Model(&existing).Updates(stored).Error
		} else if err == gorm.ErrRecordNotFound {
			// Create new
			return tx.Create(stored).Error
		}
		return err
	})
}

// Get retrieves an execution context by ID
func (s *SQLiteStore) Get(ctx context.Context, id string) (*ExecutionContext, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var stored StoredContext
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&stored).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrContextNotFound
		}
		return nil, fmt.Errorf("query: %w", err)
	}

	// Deserialize context
	execCtx := &ExecutionContext{}
	if err := execCtx.FromJSON([]byte(stored.Data)); err != nil {
		return nil, fmt.Errorf("deserialize context: %w", err)
	}

	return execCtx, nil
}

// Delete deletes an execution context
func (s *SQLiteStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.db.WithContext(ctx).Where("id = ?", id).Delete(&StoredContext{}).Error
}

// ListBySession lists contexts for a session
func (s *SQLiteStore) ListBySession(ctx context.Context, sessionID string) ([]*ExecutionContext, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var stored []StoredContext
	if err := s.db.WithContext(ctx).
		Where("session_id = ?", sessionID).
		Order("created_at DESC").
		Find(&stored).Error; err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}

	contexts := make([]*ExecutionContext, 0, len(stored))
	for _, s := range stored {
		execCtx := &ExecutionContext{}
		if err := execCtx.FromJSON([]byte(s.Data)); err == nil {
			contexts = append(contexts, execCtx)
		}
	}

	return contexts, nil
}

// ListRecent lists recent contexts
func (s *SQLiteStore) ListRecent(ctx context.Context, limit int) ([]*ExecutionContext, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var stored []StoredContext
	if err := s.db.WithContext(ctx).
		Order("created_at DESC").
		Limit(limit).
		Find(&stored).Error; err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}

	contexts := make([]*ExecutionContext, 0, len(stored))
	for _, s := range stored {
		execCtx := &ExecutionContext{}
		if err := execCtx.FromJSON([]byte(s.Data)); err == nil {
			contexts = append(contexts, execCtx)
		}
	}

	return contexts, nil
}

// CleanupOld deletes contexts older than duration
func (s *SQLiteStore) CleanupOld(ctx context.Context, olderThan time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-olderThan)
	return s.db.WithContext(ctx).
		Where("created_at < ?", cutoff).
		Delete(&StoredContext{}).Error
}

// Close closes the database connection
func (s *SQLiteStore) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// MemoryStore is an in-memory context store (for testing)
type MemoryStore struct {
	mu        sync.RWMutex
	contexts  map[string]*ExecutionContext
}

// NewMemoryStore creates a new in-memory context store
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		contexts: make(map[string]*ExecutionContext),
	}
}

// Save saves an execution context
func (s *MemoryStore) Save(ctx context.Context, execCtx *ExecutionContext) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.contexts[execCtx.ID] = execCtx
	return nil
}

// Get retrieves an execution context by ID
func (s *MemoryStore) Get(ctx context.Context, id string) (*ExecutionContext, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	execCtx, ok := s.contexts[id]
	if !ok {
		return nil, ErrContextNotFound
	}

	return execCtx, nil
}

// Delete deletes an execution context
func (s *MemoryStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.contexts, id)
	return nil
}

// ListBySession lists contexts for a session
func (s *MemoryStore) ListBySession(ctx context.Context, sessionID string) ([]*ExecutionContext, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	contexts := make([]*ExecutionContext, 0)
	for _, execCtx := range s.contexts {
		if execCtx.SessionID == sessionID {
			contexts = append(contexts, execCtx)
		}
	}

	return contexts, nil
}

// ListRecent lists recent contexts
func (s *MemoryStore) ListRecent(ctx context.Context, limit int) ([]*ExecutionContext, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	contexts := make([]*ExecutionContext, 0, len(s.contexts))
	for _, execCtx := range s.contexts {
		contexts = append(contexts, execCtx)
	}

	if len(contexts) > limit {
		contexts = contexts[:limit]
	}

	return contexts, nil
}

// CleanupOld deletes contexts older than duration (not implemented for memory store)
func (s *MemoryStore) CleanupOld(ctx context.Context, olderThan time.Duration) error {
	return nil
}

// generateUUID generates a UUID string
func generateUUID() string {
	return uuid.New().String()
}