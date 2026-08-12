package dashboard

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/frey788/heimdall/core/event"
	"github.com/frey788/heimdall/installer/config"
)

const (
	DefaultEventLimit = 100
	DefaultMaxLimit   = 500
)

type EventReader interface {
	Count() int
	Snapshot(limit int) []event.Event
}

type HandlerOptions struct {
	Protection   config.ProtectionConfig
	DefaultLimit int
	MaxLimit     int
}

type Handler struct {
	reader EventReader
	opts   HandlerOptions
}

type eventQuery struct {
	Transport     event.Transport
	Status        string
	MinDurationMS *int64
	MaxDurationMS *int64
	Path          string
	Method        string
	Service       string
	Page          int
	PerPage       int
}

func NewHandler(reader EventReader, opts HandlerOptions) http.Handler {
	if opts.DefaultLimit <= 0 {
		opts.DefaultLimit = DefaultEventLimit
	}
	if opts.MaxLimit <= 0 {
		opts.MaxLimit = DefaultMaxLimit
	}

	h := &Handler{reader: reader, opts: opts}
	mux := http.NewServeMux()
	mux.HandleFunc("/", h.index)
	mux.HandleFunc("/health", h.health)
	mux.HandleFunc("/overview", h.overview)
	mux.HandleFunc("/events", h.events)
	mux.HandleFunc("/errors", h.errors)
	mux.HandleFunc("/performance", h.performance)
	mux.HandleFunc("/watchers", h.watchers)

	if !opts.Protection.Enabled {
		return mux
	}

	return h.withPINProtection(mux)
}

func (h *Handler) withPINProtection(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		pin := req.Header.Get("X-Heimdall-PIN")
		if pin == "" {
			pin = req.URL.Query().Get("pin")
		}

		ok, err := h.opts.Protection.VerifyPIN(pin)
		if err != nil || !ok {
			writeJSON(rw, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
			return
		}

		next.ServeHTTP(rw, req)
	})
}

func (h *Handler) index(rw http.ResponseWriter, req *http.Request) {
	if req.URL.Path != "/" {
		http.NotFound(rw, req)
		return
	}

	rw.Header().Set("Content-Type", "text/html; charset=utf-8")
	rw.WriteHeader(http.StatusOK)
	_, _ = rw.Write([]byte(renderIndexHTML()))
}

func (h *Handler) health(rw http.ResponseWriter, _ *http.Request) {
	writeJSON(rw, http.StatusOK, map[string]any{
		"status": "ok",
		"events": h.reader.Count(),
	})
}

func (h *Handler) overview(rw http.ResponseWriter, _ *http.Request) {
	events := h.reader.Snapshot(h.reader.Count())
	overview := buildOverview(events)
	writeJSON(rw, http.StatusOK, overview)
}

func (h *Handler) events(rw http.ResponseWriter, req *http.Request) {
	query, filtered, paged, totalPages, err := h.queryEvents(req)
	if err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	writeJSON(rw, http.StatusOK, map[string]any{
		"page":        query.Page,
		"per_page":    query.PerPage,
		"total":       len(filtered),
		"total_pages": totalPages,
		"count":       len(paged),
		"filters": map[string]any{
			"transport":       query.Transport,
			"status":          query.Status,
			"min_duration_ms": query.MinDurationMS,
			"max_duration_ms": query.MaxDurationMS,
			"path":            query.Path,
			"method":          query.Method,
			"service":         query.Service,
		},
		"events": paged,
	})
}

func (h *Handler) errors(rw http.ResponseWriter, req *http.Request) {
	query, err := h.parseEventQuery(req)
	if err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	events := h.reader.Snapshot(h.reader.Count())
	filtered := filterEvents(events, query)
	errorEvents := make([]event.Event, 0, len(filtered))
	for _, e := range filtered {
		if isErrorEvent(e) {
			errorEvents = append(errorEvents, e)
		}
	}

	paged, totalPages := paginateEvents(errorEvents, query.Page, query.PerPage)
	writeJSON(rw, http.StatusOK, map[string]any{
		"page":        query.Page,
		"per_page":    query.PerPage,
		"total":       len(errorEvents),
		"total_pages": totalPages,
		"count":       len(paged),
		"events":      paged,
	})
}

