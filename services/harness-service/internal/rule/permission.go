// Package rule provides rule engine and guardrail functionality
package rule

import (
	"context"
	"fmt"
		"strings"
	"sync"
	"time"
)

// ============================================================
// Permission Types
// ============================================================

// ActionType defines the type of action
type ActionType string

const (
	ActionRead    ActionType = "read"
	ActionWrite   ActionType = "write"
	ActionDelete  ActionType = "delete"
	ActionExecute ActionType = "execute"
	ActionAdmin   ActionType = "admin"
)

// Resource represents a protected resource
type Resource struct {
	Type       string                 `json:"type"`        // tool, data, api, file, etc.
	ID         string                 `json:"id"`          // Resource identifier
	Attributes map[string]interface{} `json:"attributes"`  // Additional attributes
}

// Permission represents a single permission
type Permission struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Resource    Resource   `json:"resource"`
	Actions     []ActionType `json:"actions"`
	Effect      string     `json:"effect"`      // "allow" or "deny"
	Conditions  []Condition `json:"conditions"` // Conditions for permission
	Priority    int        `json:"priority"`    // Higher = more important
	Enabled     bool       `json:"enabled"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// Condition represents a permission condition
type Condition struct {
	Type     string `json:"type"`     // "time", "rate", "context", "custom"
	Key      string `json:"key"`      // Condition key
	Operator string `json:"operator"` // "eq", "ne", "gt", "lt", "in", "contains"
	Value    interface{} `json:"value"`    // Condition value
}

// ============================================================
// Permission Matrix
// ============================================================

// PermissionMatrixV2 manages fine-grained permissions
type PermissionMatrixV2 struct {
	permissions   map[string]*Permission     // ID -> Permission
	byAgent       map[string][]string        // AgentType -> Permission IDs
	byResource    map[string][]string        // ResourceType -> Permission IDs
	rolePermissions map[string][]string      // Role -> Permission IDs
	userRoles     map[string][]string        // UserID -> Role IDs
	defaultPolicy string                      // "allow" or "deny"
	mu            sync.RWMutex
}

// NewPermissionMatrixV2 creates a new permission matrix
func NewPermissionMatrixV2() *PermissionMatrixV2 {
	m := &PermissionMatrixV2{
		permissions:     make(map[string]*Permission),
		byAgent:         make(map[string][]string),
		byResource:      make(map[string][]string),
		rolePermissions: make(map[string][]string),
		userRoles:       make(map[string][]string),
		defaultPolicy:   "deny", // Default deny for security
	}
	m.initDefaultPermissions()
	return m
}

// initDefaultPermissions initializes default permissions
func (m *PermissionMatrixV2) initDefaultPermissions() {
	// Define default permissions for different agent types
	defaults := map[string][]Permission{
		"browser": {
			{ID: "browser-nav", Name: "Browser Navigation", Resource: Resource{Type: "tool", ID: "browser_navigate"}, Actions: []ActionType{ActionExecute}, Effect: "allow", Priority: 100},
			{ID: "browser-click", Name: "Browser Click", Resource: Resource{Type: "tool", ID: "browser_click"}, Actions: []ActionType{ActionExecute}, Effect: "allow", Priority: 100},
			{ID: "browser-type", Name: "Browser Type", Resource: Resource{Type: "tool", ID: "browser_type"}, Actions: []ActionType{ActionExecute}, Effect: "allow", Priority: 100},
			{ID: "browser-screenshot", Name: "Browser Screenshot", Resource: Resource{Type: "tool", ID: "browser_screenshot"}, Actions: []ActionType{ActionExecute}, Effect: "allow", Priority: 100},
			{ID: "browser-deny-exec", Name: "Deny Exec", Resource: Resource{Type: "tool", ID: "exec_*"}, Actions: []ActionType{ActionExecute}, Effect: "deny", Priority: 200},
		},
		"code": {
			{ID: "code-read", Name: "Code Read", Resource: Resource{Type: "file", ID: "*"}, Actions: []ActionType{ActionRead}, Effect: "allow", Priority: 100},
			{ID: "code-write", Name: "Code Write", Resource: Resource{Type: "file", ID: "*.go"}, Actions: []ActionType{ActionWrite}, Effect: "allow", Priority: 100},
			{ID: "code-deny-delete", Name: "Deny Delete", Resource: Resource{Type: "file", ID: "*"}, Actions: []ActionType{ActionDelete}, Effect: "deny", Priority: 200},
			{ID: "code-deny-exec", Name: "Deny Exec", Resource: Resource{Type: "tool", ID: "exec_*"}, Actions: []ActionType{ActionExecute}, Effect: "deny", Priority: 200},
		},
		"search": {
			{ID: "search-web", Name: "Web Search", Resource: Resource{Type: "tool", ID: "web_search"}, Actions: []ActionType{ActionExecute}, Effect: "allow", Priority: 100},
			{ID: "search-read", Name: "Data Read", Resource: Resource{Type: "data", ID: "*"}, Actions: []ActionType{ActionRead}, Effect: "allow", Priority: 100},
			{ID: "search-deny-write", Name: "Deny Write", Resource: Resource{Type: "data", ID: "*"}, Actions: []ActionType{ActionWrite, ActionDelete}, Effect: "deny", Priority: 200},
		},
	}

	for agentType, perms := range defaults {
		for _, perm := range perms {
			perm.Enabled = true
			perm.CreatedAt = time.Now()
			perm.UpdatedAt = time.Now()
			m.permissions[perm.ID] = &perm
			m.byAgent[agentType] = append(m.byAgent[agentType], perm.ID)
		}
	}

	// Define roles
	m.rolePermissions["admin"] = []string{} // Admin has all permissions
	m.rolePermissions["developer"] = []string{"code-read", "code-write"}
	m.rolePermissions["viewer"] = []string{"search-web", "search-read"}
}

// AddPermission adds a permission
func (m *PermissionMatrixV2) AddPermission(ctx context.Context, perm *Permission) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if perm.ID == "" {
		perm.ID = generatePermissionID()
	}
	perm.CreatedAt = time.Now()
	perm.UpdatedAt = time.Now()
	perm.Enabled = true

	m.permissions[perm.ID] = perm

	return nil
}

// RemovePermission removes a permission
func (m *PermissionMatrixV2) RemovePermission(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.permissions, id)
}

// AssignRoleToUser assigns a role to a user
func (m *PermissionMatrixV2) AssignRoleToUser(userID, role string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.userRoles[userID] = append(m.userRoles[userID], role)
}

// RemoveRoleFromUser removes a role from a user
func (m *PermissionMatrixV2) RemoveRoleFromUser(userID, role string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	roles := m.userRoles[userID]
	for i, r := range roles {
		if r == role {
			m.userRoles[userID] = append(roles[:i], roles[i+1:]...)
			break
		}
	}
}

// GetPermissionsByAgent gets permissions for an agent type
func (m *PermissionMatrixV2) GetPermissionsByAgent(agentType string) []*Permission {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var perms []*Permission
	for _, id := range m.byAgent[agentType] {
		if p, ok := m.permissions[id]; ok && p.Enabled {
			perms = append(perms, p)
		}
	}
	return perms
}

// GetPermissionsByUser gets permissions for a user (via roles)
func (m *PermissionMatrixV2) GetPermissionsByUser(userID string) []*Permission {
	m.mu.RLock()
	defer m.mu.RUnlock()

	permMap := make(map[string]*Permission)

	// Get permissions from roles
	for _, role := range m.userRoles[userID] {
		for _, permID := range m.rolePermissions[role] {
			if p, ok := m.permissions[permID]; ok && p.Enabled {
				permMap[permID] = p
			}
		}
	}

	var perms []*Permission
	for _, p := range permMap {
		perms = append(perms, p)
	}
	return perms
}

// ============================================================
// Permission Evaluator
// ============================================================

// EvaluationContext provides context for permission evaluation
type EvaluationContext struct {
	UserID       string
	AgentType    string
	SessionID    string
	Resource     Resource
	Action       ActionType
	Environment  map[string]interface{}
	Time         time.Time
}

// EvaluationResult represents the result of permission evaluation
type EvaluationResult struct {
	Allowed      bool       `json:"allowed"`
	DeniedReason string     `json:"denied_reason,omitempty"`
	MatchedPerms []string   `json:"matched_permissions"`
	Conditions   []Condition `json:"checked_conditions"`
	Score        float64    `json:"score"` // Confidence score
}

// PermissionEvaluator evaluates permissions dynamically
type PermissionEvaluator struct {
	matrix      *PermissionMatrixV2
	cache       map[string]*EvaluationResult
	cacheMu     sync.RWMutex
	enableCache bool
}

// NewPermissionEvaluator creates a new permission evaluator
func NewPermissionEvaluator(matrix *PermissionMatrixV2) *PermissionEvaluator {
	return &PermissionEvaluator{
		matrix:      matrix,
		cache:       make(map[string]*EvaluationResult),
		enableCache: true,
	}
}

// Evaluate evaluates if an action is allowed
func (e *PermissionEvaluator) Evaluate(ctx context.Context, evalCtx *EvaluationContext) *EvaluationResult {
	// Check cache
	cacheKey := e.buildCacheKey(evalCtx)
	if e.enableCache {
		e.cacheMu.RLock()
		if cached, ok := e.cache[cacheKey]; ok {
			e.cacheMu.RUnlock()
			return cached
		}
		e.cacheMu.RUnlock()
	}

	result := &EvaluationResult{
		Allowed:      false,
		MatchedPerms: make([]string, 0),
		Conditions:   make([]Condition, 0),
	}

	// Get applicable permissions
	var perms []*Permission
	perms = append(perms, e.matrix.GetPermissionsByAgent(evalCtx.AgentType)...)
	perms = append(perms, e.matrix.GetPermissionsByUser(evalCtx.UserID)...)

	// Sort by priority (higher first)
	e.sortPermissionsByPriority(perms)

	// Evaluate each permission
	for _, perm := range perms {
		if !perm.Enabled {
			continue
		}

		// Check resource match
		if !e.matchResource(perm.Resource, evalCtx.Resource) {
			continue
		}

		// Check action match
		if !e.matchAction(perm.Actions, evalCtx.Action) {
			continue
		}

		// Check conditions
		conditionsMet := true
		for _, cond := range perm.Conditions {
			if !e.evaluateCondition(cond, evalCtx) {
				conditionsMet = false
				result.Conditions = append(result.Conditions, cond)
				break
			}
		}

		if conditionsMet {
			result.MatchedPerms = append(result.MatchedPerms, perm.ID)

			if perm.Effect == "deny" {
				result.Allowed = false
				result.DeniedReason = fmt.Sprintf("Permission %s denies this action", perm.Name)
				result.Score = 1.0
				break
			} else if perm.Effect == "allow" {
				result.Allowed = true
				result.Score = 1.0
			}
		}
	}

	// Apply default policy
	if len(result.MatchedPerms) == 0 {
		result.Allowed = e.matrix.defaultPolicy == "allow"
		result.DeniedReason = "No matching permission found, using default policy"
		result.Score = 0.5
	}

	// Cache result
	if e.enableCache {
		e.cacheMu.Lock()
		e.cache[cacheKey] = result
		e.cacheMu.Unlock()
	}

	return result
}

// Check checks if an action is allowed (simplified method)
func (e *PermissionEvaluator) Check(ctx context.Context, userID, agentType, resourceType, resourceID string, action ActionType) error {
	evalCtx := &EvaluationContext{
		UserID:    userID,
		AgentType: agentType,
		Resource:  Resource{Type: resourceType, ID: resourceID},
		Action:    action,
		Time:      time.Now(),
	}

	result := e.Evaluate(ctx, evalCtx)

	if !result.Allowed {
		return fmt.Errorf("permission denied: %s", result.DeniedReason)
	}

	return nil
}

// matchResource checks if resources match
func (e *PermissionEvaluator) matchResource(permRes, reqRes Resource) bool {
	// Check type
	if permRes.Type != "*" && permRes.Type != reqRes.Type {
		return false
	}

	// Check ID (support wildcards)
	if permRes.ID == "*" || permRes.ID == "" {
		return true
	}

	// Wildcard matching
	if strings.HasSuffix(permRes.ID, "*") {
		prefix := strings.TrimSuffix(permRes.ID, "*")
		return strings.HasPrefix(reqRes.ID, prefix)
	}
	if strings.HasPrefix(permRes.ID, "*") {
		suffix := strings.TrimPrefix(permRes.ID, "*")
		return strings.HasSuffix(reqRes.ID, suffix)
	}

	return permRes.ID == reqRes.ID
}

// matchAction checks if action matches
func (e *PermissionEvaluator) matchAction(permActions []ActionType, reqAction ActionType) bool {
	for _, a := range permActions {
		if a == reqAction || a == "*" {
			return true
		}
	}
	return false
}

// evaluateCondition evaluates a condition
func (e *PermissionEvaluator) evaluateCondition(cond Condition, ctx *EvaluationContext) bool {
	switch cond.Type {
	case "time":
		return e.evaluateTimeCondition(cond, ctx)
	case "rate":
		return e.evaluateRateCondition(cond, ctx)
	case "context":
		return e.evaluateContextCondition(cond, ctx)
	default:
		return true // Unknown conditions pass
	}
}

// evaluateTimeCondition evaluates time-based condition
func (e *PermissionEvaluator) evaluateTimeCondition(cond Condition, ctx *EvaluationContext) bool {
	// Example: Only allow during business hours
	if cond.Key == "hour" {
		hour := ctx.Time.Hour()
		switch cond.Operator {
		case "in":
			if hours, ok := cond.Value.([]int); ok {
				for _, h := range hours {
					if hour == h {
						return true
					}
				}
				return false
			}
		}
	}
	return true
}

// evaluateRateCondition evaluates rate-based condition
func (e *PermissionEvaluator) evaluateRateCondition(cond Condition, ctx *EvaluationContext) bool {
	// Rate limiting would be checked here
	// For now, always pass
	return true
}

// evaluateContextCondition evaluates context-based condition
func (e *PermissionEvaluator) evaluateContextCondition(cond Condition, ctx *EvaluationContext) bool {
	value, ok := ctx.Environment[cond.Key]
	if !ok {
		return false
	}

	switch cond.Operator {
	case "eq":
		return value == cond.Value
	case "ne":
		return value != cond.Value
	case "contains":
		if str, ok := value.(string); ok {
			if pattern, ok := cond.Value.(string); ok {
				return strings.Contains(str, pattern)
			}
		}
	}

	return true
}

// sortPermissionsByPriority sorts permissions by priority
func (e *PermissionEvaluator) sortPermissionsByPriority(perms []*Permission) {
	// Simple bubble sort (sufficient for small lists)
	for i := 0; i < len(perms)-1; i++ {
		for j := i + 1; j < len(perms); j++ {
			if perms[j].Priority > perms[i].Priority {
				perms[i], perms[j] = perms[j], perms[i]
			}
		}
	}
}

// buildCacheKey builds a cache key
func (e *PermissionEvaluator) buildCacheKey(ctx *EvaluationContext) string {
	return fmt.Sprintf("%s:%s:%s:%s:%s", ctx.UserID, ctx.AgentType, ctx.Resource.Type, ctx.Resource.ID, ctx.Action)
}

// ClearCache clears the evaluation cache
func (e *PermissionEvaluator) ClearCache() {
	e.cacheMu.Lock()
	defer e.cacheMu.Unlock()
	e.cache = make(map[string]*EvaluationResult)
}

// ============================================================
// Permission Inheritance
// ============================================================

// PermissionInheritance manages permission inheritance
type PermissionInheritance struct {
	agentHierarchy map[string][]string // Agent -> Parent agents
	mu              sync.RWMutex
}

// NewPermissionInheritance creates a new inheritance manager
func NewPermissionInheritance() *PermissionInheritance {
	h := &PermissionInheritance{
		agentHierarchy: make(map[string][]string),
	}

	// Set up default hierarchy
	h.agentHierarchy["browser_advanced"] = []string{"browser"}
	h.agentHierarchy["code_advanced"] = []string{"code"}
	h.agentHierarchy["search_advanced"] = []string{"search"}

	return h
}

// SetParent sets parent agents for inheritance
func (h *PermissionInheritance) SetParent(agentType string, parents []string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.agentHierarchy[agentType] = parents
}

// GetInheritedPermissions gets all inherited permissions
func (h *PermissionInheritance) GetInheritedPermissions(matrix *PermissionMatrixV2, agentType string) []*Permission {
	h.mu.RLock()
	defer h.mu.RUnlock()

	visited := make(map[string]bool)
	var perms []*Permission

	var collect func(string)
	collect = func(agent string) {
		if visited[agent] {
			return
		}
		visited[agent] = true

		// Get permissions for this agent
		perms = append(perms, matrix.GetPermissionsByAgent(agent)...)

		// Get permissions from parents
		for _, parent := range h.agentHierarchy[agent] {
			collect(parent)
		}
	}

	collect(agentType)
	return perms
}

// ============================================================
// Helper Functions
// ============================================================

func generatePermissionID() string {
	return fmt.Sprintf("perm-%d", time.Now().UnixNano())
}

