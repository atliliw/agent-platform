// Package audit provides audit logging for permission decisions and security events
package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// ============================================================
// Audit Event Types
// ============================================================

// AuditEventType defines the type of audit event
type AuditEventType string

const (
	EventTypePermissionCheck  AuditEventType = "permission_check"
	EventTypePermissionGrant  AuditEventType = "permission_grant"
	EventTypePermissionDeny   AuditEventType = "permission_deny"
	EventTypeToolAccess       AuditEventType = "tool_access"
	EventTypeDataAccess       AuditEventType = "data_access"
	EventTypeAPIAccess        AuditEventType = "api_access"
	EventTypeAuthAttempt      AuditEventType = "auth_attempt"
	EventTypeAuthSuccess      AuditEventType = "auth_success"
	EventTypeAuthFailure      AuditEventType = "auth_failure"
	EventTypeConfigChange     AuditEventType = "config_change"
	EventTypeSecurityAlert    AuditEventType = "security_alert"
	EventTypeGuardrailTrigger AuditEventType = "guardrail_trigger"
)

// AuditSeverity defines the severity of an audit event
type AuditSeverity string

const (
	SeverityInfo     AuditSeverity = "info"
	SeverityWarning  AuditSeverity = "warning"
	SeverityError    AuditSeverity = "error"
	SeverityCritical AuditSeverity = "critical"
)

// ============================================================
// Audit Event
// ============================================================

// AuditEvent represents an audit event
type AuditEvent struct {
	ID           string                 `json:"id"`
	Type         AuditEventType         `json:"type"`
	Severity     AuditSeverity          `json:"severity"`
	Timestamp    time.Time              `json:"timestamp"`

	// Actor information
	UserID       string                 `json:"user_id"`
	AgentID      string                 `json:"agent_id"`
	AgentType    string                 `json:"agent_type"`
	SessionID    string                 `json:"session_id"`
	ClientIP     string                 `json:"client_ip,omitempty"`
	UserAgent    string                 `json:"user_agent,omitempty"`

	// Action information
	Action       string                 `json:"action"`
	ResourceType string                 `json:"resource_type"`
	ResourceID   string                 `json:"resource_id"`
	Decision     string                 `json:"decision"` // "allow", "deny", "n/a"
	Reason       string                 `json:"reason,omitempty"`

	// Context
	Environment  map[string]interface{} `json:"environment,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`

	// Result
	Success      bool                   `json:"success"`
	ErrorCode    string                 `json:"error_code,omitempty"`
	ErrorMessage string                 `json:"error_message,omitempty"`

	// Tracing
	TraceID      string                 `json:"trace_id,omitempty"`
	SpanID       string                 `json:"span_id,omitempty"`
	ParentSpanID string                 `json:"parent_span_id,omitempty"`

	// Timing
	Duration     int64                  `json:"duration_ms"`
}

// ============================================================
// Audit Logger
// ============================================================

// AuditLogger logs audit events
type AuditLogger struct {
	events     []AuditEvent
	byUser     map[string][]string // UserID -> Event IDs
	byAgent    map[string][]string // AgentID -> Event IDs
	byType     map[AuditEventType][]string // EventType -> Event IDs
	bySession  map[string][]string // SessionID -> Event IDs
	maxEvents  int
	mu         sync.RWMutex
	hooks      []AuditHook
	exporter   AuditExporter
}

// AuditHook provides hooks for audit events
type AuditHook interface {
	OnEvent(event *AuditEvent)
}

// AuditExporter exports audit events
type AuditExporter interface {
	Export(ctx context.Context, events []AuditEvent) error
}

// NewAuditLogger creates a new audit logger
func NewAuditLogger(maxEvents int) *AuditLogger {
	if maxEvents <= 0 {
		maxEvents = 100000
	}
	return &AuditLogger{
		events:    make([]AuditEvent, 0),
		byUser:    make(map[string][]string),
		byAgent:   make(map[string][]string),
		byType:    make(map[AuditEventType][]string),
		bySession: make(map[string][]string),
		maxEvents: maxEvents,
		hooks:     make([]AuditHook, 0),
	}
}

