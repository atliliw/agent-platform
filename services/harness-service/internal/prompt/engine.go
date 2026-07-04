// Package prompt provides prompt version management with LRU caching
package prompt

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Engine manages prompts and versions with caching
type Engine struct {
	db          *gorm.DB
	renderer    *Renderer
	tracker     *PerformanceTracker
	prompts     map[string]*Prompt      // key -> prompt (in-memory cache)
	versions    map[string]*PromptVersion // promptID -> active version (LRU cache)
	versionList map[string][]PromptVersion // promptID -> all versions
	mu          sync.RWMutex
	cacheSize   int
}

// NewEngine creates a new prompt engine with database
func NewEngine(db *gorm.DB) *Engine {
	e := &Engine{
		db:          db,
		renderer:    NewRenderer(),
		tracker:     NewPerformanceTracker(db),
		prompts:     make(map[string]*Prompt),
		versions:    make(map[string]*PromptVersion),
		versionList: make(map[string][]PromptVersion),
		cacheSize:   100,
	}
	e.loadFromDB()
	return e
}

// NewEngineMemory creates an in-memory prompt engine
func NewEngineMemory() *Engine {
	return &Engine{
		renderer:    NewRenderer(),
		tracker:     NewPerformanceTrackerMemory(),
		prompts:     make(map[string]*Prompt),
		versions:    make(map[string]*PromptVersion),
		versionList: make(map[string][]PromptVersion),
		cacheSize:   100,
	}
}

// AutoMigrate creates database tables for prompt models
func (e *Engine) AutoMigrate() error {
	if e.db == nil {
		return nil
	}
	return e.db.AutoMigrate(&Prompt{}, &PromptVersion{}, &PromptPerformance{}, &UsageRecord{})
}

// loadFromDB loads prompts from database into cache
func (e *Engine) loadFromDB() {
	if e.db == nil {
		return
	}

	var prompts []Prompt
	if err := e.db.Find(&prompts).Error; err != nil {
		return
	}

	for _, p := range prompts {
		e.prompts[p.Key] = &p

		// Load active version
		var version PromptVersion
		if err := e.db.Where("prompt_id = ? AND is_active = ?", p.ID, true).First(&version).Error; err == nil {
			e.versions[p.ID] = &version
		}

		// Load all versions
		var versions []PromptVersion
		if err := e.db.Where("prompt_id = ?", p.ID).Order("created_at DESC").Find(&versions).Error; err == nil {
			e.versionList[p.ID] = versions
		}
	}
}

// CreatePrompt creates a new prompt
func (e *Engine) CreatePrompt(ctx context.Context, prompt *Prompt) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if prompt.ID == "" {
		prompt.ID = uuid.New().String()
	}
	prompt.CreatedAt = time.Now()
	prompt.UpdatedAt = time.Now()

	if e.db != nil {
		if err := e.db.Create(prompt).Error; err != nil {
			return fmt.Errorf("create prompt: %w", err)
		}
	}

	e.prompts[prompt.Key] = prompt
	return nil
}

// GetPrompt retrieves a prompt by key
func (e *Engine) GetPrompt(ctx context.Context, key string) (*Prompt, error) {
	e.mu.RLock()
	prompt, exists := e.prompts[key]
	e.mu.RUnlock()

	if exists {
		return prompt, nil
	}

	if e.db != nil {
		var p Prompt
		if err := e.db.Where("key = ?", key).First(&p).Error; err != nil {
			return nil, fmt.Errorf("prompt not found: %s", key)
		}
		e.mu.Lock()
		e.prompts[key] = &p
		e.mu.Unlock()
		return &p, nil
	}

	return nil, fmt.Errorf("prompt not found: %s", key)
}

