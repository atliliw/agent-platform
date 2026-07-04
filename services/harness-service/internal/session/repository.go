// Package session provides session replay functionality
package session

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Repository provides data access for session data
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new session repository
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// AutoMigrate migrates session tables
func (r *Repository) AutoMigrate() error {
	return r.db.AutoMigrate(
		&Session{},
		&SessionStep{},
		&ReplaySession{},
	)
}

// CreateSession creates a new session
func (r *Repository) CreateSession(ctx context.Context, session *Session) error {
	if session.ID == "" {
		session.ID = uuid.New().String()
	}
	session.CreatedAt = time.Now()
	session.StartTime = time.Now()
	session.Status = SessionStatusRunning

	// Encode metadata to JSON
	if session.Metadata != nil {
		metaJSON, err := json.Marshal(session.Metadata)
		if err != nil {
			return fmt.Errorf("marshal metadata: %w", err)
		}
		session.MetadataJSON = string(metaJSON)
	}

	return r.db.WithContext(ctx).Create(session).Error
}

// GetSession retrieves a session by ID
func (r *Repository) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	var session Session
	if err := r.db.WithContext(ctx).Where("id = ?", sessionID).First(&session).Error; err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}

	// Decode metadata from JSON
	if session.MetadataJSON != "" {
		if err := json.Unmarshal([]byte(session.MetadataJSON), &session.Metadata); err != nil {
			// Log but don't fail - metadata is optional
			session.Metadata = make(map[string]string)
		}
	}

	return &session, nil
}

// UpdateSession updates a session
func (r *Repository) UpdateSession(ctx context.Context, session *Session) error {
	// Encode metadata to JSON
	if session.Metadata != nil {
		metaJSON, err := json.Marshal(session.Metadata)
		if err != nil {
			return fmt.Errorf("marshal metadata: %w", err)
		}
		session.MetadataJSON = string(metaJSON)
	}

	return r.db.WithContext(ctx).Save(session).Error
}

// EndSession ends a session with a status
func (r *Repository) EndSession(ctx context.Context, sessionID string, status SessionStatus) (*Session, error) {
	session, err := r.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	session.EndTime = &now
	session.Status = status
	session.Duration = now.Sub(session.StartTime).Milliseconds()

	// Calculate totals from steps
	steps, err := r.ListSteps(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("list steps: %w", err)
	}

	var totalTokens int64
	var totalCost float64
	for _, step := range steps {
		// Parse metadata to extract tokens and cost if present
		if step.Metadata != "" {
			var meta map[string]interface{}
			if err := json.Unmarshal([]byte(step.Metadata), &meta); err == nil {
				if tokens, ok := meta["tokens"].(float64); ok {
					totalTokens += int64(tokens)
				}
				if cost, ok := meta["cost"].(float64); ok {
					totalCost += cost
				}
			}
		}
	}
	session.TotalTokens = totalTokens
	session.TotalCost = totalCost

	if err := r.UpdateSession(ctx, session); err != nil {
		return nil, fmt.Errorf("update session: %w", err)
	}

	return session, nil
}

// ListSessions lists sessions with filters
func (r *Repository) ListSessions(ctx context.Context, filter *ListSessionsFilter) ([]*Session, int64, error) {
	query := r.db.WithContext(ctx).Model(&Session{})

	if filter.AgentID != "" {
		query = query.Where("agent_id = ?", filter.AgentID)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.StartTime > 0 {
		query = query.Where("start_time >= ?", time.Unix(filter.StartTime, 0))
	}
	if filter.EndTime > 0 {
		query = query.Where("start_time <= ?", time.Unix(filter.EndTime, 0))
	}
	if filter.TenantID != "" {
		query = query.Where("tenant_id = ?", filter.TenantID)
	}

	// Get total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count sessions: %w", err)
	}

	// Apply pagination
	page := filter.Page
	if page <= 0 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	offset := (page - 1) * pageSize
	query = query.Offset(int(offset)).Limit(int(pageSize)).Order("created_at DESC")

	var sessions []*Session
	if err := query.Find(&sessions).Error; err != nil {
		return nil, 0, fmt.Errorf("find sessions: %w", err)
	}

	// Decode metadata for each session
	for _, s := range sessions {
		if s.MetadataJSON != "" {
			if err := json.Unmarshal([]byte(s.MetadataJSON), &s.Metadata); err != nil {
				s.Metadata = make(map[string]string)
			}
		}
	}

	return sessions, total, nil
}

// CreateStep creates a new session step
func (r *Repository) CreateStep(ctx context.Context, step *SessionStep) error {
	if step.ID == "" {
		step.ID = uuid.New().String()
	}
	step.Timestamp = time.Now()
	step.Status = StepStatusSuccess

	// Get the next step number for this session
	var maxStepNumber int32
	r.db.WithContext(ctx).Model(&SessionStep{}).
		Where("session_id = ?", step.SessionID).
		Select("COALESCE(MAX(step_number), 0)").
		Scan(&maxStepNumber)
	step.StepNumber = maxStepNumber + 1

	return r.db.WithContext(ctx).Create(step).Error
}

// GetStep retrieves a step by ID
func (r *Repository) GetStep(ctx context.Context, stepID string) (*SessionStep, error) {
	var step SessionStep
	if err := r.db.WithContext(ctx).Where("id = ?", stepID).First(&step).Error; err != nil {
		return nil, fmt.Errorf("get step: %w", err)
	}
	return &step, nil
}

