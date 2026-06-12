// Package slo provides SLO (Service Level Objective) management
package slo

import (
	"sync"
	"time"

	pb "agent-platform/pkg/pb/harness"
)

// SLODefinition defines an SLO
type SLODefinition struct {
	ID          string
	AgentID     string
	Name        string
	Type        string  // "latency", "success_rate", "availability"
	Target      float64 // e.g., 0.999 for 99.9%
	Window      time.Duration
	AlertThreshold float64 // Alert when burn rate exceeds this
}

// SLOStatus tracks current SLO status
type SLOStatus struct {
	Name            string
	Target          float64
	Current         float64
	BudgetRemaining float64
	Status          string // "healthy", "warning", "critical"
	BurnRate        float64
}

// Manager is the SLO manager
type Manager struct {
	definitions map[string]*SLODefinition
	statuses    map[string]*sloStatusInternal
	metrics     map[string]*metricBuffer
	mu          sync.RWMutex
}

type sloStatusInternal struct {
	current         float64
	target          float64
	budgetRemaining float64
	status          string
	burnRate        float64
	totalRequests   int64
	failedRequests  int64
	totalLatency    float64
}

type metricBuffer struct {
	latencies []float64
	successes []bool
	timestamps []time.Time
}

// NewManager creates a new SLO manager
func NewManager() *Manager {
	return &Manager{
		definitions: make(map[string]*SLODefinition),
		statuses:    make(map[string]*sloStatusInternal),
		metrics:     make(map[string]*metricBuffer),
	}
}

// RegisterSLO registers an SLO definition
func (m *Manager) RegisterSLO(def *SLODefinition) {
	if def == nil || def.ID == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	m.definitions[def.ID] = def
	m.statuses[def.ID] = &sloStatusInternal{
		target:          def.Target,
		budgetRemaining: 1.0,
		status:          "healthy",
	}
	m.metrics[def.ID] = &metricBuffer{
		latencies:  make([]float64, 0, 1000),
		successes:  make([]bool, 0, 1000),
		timestamps: make([]time.Time, 0, 1000),
	}
}

// RecordLatency records a latency measurement
func (m *Manager) RecordLatency(sloID string, latencyMs float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	buf, exists := m.metrics[sloID]
	if !exists {
		return
	}

	buf.latencies = append(buf.latencies, latencyMs)
	buf.timestamps = append(buf.timestamps, time.Now())

	// Keep only last 1000 entries
	if len(buf.latencies) > 1000 {
		buf.latencies = buf.latencies[1:]
		buf.timestamps = buf.timestamps[1:]
	}

	m.recalculateStatus(sloID)
}

// RecordSuccess records a success/failure
func (m *Manager) RecordSuccess(sloID string, success bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	buf, exists := m.metrics[sloID]
	if !exists {
		return
	}

	buf.successes = append(buf.successes, success)
	if !success {
		if status, ok := m.statuses[sloID]; ok {
			status.failedRequests++
		}
	}
	if status, ok := m.statuses[sloID]; ok {
		status.totalRequests++
	}

	// Keep only last 1000 entries
	if len(buf.successes) > 1000 {
		buf.successes = buf.successes[1:]
	}

	m.recalculateStatus(sloID)
}

// recalculateStatus recalculates SLO status
func (m *Manager) recalculateStatus(sloID string) {
	def, exists := m.definitions[sloID]
	if !exists {
		return
	}

	buf := m.metrics[sloID]
	status := m.statuses[sloID]

	switch def.Type {
	case "latency":
		if len(buf.latencies) == 0 {
			return
		}
		// Calculate p99 latency
		status.current = calculatePercentile(buf.latencies, 99)
		// Budget = target / current
		if status.current > 0 {
			status.budgetRemaining = def.Target / status.current
		}

	case "success_rate":
		if len(buf.successes) == 0 {
			return
		}
		// Calculate success rate
		successCount := 0
		for _, s := range buf.successes {
			if s {
				successCount++
			}
		}
		status.current = float64(successCount) / float64(len(buf.successes))
		// Budget = (current - target) / (1 - target)
		if def.Target < 1 {
			status.budgetRemaining = (status.current - def.Target) / (1 - def.Target)
		}

	case "availability":
		if status.totalRequests == 0 {
			return
		}
		status.current = 1 - float64(status.failedRequests)/float64(status.totalRequests)
		if def.Target < 1 {
			status.budgetRemaining = (status.current - def.Target) / (1 - def.Target)
		}
	}

	// Determine status
	if status.budgetRemaining < 0 {
		status.status = "critical"
	} else if status.budgetRemaining < 0.2 {
		status.status = "warning"
	} else {
		status.status = "healthy"
	}

	// Calculate burn rate (simplified)
	if len(buf.timestamps) >= 2 {
		timeDiff := buf.timestamps[len(buf.timestamps)-1].Sub(buf.timestamps[0]).Hours()
		if timeDiff > 0 {
			status.burnRate = (1 - status.current) / timeDiff
		}
	}
}

// GetStatus gets SLO status
func (m *Manager) GetStatus(agentID string) (*pb.GetSLOStatusResponse, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var statuses []*pb.SLOStatus

	for id, def := range m.definitions {
		if agentID != "" && def.AgentID != agentID {
			continue
		}

		status := m.statuses[id]
		if status == nil {
			continue
		}

		statuses = append(statuses, &pb.SLOStatus{
			Name:            def.Name,
			Current:         status.current,
			Target:          status.target,
			BudgetRemaining: status.budgetRemaining,
			Status:          status.status,
		})
	}

	return &pb.GetSLOStatusResponse{Statuses: statuses}, nil
}

// GetAlertStatus returns if any SLO is in alert state
func (m *Manager) GetAlertStatus() map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	alerts := make(map[string]string)
	for id, status := range m.statuses {
		if status.status != "healthy" {
			if def, ok := m.definitions[id]; ok {
				alerts[def.Name] = status.status
			}
		}
	}
	return alerts
}

// calculatePercentile calculates a percentile value
func calculatePercentile(values []float64, percentile float64) float64 {
	if len(values) == 0 {
		return 0
	}

	// Simple implementation - sort and pick
	sorted := make([]float64, len(values))
	copy(sorted, values)

	// Sort using simple insertion sort
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j] < sorted[j-1]; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}

	index := int(float64(len(sorted)-1) * percentile / 100)
	if index >= len(sorted) {
		index = len(sorted) - 1
	}

	return sorted[index]
}
