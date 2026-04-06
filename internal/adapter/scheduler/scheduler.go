package scheduler

import (
	"context"
	"log"
	"time"
)

// Runner is a function that performs one sync cycle.
type Runner func(ctx context.Context) error

// Start runs the given runner immediately, then on every tick of interval.
// It blocks until ctx is cancelled.
func Start(ctx context.Context, interval time.Duration, run Runner) {
	log.Printf("[scheduler] starting – poll interval %v", interval)

	// Run immediately on startup.
	runOnce(ctx, run)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("[scheduler] shutting down")
			return
		case <-ticker.C:
			runOnce(ctx, run)
		}
	}
}

func runOnce(ctx context.Context, run Runner) {
	if err := run(ctx); err != nil {
		log.Printf("[scheduler] sync error: %v", err)
	}
}
