// Package approval provides approval flow functionality for human-in-the-loop
package approval

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ============================================================
// Approval Types
// ============================================================

// ApprovalType defines the type of approval needed
type ApprovalType string

const (
	ApprovalTypeToolCall    ApprovalType = "tool_call"    // Approve tool execution
	ApprovalTypeDataAccess  ApprovalType = "data_access"  // Approve data access
	ApprovalTypeExternalAPI ApprovalType = "external_api" // Approve external API call
	ApprovalTypeFileWrite   ApprovalType = "file_write"   // Approve file write
	ApprovalTypeEmailSend   ApprovalType = "email_send"   // Approve email sending
	ApprovalTypePayment     ApprovalType = "payment"      // Approve payment
	ApprovalTypePublish     ApprovalType = "publish"      // Approve publishing
	ApprovalTypeCustom      ApprovalType = "custom"       // Custom approval
)

// ApprovalStatus defines the status of an approval request
type ApprovalStatus string

const (
	StatusPending    ApprovalStatus = "pending"
	StatusApproved   ApprovalStatus = "approved"
	StatusRejected   ApprovalStatus = "rejected"
	StatusTimeout    ApprovalStatus = "timeout"
	StatusCancelled  ApprovalStatus = "cancelled"
	StatusEscalated  ApprovalStatus = "escalated"
)

// ApprovalPriority defines the priority level
type ApprovalPriority string

const (
	PriorityLow    ApprovalPriority = "low"
	PriorityMedium ApprovalPriority = "medium"
	PriorityHigh   ApprovalPriority = "high"
	PriorityCritical ApprovalPriority = "critical"
)

// ============================================================
// Approval Request
// ============================================================

