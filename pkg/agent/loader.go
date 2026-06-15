package agent

import (
	"context"
	"fmt"
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

// SaveYAML saves an agent to YAML format (for export/backup)
func SaveYAML(agent *Agent) ([]byte, error) {
	return yamlMarshal(agent)
}

// LoadFromYAML loads an agent from YAML data (for import)
func LoadFromYAML(data []byte) (*Agent, error) {
	var agent Agent
	if err := yamlUnmarshal(data, &agent); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}

	if err := agent.Validate(); err != nil {
		return nil, fmt.Errorf("validate agent: %w", err)
	}

	return &agent, nil
}

// ImportFromYAML imports an agent from YAML and registers it with persistence
func ImportFromYAML(ctx context.Context, registry *Registry, data []byte) (*Agent, error) {
	agent, err := LoadFromYAML(data)
	if err != nil {
		return nil, err
	}

	if err := registry.RegisterOrUpdateWithPersistence(ctx, agent); err != nil {
		return nil, fmt.Errorf("register agent: %w", err)
	}

	return agent, nil
}

// ExportToYAML exports an agent to YAML format
func ExportToYAML(registry *Registry, agentID string) ([]byte, error) {
	agent := registry.Get(agentID)
	if agent == nil {
		return nil, ErrAgentNotFound
	}
	return SaveYAML(agent)
}