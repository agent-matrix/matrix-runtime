package security

import (
	"fmt"
	"time"
)

// ClampTTL clamps a requested TTL (in seconds) to the [1, max] range. A
// non-positive request falls back to max.
func ClampTTL(requested, max int) time.Duration {
	if max <= 0 {
		max = 600
	}
	if requested <= 0 || requested > max {
		requested = max
	}
	return time.Duration(requested) * time.Second
}

// CheckTTL validates a requested TTL against the configured maximum.
func CheckTTL(requested, max int) error {
	if requested < 0 {
		return fmt.Errorf("ttl_seconds must not be negative")
	}
	if max > 0 && requested > max {
		return fmt.Errorf("ttl_seconds %d exceeds maximum %d", requested, max)
	}
	return nil
}
