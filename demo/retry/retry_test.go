package retry

import (
	"errors"
	"testing"
)

// Test 1: retries failed operations 3 times
func TestRetriesFailedOperations3Times(t *testing.T) {
	attempts := 0
	operation := func() (string, error) {
		attempts++
		if attempts < 3 {
			return "", errors.New("fail")
		}
		return "success", nil
	}

	result, err := RetryOperation(operation)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result != "success" {
		t.Errorf("expected 'success', got '%s'", result)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

// Test 2: returns error after max retries
func TestReturnsErrorAfterMaxRetries(t *testing.T) {
	attempts := 0
	operation := func() (string, error) {
		attempts++
		return "", errors.New("always fails")
	}

	result, err := RetryOperation(operation)

	if err == nil {
		t.Error("expected error, got nil")
	}
	if result != "" {
		t.Errorf("expected empty result, got '%s'", result)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

// Test 3: succeeds immediately on first try
func TestSucceedsImmediately(t *testing.T) {
	attempts := 0
	operation := func() (string, error) {
		attempts++
		return "success", nil
	}

	result, err := RetryOperation(operation)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result != "success" {
		t.Errorf("expected 'success', got '%s'", result)
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", attempts)
	}
}