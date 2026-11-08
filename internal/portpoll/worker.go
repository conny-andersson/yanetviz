package portpoll

import (
	"context"
	"log"
	"math/rand/v2"
	"sync"
	"time"
	"yanetviz/internal/plan"
)

type channelPoll struct {
	read    func(context.Context, time.Time) (plan.PortSnapshot, error)
	next    time.Time
	backoff time.Duration
}

// Run polls each source sequentially. Channel-specific retries ensure failed PoE
// collection cannot slow healthy link updates. Gates also serialize Wi-Fi reads.
func Run(ctx context.Context, config Config, store *plan.Store) {
	var workers sync.WaitGroup
	for _, source := range config.Sources {
		workers.Add(1)
		go func() {
			defer workers.Done()
			collector := NewCollector(source)
			channels := []channelPoll{{read: collector.PollLink}, {read: collector.PollPoE}}
			interval := time.Duration(config.PollSeconds) * time.Second
			for {
				now := time.Now()
				collector.Discover(ctx, now)
				for i := range channels {
					channel := &channels[i]
					if now.Before(channel.next) {
						continue
					}
					snapshot, err := channel.read(ctx, now)
					if ctx.Err() != nil {
						return
					}
					switch {
					case err != nil:
						snapshot.Error = err.Error()
						channel.backoff = min(max(channel.backoff*2, 15*time.Second), 2*time.Minute)
						channel.next = time.Now().Add(channel.backoff + time.Duration(rand.IntN(1000))*time.Millisecond)
					case snapshot.Capability == "disabled" || snapshot.Capability == "unsupported":
						channel.next = time.Now().Add(24 * time.Hour)
					case snapshot.Capability == "unavailable":
						channel.next = time.Now().Add(5 * time.Minute)
					default:
						channel.backoff = 0
						channel.next = time.Time{}
					}
					if err := store.RecordPortSnapshot(snapshot); err != nil {
						log.Printf("persist %s observation for %s: %v", snapshot.Channel, source.ID, err)
					}
				}
				timer := time.NewTimer(interval)
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
