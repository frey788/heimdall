package memory

import (
	"context"
	"sync"

	"github.com/frey788/heimdall/core/event"
)

const DefaultMaxEvents = 5000

type Store struct {
	mu        sync.RWMutex
	maxEvents int
	events    []event.Event
}

func NewStore(maxEvents int) *Store {
	if maxEvents <= 0 {
		maxEvents = DefaultMaxEvents
	}

	return &Store{
		maxEvents: maxEvents,
		events:    make([]event.Event, 0, maxEvents),
	}
}

func (s *Store) Emit(_ context.Context, e event.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.events = append(s.events, cloneEvent(e))
	if overflow := len(s.events) - s.maxEvents; overflow > 0 {
		copy(s.events, s.events[overflow:])
		s.events = s.events[:s.maxEvents]
	}

	return nil
}

func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.events)
}

func (s *Store) Snapshot(limit int) []event.Event {
	s.mu.RLock()
	defer s.mu.RUnlock()

	total := len(s.events)
	if total == 0 {
		return []event.Event{}
	}

	if limit <= 0 || limit > total {
		limit = total
	}

	out := make([]event.Event, 0, limit)
	for i := 0; i < limit; i++ {
		index := total - 1 - i
		out = append(out, cloneEvent(s.events[index]))
	}

	return out
}

func cloneEvent(in event.Event) event.Event {
	out := in
	out.Tags = cloneStringMap(in.Tags)
	out.Payload = cloneAnyMap(in.Payload)

	if in.HTTP != nil {
		h := *in.HTTP
		out.HTTP = &h
	}

	if in.GRPC != nil {
		g := *in.GRPC
		if in.GRPC.Deadline != nil {
			d := *in.GRPC.Deadline
			g.Deadline = &d
		}
		g.Metadata = cloneMetadata(in.GRPC.Metadata)
		out.GRPC = &g
	}

	return out
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}

	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}

	return out
}

func cloneAnyMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}

	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}

	return out
}

func cloneMetadata(in map[string][]string) map[string][]string {
	if len(in) == 0 {
		return nil
	}

	out := make(map[string][]string, len(in))
	for key, values := range in {
		copied := make([]string, len(values))
		copy(copied, values)
		out[key] = copied
	}

	return out
}
