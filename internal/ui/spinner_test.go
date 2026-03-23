package ui

import (
	"testing"
	"time"
)

func TestSpinner_StartsAndStops(t *testing.T) {
	// Just verify it doesn't panic and stop works
	stop := Spinner("testing...")
	time.Sleep(200 * time.Millisecond)
	stop()
	// Call stop again to verify idempotency
	stop()
}
