package agent

import (
	"sync"
	"time"
)

// Registry manages agent registrations
type Registry struct {
	mu          sync.RWMutex
	agents      map[string]*Agent
	defaultID   string
}

// NewRegistry creates a new agent registry
func NewRegistry() *Registry {
	return &Registry{
		agents: make(map[string]*Agent),
	}
}

// Register adds an agent to the registry
func (r *Registry) Register(agent *Agent) error {
	if err := agent.Validate(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.agents[agent.ID]; exists {
		return ErrAgentAlreadyExists
	}

	agent.UpdatedAt = time.Now()
	r.agents[agent.ID] = agent

	// Set as default if first agent
	if r.defaultID == "" {
		r.defaultID = agent.ID
	}

	return nil
}

// RegisterOrUpdate registers a new agent or updates existing one
func (r *Registry) RegisterOrUpdate(agent *Agent) error {
	if err := agent.Validate(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	agent.UpdatedAt = time.Now()
	r.agents[agent.ID] = agent

	// Set as default if first agent
	if r.defaultID == "" {
		r.defaultID = agent.ID
	}

	return nil
}

// Unregister removes an agent from the registry
func (r *Registry) Unregister(agentID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.agents[agentID]; !exists {
		return ErrAgentNotFound
	}

	delete(r.agents, agentID)

	// Update default if needed
	if r.defaultID == agentID {
		r.defaultID = ""
		for id := range r.agents {
			r.defaultID = id
			break
		}
	}

	return nil
}

// Get retrieves an agent by ID
func (r *Registry) Get(agentID string) *Agent {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.agents[agentID]
}

// GetDefault retrieves the default agent
func (r *Registry) GetDefault() *Agent {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.defaultID == "" {
		return nil
	}

	return r.agents[r.defaultID]
}

// SetDefault sets the default agent
func (r *Registry) SetDefault(agentID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.agents[agentID]; !exists {
		return ErrAgentNotFound
	}

	r.defaultID = agentID
	return nil
}

// List returns all registered agents
func (r *Registry) List() []*Agent {
	r.mu.RLock()
	defer r.mu.RUnlock()

	agents := make([]*Agent, 0, len(r.agents))
	for _, agent := range r.agents {
		agents = append(agents, agent)
	}

	return agents
}

// ListIDs returns all agent IDs
func (r *Registry) ListIDs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := make([]string, 0, len(r.agents))
	for id := range r.agents {
		ids = append(ids, id)
	}

	return ids
}

// Count returns the number of registered agents
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.agents)
}

// Exists checks if an agent exists
func (r *Registry) Exists(agentID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, exists := r.agents[agentID]
	return exists
}

// ValidateHandoff validates that a handoff is possible
func (r *Registry) ValidateHandoff(fromID, toID string) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	fromAgent, exists := r.agents[fromID]
	if !exists {
		return ErrAgentNotFound
	}

	if !fromAgent.CanHandoffTo(toID) {
		return ErrInvalidHandoff
	}

	if _, exists := r.agents[toID]; !exists {
		return ErrAgentNotFound
	}

	return nil
}

// GetAgentTools returns all tools available to an agent
func (r *Registry) GetAgentTools(agentID string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	agent, exists := r.agents[agentID]
	if !exists {
		return nil
	}

	return agent.Tools
}

// GetAgentHandoffs returns all handoff targets for an agent
func (r *Registry) GetAgentHandoffs(agentID string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	agent, exists := r.agents[agentID]
	if !exists {
		return nil
	}

	return agent.Handoffs
}

// Clear removes all agents from the registry
func (r *Registry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.agents = make(map[string]*Agent)
	r.defaultID = ""
}
