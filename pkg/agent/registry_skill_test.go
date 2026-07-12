package agent

import (
	"context"
	"errors"
	"testing"
)

// memAgentStore is a minimal in-memory AgentStore for testing the registry's
// persistence-aware methods. It records every Save so tests can assert which
// agents were persisted and in what call order.
type memAgentStore struct {
	byID    map[string]*Agent
	saved   []string // IDs passed to Save, in call order
	saveErr error    // when set, Save returns this error
}

func newMemAgentStore(agents ...*Agent) *memAgentStore {
	m := &memAgentStore{byID: map[string]*Agent{}}
	for _, a := range agents {
		m.byID[a.ID] = a
	}
	return m
}

func (m *memAgentStore) Save(_ context.Context, a *Agent) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.saved = append(m.saved, a.ID)
	m.byID[a.ID] = a
	return nil
}

func (m *memAgentStore) Get(_ context.Context, id string) (*Agent, error) {
	a, ok := m.byID[id]
	if !ok {
		return nil, ErrAgentNotFound
	}
	return a, nil
}

func (m *memAgentStore) Delete(_ context.Context, id string) error {
	if _, ok := m.byID[id]; !ok {
		return ErrAgentNotFound
	}
	delete(m.byID, id)
	return nil
}

func (m *memAgentStore) List(_ context.Context) ([]*Agent, error) {
	out := make([]*Agent, 0, len(m.byID))
	for _, a := range m.byID {
		out = append(out, a)
	}
	return out, nil
}

func (m *memAgentStore) Exists(_ context.Context, id string) (bool, error) {
	_, ok := m.byID[id]
	return ok, nil
}

func (m *memAgentStore) Clear(_ context.Context) error {
	m.byID = map[string]*Agent{}
	return nil
}

func (m *memAgentStore) Count(_ context.Context) (int64, error) {
	return int64(len(m.byID)), nil
}

// newTestAgent builds a valid agent (passes Validate) with the given mounted
// skill IDs.
func newTestAgent(id, name string, skills ...string) *Agent {
	return &Agent{
		ID:           id,
		Name:         name,
		Instructions: "test instructions",
		Skills:       skills,
	}
}

// equalStringSets reports whether two slices hold the same strings regardless
// of order. Used where map iteration makes call order non-deterministic.
func equalStringSets(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	count := map[string]int{}
	for _, s := range a {
		count[s]++
	}
	for _, s := range b {
		count[s]--
		if count[s] < 0 {
			return false
		}
	}
	return true
}

func TestRegistry_RemoveSkillFromAllAgents_RemovesFromAffected(t *testing.T) {
	store := newMemAgentStore()
	registry := NewRegistryWithStore(store)
	// a1 and a2 mount skill s1; a3 does not.
	agents := []*Agent{
		newTestAgent("a1", "Agent One", "s1", "s2"),
		newTestAgent("a2", "Agent Two", "s1"),
		newTestAgent("a3", "Agent Three", "s2"),
	}
	for _, a := range agents {
		if err := registry.RegisterOrUpdateWithPersistence(context.Background(), a); err != nil {
			t.Fatalf("register %s: %v", a.ID, err)
		}
	}
	// Reset the Save log so it records only cleanup-time saves.
	store.saved = nil

	count, err := registry.RemoveSkillFromAllAgents(context.Background(), "s1")
	if err != nil {
		t.Fatalf("RemoveSkillFromAllAgents: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 affected agents, got %d", count)
	}

	// In-memory cache: s1 gone from a1 and a2; a3 untouched.
	if got := registry.Get("a1").Skills; !equalStringSets(got, []string{"s2"}) {
		t.Fatalf("a1 skills: got %v, want [s2]", got)
	}
	if got := registry.Get("a2").Skills; len(got) != 0 {
		t.Fatalf("a2 skills: got %v, want []", got)
	}
	if got := registry.Get("a3").Skills; !equalStringSets(got, []string{"s2"}) {
		t.Fatalf("a3 skills: got %v, want [s2]", got)
	}

	// Persistence: only the two changed agents were saved (order-independent).
	if !equalStringSets(store.saved, []string{"a1", "a2"}) {
		t.Fatalf("saved agents: got %v, want [a1 a2]", store.saved)
	}
}

func TestRegistry_RemoveSkillFromAllAgents_NoMatch(t *testing.T) {
	registry := NewRegistry() // no store - in-memory only
	if err := registry.RegisterOrUpdate(newTestAgent("a1", "A1", "s1")); err != nil {
		t.Fatalf("register: %v", err)
	}

	count, err := registry.RemoveSkillFromAllAgents(context.Background(), "missing")
	if err != nil {
		t.Fatalf("RemoveSkillFromAllAgents: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 affected, got %d", count)
	}
	if got := registry.Get("a1").Skills; !equalStringSets(got, []string{"s1"}) {
		t.Fatalf("skills should be unchanged: got %v", got)
	}
}

func TestRegistry_RemoveSkillFromAllAgents_EmptySkillID(t *testing.T) {
	registry := NewRegistry()
	if err := registry.RegisterOrUpdate(newTestAgent("a1", "A1", "s1")); err != nil {
		t.Fatalf("register: %v", err)
	}

	count, err := registry.RemoveSkillFromAllAgents(context.Background(), "")
	if err != nil || count != 0 {
		t.Fatalf("expected 0/nil for empty skillID, got %d/%v", count, err)
	}
}

func TestRegistry_RemoveSkillFromAllAgents_SaveErrorSurfaces(t *testing.T) {
	store := newMemAgentStore()
	store.saveErr = errors.New("disk full")
	registry := NewRegistryWithStore(store)
	for _, a := range []*Agent{
		newTestAgent("a1", "A1", "s1"),
		newTestAgent("a2", "A2", "s1"),
	} {
		// Register directly into the in-memory map to bypass Save (which would
		// now fail) - we want Save to fire only during cleanup.
		registry.agents[a.ID] = a
	}

	if _, err := registry.RemoveSkillFromAllAgents(context.Background(), "s1"); err == nil {
		t.Fatalf("expected save error to surface, got nil")
	}
}
