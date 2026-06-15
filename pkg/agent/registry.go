package agent

import (
	"context"
	"sync"
	"time"
)

// AgentStore defines the interface for agent persistence
type AgentStore interface {
	Save(ctx context.Context, agent *Agent) error
	Get(ctx context.Context, id string) (*Agent, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context) ([]*Agent, error)
	Exists(ctx context.Context, id string) (bool, error)
	Clear(ctx context.Context) error
	Count(ctx context.Context) (int64, error)
}

// Registry manages agent registrations with optional persistence
type Registry struct {
	mu          sync.RWMutex
	agents      map[string]*Agent  // In-memory cache
	defaultID   string
	store       AgentStore          // Optional persistence layer
}

// NewRegistry creates a new agent registry (without persistence)
func NewRegistry() *Registry {
	return &Registry{
		agents: make(map[string]*Agent),
	}
}

// NewRegistryWithStore creates a new agent registry with persistence
func NewRegistryWithStore(store AgentStore) *Registry {
	return &Registry{
		agents: make(map[string]*Agent),
		store:  store,
	}
}

// LoadFromStore loads all agents from the persistence layer into memory
func (r *Registry) LoadFromStore(ctx context.Context) error {
	if r.store == nil {
		return nil
	}

	agents, err := r.store.List(ctx)
	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, agent := range agents {
		r.agents[agent.ID] = agent
		if r.defaultID == "" {
			r.defaultID = agent.ID
		}
	}

	return nil
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

// RegisterWithPersistence adds an agent and persists it to storage
func (r *Registry) RegisterWithPersistence(ctx context.Context, agent *Agent) error {
	if err := agent.Validate(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.agents[agent.ID]; exists {
		return ErrAgentAlreadyExists
	}

	agent.UpdatedAt = time.Now()

	// Persist to storage if available
	if r.store != nil {
		if err := r.store.Save(ctx, agent); err != nil {
			return err
		}
	}

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

// RegisterOrUpdateWithPersistence registers or updates with persistence
func (r *Registry) RegisterOrUpdateWithPersistence(ctx context.Context, agent *Agent) error {
	if err := agent.Validate(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	agent.UpdatedAt = time.Now()

	// Persist to storage if available
	if r.store != nil {
		if err := r.store.Save(ctx, agent); err != nil {
			return err
		}
	}

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

// UnregisterWithPersistence removes an agent and deletes from storage
func (r *Registry) UnregisterWithPersistence(ctx context.Context, agentID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.agents[agentID]; !exists {
		return ErrAgentNotFound
	}

	// Delete from storage if available
	if r.store != nil {
		if err := r.store.Delete(ctx, agentID); err != nil {
			return err
		}
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

// ClearWithPersistence removes all agents and clears storage
func (r *Registry) ClearWithPersistence(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.store != nil {
		if err := r.store.Clear(ctx); err != nil {
			return err
		}
	}

	r.agents = make(map[string]*Agent)
	r.defaultID = ""
	return nil
}

// GetStore returns the underlying store
func (r *Registry) GetStore() AgentStore {
	return r.store
}