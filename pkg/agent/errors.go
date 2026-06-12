package agent

import (
	"errors"
)

// Agent errors
var (
	// ErrAgentNotFound is returned when agent is not found
	ErrAgentNotFound = errors.New("agent not found")

	// ErrAgentAlreadyExists is returned when trying to register an existing agent
	ErrAgentAlreadyExists = errors.New("agent already exists")

	// ErrAgentIDRequired is returned when agent ID is empty
	ErrAgentIDRequired = errors.New("agent ID is required")

	// ErrAgentNameRequired is returned when agent name is empty
	ErrAgentNameRequired = errors.New("agent name is required")

	// ErrAgentInstructionsRequired is returned when instructions are empty
	ErrAgentInstructionsRequired = errors.New("agent instructions are required")

	// ErrInvalidHandoff is returned when handoff target is invalid
	ErrInvalidHandoff = errors.New("invalid handoff target")

	// ErrMaxStepsReached is returned when max execution steps reached
	ErrMaxStepsReached = errors.New("maximum execution steps reached")

	// ErrContextNotFound is returned when context is not found
	ErrContextNotFound = errors.New("context not found")

	// ErrToolNotFound is returned when tool is not found
	ErrToolNotFound = errors.New("tool not found")

	// ErrExecutionFailed is returned when execution fails
	ErrExecutionFailed = errors.New("execution failed")

	// ErrNoDefaultAgent is returned when no default agent is set
	ErrNoDefaultAgent = errors.New("no default agent set")
)