// AddHook adds an audit hook
func (l *AuditLogger) AddHook(hook AuditHook) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.hooks = append(l.hooks, hook)
}

// SetExporter sets the audit exporter
func (l *AuditLogger) SetExporter(exporter AuditExporter) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.exporter = exporter
}

// Log logs an audit event
func (l *AuditLogger) Log(ctx context.Context, event *AuditEvent) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Generate ID if not set
	if event.ID == "" {
		event.ID = generateAuditID()
	}

	// Set timestamp if not set
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// Check capacity
	if len(l.events) >= l.maxEvents {
		l.evictOldest()
	}

	// Store event
	l.events = append(l.events, *event)

	// Update indexes
	if event.UserID != "" {
		l.byUser[event.UserID] = append(l.byUser[event.UserID], event.ID)
	}
	if event.AgentID != "" {
		l.byAgent[event.AgentID] = append(l.byAgent[event.AgentID], event.ID)
	}
	if event.SessionID != "" {
		l.bySession[event.SessionID] = append(l.bySession[event.SessionID], event.ID)
	}
	l.byType[event.Type] = append(l.byType[event.Type], event.ID)

	// Notify hooks
	for _, hook := range l.hooks {
		go hook.OnEvent(event)
	}

	return nil
}

// LogPermissionCheck logs a permission check event
func (l *AuditLogger) LogPermissionCheck(ctx context.Context, userID, agentID, agentType, sessionID, resourceType, resourceID, action, decision, reason string) error {
	event := &AuditEvent{
		Type:         EventTypePermissionCheck,
		Severity:     SeverityInfo,
		UserID:       userID,
		AgentID:      agentID,
		AgentType:    agentType,
		SessionID:    sessionID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Decision:     decision,
		Reason:       reason,
		Success:      decision == "allow",
	}

	if decision == "deny" {
		event.Severity = SeverityWarning
	}

	return l.Log(ctx, event)
}

// LogToolAccess logs a tool access event
func (l *AuditLogger) LogToolAccess(ctx context.Context, userID, agentID, agentType, sessionID, toolName string, success bool, duration int64) error {
	event := &AuditEvent{
		Type:         EventTypeToolAccess,
		Severity:     SeverityInfo,
		UserID:       userID,
		AgentID:      agentID,
		AgentType:    agentType,
		SessionID:    sessionID,
		Action:       "tool_execute",
		ResourceType: "tool",
		ResourceID:   toolName,
		Decision:     "executed",
		Success:      success,
		Duration:     duration,
	}

	if !success {
		event.Severity = SeverityWarning
	}

	return l.Log(ctx, event)
}

// LogDataAccess logs a data access event
func (l *AuditLogger) LogDataAccess(ctx context.Context, userID, agentID, agentType, sessionID, dataType, dataID, operation string, success bool) error {
	event := &AuditEvent{
		Type:         EventTypeDataAccess,
		Severity:     SeverityInfo,
		UserID:       userID,
		AgentID:      agentID,
		AgentType:    agentType,
		SessionID:    sessionID,
		Action:       operation,
		ResourceType: dataType,
		ResourceID:   dataID,
		Decision:     "accessed",
		Success:      success,
	}

	// Sensitive operations get higher severity
	if operation == "delete" || operation == "export" {
		event.Severity = SeverityWarning
	}

	return l.Log(ctx, event)
}

// LogAuthAttempt logs an authentication attempt
func (l *AuditLogger) LogAuthAttempt(ctx context.Context, userID, clientIP, userAgent string, success bool, reason string) error {
	var eventType AuditEventType
	var severity AuditSeverity

	if success {
		eventType = EventTypeAuthSuccess
		severity = SeverityInfo
	} else {
		eventType = EventTypeAuthFailure
		severity = SeverityWarning
	}

	event := &AuditEvent{
		Type:      eventType,
		Severity:  severity,
		UserID:    userID,
		ClientIP:  clientIP,
		UserAgent: userAgent,
		Action:    "authenticate",
		Decision:  "attempt",
		Success:   success,
		Reason:    reason,
	}

	return l.Log(ctx, event)
}

