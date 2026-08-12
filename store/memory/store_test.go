package memory

import (
	"context"
	"testing"
	"time"

	"github.com/frey788/heimdall/core/event"
)

func TestStoreSnapshotNewestFirst(t *testing.T) {
	store := NewStore(10)

	e1 := makeEvent("1", time.Now().UTC())
	e2 := makeEvent("2", time.Now().UTC().Add(1*time.Second))
	_ = store.Emit(context.Background(), e1)
	_ = store.Emit(context.Background(), e2)

	snapshot := store.Snapshot(10)
	if len(snapshot) != 2 {
		t.Fatalf("expected 2 events, got %d", len(snapshot))
	}
	if snapshot[0].ID != "2" {
		t.Fatalf("expected newest event first, got %s", snapshot[0].ID)
	}
}

func TestStoreRetention(t *testing.T) {
	store := NewStore(2)
	_ = store.Emit(context.Background(), makeEvent("1", time.Now().UTC()))
	_ = store.Emit(context.Background(), makeEvent("2", time.Now().UTC()))
	_ = store.Emit(context.Background(), makeEvent("3", time.Now().UTC()))

	snapshot := store.Snapshot(10)
	if len(snapshot) != 2 {
		t.Fatalf("expected retained size 2, got %d", len(snapshot))
	}
	if snapshot[0].ID != "3" || snapshot[1].ID != "2" {
		t.Fatalf("unexpected retained order: %s, %s", snapshot[0].ID, snapshot[1].ID)
	}
}

func makeEvent(id string, at time.Time) event.Event {
	return event.Event{
		Version:    event.CurrentVersion,
		ID:         id,
		Timestamp:  at,
		Type:       event.EventTypeRequest,
		Source:     "test",
		Transport:  event.TransportHTTP,
		Direction:  event.DirectionInbound,
		DurationMS: 1,
		Status:     "200",
		HTTP: &event.HTTPContext{
			Method:     "GET",
			Path:       "/",
			StatusCode: 200,
		},
	}
}
