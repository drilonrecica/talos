// SPDX-License-Identifier: AGPL-3.0-only
package demo

import (
	"context"
	"github.com/drilonrecica/talos/internal/metrics"
	"time"
)

type Component struct {
	Generator *Generator
	Engine    *metrics.Engine
	Interval  time.Duration
}

func (c *Component) Start(ctx context.Context) error {
	if c.Interval <= 0 {
		c.Interval = 2 * time.Second
	}
	go func() {
		tick := uint64(0)
		t := time.NewTicker(c.Interval)
		defer t.Stop()
		for {
			c.Engine.Publish(c.Generator.Snapshot(tick), c.Generator.Events(tick)...)
			tick++
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			}
		}
	}()
	return nil
}
func (c *Component) Stop(context.Context) error { return nil }
