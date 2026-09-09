package infra

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/cockroachdb/errors"
)

// Retry executes an operation with retries until it succeeds, the context is cancelled,
// or maxAttempts is reached. It logs a warning on each failed attempt and respects context cancellation.
func Retry(ctx context.Context, name string, maxAttempts int, delay time.Duration, op func() error) error {
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			slog.Info(fmt.Sprintf("Shutdown received while waiting for %s", name))
			return ctx.Err()
		default:
		}

		lastErr = op()
		if lastErr == nil {
			return nil
		}

		if attempt < maxAttempts {
			slog.Warn(fmt.Sprintf("Waiting for %s connection...", name),
				"attempt", attempt,
				"max_attempts", maxAttempts,
				"error", lastErr,
			)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}
	}
	return errors.Wrapf(lastErr, "%s failed after %d attempts", name, maxAttempts)
}
