package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// SkillStatus represents the lifecycle state of a skill.
type SkillStatus string

const (
	SkillStatusDraft  SkillStatus = "draft"
	SkillStatusActive SkillStatus = "active"
)

// Skill is a reusable capability module that an agent can mount.
//
// Skills follow the progressive-disclosure model: only the Name + Description
// are injected into the agent's system prompt (cheap, always-on). The full
// Instructions are loaded on demand when the agent calls the built-in
// load_skill tool. This keeps the prompt small while letting an agent carry
// many skills without paying for all of them up front.
//
// Mounting is many-to-many: an Agent.Skills []string holds skill IDs, and a
// skill can be mounted by any number of agents. Skills are independent of
// agents - they live in their own store and are referenced by ID.
type Skill struct {
	// ID is the unique identifier for the skill
	ID string `json:"id" yaml:"id"`

	// Name is the short, unique, machine-friendly name the agent passes to
	// load_skill (e.g. "code-review"). Must be unique within a tenant.
	Name string `json:"name" yaml:"name"`

	// Description is the one-line summary injected into the system prompt so
	// the agent knows what the skill does without loading it.
	Description string `json:"description" yaml:"description"`

	// Instructions is the full, detailed prompt loaded on demand via the
	// load_skill tool. This is the body of the skill - the part that would
	// bloat the prompt if always injected.
	Instructions string `json:"instructions" yaml:"instructions"`

	// Tools optionally lists tool names the skill expects to use. When the
	// skill is mounted on an agent, the engine grants these tools to the agent
	// at runtime (dynamic tool gating) - but only tools that already exist in
	// the registry. A skill cannot invent tools, only unlock existing ones.
	Tools []string `json:"tools,omitempty" yaml:"tools,omitempty"`

	// Tags are free-form labels for filtering and organization.
	Tags []string `json:"tags,omitempty" yaml:"tags,omitempty"`

	// Status is the lifecycle state (draft/active). Only active skills are
	// injected into agent prompts.
	Status SkillStatus `json:"status" yaml:"status"`

	// Version is a monotonically increasing version counter for change tracking.
	Version int `json:"version" yaml:"version"`

	// CreatedAt is the creation timestamp
	CreatedAt time.Time `json:"created_at" yaml:"created_at"`

	// UpdatedAt is the last update timestamp
	UpdatedAt time.Time `json:"updated_at" yaml:"updated_at"`
}

// NewSkill creates a new skill with defaults and a generated ID.
func NewSkill(name, description string) *Skill {
	return &Skill{
		ID:          "skill-" + uuid.New().String()[:8],
		Name:        name,
		Description: description,
		Tools:       []string{},
		Tags:        []string{},
		Status:      SkillStatusActive,
		Version:     1,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

// Validate validates the skill configuration.
func (s *Skill) Validate() error {
	if s.Name == "" {
		return ErrSkillNameRequired
	}
	if s.Instructions == "" {
		return ErrSkillInstructionsRequired
	}
	return nil
}

// SkillStore defines the interface for skill persistence. Mirrors AgentStore
// but for skills, plus GetSkillsByIDs for the engine to resolve an agent's
// mounted skills in one batched call.
type SkillStore interface {
	// SaveSkill inserts or updates a skill (upsert by ID).
	SaveSkill(ctx context.Context, skill *Skill) error

	// GetSkill retrieves a skill by ID.
	GetSkill(ctx context.Context, id string) (*Skill, error)

	// GetSkillByName retrieves a skill by its unique Name.
	GetSkillByName(ctx context.Context, name string) (*Skill, error)

	// DeleteSkill removes a skill by ID.
	DeleteSkill(ctx context.Context, id string) error

	// ListSkills returns all skills.
	ListSkills(ctx context.Context) ([]*Skill, error)

	// GetSkillsByIDs returns the skills matching the given IDs (used by the
	// engine to resolve an agent's mounted skills). Missing IDs are skipped.
	GetSkillsByIDs(ctx context.Context, ids []string) ([]*Skill, error)
}

// DefaultSkills returns a starter set of skills seeded into a fresh database.
// Each is a small, self-contained capability an agent can mount.
func DefaultSkills() []*Skill {
	return []*Skill{
		{
			ID:          "skill-code-review",
			Name:        "code-review",
			Description: "Review code for bugs, security issues, and style problems.",
			Instructions: `You are reviewing code. For the given code:
1. Identify correctness bugs (off-by-one, nil deref, race conditions).
2. Flag security issues (injection, secrets, unsafe input handling).
3. Note maintainability problems (large functions, deep nesting, missing errors).
Reply as a prioritized list: CRITICAL / HIGH / MEDIUM / LOW, each with file:line and a concrete fix.`,
			Tools:   []string{"code_execute"},
			Tags:    []string{"quality", "code"},
			Status:  SkillStatusActive,
			Version: 1,
		},
		{
			ID:          "skill-summarize",
			Name:        "summarize",
			Description: "Condense long text or tool output into a tight summary.",
			Instructions: `Summarize the input faithfully and losslessly:
- Lead with the single most important point.
- Then 3-7 bullet points covering key facts, numbers, and decisions.
- End with "Open questions:" if anything is unresolved.
Do not invent facts. If the input is already short, say so and return it unchanged.`,
			Tags:    []string{"writing"},
			Status:  SkillStatusActive,
			Version: 1,
		},
		{
			ID:          "skill-translate",
			Name:        "translate",
			Description: "Translate text between languages preserving tone and intent.",
			Instructions: `Translate the input to the target language.
- Preserve tone, formality, and formatting.
- Keep proper nouns, code, and identifiers unchanged unless asked.
- If the target language is ambiguous, ask. Otherwise detect source and translate to the most likely target.`,
			Tags:    []string{"writing", "i18n"},
			Status:  SkillStatusActive,
			Version: 1,
		},
	}
}

// InitializeDefaultSkills seeds the store with DefaultSkills when it is empty.
// Returns the number of skills inserted. Idempotent: if skills already exist,
// it does nothing. Mirrors InitializeDefaultAgents.
func InitializeDefaultSkills(ctx context.Context, store SkillStore) (int, error) {
	if store == nil {
		return 0, nil
	}
	existing, err := store.ListSkills(ctx)
	if err != nil {
		return 0, fmt.Errorf("list skills: %w", err)
	}
	if len(existing) > 0 {
		return 0, nil
	}
	inserted := 0
	for _, skill := range DefaultSkills() {
		if skill.CreatedAt.IsZero() {
			skill.CreatedAt = time.Now()
		}
		skill.UpdatedAt = time.Now()
		if err := store.SaveSkill(ctx, skill); err != nil {
			return inserted, fmt.Errorf("save skill %s: %w", skill.ID, err)
		}
		inserted++
	}
	return inserted, nil
}