func (h *Handler) performance(rw http.ResponseWriter, req *http.Request) {
	query, err := h.parseEventQuery(req)
	if err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	events := h.reader.Snapshot(h.reader.Count())
	filtered := filterEvents(events, query)
	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].DurationMS == filtered[j].DurationMS {
			return filtered[i].Timestamp.After(filtered[j].Timestamp)
		}
		return filtered[i].DurationMS > filtered[j].DurationMS
	})

	paged, totalPages := paginateEvents(filtered, query.Page, query.PerPage)
	writeJSON(rw, http.StatusOK, map[string]any{
		"page":        query.Page,
		"per_page":    query.PerPage,
		"total":       len(filtered),
		"total_pages": totalPages,
		"count":       len(paged),
		"events":      paged,
	})
}

func (h *Handler) watchers(rw http.ResponseWriter, _ *http.Request) {
	events := h.reader.Snapshot(h.reader.Count())
	statsBySource := map[string]*watcherStat{}

	for _, e := range events {
		source := e.Source
		if source == "" {
			source = "unknown"
		}

		stat := statsBySource[source]
		if stat == nil {
			stat = &watcherStat{Source: source}
			statsBySource[source] = stat
		}

		stat.Events++
		stat.DurationTotalMS += e.DurationMS
		if isErrorEvent(e) {
			stat.Errors++
		}
		if stat.LastSeen.IsZero() || e.Timestamp.After(stat.LastSeen) {
			stat.LastSeen = e.Timestamp
		}
	}

	stats := make([]watcherStat, 0, len(statsBySource))
	for _, stat := range statsBySource {
		if stat.Events > 0 {
			stat.AvgDurationMS = float64(stat.DurationTotalMS) / float64(stat.Events)
		}
		stats = append(stats, *stat)
	}

	sort.SliceStable(stats, func(i, j int) bool {
		if stats[i].Events == stats[j].Events {
			return stats[i].Source < stats[j].Source
		}
		return stats[i].Events > stats[j].Events
	})

	writeJSON(rw, http.StatusOK, map[string]any{
		"count":    len(stats),
		"watchers": stats,
	})
}

type watcherStat struct {
	Source          string    `json:"source"`
	Events          int       `json:"events"`
	Errors          int       `json:"errors"`
	AvgDurationMS   float64   `json:"avg_duration_ms"`
	DurationTotalMS int64     `json:"-"`
	LastSeen        time.Time `json:"last_seen"`
}

func (h *Handler) queryEvents(req *http.Request) (eventQuery, []event.Event, []event.Event, int, error) {
	query, err := h.parseEventQuery(req)
	if err != nil {
		return eventQuery{}, nil, nil, 0, err
	}

	events := h.reader.Snapshot(h.reader.Count())
	filtered := filterEvents(events, query)
	paged, totalPages := paginateEvents(filtered, query.Page, query.PerPage)

	return query, filtered, paged, totalPages, nil
}

func (h *Handler) parseEventQuery(req *http.Request) (eventQuery, error) {
	q := req.URL.Query()
	result := eventQuery{
		Page:    1,
		PerPage: h.opts.DefaultLimit,
	}

	if rawTransport := strings.TrimSpace(q.Get("transport")); rawTransport != "" {
		transport := event.Transport(strings.ToLower(rawTransport))
		if !transport.IsValid() {
			return eventQuery{}, errf("transport must be one of: http, grpc")
		}
		result.Transport = transport
	}

	result.Status = strings.TrimSpace(q.Get("status"))
	result.Path = strings.TrimSpace(q.Get("path"))
	result.Method = strings.ToUpper(strings.TrimSpace(q.Get("method")))
	result.Service = strings.TrimSpace(q.Get("service"))

	if raw := strings.TrimSpace(q.Get("min_duration_ms")); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || value < 0 {
			return eventQuery{}, errf("min_duration_ms must be a non-negative integer")
		}
		result.MinDurationMS = &value
	}

	if raw := strings.TrimSpace(q.Get("max_duration_ms")); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || value < 0 {
			return eventQuery{}, errf("max_duration_ms must be a non-negative integer")
		}
		result.MaxDurationMS = &value
	}

	if result.MinDurationMS != nil && result.MaxDurationMS != nil && *result.MaxDurationMS < *result.MinDurationMS {
		return eventQuery{}, errf("max_duration_ms must be greater than or equal to min_duration_ms")
	}

	if raw := strings.TrimSpace(q.Get("page")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value <= 0 {
			return eventQuery{}, errf("page must be a positive integer")
		}
		result.Page = value
	}

	if raw := strings.TrimSpace(q.Get("per_page")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value <= 0 {
			return eventQuery{}, errf("per_page must be a positive integer")
		}
		result.PerPage = value
	} else if raw := strings.TrimSpace(q.Get("limit")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value <= 0 {
			return eventQuery{}, errf("limit must be a positive integer")
		}
		result.PerPage = value
	}

	if result.PerPage > h.opts.MaxLimit {
		result.PerPage = h.opts.MaxLimit
	}

	return result, nil
}

