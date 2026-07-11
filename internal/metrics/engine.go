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
	live        map[uint64]chan LiveMessage
	nextSub     uint64
}

func NewEngine(maxEvents int) *Engine {
	if maxEvents < 1 {
		maxEvents = 128
	}
	return &Engine{maxEvents: maxEvents, subscribers: map[uint64]chan Snapshot{}, live: map[uint64]chan LiveMessage{}}
}

type LiveMessage struct {
	Snapshot *Snapshot
	Event    *Event
}
type Subscription struct {
	C      <-chan LiveMessage
	cancel func()
}

func (s *Subscription) Close() {
	if s.cancel != nil {
		s.cancel()
	}
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
	for id, ch := range e.live {
		sendLive(ch, LiveMessage{Snapshot: snapshotPtr(clone(snapshot))})
		for _, event := range events {
			eventCopy := event
			if !sendLive(ch, LiveMessage{Event: &eventCopy}) {
				delete(e.live, id)
				close(ch)
				break
			}
		}
	}
}
func (e *Engine) Subscribe() *Subscription {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.nextSub++
	id := e.nextSub
	ch := make(chan LiveMessage, 33)
	e.live[id] = ch
	if e.snapshot.Sequence > 0 {
		sendLive(ch, LiveMessage{Snapshot: snapshotPtr(clone(e.snapshot))})
	}
	return &Subscription{C: ch, cancel: func() {
		e.mu.Lock()
		defer e.mu.Unlock()
		if c, ok := e.live[id]; ok {
			delete(e.live, id)
			close(c)
		}
	}}
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
func snapshotPtr(s Snapshot) *Snapshot { return &s }
func sendLive(ch chan LiveMessage, message LiveMessage) bool {
	if message.Snapshot != nil {
		select {
		case <-ch:
		default:
		}
		select {
		case ch <- message:
		default:
		}
		return true
	}
	select {
	case ch <- message:
		return true
	default:
		return false
	}
}
