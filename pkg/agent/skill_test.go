package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

// memorySkillStore is an in-memory SkillStore for testing.
type memorySkillStore struct {
	byID   map[string]*Skill
	byName map[string]*Skill
}

func newMemorySkillStore(skills ...*Skill) *memorySkillStore {
	s := &memorySkillStore{byID: map[string]*Skill{}, byName: map[string]*Skill{}}
	for _, sk := range skills {
		_ = s.SaveSkill(context.Background(), sk)
	}
	return s
}

func (s *memorySkillStore) SaveSkill(_ context.Context, skill *Skill) error {
	if skill.CreatedAt.IsZero() {
		skill.CreatedAt = time.Now()
	}
	skill.UpdatedAt = time.Now()
	s.byID[skill.ID] = skill
	s.byName[skill.Name] = skill
	return nil
}
func (s *memorySkillStore) GetSkill(_ context.Context, id string) (*Skill, error) {
	if sk, ok := s.byID[id]; ok {
		return sk, nil
	}
	return nil, ErrSkillNotFound
}
func (s *memorySkillStore) GetSkillByName(_ context.Context, name string) (*Skill, error) {
	if sk, ok := s.byName[name]; ok {
		return sk, nil
	}
	return nil, ErrSkillNotFound
}
func (s *memorySkillStore) DeleteSkill(_ context.Context, id string) error {
	if sk, ok := s.byID[id]; ok {
		delete(s.byID, id)
		delete(s.byName, sk.Name)
		return nil
	}
	return ErrSkillNotFound
}
func (s *memorySkillStore) ListSkills(_ context.Context) ([]*Skill, error) {
	out := make([]*Skill, 0, len(s.byID))
	for _, sk := range s.byID {
		out = append(out, sk)
	}
	return out, nil
}
func (s *memorySkillStore) GetSkillsByIDs(_ context.Context, ids []string) ([]*Skill, error) {
	out := make([]*Skill, 0, len(ids))
	for _, id := range ids {
		if sk, ok := s.byID[id]; ok {
			out = append(out, sk)
		}
	}
	return out, nil
}

