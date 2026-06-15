// Package repository provides data access for chat service
package repository

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// Session represents a chat session
type Session struct {
	ID        string     `gorm:"primaryKey"`
	TenantID  string     `gorm:"index"`
	UserID    string     `gorm:"index"`
	Title     string
	Messages  []*Message `gorm:"foreignKey:SessionID"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Message represents a chat message
type Message struct {
	ID         string                  `gorm:"primaryKey"`
	SessionID  string                  `gorm:"index"`
	Role       string
	Content    string
	AgentTrace []AgentState            `gorm:"serializer:json"` // Agent execution trace
	ToolCalls  []ToolCall              `gorm:"serializer:json"` // Tool calls made
	CreatedAt  time.Time
}

// AgentState represents a step in agent execution
type AgentState struct {
	Thought   string                 `json:"thought"`
	Action    string                 `json:"action"`
	Arguments map[string]interface{} `json:"arguments"`
	Result    string                 `json:"result"`
	Step      int                    `json:"step"`
}

// ToolCall represents a tool call
type ToolCall struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
	Result    string                 `json:"result"`
	Status    string                 `json:"status"` // pending, running, completed, error
	CreatedAt time.Time              `json:"created_at"`
}

// SessionRepository manages sessions
type SessionRepository struct {
	db   *gorm.DB
	mu   sync.RWMutex
}

// NewSessionRepository creates a new session repository
func NewSessionRepository(dbPath string) (*SessionRepository, error) {
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Auto migrate
	if err := db.AutoMigrate(&Session{}, &Message{}); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &SessionRepository{db: db}, nil
}

// Save saves a session
func (r *SessionRepository) Save(ctx context.Context, session *Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if session.ID == "" {
		session.ID = uuid.New().String()
		session.CreatedAt = time.Now()
	}
	session.UpdatedAt = time.Now()

	// ★ 清理无效 UTF-8 字符，防止 gRPC 序列化错误
	session.Title = strings.ToValidUTF8(session.Title, "")

	// Save session
	if err := r.db.WithContext(ctx).Save(session).Error; err != nil {
		return err
	}

	// Save messages
	for _, msg := range session.Messages {
		if msg.ID == "" {
			msg.ID = uuid.New().String()
			msg.SessionID = session.ID
			msg.CreatedAt = time.Now()
		}
		// ★ 清理无效 UTF-8 字符
		msg.Content = strings.ToValidUTF8(msg.Content, "")
		if err := r.db.WithContext(ctx).Save(msg).Error; err != nil {
			return err
		}
	}

	return nil
}

// Get gets a session by ID
func (r *SessionRepository) Get(ctx context.Context, id, tenantID string) (*Session, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var session Session
	if err := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Preload("Messages", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC")
		}).
		First(&session).Error; err != nil {
		return nil, err
	}

	return &session, nil
}

// List lists sessions
func (r *SessionRepository) List(ctx context.Context, tenantID, userID string, page, pageSize int) ([]*Session, int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var sessions []*Session
	var total int64

	query := r.db.WithContext(ctx).Model(&Session{}).Where("tenant_id = ?", tenantID)
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Order("updated_at DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&sessions).Error; err != nil {
		return nil, 0, err
	}

	return sessions, total, nil
}

// Delete deletes a session
func (r *SessionRepository) Delete(ctx context.Context, id, tenantID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Delete messages first
	if err := r.db.WithContext(ctx).
		Where("session_id IN (SELECT id FROM sessions WHERE id = ? AND tenant_id = ?)", id, tenantID).
		Delete(&Message{}).Error; err != nil {
		return err
	}

	// Delete session
	return r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Delete(&Session{}).Error
}

// Close closes the database connection
func (r *SessionRepository) Close() error {
	sqlDB, err := r.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}