// ListPrompts lists all prompts
func (e *Engine) ListPrompts(ctx context.Context, tenantID string, category PromptCategory) ([]*Prompt, error) {
	if e.db != nil {
		query := e.db.Model(&Prompt{})
		if tenantID != "" {
			query = query.Where("tenant_id = ?", tenantID)
		}
		if category != "" {
			query = query.Where("category = ?", category)
		}

		var prompts []*Prompt
		if err := query.Order("created_at DESC").Find(&prompts).Error; err != nil {
			return nil, fmt.Errorf("list prompts: %w", err)
		}
		return prompts, nil
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []*Prompt
	for _, p := range e.prompts {
		if tenantID != "" && p.TenantID != tenantID {
			continue
		}
		if category != "" && p.Category != category {
			continue
		}
		result = append(result, p)
	}
	return result, nil
}

// DeletePrompt deletes a prompt and all its versions
func (e *Engine) DeletePrompt(ctx context.Context, key string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	prompt, exists := e.prompts[key]
	if !exists {
		return fmt.Errorf("prompt not found: %s", key)
	}

	if e.db != nil {
		// Delete all versions first
		if err := e.db.Where("prompt_id = ?", prompt.ID).Delete(&PromptVersion{}).Error; err != nil {
			return fmt.Errorf("delete versions: %w", err)
		}
		// Delete performance records
		if err := e.db.Where("version_id IN (SELECT id FROM prompt_versions WHERE prompt_id = ?)", prompt.ID).Delete(&UsageRecord{}).Error; err != nil {
			// Ignore error, might not exist
		}
		// Delete prompt
		if err := e.db.Delete(prompt).Error; err != nil {
			return fmt.Errorf("delete prompt: %w", err)
		}
	}

	delete(e.prompts, key)
	delete(e.versions, prompt.ID)
	delete(e.versionList, prompt.ID)

	return nil
}

// CreateVersion creates a new version of a prompt
func (e *Engine) CreateVersion(ctx context.Context, version *PromptVersion) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	prompt, exists := e.prompts[version.PromptID]
	if !exists {
		// Try to load from DB
		if e.db != nil {
			var p Prompt
			if err := e.db.Where("id = ?", version.PromptID).First(&p).Error; err != nil {
				return fmt.Errorf("prompt not found: %s", version.PromptID)
			}
			prompt = &p
			e.prompts[p.Key] = prompt
		} else {
			return fmt.Errorf("prompt not found: %s", version.PromptID)
		}
	}

	if version.ID == "" {
		version.ID = uuid.New().String()
	}
	version.CreatedAt = time.Now()
	if version.Status == "" {
		version.Status = VersionStatusDraft
	}

	// Infer variables if not provided
	if version.Variables == "" {
		schema := e.renderer.InferSchema(version.Content)
		schemaBytes, _ := json.Marshal(schema)
		version.Variables = string(schemaBytes)
	}

	if e.db != nil {
		if err := e.db.Create(version).Error; err != nil {
			return fmt.Errorf("create version: %w", err)
		}
	}

	e.versionList[prompt.ID] = append(e.versionList[prompt.ID], *version)

	// Update prompt's UpdatedAt
	prompt.UpdatedAt = time.Now()
	if e.db != nil {
		e.db.Model(prompt).Update("updated_at", time.Now())
	}

	return nil
}

// GetVersion retrieves a specific version
func (e *Engine) GetVersion(ctx context.Context, versionID string) (*PromptVersion, error) {
	// Check cache first
	e.mu.RLock()
	for _, versions := range e.versionList {
		for _, v := range versions {
			if v.ID == versionID {
				e.mu.RUnlock()
				return &v, nil
			}
		}
	}
	e.mu.RUnlock()

	if e.db != nil {
		var v PromptVersion
		if err := e.db.Where("id = ?", versionID).First(&v).Error; err != nil {
			return nil, fmt.Errorf("version not found: %s", versionID)
		}
		return &v, nil
	}

	return nil, fmt.Errorf("version not found: %s", versionID)
}

// GetActiveVersion retrieves the active version for a prompt
func (e *Engine) GetActiveVersion(ctx context.Context, promptKey string) (*PromptVersion, error) {
	prompt, err := e.GetPrompt(ctx, promptKey)
	if err != nil {
		return nil, err
	}

	e.mu.RLock()
	version, exists := e.versions[prompt.ID]
	e.mu.RUnlock()

	if exists {
		return version, nil
	}

	if e.db != nil {
		var v PromptVersion
		if err := e.db.Where("prompt_id = ? AND is_active = ?", prompt.ID, true).First(&v).Error; err != nil {
			// Return latest version if no active version
			if err := e.db.Where("prompt_id = ?", prompt.ID).Order("created_at DESC").First(&v).Error; err != nil {
				return nil, fmt.Errorf("no version found for prompt: %s", promptKey)
			}
		}
		e.mu.Lock()
		e.versions[prompt.ID] = &v
		e.mu.Unlock()
		return &v, nil
	}

	// Return latest version from in-memory list
	e.mu.RLock()
	versions := e.versionList[prompt.ID]
	e.mu.RUnlock()

	if len(versions) == 0 {
		return nil, fmt.Errorf("no version found for prompt: %s", promptKey)
	}

	return &versions[0], nil
}

// ListVersions lists all versions of a prompt
func (e *Engine) ListVersions(ctx context.Context, promptKey string) ([]PromptVersion, error) {
	prompt, err := e.GetPrompt(ctx, promptKey)
	if err != nil {
		return nil, err
	}

	e.mu.RLock()
	versions, exists := e.versionList[prompt.ID]
	e.mu.RUnlock()

	if exists && len(versions) > 0 {
		return versions, nil
	}

	if e.db != nil {
		var v []PromptVersion
		if err := e.db.Where("prompt_id = ?", prompt.ID).Order("created_at DESC").Find(&v).Error; err != nil {
			return nil, fmt.Errorf("list versions: %w", err)
		}
		e.mu.Lock()
		e.versionList[prompt.ID] = v
		e.mu.Unlock()
		return v, nil
	}

	return nil, fmt.Errorf("no versions found for prompt: %s", promptKey)
}

