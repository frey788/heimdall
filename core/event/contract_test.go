package event

import (
	"strings"
	"testing"
	"time"
)

func TestEventValidateValidGRPCEvent(t *testing.T) {
	deadline := time.Now().UTC().Add(2 * time.Second)

	e := Event{
		Version:    CurrentVersion,
		ID:         "evt_123",
		Timestamp:  time.Now().UTC(),
		Type:       EventTypeRequest,
		Source:     "grpc.server",
		Transport:  TransportGRPC,
		Direction:  DirectionInbound,
		DurationMS: 24,
		GRPC: &GRPCContext{
			Service:       "users.UserService",
			Method:        "GetProfile",
			FullMethod:    "/users.UserService/GetProfile",
			RPCType:       RPCTypeUnary,
			StatusCode:    "OK",
			Peer:          "127.0.0.1:40000",
			Deadline:      &deadline,
			RequestCount:  1,
			ResponseCount: 1,
			RequestBytes:  128,
			ResponseBytes: 256,
		},
	}

	if err := e.Validate(); err != nil {
		t.Fatalf("expected no validation error, got %v", err)
	}
}

func TestEventValidateMissingRequiredFields(t *testing.T) {
	e := Event{}
	err := e.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}

	msg := err.Error()
	checks := []string{"version", "id", "timestamp", "type", "source"}
	for _, field := range checks {
		if !strings.Contains(msg, field) {
			t.Fatalf("expected validation error containing %s, got %s", field, msg)
		}
	}
}

func TestEventValidateRequiresTransportContext(t *testing.T) {
	e := Event{
		Version:   CurrentVersion,
		ID:        "evt_123",
		Timestamp: time.Now().UTC(),
		Type:      EventTypeRequest,
		Source:    "grpc.server",
		Transport: TransportGRPC,
	}

	err := e.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "grpc") {
		t.Fatalf("expected grpc validation error, got %v", err)
	}
}

func TestNewID(t *testing.T) {
	id1, err := NewID()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	id2, err := NewID()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(id1) != 32 || len(id2) != 32 {
		t.Fatalf("expected 32-char IDs, got %d and %d", len(id1), len(id2))
	}
	if id1 == id2 {
		t.Fatalf("expected unique IDs, got duplicate %s", id1)
	}
}

func TestRedactMetadata(t *testing.T) {
	meta := map[string][]string{
		"authorization": {"Bearer token"},
		"x-api-key":     {"abc"},
		"x-trace-id":    {"trace-1"},
		"x-secret":      {"secret-value"},
	}

	redacted := RedactMetadata(meta, "x-secret")

	if redacted["authorization"][0] != "[REDACTED]" {
		t.Fatalf("expected authorization header to be redacted")
	}
	if redacted["x-api-key"][0] != "[REDACTED]" {
		t.Fatalf("expected x-api-key header to be redacted")
	}
	if redacted["x-secret"][0] != "[REDACTED]" {
		t.Fatalf("expected custom sensitive header to be redacted")
	}
	if redacted["x-trace-id"][0] != "trace-1" {
		t.Fatalf("expected non-sensitive header to be preserved")
	}
}

func TestNewEvent(t *testing.T) {
	e, err := NewEvent(EventTypeRequest, "http.server")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if e.Version != CurrentVersion {
		t.Fatalf("expected version %s, got %s", CurrentVersion, e.Version)
	}
	if e.ID == "" {
		t.Fatal("expected generated id")
	}
	if e.Type != EventTypeRequest {
		t.Fatalf("expected type %s, got %s", EventTypeRequest, e.Type)
	}
	if e.Source != "http.server" {
		t.Fatalf("expected source http.server, got %s", e.Source)
	}
}
