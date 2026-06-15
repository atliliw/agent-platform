// Package featureflag provides feature flag management with targeting rules
package featureflag

import (
	"context"
	"crypto/md5"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// FlagStatus represents the status of a feature flag
type FlagStatus string

const (
	FlagStatusActive   FlagStatus = "active"
	FlagStatusInactive FlagStatus = "inactive"
	FlagStatusArchived FlagStatus = "archived"
)

// Operator defines comparison operators for targeting rules
type Operator string

const (
	OpEq      Operator = "eq"
	OpNeq     Operator = "neq"
	OpContains Operator = "contains"
	OpStartsWith Operator = "starts_with"
	OpEndsWith Operator = "ends_with"
	OpGt      Operator = "gt"
	OpLt      Operator = "lt"
	OpGte     Operator = "gte"
	OpLte     Operator = "lte"
	OpIn      Operator = "in"
	OpNotIn   Operator = "not_in"
	OpRegex   Operator = "regex"
)

// Rule represents a targeting rule
type Rule struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Conditions []Condition           `json:"conditions"`
	Variants  map[string]interface{} `json:"variants,omitempty"` // Variant name -> value
	Priority  int                    `json:"priority"` // Higher priority rules are evaluated first
}

// Condition represents a single condition in a rule
type Condition struct {
	Attribute string      `json:"attribute"` // e.g., "user_id", "region", "plan"
	Operator  Operator    `json:"operator"`
	Value     interface{} `json:"value"` // Can be string, number, array
}