// ListSteps lists all steps for a session
func (r *Repository) ListSteps(ctx context.Context, sessionID string) ([]SessionStep, error) {
	var steps []SessionStep
	if err := r.db.WithContext(ctx).
		Where("session_id = ?", sessionID).
		Order("step_number ASC").
		Find(&steps).Error; err != nil {
		return nil, fmt.Errorf("list steps: %w", err)
	}
	return steps, nil
}

// ListStepsByParent lists all steps with a given parent
func (r *Repository) ListStepsByParent(ctx context.Context, parentStepID string) ([]SessionStep, error) {
	var steps []SessionStep
	if err := r.db.WithContext(ctx).
		Where("parent_step_id = ?", parentStepID).
		Order("step_number ASC").
		Find(&steps).Error; err != nil {
		return nil, fmt.Errorf("list steps by parent: %w", err)
	}
	return steps, nil
}

// DeleteSession deletes a session and all its steps
func (r *Repository) DeleteSession(ctx context.Context, sessionID string) error {
	// Delete steps first
	if err := r.db.WithContext(ctx).Where("session_id = ?", sessionID).Delete(&SessionStep{}).Error; err != nil {
		return fmt.Errorf("delete steps: %w", err)
	}

	// Delete session
	if err := r.db.WithContext(ctx).Where("id = ?", sessionID).Delete(&Session{}).Error; err != nil {
		return fmt.Errorf("delete session: %w", err)
	}

	return nil
}

// CreateReplaySession creates a replay session record
func (r *Repository) CreateReplaySession(ctx context.Context, replay *ReplaySession) error {
	if replay.ID == "" {
		replay.ID = uuid.New().String()
	}
	replay.CreatedAt = time.Now()

	// Encode diffs to JSON
	if replay.Diffs != nil {
		diffsJSON, err := json.Marshal(replay.Diffs)
		if err != nil {
			return fmt.Errorf("marshal diffs: %w", err)
		}
		replay.DiffsJSON = string(diffsJSON)
	}

	return r.db.WithContext(ctx).Create(replay).Error
}

// UpdateReplaySession updates a replay session
func (r *Repository) UpdateReplaySession(ctx context.Context, replay *ReplaySession) error {
	// Encode diffs to JSON
	if replay.Diffs != nil {
		diffsJSON, err := json.Marshal(replay.Diffs)
		if err != nil {
			return fmt.Errorf("marshal diffs: %w", err)
		}
		replay.DiffsJSON = string(diffsJSON)
	}

	return r.db.WithContext(ctx).Save(replay).Error
}

// GetReplaySession retrieves a replay session by ID
func (r *Repository) GetReplaySession(ctx context.Context, replayID string) (*ReplaySession, error) {
	var replay ReplaySession
	if err := r.db.WithContext(ctx).Where("id = ?", replayID).First(&replay).Error; err != nil {
		return nil, fmt.Errorf("get replay session: %w", err)
	}

	// Decode diffs from JSON
	if replay.DiffsJSON != "" {
		if err := json.Unmarshal([]byte(replay.DiffsJSON), &replay.Diffs); err != nil {
			replay.Diffs = []ReplayDiff{}
		}
	}

	return &replay, nil
}

// GetSessionStats returns statistics for sessions
func (r *Repository) GetSessionStats(ctx context.Context, agentID string) (*SessionStats, error) {
	stats := &SessionStats{}

	query := r.db.WithContext(ctx).Model(&Session{}).Where("agent_id = ?", agentID)

	// Count sessions
	if err := query.Count(&stats.TotalSessions).Error; err != nil {
		return nil, fmt.Errorf("count sessions: %w", err)
	}

	// Count completed
	var completedSessions int64
	if err := query.Where("status = ?", SessionStatusCompleted).Count(&completedSessions).Error; err != nil {
		return nil, fmt.Errorf("count completed: %w", err)
	}
	stats.CompletedSessions = completedSessions

	// Count failed
	var failedSessions int64
	if err := query.Where("status = ?", SessionStatusFailed).Count(&failedSessions).Error; err != nil {
		return nil, fmt.Errorf("count failed: %w", err)
	}
	stats.FailedSessions = failedSessions

	// Average duration
	var avgDuration float64
	r.db.WithContext(ctx).Model(&Session{}).
		Where("agent_id = ? AND status = ?", agentID, SessionStatusCompleted).
		Select("AVG(duration)").
		Scan(&avgDuration)
	stats.AvgDuration = int64(avgDuration)

	// Total tokens and cost
	r.db.WithContext(ctx).Model(&Session{}).
		Where("agent_id = ?", agentID).
		Select("SUM(total_tokens), SUM(total_cost)").
		Scan(&stats.TotalTokens)
	r.db.WithContext(ctx).Model(&Session{}).
		Where("agent_id = ?", agentID).
		Select("SUM(total_cost)").
		Scan(&stats.TotalCost)

	return stats, nil
}

// SessionStats represents session statistics
type SessionStats struct {
	TotalSessions     int64   `json:"total_sessions"`
	CompletedSessions int64   `json:"completed_sessions"`
	FailedSessions    int64   `json:"failed_sessions"`
	AvgDuration       int64   `json:"avg_duration"` // milliseconds
	TotalTokens       int64   `json:"total_tokens"`
	TotalCost         float64 `json:"total_cost"`
}