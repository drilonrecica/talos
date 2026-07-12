// SPDX-License-Identifier: AGPL-3.0-only
package docker

import (
	"context"
	"github.com/drilonrecica/binnacle/internal/dockerapi"
	"time"
)

func (c *Cache) Maintain(ctx context.Context, client dockerapi.Client, interval time.Duration) {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	seen := map[string]struct{}{}
	tick := time.NewTicker(interval)
	defer tick.Stop()
	events := client.Events(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			_ = c.Discover(ctx, client)
		case e, ok := <-events:
			if !ok {
				return
			}
			key := e.ID + e.Action + e.Time
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			if e.Action == "destroy" {
				c.Remove(e.ID)
			} else if e.Action == "create" || e.Action == "start" || e.Action == "rename" || e.Action == "health_status" {
				_ = c.Discover(ctx, client)
			}
		}
	}
}
