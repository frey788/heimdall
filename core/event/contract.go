package event

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
)

const CurrentVersion = "1"

type EventType string

const (
	EventTypeRequest EventType = "request"
	EventTypeQuery   EventType = "query"
	EventTypeLog     EventType = "log"
	EventTypeError   EventType = "error"
	EventTypeJob     EventType = "job"
	EventTypeCache   EventType = "cache"
	EventTypeCustom  EventType = "custom"
)

func (t EventType) IsValid() bool {
	switch t {
	case EventTypeRequest, EventTypeQuery, EventTypeLog, EventTypeError, EventTypeJob, EventTypeCache, EventTypeCustom:
		return true
	default:
		return false
	}
}

type Transport string

const (
	TransportHTTP Transport = "http"
	TransportGRPC Transport = "grpc"
)

func (t Transport) IsValid() bool {
	switch t {
	case TransportHTTP, TransportGRPC:
		return true
	default:
		return false
	}
}

type Direction string

const (
	DirectionInbound  Direction = "inbound"
	DirectionOutbound Direction = "outbound"
)

func (d Direction) IsValid() bool {
	switch d {
	case DirectionInbound, DirectionOutbound:
		return true
	default:
		return false
	}
}

type RPCType string

const (
	RPCTypeUnary        RPCType = "unary"
	RPCTypeClientStream RPCType = "client_stream"
	RPCTypeServerStream RPCType = "server_stream"
	RPCTypeBidiStream   RPCType = "bidi_stream"
)

func (r RPCType) IsValid() bool {
	switch r {
	case RPCTypeUnary, RPCTypeClientStream, RPCTypeServerStream, RPCTypeBidiStream:
		return true
	default:
		return false
	}
}

type Event struct {
	Version    string            `json:"version"`
	ID         string            `json:"id"`
	Timestamp  time.Time         `json:"timestamp"`
	Type       EventType         `json:"type"`
	Source     string            `json:"source"`
	Transport  Transport         `json:"transport,omitempty"`
	Direction  Direction         `json:"direction,omitempty"`
	TraceID    string            `json:"trace_id,omitempty"`
	RequestID  string            `json:"request_id,omitempty"`
	UserID     string            `json:"user_id,omitempty"`
	DurationMS int64             `json:"duration_ms,omitempty"`
	Status     string            `json:"status,omitempty"`
	Tags       map[string]string `json:"tags,omitempty"`
	Payload    map[string]any    `json:"payload,omitempty"`
	HTTP       *HTTPContext      `json:"http,omitempty"`
	GRPC       *GRPCContext      `json:"grpc,omitempty"`
}

type HTTPContext struct {
	Method        string `json:"method"`
	Path          string `json:"path"`
	Route         string `json:"route,omitempty"`
	StatusCode    int    `json:"status_code"`
	RequestBytes  int64  `json:"request_bytes,omitempty"`
	ResponseBytes int64  `json:"response_bytes,omitempty"`
	ClientIP      string `json:"client_ip,omitempty"`
	UserAgent     string `json:"user_agent,omitempty"`
}

type GRPCContext struct {
	Service       string              `json:"service"`
	Method        string              `json:"method"`
	FullMethod    string              `json:"full_method,omitempty"`
	RPCType       RPCType             `json:"rpc_type"`
	StatusCode    string              `json:"status_code"`
	Peer          string              `json:"peer,omitempty"`
	Deadline      *time.Time          `json:"deadline,omitempty"`
	Metadata      map[string][]string `json:"metadata,omitempty"`
	RequestCount  uint64              `json:"request_count,omitempty"`
	ResponseCount uint64              `json:"response_count,omitempty"`
	RequestBytes  int64               `json:"request_bytes,omitempty"`
	ResponseBytes int64               `json:"response_bytes,omitempty"`
}

type FieldError struct {
	Field   string
	Message string
}

type ValidationErrors []FieldError

func (v ValidationErrors) Error() string {
	parts := make([]string, 0, len(v))
	for _, issue := range v {
		parts = append(parts, fmt.Sprintf("%s: %s", issue.Field, issue.Message))
	}
	sort.Strings(parts)
	return strings.Join(parts, "; ")
}

