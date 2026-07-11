// SPDX-License-Identifier: AGPL-3.0-only

package metrics

import (
	"context"
	"sync"
	"time"
)

type Engine struct {
	mu          sync.RWMutex
	snapshot    Snapshot
	events      []Event
	maxEvents   int
	subscribers map[uint64]chan Snapshot
	nextSub     uint64
}

func NewEngine(maxEvents int) *Engine {
	if maxEvents < 1 {
		maxEvents = 128
	}
	return &Engine{maxEvents: maxEvents, subscribers: map[uint64]chan Snapshot{}}
}
func (e *Engine) Start(context.Context) error { return nil }
func (e *Engine) Stop(context.Context) error  { return nil }
func (e *Engine) Publish(snapshot Snapshot, events ...Event) {
	e.mu.Lock()
	defer e.mu.Unlock()
	snapshot.Sequence = e.snapshot.Sequence + 1
	if snapshot.At.IsZero() {
		snapshot.At = time.Now().UTC()
	}
	snapshot.At = snapshot.At.UTC()
	snapshot.Resources = append([]ResourceSnapshot(nil), snapshot.Resources...)
	snapshot.Collectors = copyCollectors(snapshot.Collectors)
	e.snapshot = snapshot
	for _, event := range events {
		if event.ID == 0 {
			event.ID = snapshot.Sequence
		}
		event.At = event.At.UTC()
		e.events = append(e.events, event)
	}
	if len(e.events) > e.maxEvents {
		e.events = append([]Event(nil), e.events[len(e.events)-e.maxEvents:]...)
	}
	for _, ch := range e.subscribers {
		select {
		case <-ch:
		default:
		}
		select {
		case ch <- snapshot:
		default:
		}
	}
}
func (e *Engine) Snapshot() Snapshot { e.mu.RLock(); defer e.mu.RUnlock(); return clone(e.snapshot) }
func (e *Engine) EventsAfter(id Sequence) []Event {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := []Event{}
	for _, v := range e.events {
		if v.ID > id {
			out = append(out, v)
		}
	}
	return out
}
func clone(s Snapshot) Snapshot {
	s.Resources = append([]ResourceSnapshot(nil), s.Resources...)
	s.Collectors = copyCollectors(s.Collectors)
	return s
}
func copyCollectors(in map[string]CollectorHealth) map[string]CollectorHealth {
	out := map[string]CollectorHealth{}
	for k, v := range in {
		out[k] = v
	}
	return out
}