// LogSecurityAlert logs a security alert
func (l *AuditLogger) LogSecurityAlert(ctx context.Context, severity AuditSeverity, alertType, description, agentID, sessionID string, metadata map[string]interface{}) error {
	event := &AuditEvent{
		Type:      EventTypeSecurityAlert,
		Severity:   severity,
		AgentID:    agentID,
		SessionID:  sessionID,
		Action:     alertType,
		Reason:     description,
		Metadata:   metadata,
		Success:    false,
	}

	return l.Log(ctx, event)
}

// LogGuardrailTrigger logs a guardrail trigger event
func (l *AuditLogger) LogGuardrailTrigger(ctx context.Context, agentID, agentType, sessionID, triggerType, content, violation string) error {
	event := &AuditEvent{
		Type:         EventTypeGuardrailTrigger,
		Severity:     SeverityWarning,
		AgentID:      agentID,
		AgentType:    agentType,
		SessionID:    sessionID,
		Action:       "guardrail_check",
		ResourceType: "content",
		Reason:       fmt.Sprintf("Guardrail triggered: %s - %s", triggerType, violation),
		Success:      false,
		Metadata: map[string]interface{}{
			"trigger_type": triggerType,
			"violation":    violation,
		},
	}

	return l.Log(ctx, event)
}

// GetEvent retrieves an event by ID
func (l *AuditLogger) GetEvent(id string) (*AuditEvent, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	for _, event := range l.events {
		if event.ID == id {
			return &event, nil
		}
	}
	return nil, fmt.Errorf("audit event not found: %s", id)
}

// GetEventsByUser retrieves events by user ID
func (l *AuditLogger) GetEventsByUser(userID string, limit int) []AuditEvent {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var events []AuditEvent
	ids := l.byUser[userID]

	for i := len(ids) - 1; i >= 0 && len(events) < limit; i-- {
		for _, event := range l.events {
			if event.ID == ids[i] {
				events = append(events, event)
				break
			}
		}
	}

	return events
}

// GetEventsByAgent retrieves events by agent ID
func (l *AuditLogger) GetEventsByAgent(agentID string, limit int) []AuditEvent {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var events []AuditEvent
	ids := l.byAgent[agentID]

	for i := len(ids) - 1; i >= 0 && len(events) < limit; i-- {
		for _, event := range l.events {
			if event.ID == ids[i] {
				events = append(events, event)
				break
			}
		}
	}

	return events
}

// GetEventsBySession retrieves events by session ID
func (l *AuditLogger) GetEventsBySession(sessionID string) []AuditEvent {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var events []AuditEvent
	ids := l.bySession[sessionID]

	for _, id := range ids {
		for _, event := range l.events {
			if event.ID == id {
				events = append(events, event)
				break
			}
		}
	}

	return events
}

// GetEventsByType retrieves events by type
func (l *AuditLogger) GetEventsByType(eventType AuditEventType, limit int) []AuditEvent {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var events []AuditEvent
	ids := l.byType[eventType]

	for i := len(ids) - 1; i >= 0 && len(events) < limit; i-- {
		for _, event := range l.events {
			if event.ID == ids[i] {
				events = append(events, event)
				break
			}
		}
	}

	return events
}

// GetEventsByTimeRange retrieves events within a time range
func (l *AuditLogger) GetEventsByTimeRange(start, end time.Time) []AuditEvent {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var events []AuditEvent
	for i := len(l.events) - 1; i >= 0; i-- {
		event := l.events[i]
		if (event.Timestamp.Equal(start) || event.Timestamp.After(start)) &&
			(event.Timestamp.Equal(end) || event.Timestamp.Before(end)) {
			events = append(events, event)
		}
	}
	return events
}

