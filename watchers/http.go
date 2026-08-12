package watchers

import (
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/frey788/heimdall/core/event"
)

type responseCapture struct {
	http.ResponseWriter
	statusCode int
	bytes      int64
}

func (w *responseCapture) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *responseCapture) Write(data []byte) (int, error) {
	if w.statusCode == 0 {
		w.statusCode = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(data)
	w.bytes += int64(n)
	return n, err
}

func HTTPMiddleware(next http.Handler, emitter Emitter) http.Handler {
	if emitter == nil {
		emitter = NoopEmitter{}
	}

	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		start := time.Now()
		capture := &responseCapture{ResponseWriter: rw, statusCode: http.StatusOK}

		next.ServeHTTP(capture, req)

		e, err := event.NewEvent(event.EventTypeRequest, "watchers.http")
		if err != nil {
			return
		}

		e.Transport = event.TransportHTTP
		e.Direction = event.DirectionInbound
		e.TraceID = req.Header.Get("x-trace-id")
		e.RequestID = req.Header.Get("x-request-id")
		e.DurationMS = time.Since(start).Milliseconds()
		e.Status = strconv.Itoa(capture.statusCode)
		e.HTTP = &event.HTTPContext{
			Method:        req.Method,
			Path:          req.URL.Path,
			StatusCode:    capture.statusCode,
			RequestBytes:  requestSize(req.ContentLength),
			ResponseBytes: capture.bytes,
			ClientIP:      remoteIP(req.RemoteAddr),
			UserAgent:     req.UserAgent(),
		}

		if err := e.Validate(); err != nil {
			return
		}

		_ = emitter.Emit(req.Context(), e)
	})
}

func requestSize(contentLength int64) int64 {
	if contentLength < 0 {
		return 0
	}
	return contentLength
}

func remoteIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}
