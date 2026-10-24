package app

import (
	"context"
	"log"
	"time"

	"github.com/akki/pidesigner/internal/knowledge"
)

// runReferenceUpdates starts immediately; network work never delays app startup.
// Shutdown cancels requests and waits for this worker before releasing the data lock.
func runReferenceUpdates(ctx context.Context, updates *knowledge.Manager, interval time.Duration) {
	for {
		if err := updates.Sync(ctx); err != nil && ctx.Err() == nil {
			log.Printf("design reference sync: %v", err)
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}
