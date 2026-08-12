package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/frey788/heimdall/core/event"
	"github.com/frey788/heimdall/installer/config"
	"github.com/frey788/heimdall/store/memory"
)

func TestHandlerRequiresPINWhenEnabled(t *testing.T) {
	store := memory.NewStore(100)
	_ = store.Emit(context.Background(), sampleHTTPEvent("1", "GET", "/", "200", 1))

	cfg, err := config.BuildEmbeddedConfig(config.InstallerInput{
		DashboardPath: "/_heimdall",
		PINEnabled:    true,
		PIN:           "1234",
	})
	if err != nil {
		t.Fatalf("failed to build config: %v", err)
	}

	h := NewHandler(store, HandlerOptions{Protection: cfg.Protection})

	unauthorized := httptest.NewRecorder()
	h.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/health", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without pin, got %d", unauthorized.Code)
	}

	authorized := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("X-Heimdall-PIN", "1234")
	h.ServeHTTP(authorized, req)
	if authorized.Code != http.StatusOK {
		t.Fatalf("expected 200 with correct pin, got %d", authorized.Code)
	}
}

func TestHandlerEventsLimit(t *testing.T) {
	store := memory.NewStore(100)
	_ = store.Emit(context.Background(), sampleHTTPEvent("1", "GET", "/", "200", 1))
	_ = store.Emit(context.Background(), sampleHTTPEvent("2", "GET", "/", "200", 2))

	h := NewHandler(store, HandlerOptions{DefaultLimit: 1, MaxLimit: 10})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/events", nil)
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var response eventsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Count != 1 {
		t.Fatalf("expected response count 1, got %d", response.Count)
	}
}

func TestHandlerEventsFilters(t *testing.T) {
	store := memory.NewStore(100)
	_ = store.Emit(context.Background(), sampleHTTPEvent("1", "GET", "/users", "200", 11))
	_ = store.Emit(context.Background(), sampleHTTPEvent("2", "POST", "/orders", "500", 120))
	_ = store.Emit(context.Background(), sampleGRPCEvent("3", "users.UserService", "GetProfile", "OK", 15))

	h := NewHandler(store, HandlerOptions{DefaultLimit: 20, MaxLimit: 50})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/events?transport=http&method=GET&path=/users&status=200&min_duration_ms=10&max_duration_ms=20", nil)
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var response eventsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Total != 1 || response.Count != 1 {
		t.Fatalf("expected total/count to be 1/1, got %d/%d", response.Total, response.Count)
	}
	if len(response.Events) != 1 {
		t.Fatalf("expected exactly one event, got %d", len(response.Events))
	}
	if response.Events[0].ID != "1" {
		t.Fatalf("expected filtered event id 1, got %s", response.Events[0].ID)
	}
}

func TestHandlerEventsServiceFilter(t *testing.T) {
	store := memory.NewStore(100)
	_ = store.Emit(context.Background(), sampleHTTPEvent("1", "GET", "/users", "200", 11))
	_ = store.Emit(context.Background(), sampleGRPCEvent("2", "users.UserService", "GetProfile", "OK", 15))
	_ = store.Emit(context.Background(), sampleGRPCEvent("3", "orders.OrderService", "Create", "OK", 25))

	h := NewHandler(store, HandlerOptions{DefaultLimit: 20, MaxLimit: 50})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/events?transport=grpc&service=users.UserService", nil)
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var response eventsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Total != 1 || response.Count != 1 {
		t.Fatalf("expected total/count to be 1/1, got %d/%d", response.Total, response.Count)
	}
	if response.Events[0].GRPC == nil || response.Events[0].GRPC.Service != "users.UserService" {
		t.Fatalf("expected grpc service users.UserService")
	}
}

func TestHandlerEventsPagination(t *testing.T) {
	store := memory.NewStore(100)
	_ = store.Emit(context.Background(), sampleHTTPEvent("1", "GET", "/a", "200", 1))
	_ = store.Emit(context.Background(), sampleHTTPEvent("2", "GET", "/b", "200", 2))
	_ = store.Emit(context.Background(), sampleHTTPEvent("3", "GET", "/c", "200", 3))

	h := NewHandler(store, HandlerOptions{DefaultLimit: 20, MaxLimit: 50})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/events?per_page=1&page=2", nil)
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var response eventsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Page != 2 || response.PerPage != 1 {
		t.Fatalf("unexpected page/per_page: %d/%d", response.Page, response.PerPage)
	}
	if response.Total != 3 || response.TotalPages != 3 {
		t.Fatalf("unexpected total/pages: %d/%d", response.Total, response.TotalPages)
	}
	if response.Count != 1 || len(response.Events) != 1 {
		t.Fatalf("expected one event on paged result")
	}
	if response.Events[0].ID != "2" {
		t.Fatalf("expected second newest event id 2, got %s", response.Events[0].ID)
	}
}

