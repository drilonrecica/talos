// SPDX-License-Identifier: AGPL-3.0-only
package storage

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/drilonrecica/binnacle/internal/metrics"
)

// Persistence schedules immutable current snapshots into a bounded writer
// queue. Storage failure never blocks collectors or the live Metrics Engine.
type Persistence struct {
	Engine         *metrics.Engine
	Store          *Manager
	Interval       time.Duration
	QueueLimit     int
	Dropped        atomic.Uint64
	QueueDepth     atomic.Int64
	LastWriteNanos atomic.Int64
	Paused         atomic.Bool
	cancel         context.CancelFunc
	wg             sync.WaitGroup
}

func NewPersistence(engine *metrics.Engine, store *Manager, interval time.Duration, limit int) *Persistence {
	return &Persistence{Engine: engine, Store: store, Interval: interval, QueueLimit: limit}
}

func (p *Persistence) Start(parent context.Context) error {
	if p.Interval <= 0 {
		p.Interval = 10 * time.Second
	}
	if p.QueueLimit < 1 {
		p.QueueLimit = 60
	}
	ctx, cancel := context.WithCancel(parent)
	p.cancel = cancel
	queue := make(chan metrics.PersistenceBatch, p.QueueLimit)
	enqueue := func() {
		batch := p.Engine.PersistenceBatch()
		if batch.Snapshot.Sequence == 0 {
			return
		}
		if p.Store != nil && p.Store.EmergencyPause() {
			p.Paused.Store(true)
			p.Dropped.Add(1)
			return
		}
		p.Paused.Store(false)
		select {
		case queue <- batch:
			p.QueueDepth.Store(int64(len(queue)))
		default:
			// Drop the oldest batch to make room for the newest one.
			select {
			case <-queue:
				p.Dropped.Add(1)
			default:
			}
			p.QueueDepth.Store(int64(len(queue)))
			select {
			case queue <- batch:
				p.QueueDepth.Store(int64(len(queue)))
			default:
				p.Dropped.Add(1)
			}
		}
	}
	p.wg.Add(2)
	go func() {
		defer p.wg.Done()
		ticker := time.NewTicker(p.Interval)
		defer ticker.Stop()
		enqueue()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				enqueue()
			}
		}
	}()
	go func() {
		defer p.wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case batch := <-queue:
				p.QueueDepth.Store(int64(len(queue)))
				started := time.Now()
				_ = Retry(ctx, func() error { return p.Store.WriteBatch(ctx, batch) })
				p.LastWriteNanos.Store(time.Since(started).Nanoseconds())
			}
		}
	}()
	return nil
}
func (p *Persistence) Queue() int64                { return p.QueueDepth.Load() }
func (p *Persistence) DroppedCount() uint64        { return p.Dropped.Load() }
func (p *Persistence) WriteLatency() time.Duration { return time.Duration(p.LastWriteNanos.Load()) }
func (p *Persistence) Stop(context.Context) error {
	if p.cancel != nil {
		p.cancel()
	}
	p.wg.Wait()
	return nil
}