// GetRecentEvents retrieves the most recent events
func (l *AuditLogger) GetRecentEvents(limit int) []AuditEvent {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var events []AuditEvent
	for i := len(l.events) - 1; i >= 0 && len(events) < limit; i-- {
		events = append(events, l.events[i])
	}
	return events
}

// Export exports events to the configured exporter
func (l *AuditLogger) Export(ctx context.Context) error {
	l.mu.RLock()
	exporter := l.exporter
	events := l.events
	l.mu.RUnlock()

	if exporter == nil {
		return nil
	}

	return exporter.Export(ctx, events)
}

// Clear clears all events
func (l *AuditLogger) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.events = make([]AuditEvent, 0)
	l.byUser = make(map[string][]string)
	l.byAgent = make(map[string][]string)
	l.byType = make(map[AuditEventType][]string)
	l.bySession = make(map[string][]string)
}

// GetStatistics returns audit statistics
func (l *AuditLogger) GetStatistics() map[string]interface{} {
	l.mu.RLock()
	defer l.mu.RUnlock()

	stats := map[string]interface{}{
		"total_events": len(l.events),
		"by_type":      make(map[AuditEventType]int),
		"by_severity":  make(map[AuditSeverity]int),
	}

	// Count by type and severity
	for _, event := range l.events {
		stats["by_type"].(map[AuditEventType]int)[event.Type]++
		stats["by_severity"].(map[AuditSeverity]int)[event.Severity]++
	}

	return stats
}

// evictOldest removes the oldest events
func (l *AuditLogger) evictOldest() {
	// Remove 10% of oldest events
	removeCount := l.maxEvents / 10
	if removeCount < 1 {
		removeCount = 1
	}

	// Remove from events list
	l.events = l.events[removeCount:]

	// Note: Indexes are not updated for simplicity
	// In production, would rebuild indexes
}

// ============================================================
// Audit Query
// ============================================================

// AuditQuery represents a query for audit events
type AuditQuery struct {
	UserID      string
	AgentID     string
	SessionID   string
	Types       []AuditEventType
	Severities  []AuditSeverity
	StartTime   time.Time
	EndTime     time.Time
	Success     *bool
	Limit       int
	Offset      int
}

// Query queries audit events
func (l *AuditLogger) Query(query AuditQuery) []AuditEvent {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var results []AuditEvent

	for _, event := range l.events {
		// Filter by user ID
		if query.UserID != "" && event.UserID != query.UserID {
			continue
		}

		// Filter by agent ID
		if query.AgentID != "" && event.AgentID != query.AgentID {
			continue
		}

		// Filter by session ID
		if query.SessionID != "" && event.SessionID != query.SessionID {
			continue
		}

		// Filter by types
		if len(query.Types) > 0 {
			found := false
			for _, t := range query.Types {
				if event.Type == t {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Filter by severities
		if len(query.Severities) > 0 {
			found := false
			for _, s := range query.Severities {
				if event.Severity == s {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Filter by time range
		if !query.StartTime.IsZero() && event.Timestamp.Before(query.StartTime) {
			continue
		}
		if !query.EndTime.IsZero() && event.Timestamp.After(query.EndTime) {
			continue
		}

		// Filter by success
		if query.Success != nil && event.Success != *query.Success {
			continue
		}

		results = append(results, event)
	}

	// Apply offset and limit
	if query.Offset > 0 && query.Offset < len(results) {
		results = results[query.Offset:]
	}
	if query.Limit > 0 && len(results) > query.Limit {
		results = results[:query.Limit]
	}

	return results
}

// ============================================================
// Helper Functions
// ============================================================

func generateAuditID() string {
	return fmt.Sprintf("audit-%d", time.Now().UnixNano())
}

// ToJSON converts an event to JSON
func (e *AuditEvent) ToJSON() string {
	data, _ := json.Marshal(e)
	return string(data)
}

// FromJSON parses an event from JSON
func FromJSON(data string) (*AuditEvent, error) {
	var event AuditEvent
	err := json.Unmarshal([]byte(data), &event)
	if err != nil {
		return nil, err
	}
	return &event, nil
}
