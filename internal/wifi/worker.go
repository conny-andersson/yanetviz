package wifi

import (
	"context"
	"log"
	"math/rand/v2"
	"sync"
	"time"
	"yanetviz/internal/plan"
)

// Run polls each configured controller independently with bounded source count,
// sequential requests, no overlapping polls and capped retry backoff with jitter.
// It returns after cancellation and after all database writes have completed.
func Run(ctx context.Context, config Config, store *plan.Store) {
	var workers sync.WaitGroup
	for _, source := range config.Sources {
		workers.Add(1)
		go func() {
			defer workers.Done()
			collector := NewCollector(source)
			delay := time.Duration(config.PollSeconds) * time.Second
			for {
				snapshot, err := collector.Poll(ctx, time.Now())
				if ctx.Err() != nil {
					return
				}
				if err != nil {
					snapshot.Error = err.Error()
					delay = min(max(delay*2, 15*time.Second), 2*time.Minute)
				} else {
					delay = time.Duration(config.PollSeconds) * time.Second
				}
				if err := store.RecordWiFiSnapshot(snapshot); err != nil {
					log.Printf("persist Wi-Fi snapshot for %s: %v", source.ID, err)
				}
				jitter := time.Duration(0)
				if snapshot.Error != "" {
					jitter = time.Duration(rand.IntN(1000)) * time.Millisecond
				}
				timer := time.NewTimer(delay + jitter)
				select {
				case <-ctx.Done():
					timer.Stop()
					return
				case <-timer.C:
				}
			}
		}()
	}
	workers.Wait()
}