// ApprovalRequest represents an approval request
type ApprovalRequest struct {
	ID             string                 `json:"id"`
	Type           ApprovalType           `json:"type"`
	Priority       ApprovalPriority       `json:"priority"`
	AgentID        string                 `json:"agent_id"`
	SessionID      string                 `json:"session_id"`
	UserID         string                 `json:"user_id"`
	Description    string                 `json:"description"`
	Details        map[string]interface{} `json:"details"`
	RiskLevel      string                 `json:"risk_level"`      // low, medium, high
	RiskReason     string                 `json:"risk_reason"`     // Reason for risk level
	Status         ApprovalStatus         `json:"status"`
	CreatedAt      time.Time              `json:"created_at"`
	ExpiresAt      time.Time              `json:"expires_at"`
	ApprovedAt     time.Time              `json:"approved_at,omitempty"`
	RejectedAt     time.Time              `json:"rejected_at,omitempty"`
	ApprovedBy     string                 `json:"approved_by,omitempty"`
	RejectedBy     string                 `json:"rejected_by,omitempty"`
	RejectionReason string                `json:"rejection_reason,omitempty"`
	Comments       []ApprovalComment      `json:"comments,omitempty"`
	AutoApprove    bool                   `json:"auto_approve"`    // Can be auto-approved
	TimeoutSeconds int                    `json:"timeout_seconds"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// ApprovalComment represents a comment on an approval request
type ApprovalComment struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// ============================================================
// Approval Decision
// ============================================================

// ApprovalDecision represents a user's decision
type ApprovalDecision struct {
	RequestID      string         `json:"request_id"`
	Decision       ApprovalStatus `json:"decision"` // approved or rejected
	UserID         string         `json:"user_id"`
	Reason         string         `json:"reason,omitempty"`
	Comment        string         `json:"comment,omitempty"`
	ModifiedParams map[string]interface{} `json:"modified_params,omitempty"` // User-modified parameters
	Timestamp      time.Time      `json:"timestamp"`
}

// ============================================================
// Approval Flow Manager
// ============================================================

// ApprovalFlowManager manages approval requests and decisions
type ApprovalFlowManager struct {
	requests       map[string]*ApprovalRequest
	pendingQueue   map[string][]string // userID -> pending request IDs
	history        map[string][]*ApprovalRequest // agentID -> history
	timeoutSeconds int
	autoApprove    bool
	hooks          ApprovalHooks
	notifier       ApprovalNotifier
	mu             sync.RWMutex
}

// ApprovalHooks provides hooks for approval events
type ApprovalHooks interface {
	OnRequestCreated(request *ApprovalRequest)
	OnApproved(request *ApprovalRequest, decision *ApprovalDecision)
	OnRejected(request *ApprovalRequest, decision *ApprovalDecision)
	OnTimeout(request *ApprovalRequest)
	OnEscalated(request *ApprovalRequest)
}

// ApprovalNotifier provides notification capability
type ApprovalNotifier interface {
	NotifyPending(request *ApprovalRequest)
	NotifyApproved(request *ApprovalRequest)
	NotifyRejected(request *ApprovalRequest)
}

// NewApprovalFlowManager creates a new approval flow manager
func NewApprovalFlowManager() *ApprovalFlowManager {
	return &ApprovalFlowManager{
		requests:       make(map[string]*ApprovalRequest),
		pendingQueue:   make(map[string][]string),
		history:        make(map[string][]*ApprovalRequest),
		timeoutSeconds: 300, // 5 minutes default
		autoApprove:    false,
	}
}

// SetHooks sets approval hooks
func (m *ApprovalFlowManager) SetHooks(hooks ApprovalHooks) {
	m.hooks = hooks
}

// SetNotifier sets approval notifier
func (m *ApprovalFlowManager) SetNotifier(notifier ApprovalNotifier) {
	m.notifier = notifier
}

// SetTimeout sets default timeout in seconds
func (m *ApprovalFlowManager) SetTimeout(seconds int) {
	m.timeoutSeconds = seconds
}

// SetAutoApprove sets auto-approve mode
func (m *ApprovalFlowManager) SetAutoApprove(enabled bool) {
	m.autoApprove = enabled
}

// CreateRequest creates a new approval request
func (m *ApprovalFlowManager) CreateRequest(ctx context.Context, req *ApprovalRequest) (*ApprovalRequest, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Validate request
	if req.ID == "" {
		req.ID = generateApprovalID()
	}

	// Set default values
	if req.Priority == "" {
		req.Priority = PriorityMedium
	}
	if req.Status == "" {
		req.Status = StatusPending
	}

	req.CreatedAt = time.Now()

	// Set timeout
	timeout := req.TimeoutSeconds
	if timeout == 0 {
		timeout = m.timeoutSeconds
	}
	req.ExpiresAt = req.CreatedAt.Add(time.Duration(timeout) * time.Second)

	// Store request
	m.requests[req.ID] = req

	// Add to pending queue
	m.pendingQueue[req.UserID] = append(m.pendingQueue[req.UserID], req.ID)

	// Add to history
	m.history[req.AgentID] = append(m.history[req.AgentID], req)

	// Notify
	if m.hooks != nil {
		m.hooks.OnRequestCreated(req)
	}
	if m.notifier != nil {
		m.notifier.NotifyPending(req)
	}

	// Check auto-approve
	if m.autoApprove && req.AutoApprove {
		decision := &ApprovalDecision{
			RequestID: req.ID,
			Decision:  StatusApproved,
			UserID:    "system",
			Reason:    "auto-approved",
			Timestamp: time.Now(),
		}
		m.applyDecision(req, decision)
	}

	return req, nil
}

// GetPendingRequests gets pending requests for a user
func (m *ApprovalFlowManager) GetPendingRequests(userID string) []*ApprovalRequest {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var requests []*ApprovalRequest
	for _, id := range m.pendingQueue[userID] {
		if req, ok := m.requests[id]; ok && req.Status == StatusPending {
			requests = append(requests, req)
		}
	}
	return requests
}

// GetRequest gets a request by ID
func (m *ApprovalFlowManager) GetRequest(id string) (*ApprovalRequest, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	req, ok := m.requests[id]
	if !ok {
		return nil, fmt.Errorf("approval request not found: %s", id)
	}
	return req, nil
}

// SubmitDecision submits an approval decision
func (m *ApprovalFlowManager) SubmitDecision(ctx context.Context, decision *ApprovalDecision) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Get request
	req, ok := m.requests[decision.RequestID]
	if !ok {
		return fmt.Errorf("approval request not found: %s", decision.RequestID)
	}

	// Check if still pending
	if req.Status != StatusPending {
		return fmt.Errorf("approval request is no longer pending: %s", req.Status)
	}

	// Check if expired
	if time.Now().After(req.ExpiresAt) {
		req.Status = StatusTimeout
		if m.hooks != nil {
			m.hooks.OnTimeout(req)
		}
		return fmt.Errorf("approval request has expired")
	}

	// Validate decision
	if decision.Decision != StatusApproved && decision.Decision != StatusRejected {
		return fmt.Errorf("invalid decision: %s", decision.Decision)
	}

	decision.Timestamp = time.Now()

	// Apply decision
	m.applyDecision(req, decision)

	return nil
}

// applyDecision applies a decision to a request
func (m *ApprovalFlowManager) applyDecision(req *ApprovalRequest, decision *ApprovalDecision) {
	if decision.Decision == StatusApproved {
		req.Status = StatusApproved
		req.ApprovedAt = decision.Timestamp
		req.ApprovedBy = decision.UserID

		if m.hooks != nil {
			m.hooks.OnApproved(req, decision)
		}
		if m.notifier != nil {
			m.notifier.NotifyApproved(req)
		}
	} else {
		req.Status = StatusRejected
		req.RejectedAt = decision.Timestamp
		req.RejectedBy = decision.UserID
		req.RejectionReason = decision.Reason

		if m.hooks != nil {
			m.hooks.OnRejected(req, decision)
		}
		if m.notifier != nil {
			m.notifier.NotifyRejected(req)
		}
	}

	// Remove from pending queue
	m.removeFromPendingQueue(req.UserID, req.ID)
}

// removeFromPendingQueue removes a request from pending queue
func (m *ApprovalFlowManager) removeFromPendingQueue(userID, requestID string) {
	queue := m.pendingQueue[userID]
	for i, id := range queue {
		if id == requestID {
			m.pendingQueue[userID] = append(queue[:i], queue[i+1:]...)
			break
		}
	}
}

// CancelRequest cancels an approval request
func (m *ApprovalFlowManager) CancelRequest(ctx context.Context, requestID, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	req, ok := m.requests[requestID]
	if !ok {
		return fmt.Errorf("approval request not found: %s", requestID)
	}

	if req.Status != StatusPending {
		return fmt.Errorf("approval request is no longer pending: %s", req.Status)
	}

	req.Status = StatusCancelled
	m.removeFromPendingQueue(req.UserID, req.ID)

	return nil
}

// EscalateRequest escalates an approval request
func (m *ApprovalFlowManager) EscalateRequest(ctx context.Context, requestID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	req, ok := m.requests[requestID]
	if !ok {
		return fmt.Errorf("approval request not found: %s", requestID)
	}

	req.Status = StatusEscalated
	req.Priority = PriorityCritical

	if m.hooks != nil {
		m.hooks.OnEscalated(req)
	}

	return nil
}

// AddComment adds a comment to an approval request
func (m *ApprovalFlowManager) AddComment(ctx context.Context, requestID, userID, content string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	req, ok := m.requests[requestID]
	if !ok {
		return fmt.Errorf("approval request not found: %s", requestID)
	}

	comment := ApprovalComment{
		ID:        generateCommentID(),
		UserID:    userID,
		Content:   content,
		CreatedAt: time.Now(),
	}

	req.Comments = append(req.Comments, comment)

	return nil
}

// GetHistory gets approval history for an agent
func (m *ApprovalFlowManager) GetHistory(agentID string) []*ApprovalRequest {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.history[agentID]
}

// WaitForApproval waits for approval decision
func (m *ApprovalFlowManager) WaitForApproval(ctx context.Context, requestID string) (*ApprovalDecision, error) {
	// Create a channel for the decision
	decisionChan := make(chan *ApprovalDecision, 1)

	// Start checking for decision
	go func() {
		for {
			m.mu.RLock()
			req, ok := m.requests[requestID]
			m.mu.RUnlock()

			if !ok {
				decisionChan <- nil
				return
			}

			if req.Status != StatusPending {
				decision := &ApprovalDecision{
					RequestID: requestID,
					Decision:  req.Status,
					Timestamp: time.Now(),
				}
				if req.Status == StatusApproved {
					decision.UserID = req.ApprovedBy
				} else if req.Status == StatusRejected {
					decision.UserID = req.RejectedBy
					decision.Reason = req.RejectionReason
				}
				decisionChan <- decision
				return
			}

			// Check timeout
			if time.Now().After(req.ExpiresAt) {
				m.mu.Lock()
				req.Status = StatusTimeout
				m.mu.Unlock()

				decisionChan <- &ApprovalDecision{
					RequestID: requestID,
					Decision:  StatusTimeout,
					Reason:    "approval request timed out",
					Timestamp: time.Now(),
				}
				return
			}

			time.Sleep(100 * time.Millisecond)
		}
	}()

	// Wait for decision or context cancellation
	select {
	case decision := <-decisionChan:
		if decision == nil {
			return nil, fmt.Errorf("approval request not found")
		}
		return decision, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// ============================================================
// Approval Rules
// ============================================================

// ApprovalRule defines when approval is needed
type ApprovalRule struct {
	ID             string         `json:"id"`
	Type           ApprovalType   `json:"type"`
	AgentID        string         `json:"agent_id"`
	ToolName       string         `json:"tool_name"`
	Condition      string         `json:"condition"`      // Condition to trigger approval
	RiskThreshold  string         `json:"risk_threshold"` // Risk level threshold
	AutoApprove    bool           `json:"auto_approve"`
	TimeoutSeconds int            `json:"timeout_seconds"`
	Enabled        bool           `json:"enabled"`
}

// RuleEngine evaluates approval rules
type RuleEngine struct {
	rules    []ApprovalRule
	mu       sync.RWMutex
}

// NewRuleEngine creates a new rule engine
func NewRuleEngine() *RuleEngine {
	return &RuleEngine{
		rules: make([]ApprovalRule, 0),
	}
}

// AddRule adds an approval rule
func (e *RuleEngine) AddRule(rule *ApprovalRule) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if rule.ID == "" {
		rule.ID = generateRuleID()
	}
	rule.Enabled = true

	e.rules = append(e.rules, *rule)
}

// RemoveRule removes a rule
func (e *RuleEngine) RemoveRule(ruleID string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	for i, rule := range e.rules {
		if rule.ID == ruleID {
			e.rules = append(e.rules[:i], e.rules[i+1:]...)
			break
		}
	}
}

// NeedsApproval checks if an action needs approval
func (e *RuleEngine) NeedsApproval(agentID, toolName string, context map[string]interface{}) (bool, *ApprovalRule) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, rule := range e.rules {
		if !rule.Enabled {
			continue
		}

		// Check agent match
		if rule.AgentID != "" && rule.AgentID != agentID {
			continue
		}

		// Check tool match
		if rule.ToolName != "" && rule.ToolName != toolName {
			continue
		}

		// Check condition
		if rule.Condition != "" {
			// Would need proper condition evaluation
			// For now, simple match
		}

		// Rule matches
		return true, &rule
	}

	return false, nil
}

// ============================================================
// Helper Functions
// ============================================================

func generateApprovalID() string {
	return fmt.Sprintf("approval-%d", time.Now().UnixNano())
}

func generateCommentID() string {
	return fmt.Sprintf("comment-%d", time.Now().UnixNano())
}

func generateRuleID() string {
	return fmt.Sprintf("rule-%d", time.Now().UnixNano())
}