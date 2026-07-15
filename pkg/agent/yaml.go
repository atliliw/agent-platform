package agent

import (
	"gopkg.in/yaml.v3"
)

// yamlMarshal marshals an agent to YAML
func yamlMarshal(agent *Agent) ([]byte, error) {
	return yaml.Marshal(agent)
}

// yamlUnmarshal unmarshals YAML data to an agent
func yamlUnmarshal(data []byte, agent *Agent) error {
	return yaml.Unmarshal(data, agent)
}