// FeatureFlag represents a feature flag
type FeatureFlag struct {
	ID          string            `gorm:"primaryKey"`
	Key         string            `gorm:"uniqueIndex"`
	Name        string
	Description string
	Type        string            // "boolean", "string", "number", "json"
	Value       string            // Default value (JSON encoded)
	Status      FlagStatus        `gorm:"index"`
	Rules       string            // JSON encoded rules
	Rollout     float64           // Percentage rollout (0-100)
	StaleAfter  time.Duration     // Time after which flag is considered stale
	LastUsed    time.Time         // Last time flag was evaluated
	TenantID    string            `gorm:"index"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// EvaluationContext provides context for flag evaluation
type EvaluationContext struct {
	UserID    string
	SessionID string
	Attributes map[string]interface{}
}

// EvaluationResult represents the result of flag evaluation
type EvaluationResult struct {
	Key      string
	Value    interface{}
	Variant  string // Which variant was selected
	Reason   string // "default", "rule_match", "rollout", "off"
	RuleID   string // ID of matched rule
}

// Engine is the feature flag engine
type Engine struct {
	db    *gorm.DB
	flags map[string]*FeatureFlag
	mu    sync.RWMutex
}

// NewEngine creates a new feature flag engine
func NewEngine(db *gorm.DB) *Engine {
	e := &Engine{
		db:    db,
		flags: make(map[string]*FeatureFlag),
	}
	e.loadFlags()
	return e
}

// NewEngineMemory creates an in-memory feature flag engine
func NewEngineMemory() *Engine {
	return &Engine{
		flags: make(map[string]*FeatureFlag),
	}
}

// loadFlags loads flags from database
func (e *Engine) loadFlags() {
	if e.db == nil {
		return
	}

	var flags []FeatureFlag
	if err := e.db.Find(&flags).Error; err != nil {
		return
	}

	for _, flag := range flags {
		e.flags[flag.Key] = &flag
	}
}

// CreateFlag creates a new feature flag
func (e *Engine) CreateFlag(ctx context.Context, flag *FeatureFlag) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if flag.ID == "" {
		flag.ID = uuid.New().String()
	}
	flag.CreatedAt = time.Now()
	flag.UpdatedAt = time.Now()
	if flag.Status == "" {
		flag.Status = FlagStatusActive
	}
	if flag.Type == "" {
		flag.Type = "boolean"
	}
	if flag.Rollout == 0 {
		flag.Rollout = 100 // Default to 100% rollout
	}

	if e.db != nil {
		if err := e.db.Create(flag).Error; err != nil {
			return fmt.Errorf("create flag: %w", err)
		}
	}

	e.flags[flag.Key] = flag
	return nil
}

// GetFlag retrieves a flag by key
func (e *Engine) GetFlag(ctx context.Context, key string) (*FeatureFlag, error) {
	e.mu.RLock()
	flag, exists := e.flags[key]
	e.mu.RUnlock()

	if exists {
		return flag, nil
	}

	if e.db != nil {
		var f FeatureFlag
		if err := e.db.First(&f, "key = ?", key).Error; err != nil {
			return nil, fmt.Errorf("get flag: %w", err)
		}
		return &f, nil
	}

	return nil, fmt.Errorf("flag not found: %s", key)
}

// ListFlags lists all flags
func (e *Engine) ListFlags(ctx context.Context, tenantID string, status FlagStatus) ([]*FeatureFlag, error) {
	if e.db != nil {
		query := e.db.Model(&FeatureFlag{})
		if tenantID != "" {
			query = query.Where("tenant_id = ?", tenantID)
		}
		if status != "" {
			query = query.Where("status = ?", status)
		}

		var flags []*FeatureFlag
		if err := query.Order("created_at DESC").Find(&flags).Error; err != nil {
			return nil, fmt.Errorf("list flags: %w", err)
		}
		return flags, nil
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []*FeatureFlag
	for _, flag := range e.flags {
		if tenantID != "" && flag.TenantID != tenantID {
			continue
		}
		if status != "" && flag.Status != status {
			continue
		}
		result = append(result, flag)
	}
	return result, nil
}

// UpdateFlag updates a feature flag
func (e *Engine) UpdateFlag(ctx context.Context, flag *FeatureFlag) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	flag.UpdatedAt = time.Now()

	if e.db != nil {
		if err := e.db.Save(flag).Error; err != nil {
			return fmt.Errorf("update flag: %w", err)
		}
	}

	e.flags[flag.Key] = flag
	return nil
}

// Toggle enables or disables a flag
func (e *Engine) Toggle(ctx context.Context, key string, enabled bool) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	flag, exists := e.flags[key]
	if !exists {
		if e.db != nil {
			flag = &FeatureFlag{}
			if err := e.db.First(flag, "key = ?", key).Error; err != nil {
				return fmt.Errorf("flag not found: %s", key)
			}
		} else {
			return fmt.Errorf("flag not found: %s", key)
		}
	}

	status := FlagStatusInactive
	if enabled {
		status = FlagStatusActive
	}
	flag.Status = status
	flag.UpdatedAt = time.Now()

	if e.db != nil {
		if err := e.db.Model(flag).Updates(map[string]interface{}{
			"status":     status,
			"updated_at": time.Now(),
		}).Error; err != nil {
			return fmt.Errorf("toggle flag: %w", err)
		}
	}

	e.flags[key] = flag
	return nil
}

// DeleteFlag deletes a feature flag
func (e *Engine) DeleteFlag(ctx context.Context, key string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.db != nil {
		if err := e.db.Delete(&FeatureFlag{}, "key = ?", key).Error; err != nil {
			return fmt.Errorf("delete flag: %w", err)
		}
	}

	delete(e.flags, key)
	return nil
}

// Evaluate evaluates a flag and returns the appropriate value
func (e *Engine) Evaluate(ctx context.Context, key string, evalCtx *EvaluationContext) (*EvaluationResult, error) {
	e.mu.RLock()
	flag, exists := e.flags[key]
	e.mu.RUnlock()

	if !exists {
		if e.db != nil {
			flag = &FeatureFlag{}
			if err := e.db.First(flag, "key = ?", key).Error; err != nil {
				return nil, fmt.Errorf("flag not found: %s", key)
			}
			e.mu.Lock()
			e.flags[key] = flag
			e.mu.Unlock()
		} else {
			return nil, fmt.Errorf("flag not found: %s", key)
		}
	}

	// Update last used
	if e.db != nil {
		e.db.Model(flag).Update("last_used", time.Now())
	}

	result := &EvaluationResult{
		Key:    key,
		Reason: "default",
	}

	// Check if flag is active
	if flag.Status != FlagStatusActive {
		result.Reason = "off"
		result.Value = getDefaultValue(flag.Type)
		return result, nil
	}

	// Parse rules
	var rules []Rule
	if flag.Rules != "" {
		if err := json.Unmarshal([]byte(flag.Rules), &rules); err == nil {
			// Sort rules by priority (highest first)
			sortRulesByPriority(rules)

			// Evaluate rules
			for _, rule := range rules {
				if e.evaluateRule(rule, evalCtx) {
					result.Reason = "rule_match"
					result.RuleID = rule.ID
					result.Variant = rule.Name

					// Get variant value if specified
					if rule.Variants != nil && len(rule.Variants) > 0 {
						// Use first variant value
						for _, v := range rule.Variants {
							result.Value = v
							break
						}
					} else {
						// Parse default value
						result.Value = parseValue(flag.Value, flag.Type)
					}
					return result, nil
				}
			}
		}
	}

	// Check rollout percentage
	if flag.Rollout < 100 {
		hash := e.computeHash(key, evalCtx)
		if hash > flag.Rollout/100 {
			result.Reason = "rollout"
			result.Value = getDefaultValue(flag.Type)
			return result, nil
		}
	}

	// Return default value
	result.Reason = "default"
	result.Value = parseValue(flag.Value, flag.Type)
	return result, nil
}

// evaluateRule evaluates a targeting rule
func (e *Engine) evaluateRule(rule Rule, ctx *EvaluationContext) bool {
	if ctx == nil {
		return false
	}

	// All conditions must match (AND logic)
	for _, cond := range rule.Conditions {
		if !e.evaluateCondition(cond, ctx) {
			return false
		}
	}

	return len(rule.Conditions) > 0
}

// evaluateCondition evaluates a single condition
func (e *Engine) evaluateCondition(cond Condition, ctx *EvaluationContext) bool {
	var attrValue interface{}

	// Get attribute value from context
	switch cond.Attribute {
	case "user_id":
		attrValue = ctx.UserID
	case "session_id":
		attrValue = ctx.SessionID
	default:
		if ctx.Attributes != nil {
			attrValue = ctx.Attributes[cond.Attribute]
		}
	}

	// Evaluate based on operator
	switch cond.Operator {
	case OpEq:
		return compareEqual(attrValue, cond.Value)
	case OpNeq:
		return !compareEqual(attrValue, cond.Value)
	case OpContains:
		return strings.Contains(fmt.Sprintf("%v", attrValue), fmt.Sprintf("%v", cond.Value))
	case OpStartsWith:
		return strings.HasPrefix(fmt.Sprintf("%v", attrValue), fmt.Sprintf("%v", cond.Value))
	case OpEndsWith:
		return strings.HasSuffix(fmt.Sprintf("%v", attrValue), fmt.Sprintf("%v", cond.Value))
	case OpGt:
		return compareNumeric(attrValue, cond.Value) > 0
	case OpLt:
		return compareNumeric(attrValue, cond.Value) < 0
	case OpGte:
		return compareNumeric(attrValue, cond.Value) >= 0
	case OpLte:
		return compareNumeric(attrValue, cond.Value) <= 0
	case OpIn:
		return isInList(attrValue, cond.Value)
	case OpNotIn:
		return !isInList(attrValue, cond.Value)
	default:
		return false
	}
}

// computeHash computes a hash for rollout determination
func (e *Engine) computeHash(key string, ctx *EvaluationContext) float64 {
	var seed string
	if ctx != nil {
		seed = key + ":" + ctx.UserID + ":" + ctx.SessionID
	} else {
		seed = key
	}

	h := md5.New()
	h.Write([]byte(seed))
	hashBytes := h.Sum(nil)

	hashValue := binary.BigEndian.Uint64(hashBytes[:8])
	return float64(hashValue) / float64(1<<64)
}

// DetectStaleFlags detects flags that haven't been used recently
func (e *Engine) DetectStaleFlags(ctx context.Context, staleDuration time.Duration) ([]*FeatureFlag, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	cutoff := time.Now().Add(-staleDuration)
	var staleFlags []*FeatureFlag

	for _, flag := range e.flags {
		if flag.LastUsed.Before(cutoff) && flag.Status == FlagStatusActive {
			staleFlags = append(staleFlags, flag)
		}
	}

	return staleFlags, nil
}

// AddRule adds a targeting rule to a flag
func (e *Engine) AddRule(ctx context.Context, key string, rule Rule) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	flag, exists := e.flags[key]
	if !exists {
		return fmt.Errorf("flag not found: %s", key)
	}

	var rules []Rule
	if flag.Rules != "" {
		json.Unmarshal([]byte(flag.Rules), &rules)
	}

	if rule.ID == "" {
		rule.ID = uuid.New().String()
	}

	rules = append(rules, rule)
	rulesJSON, _ := json.Marshal(rules)
	flag.Rules = string(rulesJSON)
	flag.UpdatedAt = time.Now()

	if e.db != nil {
		if err := e.db.Model(flag).Updates(map[string]interface{}{
			"rules":      flag.Rules,
			"updated_at": time.Now(),
		}).Error; err != nil {
			return fmt.Errorf("add rule: %w", err)
		}
	}

	e.flags[key] = flag
	return nil
}

// RemoveRule removes a targeting rule from a flag
func (e *Engine) RemoveRule(ctx context.Context, key, ruleID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	flag, exists := e.flags[key]
	if !exists {
		return fmt.Errorf("flag not found: %s", key)
	}

	var rules []Rule
	if flag.Rules != "" {
		json.Unmarshal([]byte(flag.Rules), &rules)
	}

	var newRules []Rule
	for _, r := range rules {
		if r.ID != ruleID {
			newRules = append(newRules, r)
		}
	}

	rulesJSON, _ := json.Marshal(newRules)
	flag.Rules = string(rulesJSON)
	flag.UpdatedAt = time.Now()

	if e.db != nil {
		if err := e.db.Model(flag).Updates(map[string]interface{}{
			"rules":      flag.Rules,
			"updated_at": time.Now(),
		}).Error; err != nil {
			return fmt.Errorf("remove rule: %w", err)
		}
	}

	e.flags[key] = flag
	return nil
}

// SetRollout sets the rollout percentage
func (e *Engine) SetRollout(ctx context.Context, key string, percentage float64) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	flag, exists := e.flags[key]
	if !exists {
		return fmt.Errorf("flag not found: %s", key)
	}

	if percentage < 0 {
		percentage = 0
	}
	if percentage > 100 {
		percentage = 100
	}

	flag.Rollout = percentage
	flag.UpdatedAt = time.Now()

	if e.db != nil {
		if err := e.db.Model(flag).Updates(map[string]interface{}{
			"rollout":    percentage,
			"updated_at": time.Now(),
		}).Error; err != nil {
			return fmt.Errorf("set rollout: %w", err)
		}
	}

	e.flags[key] = flag
	return nil
}

// Helper functions

func sortRulesByPriority(rules []Rule) {
	for i := 0; i < len(rules)-1; i++ {
		for j := i + 1; j < len(rules); j++ {
			if rules[i].Priority < rules[j].Priority {
				rules[i], rules[j] = rules[j], rules[i]
			}
		}
	}
}

func getDefaultValue(flagType string) interface{} {
	switch flagType {
	case "boolean":
		return false
	case "string":
		return ""
	case "number":
		return 0
	case "json":
		return nil
	default:
		return false
	}
}

func parseValue(value string, flagType string) interface{} {
	switch flagType {
	case "boolean":
		return value == "true"
	case "number":
		var f float64
		fmt.Sscanf(value, "%f", &f)
		return f
	case "json":
		var v interface{}
		json.Unmarshal([]byte(value), &v)
		return v
	default:
		return value
	}
}

func compareEqual(a, b interface{}) bool {
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

func compareNumeric(a, b interface{}) float64 {
	var af, bf float64
	fmt.Sscanf(fmt.Sprintf("%v", a), "%f", &af)
	fmt.Sscanf(fmt.Sprintf("%v", b), "%f", &bf)
	return af - bf
}

func isInList(value, list interface{}) bool {
	strValue := fmt.Sprintf("%v", value)
	switch v := list.(type) {
	case []interface{}:
		for _, item := range v {
			if fmt.Sprintf("%v", item) == strValue {
				return true
			}
		}
	case string:
		// Assume comma-separated
		items := strings.Split(v, ",")
		for _, item := range items {
			if strings.TrimSpace(item) == strValue {
				return true
			}
		}
	}
	return false
}
