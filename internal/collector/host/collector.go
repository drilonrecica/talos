// SPDX-License-Identifier: AGPL-3.0-only
package host

import (
	"context"
	"github.com/drilonrecica/binnacle/internal/metrics"
	"time"
)

type CollectFunc func(context.Context) (metrics.HostObservation, error)
type Collector struct {
	Interval time.Duration
	Collect  CollectFunc
	Engine   *metrics.Engine
	failures int
	cancel   context.CancelFunc
}

func (c *Collector) Start(ctx context.Context) error {
	if c.Interval < time.Second {
		c.Interval = time.Second
	}
	ctx, c.cancel = context.WithCancel(ctx)
	go func() {
		tick := time.NewTicker(c.Interval)
		defer tick.Stop()
		for {
			c.run(ctx)
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
			}
		}
	}()
	return nil
}
func (c *Collector) Stop(context.Context) error {
	if c.cancel != nil {
		c.cancel()
	}
	return nil
}
func (c *Collector) run(ctx context.Context) {
	o, e := c.Collect(ctx)
	s := metrics.CollectorHealthy
	reason := ""
	if e != nil {
		c.failures++
		reason = e.Error()
		if c.failures >= 3 {
			s = metrics.CollectorDegraded
		}
		if c.failures >= 6 {
			s = metrics.CollectorDown
		}
	} else {
		c.failures = 0
	}
	if c.Engine != nil {
		c.Engine.Publish(metrics.Snapshot{At: time.Now().UTC(), Host: o, Collectors: map[string]metrics.CollectorHealth{"host": {Name: "host", State: s, Reason: reason, FreshAt: time.Now().UTC()}}})
	}
}