func TestSkill_Validate(t *testing.T) {
	tests := []struct {
		name    string
		skill   *Skill
		wantErr error
	}{
		{"valid", &Skill{Name: "x", Instructions: "do x"}, nil},
		{"missing name", &Skill{Instructions: "do x"}, ErrSkillNameRequired},
		{"missing instructions", &Skill{Name: "x"}, ErrSkillInstructionsRequired},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.skill.Validate()
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Validate() = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestDefaultSkills(t *testing.T) {
	skills := DefaultSkills()
	if len(skills) == 0 {
		t.Fatal("DefaultSkills returned no skills")
	}
	seen := map[string]bool{}
	for _, sk := range skills {
		if sk.Name == "" {
			t.Error("default skill has empty name")
		}
		if seen[sk.Name] {
			t.Errorf("duplicate default skill name: %s", sk.Name)
		}
		seen[sk.Name] = true
		if err := sk.Validate(); err != nil {
			t.Errorf("default skill %s invalid: %v", sk.Name, err)
		}
	}
}

func TestInitializeDefaultSkills_Idempotent(t *testing.T) {
	store := newMemorySkillStore()

	n1, err := InitializeDefaultSkills(context.Background(), store)
	if err != nil {
		t.Fatalf("first init: %v", err)
	}
	if n1 == 0 {
		t.Fatal("expected skills to be inserted on empty store")
	}

	// Second call must not re-insert (store already has skills).
	n2, err := InitializeDefaultSkills(context.Background(), store)
	if err != nil {
		t.Fatalf("second init: %v", err)
	}
	if n2 != 0 {
		t.Errorf("expected 0 inserts on non-empty store, got %d", n2)
	}
}

func TestInitializeDefaultSkills_NilStore(t *testing.T) {
	n, err := InitializeDefaultSkills(context.Background(), nil)
	if err != nil {
		t.Fatalf("nil store should not error: %v", err)
	}
	if n != 0 {
		t.Errorf("nil store should insert 0, got %d", n)
	}
}

// newTestEngine builds a minimal Engine suitable for testing skill helpers
// without wiring LLM/tools/store.
func newTestEngine(skillStore SkillStore) *Engine {
	e := NewEngine(NewRegistry(), nil, nil, nil, DefaultEngineConfig())
	if skillStore != nil {
		e.SetSkillStore(skillStore)
	}
	return e
}

func TestExecuteLoadSkill_ByName(t *testing.T) {
	sk := &Skill{ID: "skill-x", Name: "code-review", Description: "d", Instructions: "review it", Status: SkillStatusActive}
	store := newMemorySkillStore(sk)
	e := newTestEngine(store)

	agent := &Agent{ID: "a1", Skills: []string{"skill-x"}}
	args, _ := json.Marshal(map[string]string{"name": "code-review"})

	result, err := e.executeLoadSkill(context.Background(), agent, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" || !strings.Contains(result, "review it") {
		t.Errorf("expected instructions in result, got: %q", result)
	}
}

func TestExecuteLoadSkill_ByID(t *testing.T) {
	sk := &Skill{ID: "skill-x", Name: "code-review", Description: "d", Instructions: "review it", Status: SkillStatusActive}
	store := newMemorySkillStore(sk)
	e := newTestEngine(store)

	agent := &Agent{ID: "a1", Skills: []string{"skill-x"}}
	args, _ := json.Marshal(map[string]string{"name": "skill-x"})

	if _, err := e.executeLoadSkill(context.Background(), agent, args); err != nil {
		t.Fatalf("load by ID should succeed: %v", err)
	}
}

func TestExecuteLoadSkill_NotMounted(t *testing.T) {
	// Skill exists in store but is NOT mounted on the agent -> must be refused.
	sk := &Skill{ID: "skill-x", Name: "code-review", Description: "d", Instructions: "review it", Status: SkillStatusActive}
	store := newMemorySkillStore(sk)
	e := newTestEngine(store)

	agent := &Agent{ID: "a1", Skills: []string{}} // nothing mounted
	args, _ := json.Marshal(map[string]string{"name": "code-review"})

	_, err := e.executeLoadSkill(context.Background(), agent, args)
	if err == nil {
		t.Fatal("expected error loading a skill not mounted on the agent")
	}
}

func TestExecuteLoadSkill_DraftSkillNotLoadable(t *testing.T) {
	sk := &Skill{ID: "skill-x", Name: "code-review", Description: "d", Instructions: "review it", Status: SkillStatusDraft}
	store := newMemorySkillStore(sk)
	e := newTestEngine(store)

	agent := &Agent{ID: "a1", Skills: []string{"skill-x"}}
	args, _ := json.Marshal(map[string]string{"name": "code-review"})

	_, err := e.executeLoadSkill(context.Background(), agent, args)
	if err == nil {
		t.Fatal("expected error loading a draft skill")
	}
}

func TestExecuteLoadSkill_NilStore(t *testing.T) {
	e := newTestEngine(nil)
	agent := &Agent{ID: "a1", Skills: []string{"skill-x"}}
	args, _ := json.Marshal(map[string]string{"name": "code-review"})

	_, err := e.executeLoadSkill(context.Background(), agent, args)
	if err == nil {
		t.Fatal("expected error when skill store is nil")
	}
}

func TestExecuteLoadSkill_EmptyName(t *testing.T) {
	store := newMemorySkillStore()
	e := newTestEngine(store)
	agent := &Agent{ID: "a1", Skills: []string{"skill-x"}}
	args, _ := json.Marshal(map[string]string{"name": ""})

	_, err := e.executeLoadSkill(context.Background(), agent, args)
	if err == nil {
		t.Fatal("expected error for empty skill name")
	}
}

// --- buildAgentTools: dynamic tool gating per mounted skill ---

// mockToolExecutor is a ToolExecutor stub returning a fixed tool set, used to
// test buildAgentTools without wiring real MCP/built-in tools.
type mockToolExecutor struct{ tools map[string]any }

func (m *mockToolExecutor) Execute(context.Context, string, json.RawMessage, *ToolSpecificConfig) (string, error) {
	return "", nil
}
func (m *mockToolExecutor) ListTools(context.Context) (map[string]any, error) {
	return m.tools, nil
}

// toolDef builds a minimal tool definition keyed by function name.
func toolDef(name string) map[string]any {
	return map[string]any{
		"type": "function",
		"function": map[string]any{"name": name, "description": "mock", "parameters": map[string]any{"type": "object"}},
	}
}

// toolNames extracts the set of function names from a buildAgentTools result.
func toolNames(defs []map[string]any) map[string]bool {
	out := map[string]bool{}
	for _, d := range defs {
		if fn, ok := d["function"].(map[string]any); ok {
			if n, ok := fn["name"].(string); ok {
				out[n] = true
			}
		}
	}
	return out
}

func newTestEngineWithTools(tools ToolExecutor, skillStore SkillStore) *Engine {
	e := NewEngine(NewRegistry(), nil, tools, nil, DefaultEngineConfig())
	if skillStore != nil {
		e.SetSkillStore(skillStore)
	}
	return e
}

func TestBuildAgentTools_SkillGrantsTool(t *testing.T) {
	// Skill declares code_execute; agent does NOT have it; registry has it.
	// Mounting the skill must grant code_execute to the agent.
	sk := &Skill{ID: "skill-cr", Name: "code-review", Description: "d",
		Instructions: "x", Tools: []string{"code_execute"}, Status: SkillStatusActive}
	store := newMemorySkillStore(sk)
	tools := &mockToolExecutor{tools: map[string]any{
		"code_execute": toolDef("code_execute"),
		"web_search":   toolDef("web_search"),
	}}
	e := newTestEngineWithTools(tools, store)

	agent := &Agent{ID: "a1", Skills: []string{"skill-cr"}}
	got, err := e.buildAgentTools(context.Background(), agent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	names := toolNames(got)
	if !names["code_execute"] {
		t.Error("expected code_execute to be granted by the mounted skill")
	}
	if !names["load_skill"] {
		t.Error("expected load_skill tool to be present")
	}
	if names["web_search"] {
		t.Error("web_search must NOT be granted (not declared by skill or agent)")
	}
}

func TestBuildAgentTools_SkillToolDedup(t *testing.T) {
	// Agent already has code_execute; skill also declares it -> appears once.
	sk := &Skill{ID: "skill-cr", Name: "code-review", Description: "d",
		Instructions: "x", Tools: []string{"code_execute"}, Status: SkillStatusActive}
	store := newMemorySkillStore(sk)
	tools := &mockToolExecutor{tools: map[string]any{"code_execute": toolDef("code_execute")}}
	e := newTestEngineWithTools(tools, store)

	agent := &Agent{ID: "a1", Tools: []string{"code_execute"}, Skills: []string{"skill-cr"}}
	got, _ := e.buildAgentTools(context.Background(), agent)

	count := 0
	for _, d := range got {
		if fn, ok := d["function"].(map[string]any); ok {
			if n, _ := fn["name"].(string); n == "code_execute" {
				count++
			}
		}
	}
	if count != 1 {
		t.Errorf("code_execute should appear exactly once (dedup), got %d", count)
	}
}

func TestBuildAgentTools_SkillNonexistentToolNotGranted(t *testing.T) {
	// Skill declares a tool not in the registry -> silently skipped, no panic,
	// no hallucinated tool. A skill cannot invent tools, only unlock existing ones.
	sk := &Skill{ID: "skill-cr", Name: "code-review", Description: "d",
		Instructions: "x", Tools: []string{"does_not_exist"}, Status: SkillStatusActive}
	store := newMemorySkillStore(sk)
	tools := &mockToolExecutor{tools: map[string]any{"code_execute": toolDef("code_execute")}}
	e := newTestEngineWithTools(tools, store)

	agent := &Agent{ID: "a1", Skills: []string{"skill-cr"}}
	got, err := e.buildAgentTools(context.Background(), agent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if names := toolNames(got); names["does_not_exist"] {
		t.Error("a tool not in the registry must not be granted")
	}
}

func TestBuildAgentTools_DraftSkillToolsNotGranted(t *testing.T) {
	// Draft skill's tools must NOT be granted - consistent with prompt injection
	// and load_skill both skipping draft skills.
	sk := &Skill{ID: "skill-cr", Name: "code-review", Description: "d",
		Instructions: "x", Tools: []string{"code_execute"}, Status: SkillStatusDraft}
	store := newMemorySkillStore(sk)
	tools := &mockToolExecutor{tools: map[string]any{"code_execute": toolDef("code_execute")}}
	e := newTestEngineWithTools(tools, store)

	agent := &Agent{ID: "a1", Skills: []string{"skill-cr"}}
	got, _ := e.buildAgentTools(context.Background(), agent)
	if names := toolNames(got); names["code_execute"] {
		t.Error("a draft skill must not grant tools")
	}
}
