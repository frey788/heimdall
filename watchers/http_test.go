package watchers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/frey788/heimdall/core/event"
)

type captureEmitter struct {
	events []event.Event
}

func (c *captureEmitter) Emit(_ context.Context, e event.Event) error {
	c.events = append(c.events, e)
	return nil
}

func TestHTTPMiddlewareEmitsEvent(t *testing.T) {
	emitter := &captureEmitter{}

	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(http.StatusCreated)
		_, _ = rw.Write([]byte("ok"))
	})

	middleware := HTTPMiddleware(next, emitter)
	req := httptest.NewRequest(http.MethodPost, "/books/42", nil)
	req.Header.Set("x-trace-id", "trace-1")
	req.Header.Set("x-request-id", "req-1")
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	if len(emitter.events) != 1 {
		t.Fatalf("expected one emitted event, got %d", len(emitter.events))
	}

	emitted := emitter.events[0]
	if emitted.Transport != event.TransportHTTP {
		t.Fatalf("expected transport %s, got %s", event.TransportHTTP, emitted.Transport)
	}
	if emitted.Direction != event.DirectionInbound {
		t.Fatalf("expected direction %s, got %s", event.DirectionInbound, emitted.Direction)
	}
	if emitted.Status != "201" {
		t.Fatalf("expected status 201, got %s", emitted.Status)
	}
	if emitted.HTTP == nil {
		t.Fatal("expected http context")
	}
	if emitted.HTTP.Path != "/books/42" {
		t.Fatalf("expected path /books/42, got %s", emitted.HTTP.Path)
	}
	if emitted.TraceID != "trace-1" {
		t.Fatalf("expected trace id trace-1, got %s", emitted.TraceID)
	}
	if emitted.RequestID != "req-1" {
		t.Fatalf("expected request id req-1, got %s", emitted.RequestID)
	}
}
