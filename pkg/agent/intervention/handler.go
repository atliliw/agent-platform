// Package intervention provides real-time intervention capabilities
package intervention

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ============================================================
// Intervention Types
// ============================================================

// InterventionType defines the type of intervention
type InterventionType string

const (
	InterventionPause      InterventionType = "pause"       // Pause execution
	InterventionResume     InterventionType = "resume"      // Resume execution
	InterventionStop       InterventionType = "stop"        // Stop execution
	InterventionModify     InterventionType = "modify"      // Modify parameters
	InterventionInject     InterventionType = "inject"      // Inject message
	InterventionOverride   InterventionType = "override"    // Override decision
	InterventionRedirect   InterventionType = "redirect"    // Redirect to different path
	InterventionFeedback   InterventionType = "feedback"    // Provide feedback
)

// InterventionStatus defines the status of an intervention
type InterventionStatus string

const (
	InterventionPending   InterventionStatus = "pending"
	InterventionApplied   InterventionStatus = "applied"
	InterventionRejected  InterventionStatus = "rejected"
	InterventionExpired   InterventionStatus = "expired"
)

// ============================================================
// Intervention Request
// ============================================================

// InterventionRequest represents an intervention request
type InterventionRequest struct {
	ID            string                 `json:"id"`
	Type          InterventionType       `json:"type"`
	AgentID       string                 `json:"agent_id"`
	SessionID     string                 `json:"session_id"`
	UserID        string                 `json:"user_id"`
	Reason        string                 `json:"reason"`
	TargetStep    string                 `json:"target_step"`    // Target step to intervene
	NewParameters map[string]interface{} `json:"new_parameters"` // Parameters to modify
	Message       string                 `json:"message"`        // Message to inject
	ExecuteAt     time.Time              `json:"execute_at"`     // When to execute
	Status        InterventionStatus     `json:"status"`
	CreatedAt     time.Time              `json:"created_at"`
	AppliedAt     time.Time              `json:"applied_at,omitempty"`
	AppliedBy     string                 `json:"applied_by,omitempty"`
	Result        string                 `json:"result,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// ============================================================
// Session State
// ============================================================

// SessionState represents the state of an agent session
type SessionState struct {
	SessionID     string                 `json:"session_id"`
	AgentID       string                 `json:"agent_id"`
	Status        string                 `json:"status"` // running, paused, stopped, completed
	CurrentStep   int                    `json:"current_step"`
	TotalSteps    int                    `json:"total_steps"`
	StartTime     time.Time              `json:"start_time"`
	PauseTime     time.Time              `json:"pause_time,omitempty"`
	ResumeTime    time.Time              `json:"resume_time,omitempty"`
	Variables     map[string]interface{} `json:"variables"`
	ExecutionLog  []ExecutionEntry       `json:"execution_log"`
	LastUpdate    time.Time              `json:"last_update"`
}

// ExecutionEntry represents an entry in the execution log
type ExecutionEntry struct {
	StepNum     int                    `json:"step_num"`
	Action      string                 `json:"action"`
	Input       map[string]interface{} `json:"input"`
	Output      interface{}            `json:"output"`
	Duration    int64                  `json:"duration_ms"`
	Success     bool                   `json:"success"`
	Error       string                 `json:"error,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
}

// ============================================================
// Intervention Manager
// ============================================================

// InterventionManager manages interventions
type InterventionManager struct {
	requests      map[string]*InterventionRequest
	sessions      map[string]*SessionState
	pending       map[string][]string // sessionID -> pending intervention IDs
	eventChannels map[string]chan InterventionEvent
	mu            sync.RWMutex
	hooks         InterventionHooks
}

