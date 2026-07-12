// SPDX-License-Identifier: AGPL-3.0-only
package docker

import (
	"context"
	"sync"
	"time"

	"github.com/drilonrecica/binnacle/internal/dockerapi"
)

// StatsCollector bounds Docker requests and keeps the newest successful sample.
type StatsCollector struct {
	Client         dockerapi.Client
	Cache          *Cache
	MaxConcurrency int
	mu             sync.RWMutex
	last           map[string]dockerapi.Stats
}

func (c *StatsCollector) Collect(ctx context.Context) map[string]dockerapi.Stats {
	limit := c.MaxConcurrency
	if limit < 1 {
		limit = 4
	}
	c.Cache.mu.RLock()
	ids := make([]string, 0, len(c.Cache.values))
	for id, metadata := range c.Cache.values {
		if metadata.State == "running" {
			ids = append(ids, id)
		}
	}
	c.Cache.mu.RUnlock()
	result := make(map[string]dockerapi.Stats, len(ids))
	var mu sync.Mutex
	jobs := make(chan string)
	var workers sync.WaitGroup
	for range limit {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for id := range jobs {
				request, cancel := context.WithTimeout(ctx, 5*time.Second)
				sample, err := c.Client.Stats(request, id)
				cancel()
				if err == nil {
					mu.Lock()
					result[id] = sample
					mu.Unlock()
				}
			}
		}()
	}
	for _, id := range ids {
		select {
		case jobs <- id:
		case <-ctx.Done():
			close(jobs)
			workers.Wait()
			return result
		}
	}
	close(jobs)
	workers.Wait()
	c.mu.Lock()
	if c.last == nil {
		c.last = map[string]dockerapi.Stats{}
	}
	for id, sample := range result {
		c.last[id] = sample
	}
	c.mu.Unlock()
	return result
}