// ActivateVersion activates a specific version
func (e *Engine) ActivateVersion(ctx context.Context, versionID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Find the version
	var targetVersion *PromptVersion
	var promptID string

	for pid, versions := range e.versionList {
		for i, v := range versions {
			if v.ID == versionID {
				targetVersion = &versions[i]
				promptID = pid
				break
			}
		}
	}

	if targetVersion == nil {
		if e.db != nil {
			var v PromptVersion
			if err := e.db.Where("id = ?", versionID).First(&v).Error; err != nil {
				return fmt.Errorf("version not found: %s", versionID)
			}
			targetVersion = &v
			promptID = v.PromptID
		} else {
			return fmt.Errorf("version not found: %s", versionID)
		}
	}

	// Deactivate all other versions
	if e.db != nil {
		if err := e.db.Model(&PromptVersion{}).
			Where("prompt_id = ? AND id != ?", promptID, versionID).
			Updates(map[string]interface{}{"is_active": false, "status": VersionStatusArchived}).Error; err != nil {
			return fmt.Errorf("deactivate other versions: %w", err)
		}

		if err := e.db.Model(targetVersion).
			Updates(map[string]interface{}{"is_active": true, "status": VersionStatusActive}).Error; err != nil {
			return fmt.Errorf("activate version: %w", err)
		}
	}

	// Update in-memory cache
	targetVersion.IsActive = true
	targetVersion.Status = VersionStatusActive

	for pid, versions := range e.versionList {
		if pid == promptID {
			for i := range versions {
				if versions[i].ID == versionID {
					versions[i].IsActive = true
					versions[i].Status = VersionStatusActive
				} else if versions[i].IsActive {
					versions[i].IsActive = false
					versions[i].Status = VersionStatusArchived
				}
			}
			break
		}
	}

	// Update active version cache
	e.versions[promptID] = targetVersion

	return nil
}

// ArchiveVersion archives a specific version
func (e *Engine) ArchiveVersion(ctx context.Context, versionID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.db != nil {
		if err := e.db.Model(&PromptVersion{}).
			Where("id = ?", versionID).
			Updates(map[string]interface{}{"is_active": false, "status": VersionStatusArchived}).Error; err != nil {
			return fmt.Errorf("archive version: %w", err)
		}
	}

	// Update in-memory cache
	for pid, versions := range e.versionList {
		for i := range versions {
			if versions[i].ID == versionID {
				versions[i].IsActive = false
				versions[i].Status = VersionStatusArchived
				// Remove from active cache if it was active
				if e.versions[pid] != nil && e.versions[pid].ID == versionID {
					delete(e.versions, pid)
				}
				break
			}
		}
	}

	return nil
}

// RollbackVersion reverts to a previous version
func (e *Engine) RollbackVersion(ctx context.Context, versionID string) error {
	// Simply activate the specified version
	return e.ActivateVersion(ctx, versionID)
}

// CompareVersions compares two versions and returns the diff
func (e *Engine) CompareVersions(ctx context.Context, version1ID, version2ID string) (*VersionDiff, error) {
	v1, err := e.GetVersion(ctx, version1ID)
	if err != nil {
		return nil, fmt.Errorf("get version1: %w", err)
	}

	v2, err := e.GetVersion(ctx, version2ID)
	if err != nil {
		return nil, fmt.Errorf("get version2: %w", err)
	}

	diff := &VersionDiff{
		Version1: version1ID,
		Version2: version2ID,
	}

	// Compare content
	diff.ContentDiff = compareContent(v1.Content, v2.Content)

	// Compare variables
	diff.VarDiff = compareVariables(v1.Variables, v2.Variables)

	// Generate summary
	diff.Summary = generateDiffSummary(diff)

	return diff, nil
}

// RenderPrompt renders a prompt with variables
func (e *Engine) RenderPrompt(ctx context.Context, promptKey string, vars map[string]interface{}) (string, error) {
	version, err := e.GetActiveVersion(ctx, promptKey)
	if err != nil {
		return "", err
	}

	renderCtx := &RenderContext{
		Variables: vars,
	}

	return e.renderer.RenderWithValidation(version.Content, version.Variables, renderCtx)
}

