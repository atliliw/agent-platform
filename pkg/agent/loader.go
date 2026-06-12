package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Loader loads agents from YAML configurations
type Loader struct {
	registry *Registry
}

// NewLoader creates a new agent loader
func NewLoader(registry *Registry) *Loader {
	return &Loader{
		registry: registry,
	}
}

// LoadFile loads an agent from a YAML file
func (l *Loader) LoadFile(filePath string) (*Agent, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	return l.LoadYAML(data)
}

// LoadYAML loads an agent from YAML data
func (l *Loader) LoadYAML(data []byte) (*Agent, error) {
	var agent Agent
	if err := yaml.Unmarshal(data, &agent); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}

	if err := agent.Validate(); err != nil {
		return nil, fmt.Errorf("validate agent: %w", err)
	}

	return &agent, nil
}

// LoadDirectory loads all agents from a directory
func (l *Loader) LoadDirectory(dirPath string) ([]*Agent, error) {
	agents := make([]*Agent, 0)

	// Find all YAML files
	files, err := filepath.Glob(filepath.Join(dirPath, "*.yaml"))
	if err != nil {
		return nil, fmt.Errorf("glob files: %w", err)
	}

	// Also check .yml extension
	ymlFiles, err := filepath.Glob(filepath.Join(dirPath, "*.yml"))
	if err != nil {
		return nil, fmt.Errorf("glob yml files: %w", err)
	}

	files = append(files, ymlFiles...)

	// Load each file
	for _, file := range files {
		agent, err := l.LoadFile(file)
		if err != nil {
			return nil, fmt.Errorf("load %s: %w", file, err)
		}

		agents = append(agents, agent)
	}

	return agents, nil
}

// LoadAndRegister loads an agent and registers it
func (l *Loader) LoadAndRegister(filePath string) (*Agent, error) {
	agent, err := l.LoadFile(filePath)
	if err != nil {
		return nil, err
	}

	if err := l.registry.Register(agent); err != nil {
		return nil, fmt.Errorf("register agent: %w", err)
	}

	return agent, nil
}

// LoadDirectoryAndRegister loads all agents from a directory and registers them
func (l *Loader) LoadDirectoryAndRegister(dirPath string) (int, error) {
	agents, err := l.LoadDirectory(dirPath)
	if err != nil {
		return 0, err
	}

	for _, agent := range agents {
		if err := l.registry.RegisterOrUpdate(agent); err != nil {
			return 0, fmt.Errorf("register %s: %w", agent.ID, err)
		}
	}

	return len(agents), nil
}

// SaveYAML saves an agent to YAML format
func SaveYAML(agent *Agent) ([]byte, error) {
	return yaml.Marshal(agent)
}

// SaveFile saves an agent to a YAML file
func SaveFile(agent *Agent, filePath string) error {
	data, err := SaveYAML(agent)
	if err != nil {
		return fmt.Errorf("marshal yaml: %w", err)
	}

	return os.WriteFile(filePath, data, 0644)
}

// LoadFromString loads an agent from a YAML string
func LoadFromString(yamlStr string) (*Agent, error) {
	return NewLoader(nil).LoadYAML([]byte(yamlStr))
}

// ReplaceTemplateVars replaces template variables in instructions
func ReplaceTemplateVars(instructions string, vars map[string]string) string {
	for k, v := range vars {
		instructions = strings.ReplaceAll(instructions, fmt.Sprintf("{%s}", k), v)
	}
	return instructions
}

// AgentTemplate represents a reusable agent template
type AgentTemplate struct {
	ID           string            `yaml:"id"`
	Name         string            `yaml:"name"`
	Description  string            `yaml:"description"`
	Instructions string            `yaml:"instructions"`
	Tools        []string          `yaml:"tools"`
	Handoffs     []string          `yaml:"handoffs"`
	Model        string            `yaml:"model,omitempty"`
}

// Instantiate creates an agent from a template with variables
func (t *AgentTemplate) Instantiate(vars map[string]string) *Agent {
	agent := &Agent{
		ID:           ReplaceTemplateVars(t.ID, vars),
		Name:         ReplaceTemplateVars(t.Name, vars),
		Description:  ReplaceTemplateVars(t.Description, vars),
		Instructions: ReplaceTemplateVars(t.Instructions, vars),
		Tools:        t.Tools,
		Handoffs:     t.Handoffs,
		Model:        t.Model,
	}

	return agent
}