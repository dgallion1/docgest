package pipeline

import (
	"errors"
	"math/rand/v2"
	"time"

	"github.com/dgallion1/docgest/internal/extract"
)

// IsRetryable checks if an error is worth retrying.
func IsRetryable(err error) bool {
	var retryErr *extract.RetryableError
	return errors.As(err, &retryErr)
}

// Backoff returns a duration for attempt n (0-indexed) with jitter.
func Backoff(attempt int) time.Duration {
	base := time.Duration(1<<uint(attempt)) * time.Second
	if base > 30*time.Second {
		base = 30 * time.Second
	}
	jitter := time.Duration(rand.Int64N(int64(base) / 2))
	return base + jitter
}

const MaxRetries = 3