// RenderPromptWithValidation renders and validates variables
func (e *Engine) RenderPromptWithValidation(ctx context.Context, promptKey string, vars map[string]interface{}) (string, []string, error) {
	version, err := e.GetActiveVersion(ctx, promptKey)
	if err != nil {
		return "", nil, err
	}

	renderCtx := &RenderContext{
		Variables: vars,
	}

	// Validate variables first
	warnings, err := e.renderer.ValidateVariables(version.Variables, vars)
	if err != nil {
		return "", nil, err
	}

	// Merge with defaults
	mergedVars := e.renderer.MergeVariables(version.Variables, vars)
	renderCtx.Variables = mergedVars

	// Render template
	result, err := e.renderer.Render(version.Content, renderCtx)
	if err != nil {
		return "", warnings, err
	}

	return result, warnings, nil
}

// RecordUsage records usage for performance tracking
func (e *Engine) RecordUsage(ctx context.Context, versionID, sessionID string, success bool, latencyMs int64, inputTokens, outputTokens int64, cost float64, userRating float64) error {
	return e.tracker.RecordUsage(ctx, versionID, sessionID, success, latencyMs, inputTokens, outputTokens, cost, userRating, nil)
}

// GetPerformance gets performance metrics for a version
func (e *Engine) GetPerformance(ctx context.Context, versionID string, periodStart, periodEnd time.Time) (*PromptPerformance, error) {
	return e.tracker.GetPerformance(ctx, versionID, periodStart, periodEnd)
}

// GetPerformanceTrend gets performance trend for a version
func (e *Engine) GetPerformanceTrend(ctx context.Context, versionID string, days int) (*PerformanceTrend, error) {
	return e.tracker.GetPerformanceTrend(ctx, versionID, days)
}

// GetRenderer returns the renderer for direct use
func (e *Engine) GetRenderer() *Renderer {
	return e.renderer
}

// GetTracker returns the performance tracker
func (e *Engine) GetTracker() *PerformanceTracker {
	return e.tracker
}

// Helper functions

func compareContent(content1, content2 string) []DiffLine {
	lines1 := strings.Split(content1, "\n")
	lines2 := strings.Split(content2, "\n")

	var diff []DiffLine
	maxLen := max(len(lines1), len(lines2))

	for i := 0; i < maxLen; i++ {
		var line1, line2 string
		if i < len(lines1) {
			line1 = lines1[i]
		}
		if i < len(lines2) {
			line2 = lines2[i]
		}

		if line1 == line2 {
			diff = append(diff, DiffLine{Type: "unchanged", Content: line1})
		} else {
			if line1 != "" {
				diff = append(diff, DiffLine{Type: "remove", Content: line1})
			}
			if line2 != "" {
				diff = append(diff, DiffLine{Type: "add", Content: line2})
			}
		}
	}

	return diff
}

func compareVariables(vars1, vars2 string) []VarDiff {
	var v1, v2 VariableSet
	if vars1 != "" {
		json.Unmarshal([]byte(vars1), &v1)
	}
	if vars2 != "" {
		json.Unmarshal([]byte(vars2), &v2)
	}

	var diff []VarDiff

	// Build maps
	map1 := make(map[string]Variable)
	for _, v := range v1.Variables {
		map1[v.Name] = v
	}
	map2 := make(map[string]Variable)
	for _, v := range v2.Variables {
		map2[v.Name] = v
	}

	// Find added/removed/changed
	for name, v2Var := range map2 {
		if v1Var, exists := map1[name]; !exists {
			diff = append(diff, VarDiff{Name: name, Type: "added", NewValue: v2Var})
		} else if !variablesEqual(v1Var, v2Var) {
			diff = append(diff, VarDiff{Name: name, Type: "changed", OldValue: v1Var, NewValue: v2Var})
		}
	}

	for name, v1Var := range map1 {
		if _, exists := map2[name]; !exists {
			diff = append(diff, VarDiff{Name: name, Type: "removed", OldValue: v1Var})
		}
	}

	return diff
}

func variablesEqual(v1, v2 Variable) bool {
	return v1.Name == v2.Name &&
		v1.Type == v2.Type &&
		v1.Required == v2.Required &&
		v1.Description == v2.Description
}

func generateDiffSummary(diff *VersionDiff) string {
	addCount := 0
	removeCount := 0
	for _, line := range diff.ContentDiff {
		if line.Type == "add" {
			addCount++
		} else if line.Type == "remove" {
			removeCount++
		}
	}

	varAdded := 0
	varRemoved := 0
	varChanged := 0
	for _, v := range diff.VarDiff {
		switch v.Type {
		case "added":
			varAdded++
		case "removed":
			varRemoved++
		case "changed":
			varChanged++
		}
	}

	return fmt.Sprintf("Content: +%d/-%d lines, Variables: +%d/-%d/%d changed",
		addCount, removeCount, varAdded, varRemoved, varChanged)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}