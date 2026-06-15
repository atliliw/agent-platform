// Package episodic provides episodic memory functionality
// Episodic memory stores event sequences and timelines
package episodic

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
)

// EpisodeType defines the type of episode
type EpisodeType string

const (
	EpisodeTypeConversation EpisodeType = "conversation"
	EpisodeTypeAction       EpisodeType = "action"
	EpisodeTypeObservation  EpisodeType = "observation"
	EpisodeTypeDecision     EpisodeType = "decision"
	EpisodeTypeError        EpisodeType = "error"
	EpisodeTypeSuccess      EpisodeType = "success"
)

// Episode represents a single episode in episodic memory
type Episode struct {
	ID          string                 `json:"id"`
	SessionID   string                 `json:"session_id"`
	AgentID     string                 `json:"agent_id"`
	Type        EpisodeType            `json:"type"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	StartTime   time.Time              `json:"start_time"`
	EndTime     time.Time              `json:"end_time"`
	Participants []string              `json:"participants"` // agents, users involved
	Location    string                 `json:"location"`     // context/location
	Emotions    map[string]float64     `json:"emotions"`     // emotional context
	Outcome     string                 `json:"outcome"`      // success, failure, neutral
	Importance  float64                `json:"importance"`   // 0-1 importance score
	Metadata    map[string]interface{} `json:"metadata"`
	Vector      []float64              `json:"vector"`       // embedding for similarity search
	CreatedAt   time.Time              `json:"created_at"`
}

// EpisodeSequence represents a sequence of related episodes
type EpisodeSequence struct {
	ID          string    `json:"id"`
	SessionID   string    `json:"session_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Episodes    []string  `json:"episodes"` // episode IDs in order
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`
	CreatedAt   time.Time `json:"created_at"`
}

// Timeline represents a temporal structure for episodes
type Timeline struct {
	SessionID string                 `json:"session_id"`
	Events    []TimelineEvent        `json:"events"`
	Duration  time.Duration          `json:"duration"`
	Stats     TimelineStats          `json:"stats"`
}

// TimelineEvent represents an event in the timeline
type TimelineEvent struct {
	Timestamp time.Time   `json:"timestamp"`
	EpisodeID string      `json:"episode_id"`
	Type      EpisodeType `json:"type"`
	Summary   string      `json:"summary"`
}

// TimelineStats represents statistics about the timeline
type TimelineStats struct {
	TotalEpisodes   int                    `json:"total_episodes"`
	ByType          map[EpisodeType]int    `json:"by_type"`
	AvgDuration     time.Duration          `json:"avg_duration"`
	SuccessRate     float64                `json:"success_rate"`
	AvgImportance   float64                `json:"avg_importance"`
}

// EpisodicMemory manages episodic memories
type EpisodicMemory struct {
	episodes   map[string]*Episode
	sequences  map[string]*EpisodeSequence
	bySession  map[string][]string // session -> episode IDs
	byAgent    map[string][]string // agent -> episode IDs
	byType     map[EpisodeType][]string
	mu         sync.RWMutex
	maxEpisodes int
}

// NewEpisodicMemory creates a new episodic memory
func NewEpisodicMemory(maxEpisodes int) *EpisodicMemory {
	if maxEpisodes <= 0 {
		maxEpisodes = 10000
	}
	return &EpisodicMemory{
		episodes:    make(map[string]*Episode),
		sequences:   make(map[string]*EpisodeSequence),
		bySession:   make(map[string][]string),
		byAgent:     make(map[string][]string),
		byType:      make(map[EpisodeType][]string),
		maxEpisodes: maxEpisodes,
	}
}

// Store stores a new episode
func (m *EpisodicMemory) Store(ctx context.Context, episode *Episode) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Generate ID if not set
	if episode.ID == "" {
		episode.ID = generateEpisodeID()
	}

	// Set timestamps
	if episode.CreatedAt.IsZero() {
		episode.CreatedAt = time.Now()
	}

	// Check capacity
	if len(m.episodes) >= m.maxEpisodes {
		m.evictOldest()
	}

	// Store episode
	m.episodes[episode.ID] = episode

	// Update indexes
	m.bySession[episode.SessionID] = append(m.bySession[episode.SessionID], episode.ID)
	m.byAgent[episode.AgentID] = append(m.byAgent[episode.AgentID], episode.ID)
	m.byType[episode.Type] = append(m.byType[episode.Type], episode.ID)

	return nil
}

// Get retrieves an episode by ID
func (m *EpisodicMemory) Get(ctx context.Context, id string) (*Episode, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	episode, ok := m.episodes[id]
	if !ok {
		return nil, fmt.Errorf("episode not found: %s", id)
	}
	return episode, nil
}

// GetBySession retrieves episodes for a session
func (m *EpisodicMemory) GetBySession(ctx context.Context, sessionID string) ([]*Episode, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := m.bySession[sessionID]
	episodes := make([]*Episode, 0, len(ids))
	for _, id := range ids {
		if ep, ok := m.episodes[id]; ok {
			episodes = append(episodes, ep)
		}
	}
	return episodes, nil
}

// GetByTimeRange retrieves episodes within a time range
func (m *EpisodicMemory) GetByTimeRange(ctx context.Context, start, end time.Time) ([]*Episode, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var episodes []*Episode
	for _, ep := range m.episodes {
		if (ep.StartTime.Equal(start) || ep.StartTime.After(start)) &&
			(ep.EndTime.Equal(end) || ep.EndTime.Before(end)) {
			episodes = append(episodes, ep)
		}
	}
	return episodes, nil
}

// GetTimeline generates a timeline for a session
func (m *EpisodicMemory) GetTimeline(ctx context.Context, sessionID string) (*Timeline, error) {
	episodes, err := m.GetBySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	timeline := &Timeline{
		SessionID: sessionID,
		Events:    make([]TimelineEvent, 0),
		Stats: TimelineStats{
			ByType: make(map[EpisodeType]int),
		},
	}

	var totalDuration time.Duration
	var totalImportance float64
	var successCount int

	for _, ep := range episodes {
		event := TimelineEvent{
			Timestamp: ep.StartTime,
			EpisodeID: ep.ID,
			Type:      ep.Type,
			Summary:   ep.Title,
		}
		timeline.Events = append(timeline.Events, event)

		// Update stats
		timeline.Stats.ByType[ep.Type]++
		totalDuration += ep.EndTime.Sub(ep.StartTime)
		totalImportance += ep.Importance
		if ep.Outcome == "success" {
			successCount++
		}
	}

	timeline.Stats.TotalEpisodes = len(episodes)
	if len(episodes) > 0 {
		timeline.Stats.AvgDuration = totalDuration / time.Duration(len(episodes))
		timeline.Stats.AvgImportance = totalImportance / float64(len(episodes))
		timeline.Stats.SuccessRate = float64(successCount) / float64(len(episodes))
	}

	return timeline, nil
}

// CreateSequence creates a new episode sequence
func (m *EpisodicMemory) CreateSequence(ctx context.Context, sessionID, name, description string, episodeIDs []string) (*EpisodeSequence, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sequence := &EpisodeSequence{
		ID:          generateSequenceID(),
		SessionID:   sessionID,
		Name:        name,
		Description: description,
		Episodes:    episodeIDs,
		CreatedAt:   time.Now(),
	}

	// Calculate time range
	if len(episodeIDs) > 0 {
		for i, epID := range episodeIDs {
			if ep, ok := m.episodes[epID]; ok {
				if i == 0 || ep.StartTime.Before(sequence.StartTime) {
					sequence.StartTime = ep.StartTime
				}
				if i == 0 || ep.EndTime.After(sequence.EndTime) {
					sequence.EndTime = ep.EndTime
				}
			}
		}
	}

	m.sequences[sequence.ID] = sequence
	return sequence, nil
}

// GetSimilarEpisodes finds similar episodes based on embedding
func (m *EpisodicMemory) GetSimilarEpisodes(ctx context.Context, embedding []float64, topK int) ([]*Episode, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Simple similarity search (in production, use vector DB)
	type scoredEpisode struct {
		episode *Episode
		score   float64
	}

	var scored []scoredEpisode
	for _, ep := range m.episodes {
		if ep.Vector != nil && len(ep.Vector) == len(embedding) {
			score := cosineSimilarity(embedding, ep.Vector)
			scored = append(scored, scoredEpisode{episode: ep, score: score})
		}
	}

	// Sort by score
	for i := 0; i < len(scored)-1; i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].score > scored[i].score {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	// Return top K
	result := make([]*Episode, 0, topK)
	for i := 0; i < topK && i < len(scored); i++ {
		result = append(result, scored[i].episode)
	}

	return result, nil
}

// Search searches episodes by content
func (m *EpisodicMemory) Search(ctx context.Context, query string) ([]*Episode, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []*Episode
	queryLower := strings.ToLower(query)

	for _, ep := range m.episodes {
		// Search in title and description
		if strings.Contains(strings.ToLower(ep.Title), queryLower) ||
			strings.Contains(strings.ToLower(ep.Description), queryLower) {
			results = append(results, ep)
		}
	}

	return results, nil
}

// evictOldest removes the oldest episodes
func (m *EpisodicMemory) evictOldest() {
	// Find oldest episodes (remove 10%)
	removeCount := m.maxEpisodes / 10
	if removeCount < 1 {
		removeCount = 1
	}

	// Find oldest by CreatedAt
	var oldestIDs []string
	for id, ep := range m.episodes {
		if len(oldestIDs) < removeCount {
			oldestIDs = append(oldestIDs, id)
		} else {
			// Check if this one is older
			for i, oldID := range oldestIDs {
				if oldEp, ok := m.episodes[oldID]; ok {
					if ep.CreatedAt.Before(oldEp.CreatedAt) {
						oldestIDs[i] = id
						break
					}
				}
			}
		}
	}

	// Remove oldest
	for _, id := range oldestIDs {
		m.removeEpisode(id)
	}
}

// removeEpisode removes an episode and updates indexes
func (m *EpisodicMemory) removeEpisode(id string) {
	ep, ok := m.episodes[id]
	if !ok {
		return
	}

	// Remove from main map
	delete(m.episodes, id)

	// Update indexes
	m.bySession[ep.SessionID] = removeFromSlice(m.bySession[ep.SessionID], id)
	m.byAgent[ep.AgentID] = removeFromSlice(m.byAgent[ep.AgentID], id)
	m.byType[ep.Type] = removeFromSlice(m.byType[ep.Type], id)
}

// GetStats returns episodic memory statistics
func (m *EpisodicMemory) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := map[string]interface{}{
		"total_episodes": len(m.episodes),
		"total_sequences": len(m.sequences),
		"by_type": make(map[EpisodeType]int),
		"by_session_count": len(m.bySession),
	}

	for t, ids := range m.byType {
		stats["by_type"].(map[EpisodeType]int)[t] = len(ids)
	}

	return stats
}

// Helper functions
func generateEpisodeID() string {
	return fmt.Sprintf("ep-%d", time.Now().UnixNano())
}

func generateSequenceID() string {
	return fmt.Sprintf("seq-%d", time.Now().UnixNano())
}

func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

func removeFromSlice(slice []string, item string) []string {
	var result []string
	for _, s := range slice {
		if s != item {
			result = append(result, s)
		}
	}
	return result
}