func (e Event) Validate() error {
	issues := make(ValidationErrors, 0)

	if e.Version == "" {
		issues = append(issues, FieldError{Field: "version", Message: "is required"})
	}
	if e.ID == "" {
		issues = append(issues, FieldError{Field: "id", Message: "is required"})
	}
	if e.Timestamp.IsZero() {
		issues = append(issues, FieldError{Field: "timestamp", Message: "is required"})
	}
	if !e.Type.IsValid() {
		issues = append(issues, FieldError{Field: "type", Message: "is invalid"})
	}
	if e.Source == "" {
		issues = append(issues, FieldError{Field: "source", Message: "is required"})
	}
	if e.DurationMS < 0 {
		issues = append(issues, FieldError{Field: "duration_ms", Message: "cannot be negative"})
	}

	if e.Transport != "" && !e.Transport.IsValid() {
		issues = append(issues, FieldError{Field: "transport", Message: "is invalid"})
	}
	if e.Direction != "" && !e.Direction.IsValid() {
		issues = append(issues, FieldError{Field: "direction", Message: "is invalid"})
	}

	switch e.Transport {
	case TransportHTTP:
		if e.HTTP == nil {
			issues = append(issues, FieldError{Field: "http", Message: "is required when transport is http"})
		} else {
			issues = append(issues, e.HTTP.validate()...)
		}
	case TransportGRPC:
		if e.GRPC == nil {
			issues = append(issues, FieldError{Field: "grpc", Message: "is required when transport is grpc"})
		} else {
			issues = append(issues, e.GRPC.validate()...)
		}
	}

	if len(issues) > 0 {
		return issues
	}

	return nil
}

func (h HTTPContext) validate() ValidationErrors {
	issues := make(ValidationErrors, 0)

	if h.Method == "" {
		issues = append(issues, FieldError{Field: "http.method", Message: "is required"})
	}
	if h.Path == "" {
		issues = append(issues, FieldError{Field: "http.path", Message: "is required"})
	}
	if h.StatusCode < 0 {
		issues = append(issues, FieldError{Field: "http.status_code", Message: "cannot be negative"})
	}
	if h.RequestBytes < 0 {
		issues = append(issues, FieldError{Field: "http.request_bytes", Message: "cannot be negative"})
	}
	if h.ResponseBytes < 0 {
		issues = append(issues, FieldError{Field: "http.response_bytes", Message: "cannot be negative"})
	}

	return issues
}

func (g GRPCContext) validate() ValidationErrors {
	issues := make(ValidationErrors, 0)

	if g.Service == "" {
		issues = append(issues, FieldError{Field: "grpc.service", Message: "is required"})
	}
	if g.Method == "" {
		issues = append(issues, FieldError{Field: "grpc.method", Message: "is required"})
	}
	if !g.RPCType.IsValid() {
		issues = append(issues, FieldError{Field: "grpc.rpc_type", Message: "is invalid"})
	}
	if g.StatusCode == "" {
		issues = append(issues, FieldError{Field: "grpc.status_code", Message: "is required"})
	}
	if g.RequestBytes < 0 {
		issues = append(issues, FieldError{Field: "grpc.request_bytes", Message: "cannot be negative"})
	}
	if g.ResponseBytes < 0 {
		issues = append(issues, FieldError{Field: "grpc.response_bytes", Message: "cannot be negative"})
	}

	return issues
}

func NewEvent(eventType EventType, source string) (Event, error) {
	id, err := NewID()
	if err != nil {
		return Event{}, err
	}

	event := Event{
		Version:   CurrentVersion,
		ID:        id,
		Timestamp: time.Now().UTC(),
		Type:      eventType,
		Source:    source,
	}

	if err := event.Validate(); err != nil {
		return Event{}, err
	}

	return event, nil
}

func NewID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

var DefaultSensitiveMetadataKeys = []string{
	"authorization",
	"proxy-authorization",
	"cookie",
	"set-cookie",
	"x-api-key",
	"x-auth-token",
}

func RedactMetadata(metadata map[string][]string, extraSensitiveKeys ...string) map[string][]string {
	if len(metadata) == 0 {
		return map[string][]string{}
	}

	sensitive := make(map[string]struct{}, len(DefaultSensitiveMetadataKeys)+len(extraSensitiveKeys))
	for _, key := range DefaultSensitiveMetadataKeys {
		sensitive[strings.ToLower(key)] = struct{}{}
	}
	for _, key := range extraSensitiveKeys {
		sensitive[strings.ToLower(key)] = struct{}{}
	}

	result := make(map[string][]string, len(metadata))
	for key, values := range metadata {
		if _, found := sensitive[strings.ToLower(key)]; found {
			result[key] = []string{"[REDACTED]"}
			continue
		}

		copied := make([]string, len(values))
		copy(copied, values)
		result[key] = copied
	}

	return result
}
