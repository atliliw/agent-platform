// Package scheduler provides scheduled evaluation functionality
package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ScheduleStatus represents the status of a schedule
type ScheduleStatus string

const (
	ScheduleStatusActive  ScheduleStatus = "active"
	ScheduleStatusPaused  ScheduleStatus = "paused"
	ScheduleStatusStopped ScheduleStatus = "stopped"
)

// ScheduleType represents the type of schedule
type ScheduleType string

const (
	ScheduleTypeCron     ScheduleType = "cron"     // Cron expression
	ScheduleTypeInterval ScheduleType = "interval" // Fixed interval (seconds)
	ScheduleTypeOnce     ScheduleType = "once"     // One-time execution
)

// EvalType represents the type of evaluation to run
type EvalType string

const (
	EvalTypeABTest      EvalType = "abtest"      // A/B test evaluation
	EvalTypeSLO         EvalType = "slo"         // SLO evaluation
	EvalTypeFeatureFlag EvalType = "featureflag" // Feature flag stale detection
	EvalTypeCost        EvalType = "cost"        // Cost analysis
	EvalTypeChaos       EvalType = "chaos"       // Chaos experiment check
	EvalTypeAll         EvalType = "all"         // Run all evaluations
)

// EvalSchedule represents a scheduled evaluation
type EvalSchedule struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Type         ScheduleType      `json:"type"`
	EvalType     EvalType          `json:"eval_type"`
	AgentID      string            `json:"agent_id"`
	ScheduleExpr string            `json:"schedule_expr"` // Cron expression or interval seconds
	Status       ScheduleStatus    `json:"status"`
	LastRunAt    *time.Time        `json:"last_run_at"`
	NextRunAt    *time.Time        `json:"next_run_at"`
	RunCount     int64             `json:"run_count"`
	LastResult   *EvalResult       `json:"last_result"`
	Enabled      bool              `json:"enabled"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
	Metadata     map[string]string `json:"metadata"`
}

// EvalResult represents the result of a scheduled evaluation
type EvalResult struct {
	ID         string    `json:"id"`
	ScheduleID string    `json:"schedule_id"`
	EvalType   EvalType  `json:"eval_type"`
	Success    bool      `json:"success"`
	Score      float64   `json:"score"`
	Details    string    `json:"details"`
	Alerts     []string  `json:"alerts"`
	DurationMs int64     `json:"duration_ms"`
	Timestamp  time.Time `json:"timestamp"`
}

// SchedulerStatus represents the overall scheduler status
type SchedulerStatus struct {
	Running          bool       `json:"running"`
	ActiveSchedules  int        `json:"active_schedules"`
	TotalRuns        int64      `json:"total_runs"`
	LastRunAt        *time.Time `json:"last_run_at"`
	NextScheduledRun *time.Time `json:"next_scheduled_run"`
	UptimeSeconds    int64      `json:"uptime_seconds"`
}

// EvalRunnerFunc is the function type for running evaluations
type EvalRunnerFunc func(ctx context.Context, evalType EvalType, agentID string) (*EvalResult, error)

// Scheduler is the scheduler engine
type Scheduler struct {
	schedules  map[string]*EvalSchedule
	results    map[string][]*EvalResult
	runnerFunc EvalRunnerFunc
	stopChan   chan struct{}
	running    bool
	startTime  time.Time
	mu         sync.RWMutex
}

// NewScheduler creates a new scheduler
func NewScheduler(runnerFunc EvalRunnerFunc) *Scheduler {
	return &Scheduler{
		schedules:  make(map[string]*EvalSchedule),
		results:    make(map[string][]*EvalResult),
		runnerFunc: runnerFunc,
		stopChan:   make(chan struct{}),
	}
}

// NewSchedulerMemory creates a new scheduler with default runner
func NewSchedulerMemory() *Scheduler {
	// Default runner that returns mock results
	defaultRunner := func(ctx context.Context, evalType EvalType, agentID string) (*EvalResult, error) {
		return &EvalResult{
			ID:        uuid.New().String(),
			EvalType:  evalType,
			Success:   true,
			Score:     0.85,
			Details:   fmt.Sprintf("Evaluation completed for %s on agent %s", evalType, agentID),
			Timestamp: time.Now(),
		}, nil
	}
	return NewScheduler(defaultRunner)
}

// SetEvalRunner sets the evaluation runner function
func (s *Scheduler) SetEvalRunner(runnerFunc EvalRunnerFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runnerFunc = runnerFunc
}

// SetEvalSchedule creates or updates an evaluation schedule
func (s *Scheduler) SetEvalSchedule(ctx context.Context, schedule *EvalSchedule) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Generate ID if not provided
	if schedule.ID == "" {
		schedule.ID = uuid.New().String()
	}

	// Set timestamps
	now := time.Now()
	if schedule.CreatedAt.IsZero() {
		schedule.CreatedAt = now
	}
	schedule.UpdatedAt = now

	// Calculate next run time
	if schedule.Status == ScheduleStatusActive && schedule.Enabled {
		schedule.NextRunAt = s.calculateNextRun(schedule)
	}

	// Store schedule
	s.schedules[schedule.ID] = schedule

	return nil
}

// GetSchedule gets a schedule by ID
func (s *Scheduler) GetSchedule(ctx context.Context, id string) (*EvalSchedule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	schedule, ok := s.schedules[id]
	if !ok {
		return nil, fmt.Errorf("schedule not found: %s", id)
	}
	return schedule, nil
}

// ListSchedules lists all schedules
func (s *Scheduler) ListSchedules(ctx context.Context, agentID string, status ScheduleStatus) ([]*EvalSchedule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var schedules []*EvalSchedule
	for _, schedule := range s.schedules {
		// Filter by agent ID
		if agentID != "" && schedule.AgentID != agentID {
			continue
		}
		// Filter by status
		if status != "" && schedule.Status != status {
			continue
		}
		schedules = append(schedules, schedule)
	}
	return schedules, nil
}

// PauseSchedule pauses a schedule
func (s *Scheduler) PauseSchedule(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	schedule, ok := s.schedules[id]
	if !ok {
		return fmt.Errorf("schedule not found: %s", id)
	}

	schedule.Status = ScheduleStatusPaused
	schedule.NextRunAt = nil
	schedule.UpdatedAt = time.Now()

	return nil
}

// ResumeSchedule resumes a paused schedule
func (s *Scheduler) ResumeSchedule(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	schedule, ok := s.schedules[id]
	if !ok {
		return fmt.Errorf("schedule not found: %s", id)
	}

	schedule.Status = ScheduleStatusActive
	schedule.Enabled = true
	schedule.NextRunAt = s.calculateNextRun(schedule)
	schedule.UpdatedAt = time.Now()

	return nil
}

// DeleteSchedule deletes a schedule
func (s *Scheduler) DeleteSchedule(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.schedules[id]; !ok {
		return fmt.Errorf("schedule not found: %s", id)
	}

	delete(s.schedules, id)
	return nil
}

// SchedulerStatus returns the scheduler status
func (s *Scheduler) SchedulerStatus(ctx context.Context) (*SchedulerStatus, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := &SchedulerStatus{
		Running:         s.running,
		ActiveSchedules: 0,
		TotalRuns:       0,
		UptimeSeconds:   0,
	}

	if s.running {
		status.UptimeSeconds = int64(time.Since(s.startTime).Seconds())
	}

	// Count active schedules
	for _, schedule := range s.schedules {
		if schedule.Status == ScheduleStatusActive {
			status.ActiveSchedules++
		}
		status.TotalRuns += schedule.RunCount
	}

	// Find next scheduled run
	var nextRun *time.Time
	for _, schedule := range s.schedules {
		if schedule.Status == ScheduleStatusActive && schedule.NextRunAt != nil {
			if nextRun == nil || schedule.NextRunAt.Before(*nextRun) {
				nextRun = schedule.NextRunAt
			}
		}
	}
	status.NextScheduledRun = nextRun

	// Find last run
	for _, schedule := range s.schedules {
		if schedule.LastRunAt != nil {
			if status.LastRunAt == nil || schedule.LastRunAt.After(*status.LastRunAt) {
				status.LastRunAt = schedule.LastRunAt
			}
		}
	}

	return status, nil
}

// Start starts the scheduler
func (s *Scheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("scheduler already running")
	}
	s.running = true
	s.startTime = time.Now()
	s.mu.Unlock()

	// Start scheduler loop
	go s.schedulerLoop(ctx)

	return nil
}

// Stop stops the scheduler
func (s *Scheduler) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return fmt.Errorf("scheduler not running")
	}

	s.stopChan <- struct{}{}
	s.running = false

	return nil
}

// RunNow manually triggers a schedule run
func (s *Scheduler) RunNow(ctx context.Context, id string) (*EvalResult, error) {
	s.mu.Lock()
	schedule, ok := s.schedules[id]
	if !ok {
		s.mu.Unlock()
		return nil, fmt.Errorf("schedule not found: %s", id)
	}
	s.mu.Unlock()

	// Run evaluation
	result, err := s.runEvaluation(ctx, schedule)
	if err != nil {
		return nil, err
	}

	// Update schedule
	s.mu.Lock()
	schedule.LastRunAt = &result.Timestamp
	schedule.RunCount++
	schedule.LastResult = result
	if schedule.Status == ScheduleStatusActive {
		schedule.NextRunAt = s.calculateNextRun(schedule)
	}
	s.mu.Unlock()

	// Store result
	s.storeResult(id, result)

	return result, nil
}

// GetResults gets results for a schedule
func (s *Scheduler) GetResults(ctx context.Context, scheduleID string, limit int) ([]*EvalResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	results, ok := s.results[scheduleID]
	if !ok {
		return nil, nil
	}

	if limit > 0 && len(results) > limit {
		return results[:limit], nil
	}
	return results, nil
}

// schedulerLoop is the main scheduler loop
func (s *Scheduler) schedulerLoop(ctx context.Context) {
	// Check every 10 seconds
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.checkAndRun(ctx)
		}
	}
}

// checkAndRun checks for schedules that need to run
func (s *Scheduler) checkAndRun(ctx context.Context) {
	s.mu.RLock()
	schedules := make([]*EvalSchedule, 0)
	for _, schedule := range s.schedules {
		if schedule.Status == ScheduleStatusActive && schedule.Enabled {
			schedules = append(schedules, schedule)
		}
	}
	s.mu.RUnlock()

	now := time.Now()
	for _, schedule := range schedules {
		if schedule.NextRunAt != nil && schedule.NextRunAt.Before(now) || schedule.NextRunAt.Equal(now) {
			// Run evaluation
			result, err := s.runEvaluation(ctx, schedule)
			if err != nil {
				result = &EvalResult{
					ID:         uuid.New().String(),
					ScheduleID: schedule.ID,
					EvalType:   schedule.EvalType,
					Success:    false,
					Details:    fmt.Sprintf("Error: %v", err),
					Timestamp:  now,
				}
			}

			// Update schedule
			s.mu.Lock()
			schedule.LastRunAt = &now
			schedule.RunCount++
			schedule.LastResult = result
			if schedule.Type != ScheduleTypeOnce {
				schedule.NextRunAt = s.calculateNextRun(schedule)
			} else {
				schedule.Status = ScheduleStatusStopped
				schedule.NextRunAt = nil
			}
			s.mu.Unlock()

			// Store result
			s.storeResult(schedule.ID, result)
		}
	}
}

// runEvaluation runs an evaluation
func (s *Scheduler) runEvaluation(ctx context.Context, schedule *EvalSchedule) (*EvalResult, error) {
	if s.runnerFunc == nil {
		return nil, fmt.Errorf("no evaluation runner configured")
	}

	startTime := time.Now()
	result, err := s.runnerFunc(ctx, schedule.EvalType, schedule.AgentID)
	if err != nil {
		return nil, err
	}

	// Fill in schedule ID and duration
	result.ID = uuid.New().String()
	result.ScheduleID = schedule.ID
	result.DurationMs = int64(time.Since(startTime).Milliseconds())

	return result, nil
}

// calculateNextRun calculates the next run time
func (s *Scheduler) calculateNextRun(schedule *EvalSchedule) *time.Time {
	now := time.Now()

	switch schedule.Type {
	case ScheduleTypeInterval:
		// Parse interval as seconds
		var seconds int
		if _, err := fmt.Sscanf(schedule.ScheduleExpr, "%d", &seconds); err != nil || seconds <= 0 {
			seconds = 60 // Default 1 minute
		}
		next := now.Add(time.Duration(seconds) * time.Second)
		return &next

	case ScheduleTypeCron:
		// For simplicity, parse common cron patterns
		// Full cron parsing would require a library like robfig/cron
		// Support basic patterns: "hourly", "daily", "weekly", or custom minutes
		switch schedule.ScheduleExpr {
		case "hourly":
			next := now.Add(1 * time.Hour)
			return &next
		case "daily":
			next := now.Add(24 * time.Hour)
			return &next
		case "weekly":
			next := now.Add(7 * 24 * time.Hour)
			return &next
		default:
			// Try to parse as minutes
			var minutes int
			if _, err := fmt.Sscanf(schedule.ScheduleExpr, "%d", &minutes); err != nil || minutes <= 0 {
				minutes = 5 // Default 5 minutes
			}
			next := now.Add(time.Duration(minutes) * time.Minute)
			return &next
		}

	case ScheduleTypeOnce:
		// One-time execution - parse the scheduled time
		if schedule.ScheduleExpr != "" {
			t, err := time.Parse(time.RFC3339, schedule.ScheduleExpr)
			if err == nil {
				return &t
			}
		}
		// Default: run immediately
		return &now

	default:
		// Default: 5 minutes
		next := now.Add(5 * time.Minute)
		return &next
	}
}

// storeResult stores an evaluation result
func (s *Scheduler) storeResult(scheduleID string, result *EvalResult) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.results[scheduleID]; !ok {
		s.results[scheduleID] = make([]*EvalResult, 0)
	}
	s.results[scheduleID] = append(s.results[scheduleID], result)

	// Keep only last 100 results
	if len(s.results[scheduleID]) > 100 {
		s.results[scheduleID] = s.results[scheduleID][1:]
	}
}

// GetScheduleByAgent gets schedules for a specific agent
func (s *Scheduler) GetScheduleByAgent(ctx context.Context, agentID string) ([]*EvalSchedule, error) {
	return s.ListSchedules(ctx, agentID, "")
}

// GetActiveSchedules gets all active schedules
func (s *Scheduler) GetActiveSchedules(ctx context.Context) ([]*EvalSchedule, error) {
	return s.ListSchedules(ctx, "", ScheduleStatusActive)
}

// ClearResults clears all results for a schedule
func (s *Scheduler) ClearResults(ctx context.Context, scheduleID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.results, scheduleID)
	return nil
}

// GetStats returns scheduler statistics
func (s *Scheduler) GetStats(ctx context.Context) map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := map[string]interface{}{
		"total_schedules":   len(s.schedules),
		"active_schedules":  0,
		"paused_schedules":  0,
		"stopped_schedules": 0,
		"total_results":     0,
		"running":           s.running,
	}

	for _, schedule := range s.schedules {
		switch schedule.Status {
		case ScheduleStatusActive:
			stats["active_schedules"] = stats["active_schedules"].(int) + 1
		case ScheduleStatusPaused:
			stats["paused_schedules"] = stats["paused_schedules"].(int) + 1
		case ScheduleStatusStopped:
			stats["stopped_schedules"] = stats["stopped_schedules"].(int) + 1
		}
	}

	for _, results := range s.results {
		stats["total_results"] = stats["total_results"].(int) + len(results)
	}

	return stats
}