func filterEvents(events []event.Event, query eventQuery) []event.Event {
	if len(events) == 0 {
		return []event.Event{}
	}

	filtered := make([]event.Event, 0, len(events))
	for _, e := range events {
		if query.Transport != "" && e.Transport != query.Transport {
			continue
		}
		if query.Status != "" && !strings.EqualFold(e.Status, query.Status) {
			continue
		}
		if query.MinDurationMS != nil && e.DurationMS < *query.MinDurationMS {
			continue
		}
		if query.MaxDurationMS != nil && e.DurationMS > *query.MaxDurationMS {
			continue
		}
		if query.Path != "" {
			if e.HTTP == nil || e.HTTP.Path != query.Path {
				continue
			}
		}
		if query.Method != "" {
			if e.HTTP == nil || !strings.EqualFold(e.HTTP.Method, query.Method) {
				continue
			}
		}
		if query.Service != "" {
			if e.GRPC == nil || e.GRPC.Service != query.Service {
				continue
			}
		}

		filtered = append(filtered, e)
	}

	return filtered
}

func isErrorEvent(e event.Event) bool {
	if e.Transport == event.TransportHTTP {
		if e.HTTP != nil && e.HTTP.StatusCode >= http.StatusBadRequest {
			return true
		}
		statusCode, err := strconv.Atoi(e.Status)
		return err == nil && statusCode >= http.StatusBadRequest
	}

	if e.Transport == event.TransportGRPC {
		statusValue := strings.ToUpper(strings.TrimSpace(e.Status))
		if statusValue == "" && e.GRPC != nil {
			statusValue = strings.ToUpper(strings.TrimSpace(e.GRPC.StatusCode))
		}
		return statusValue != "" && statusValue != "OK"
	}

	return false
}

func buildOverview(events []event.Event) map[string]any {
	httpEvents := 0
	grpcEvents := 0
	errorEvents := 0
	durations := make([]int64, 0, len(events))

	for _, e := range events {
		switch e.Transport {
		case event.TransportHTTP:
			httpEvents++
		case event.TransportGRPC:
			grpcEvents++
		}

		durations = append(durations, e.DurationMS)
		if isErrorEvent(e) {
			errorEvents++
		}
	}

	errorRate := 0.0
	avgDuration := 0.0
	p95Duration := int64(0)

	if len(events) > 0 {
		errorRate = float64(errorEvents) / float64(len(events))
		var total int64
		for _, duration := range durations {
			total += duration
		}
		avgDuration = float64(total) / float64(len(durations))
		p95Duration = percentileDuration(durations, 95)
	}

	return map[string]any{
		"status":                  "ok",
		"generated_at":            time.Now().UTC(),
		"total_events":            len(events),
		"http_events":             httpEvents,
		"grpc_events":             grpcEvents,
		"error_events":            errorEvents,
		"error_rate":              errorRate,
		"avg_duration_ms":         avgDuration,
		"p95_duration_ms":         p95Duration,
		"dashboard_menu_sections": []string{"Overview", "Event Explorer", "HTTP Requests", "gRPC Requests", "Errors", "Performance", "Watchers"},
	}
}

func percentileDuration(durations []int64, percentile int) int64 {
	if len(durations) == 0 {
		return 0
	}

	copyDurations := make([]int64, len(durations))
	copy(copyDurations, durations)
	sort.Slice(copyDurations, func(i, j int) bool {
		return copyDurations[i] < copyDurations[j]
	})

	index := (percentile*len(copyDurations) + 99) / 100
	if index <= 0 {
		index = 1
	}
	if index > len(copyDurations) {
		index = len(copyDurations)
	}

	return copyDurations[index-1]
}

func paginateEvents(events []event.Event, page int, perPage int) ([]event.Event, int) {
	total := len(events)
	if total == 0 {
		return []event.Event{}, 0
	}

	totalPages := (total + perPage - 1) / perPage
	start := (page - 1) * perPage
	if start >= total {
		return []event.Event{}, totalPages
	}

	end := start + perPage
	if end > total {
		end = total
	}

	return events[start:end], totalPages
}

type queryError struct {
	message string
}

func (e queryError) Error() string {
	return e.message
}

func errf(message string) error {
	return queryError{message: message}
}

func writeJSON(rw http.ResponseWriter, status int, payload any) {
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(status)
	_ = json.NewEncoder(rw).Encode(payload)
}
