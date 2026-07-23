// Package retry is a minimal demo of a retry helper for fallible operations.
package retry

// maxAttempts is the total number of tries before giving up.
const maxAttempts = 3

// RetryOperation runs operation up to maxAttempts times. It returns the first
// successful result and a nil error, or the empty string and the last error if
// every attempt fails.
func RetryOperation(operation func() (string, error)) (string, error) {
	var lastErr error
	for i := 0; i < maxAttempts; i++ {
		result, err := operation()
		if err == nil {
			return result, nil
		}
		lastErr = err
	}
	return "", lastErr
}
