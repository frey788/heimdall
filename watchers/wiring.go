package watchers

import (
	"net/http"

	"google.golang.org/grpc"
)

type Option func(*Wiring)

type Wiring struct {
	emitter               Emitter
	sensitiveMetadataKeys []string
}

func NewWiring(options ...Option) *Wiring {
	w := &Wiring{emitter: NoopEmitter{}}
	for _, opt := range options {
		opt(w)
	}
	if w.emitter == nil {
		w.emitter = NoopEmitter{}
	}
	return w
}

func WithEmitter(emitter Emitter) Option {
	return func(w *Wiring) {
		w.emitter = emitter
	}
}

func WithSensitiveMetadataKeys(keys ...string) Option {
	return func(w *Wiring) {
		w.sensitiveMetadataKeys = append([]string{}, keys...)
	}
}

func (w *Wiring) HTTPMiddleware(next http.Handler) http.Handler {
	return HTTPMiddleware(next, w.emitter)
}

func (w *Wiring) GRPCUnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return GRPCUnaryServerInterceptor(w.emitter, w.sensitiveMetadataKeys...)
}

func (w *Wiring) GRPCStreamServerInterceptor() grpc.StreamServerInterceptor {
	return GRPCStreamServerInterceptor(w.emitter, w.sensitiveMetadataKeys...)
}

func (w *Wiring) GRPCUnaryClientInterceptor() grpc.UnaryClientInterceptor {
	return GRPCUnaryClientInterceptor(w.emitter, w.sensitiveMetadataKeys...)
}

func (w *Wiring) GRPCStreamClientInterceptor() grpc.StreamClientInterceptor {
	return GRPCStreamClientInterceptor(w.emitter, w.sensitiveMetadataKeys...)
}