// InterventionEvent represents an intervention event
type InterventionEvent struct {
	Type      InterventionType   `json:"type"`
	SessionID string             `json:"session_id"`
	RequestID string             `json:"request_id"`
	Timestamp time.Time          `json:"timestamp"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

// InterventionHooks provides hooks for intervention events
type InterventionHooks interface {
	OnInterventionRequested(request *InterventionRequest)
	OnInterventionApplied(request *InterventionRequest)
	OnSessionPaused(sessionID string)
	OnSessionResumed(sessionID string)
	OnSessionStopped(sessionID string)
}

// NewInterventionManager creates a new intervention manager
func NewInterventionManager() *InterventionManager {
	return &InterventionManager{
		requests:      make(map[string]*InterventionRequest),
		sessions:      make(map[string]*SessionState),
		pending:       make(map[string][]string),
		eventChannels: make(map[string]chan InterventionEvent),
	}
}

// SetHooks sets intervention hooks
func (m *InterventionManager) SetHooks(hooks InterventionHooks) {
	m.hooks = hooks
}

// RegisterSession registers a new session
func (m *InterventionManager) RegisterSession(sessionID, agentID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.sessions[sessionID] = &SessionState{
		SessionID:    sessionID,
		AgentID:      agentID,
		Status:       "running",
		Variables:    make(map[string]interface{}),
		ExecutionLog: make([]ExecutionEntry, 0),
		StartTime:    time.Now(),
		LastUpdate:   time.Now(),
	}

	// Create event channel
	m.eventChannels[sessionID] = make(chan InterventionEvent, 100)
}

// UnregisterSession unregisters a session
func (m *InterventionManager) UnregisterSession(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.sessions, sessionID)
	delete(m.pending, sessionID)

	if ch, ok := m.eventChannels[sessionID]; ok {
		close(ch)
		delete(m.eventChannels, sessionID)
	}
}

// GetSessionState gets the state of a session
func (m *InterventionManager) GetSessionState(sessionID string) (*SessionState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state, ok := m.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}
	return state, nil
}

// UpdateSessionState updates session state
func (m *InterventionManager) UpdateSessionState(sessionID string, update func(*SessionState)) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, ok := m.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	update(state)
	state.LastUpdate = time.Now()

	return nil
}

// LogExecution logs an execution entry
func (m *InterventionManager) LogExecution(sessionID string, entry ExecutionEntry) error {
	return m.UpdateSessionState(sessionID, func(state *SessionState) {
		entry.Timestamp = time.Now()
		state.ExecutionLog = append(state.ExecutionLog, entry)
		state.CurrentStep++
	})
}

// RequestIntervention requests an intervention
func (m *InterventionManager) RequestIntervention(ctx context.Context, req *InterventionRequest) (*InterventionRequest, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Validate session exists
	if _, ok := m.sessions[req.SessionID]; !ok {
		return nil, fmt.Errorf("session not found: %s", req.SessionID)
	}

	// Generate ID if not set
	if req.ID == "" {
		req.ID = generateInterventionID()
	}

	req.CreatedAt = time.Now()
	req.Status = InterventionPending

	// Store request
	m.requests[req.ID] = req
	m.pending[req.SessionID] = append(m.pending[req.SessionID], req.ID)

	// Send event
	if ch, ok := m.eventChannels[req.SessionID]; ok {
		ch <- InterventionEvent{
			Type:      req.Type,
			SessionID: req.SessionID,
			RequestID: req.ID,
			Timestamp: time.Now(),
		}
	}

	// Notify hooks
	if m.hooks != nil {
		m.hooks.OnInterventionRequested(req)
	}

	return req, nil
}

// PauseSession pauses a session
func (m *InterventionManager) PauseSession(ctx context.Context, sessionID, userID, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, ok := m.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	if state.Status != "running" {
		return fmt.Errorf("session is not running: %s", state.Status)
	}

	state.Status = "paused"
	state.PauseTime = time.Now()
	state.LastUpdate = time.Now()

	// Send event
	if ch, ok := m.eventChannels[sessionID]; ok {
		ch <- InterventionEvent{
			Type:      InterventionPause,
			SessionID: sessionID,
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"reason": reason,
				"user_id": userID,
			},
		}
	}

	if m.hooks != nil {
		m.hooks.OnSessionPaused(sessionID)
	}

	return nil
}

// ResumeSession resumes a paused session
func (m *InterventionManager) ResumeSession(ctx context.Context, sessionID, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, ok := m.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	if state.Status != "paused" {
		return fmt.Errorf("session is not paused: %s", state.Status)
	}

	state.Status = "running"
	state.ResumeTime = time.Now()
	state.LastUpdate = time.Now()

	// Send event
	if ch, ok := m.eventChannels[sessionID]; ok {
		ch <- InterventionEvent{
			Type:      InterventionResume,
			SessionID: sessionID,
			Timestamp: time.Now(),
		}
	}

	if m.hooks != nil {
		m.hooks.OnSessionResumed(sessionID)
	}

	return nil
}

// StopSession stops a session
func (m *InterventionManager) StopSession(ctx context.Context, sessionID, userID, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, ok := m.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	state.Status = "stopped"
	state.LastUpdate = time.Now()

	// Send event
	if ch, ok := m.eventChannels[sessionID]; ok {
		ch <- InterventionEvent{
			Type:      InterventionStop,
			SessionID: sessionID,
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"reason": reason,
				"user_id": userID,
			},
		}
	}

	if m.hooks != nil {
		m.hooks.OnSessionStopped(sessionID)
	}

	return nil
}

// ModifyParameters modifies session parameters
func (m *InterventionManager) ModifyParameters(ctx context.Context, sessionID string, params map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, ok := m.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	for k, v := range params {
		state.Variables[k] = v
	}
	state.LastUpdate = time.Now()

	// Send event
	if ch, ok := m.eventChannels[sessionID]; ok {
		ch <- InterventionEvent{
			Type:      InterventionModify,
			SessionID: sessionID,
			Timestamp: time.Now(),
			Data:      params,
		}
	}

	return nil
}

// InjectMessage injects a message into the session
func (m *InterventionManager) InjectMessage(ctx context.Context, sessionID, message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, ok := m.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// Send event
	if ch, ok := m.eventChannels[sessionID]; ok {
		ch <- InterventionEvent{
			Type:      InterventionInject,
			SessionID: sessionID,
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"message": message,
			},
		}
	}

	return nil
}

// GetPendingInterventions gets pending interventions for a session
func (m *InterventionManager) GetPendingInterventions(sessionID string) []*InterventionRequest {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var requests []*InterventionRequest
	for _, id := range m.pending[sessionID] {
		if req, ok := m.requests[id]; ok && req.Status == InterventionPending {
			requests = append(requests, req)
		}
	}
	return requests
}

// MarkInterventionApplied marks an intervention as applied
func (m *InterventionManager) MarkInterventionApplied(requestID, userID, result string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	req, ok := m.requests[requestID]
	if !ok {
		return fmt.Errorf("intervention request not found: %s", requestID)
	}

	req.Status = InterventionApplied
	req.AppliedAt = time.Now()
	req.AppliedBy = userID
	req.Result = result

	// Remove from pending
	m.removeFromPending(req.SessionID, requestID)

	if m.hooks != nil {
		m.hooks.OnInterventionApplied(req)
	}

	return nil
}

// removeFromPending removes from pending list
func (m *InterventionManager) removeFromPending(sessionID, requestID string) {
	pending := m.pending[sessionID]
	for i, id := range pending {
		if id == requestID {
			m.pending[sessionID] = append(pending[:i], pending[i+1:]...)
			break
		}
	}
}

// WaitForEvent waits for an intervention event
func (m *InterventionManager) WaitForEvent(ctx context.Context, sessionID string) (*InterventionEvent, error) {
	m.mu.RLock()
	ch, ok := m.eventChannels[sessionID]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	select {
	case event := <-ch:
		return &event, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// WatchSession creates a channel to watch session events
func (m *InterventionManager) WatchSession(sessionID string) (<-chan InterventionEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.sessions[sessionID]; !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	ch, ok := m.eventChannels[sessionID]
	if !ok {
		ch = make(chan InterventionEvent, 100)
		m.eventChannels[sessionID] = ch
	}

	return ch, nil
}

// ListActiveSessions lists all active sessions
func (m *InterventionManager) ListActiveSessions() []*SessionState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var sessions []*SessionState
	for _, state := range m.sessions {
		if state.Status == "running" || state.Status == "paused" {
			sessions = append(sessions, state)
		}
	}
	return sessions
}

// ============================================================
// Feedback Collector
// ============================================================

// Feedback represents user feedback
type Feedback struct {
	ID           string                 `json:"id"`
	SessionID    string                 `json:"session_id"`
	StepNum      int                    `json:"step_num"`
	UserID       string                 `json:"user_id"`
	Rating       int                    `json:"rating"` // 1-5
	Comment      string                 `json:"comment"`
	Category     string                 `json:"category"` // helpful, incorrect, confusing, other
	IsPositive   bool                   `json:"is_positive"`
	CreatedAt    time.Time              `json:"created_at"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// FeedbackCollector collects and manages feedback
type FeedbackCollector struct {
	feedback map[string][]*Feedback // sessionID -> feedback
	byUser   map[string][]*Feedback // userID -> feedback
	mu       sync.RWMutex
}

// NewFeedbackCollector creates a new feedback collector
func NewFeedbackCollector() *FeedbackCollector {
	return &FeedbackCollector{
		feedback: make(map[string][]*Feedback),
		byUser:   make(map[string][]*Feedback),
	}
}

// SubmitFeedback submits feedback
func (c *FeedbackCollector) SubmitFeedback(ctx context.Context, fb *Feedback) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if fb.ID == "" {
		fb.ID = generateFeedbackID()
	}
	fb.CreatedAt = time.Now()
	fb.IsPositive = fb.Rating >= 4

	c.feedback[fb.SessionID] = append(c.feedback[fb.SessionID], fb)
	c.byUser[fb.UserID] = append(c.byUser[fb.UserID], fb)

	return nil
}