func TestHandlerEventsRejectsInvalidRange(t *testing.T) {
	store := memory.NewStore(100)
	h := NewHandler(store, HandlerOptions{DefaultLimit: 20, MaxLimit: 50})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/events?min_duration_ms=20&max_duration_ms=10", nil)
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

type eventsResponse struct {
	Page       int           `json:"page"`
	PerPage    int           `json:"per_page"`
	Total      int           `json:"total"`
	TotalPages int           `json:"total_pages"`
	Count      int           `json:"count"`
	Events     []event.Event `json:"events"`
}

type overviewResponse struct {
	Status      string  `json:"status"`
	TotalEvents int     `json:"total_events"`
	HTTPEvents  int     `json:"http_events"`
	GRPCEvents  int     `json:"grpc_events"`
	ErrorEvents int     `json:"error_events"`
	ErrorRate   float64 `json:"error_rate"`
}

type watcherResponse struct {
	Count    int           `json:"count"`
	Watchers []watcherItem `json:"watchers"`
}

type watcherItem struct {
	Source string `json:"source"`
	Events int    `json:"events"`
	Errors int    `json:"errors"`
}

func TestHandlerOverviewEndpoint(t *testing.T) {
	store := memory.NewStore(100)
	_ = store.Emit(context.Background(), sampleHTTPEvent("1", "GET", "/users", "200", 10))
	_ = store.Emit(context.Background(), sampleHTTPEvent("2", "POST", "/orders", "500", 20))
	_ = store.Emit(context.Background(), sampleGRPCEvent("3", "users.UserService", "Get", "OK", 30))

	h := NewHandler(store, HandlerOptions{DefaultLimit: 20, MaxLimit: 50})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/overview", nil)
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var response overviewResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Status != "ok" {
		t.Fatalf("expected status ok, got %s", response.Status)
	}
	if response.TotalEvents != 3 {
		t.Fatalf("expected total events 3, got %d", response.TotalEvents)
	}
	if response.HTTPEvents != 2 || response.GRPCEvents != 1 {
		t.Fatalf("expected http/grpc counts 2/1, got %d/%d", response.HTTPEvents, response.GRPCEvents)
	}
	if response.ErrorEvents != 1 {
		t.Fatalf("expected 1 error event, got %d", response.ErrorEvents)
	}
}

func TestHandlerErrorsEndpoint(t *testing.T) {
	store := memory.NewStore(100)
	_ = store.Emit(context.Background(), sampleHTTPEvent("1", "GET", "/users", "200", 10))
	_ = store.Emit(context.Background(), sampleHTTPEvent("2", "POST", "/orders", "500", 20))
	_ = store.Emit(context.Background(), sampleGRPCEvent("3", "users.UserService", "Get", "INTERNAL", 30))

	h := NewHandler(store, HandlerOptions{DefaultLimit: 20, MaxLimit: 50})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/errors", nil)
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var response eventsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Total != 2 {
		t.Fatalf("expected 2 error events, got %d", response.Total)
	}
}

func TestHandlerPerformanceEndpointSortsByDurationDesc(t *testing.T) {
	store := memory.NewStore(100)
	_ = store.Emit(context.Background(), sampleHTTPEvent("1", "GET", "/a", "200", 20))
	_ = store.Emit(context.Background(), sampleHTTPEvent("2", "GET", "/b", "200", 80))
	_ = store.Emit(context.Background(), sampleHTTPEvent("3", "GET", "/c", "200", 40))

	h := NewHandler(store, HandlerOptions{DefaultLimit: 20, MaxLimit: 50})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/performance", nil)
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var response eventsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Events) < 3 {
		t.Fatalf("expected at least 3 events")
	}
	if response.Events[0].ID != "2" || response.Events[1].ID != "3" || response.Events[2].ID != "1" {
		t.Fatalf("unexpected performance ordering: %s, %s, %s", response.Events[0].ID, response.Events[1].ID, response.Events[2].ID)
	}
}

func TestHandlerWatchersEndpoint(t *testing.T) {
	store := memory.NewStore(100)
	first := sampleHTTPEvent("1", "GET", "/a", "200", 20)
	first.Source = "watchers.http"
	second := sampleHTTPEvent("2", "GET", "/b", "500", 30)
	second.Source = "watchers.http"
	third := sampleGRPCEvent("3", "svc.A", "Get", "OK", 40)
	third.Source = "watchers.grpc.server"

	_ = store.Emit(context.Background(), first)
	_ = store.Emit(context.Background(), second)
	_ = store.Emit(context.Background(), third)

	h := NewHandler(store, HandlerOptions{DefaultLimit: 20, MaxLimit: 50})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/watchers", nil)
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var response watcherResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Count != 2 {
		t.Fatalf("expected 2 watcher rows, got %d", response.Count)
	}
	if len(response.Watchers) == 0 || response.Watchers[0].Source != "watchers.http" {
		t.Fatalf("expected first watcher to be watchers.http")
	}
	if response.Watchers[0].Events != 2 || response.Watchers[0].Errors != 1 {
		t.Fatalf("unexpected watcher aggregate for watchers.http: events=%d errors=%d", response.Watchers[0].Events, response.Watchers[0].Errors)
	}
}

func sampleHTTPEvent(id, method, path, status string, durationMS int64) event.Event {
	return event.Event{
		Version:    event.CurrentVersion,
		ID:         id,
		Timestamp:  time.Now().UTC(),
		Type:       event.EventTypeRequest,
		Source:     "test",
		Transport:  event.TransportHTTP,
		Direction:  event.DirectionInbound,
		DurationMS: durationMS,
		Status:     status,
		HTTP: &event.HTTPContext{
			Method:     method,
			Path:       path,
			StatusCode: 200,
		},
	}
}

func sampleGRPCEvent(id, service, method, status string, durationMS int64) event.Event {
	return event.Event{
		Version:    event.CurrentVersion,
		ID:         id,
		Timestamp:  time.Now().UTC(),
		Type:       event.EventTypeRequest,
		Source:     "test",
		Transport:  event.TransportGRPC,
		Direction:  event.DirectionInbound,
		DurationMS: durationMS,
		Status:     status,
		GRPC: &event.GRPCContext{
			Service:    service,
			Method:     method,
			RPCType:    event.RPCTypeUnary,
			StatusCode: status,
		},
	}
}
