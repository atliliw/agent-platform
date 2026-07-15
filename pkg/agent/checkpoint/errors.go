package checkpoint

import "errors"

// Checkpoint errors
var (
	// ErrCheckpointNotFound is returned when a checkpoint is not found.
	ErrCheckpointNotFound = errors.New("checkpoint not found")

	// ErrCheckpointInvalidID is returned when a checkpoint ID is invalid.
	ErrCheckpointInvalidID = errors.New("invalid checkpoint id")
)