// GetSessionFeedback gets feedback for a session
func (c *FeedbackCollector) GetSessionFeedback(sessionID string) []*Feedback {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.feedback[sessionID]
}

// GetUserFeedback gets feedback by a user
func (c *FeedbackCollector) GetUserFeedback(userID string) []*Feedback {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.byUser[userID]
}

// GetStats gets feedback statistics
func (c *FeedbackCollector) GetStats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var total, positive, negative int
	var totalRating float64
	categoryCounts := make(map[string]int)

	for _, feedbacks := range c.feedback {
		for _, fb := range feedbacks {
			total++
			totalRating += float64(fb.Rating)
			if fb.IsPositive {
				positive++
			} else {
				negative++
			}
			categoryCounts[fb.Category]++
		}
	}

	avgRating := 0.0
	if total > 0 {
		avgRating = totalRating / float64(total)
	}

	return map[string]interface{}{
		"total_feedback":   total,
		"positive_count":   positive,
		"negative_count":   negative,
		"average_rating":   avgRating,
		"category_counts":  categoryCounts,
	}
}

// ============================================================
// Helper Functions
// ============================================================

func generateInterventionID() string {
	return fmt.Sprintf("intervention-%d", time.Now().UnixNano())
}

func generateFeedbackID() string {
	return fmt.Sprintf("feedback-%d", time.Now().UnixNano())
}