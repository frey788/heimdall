package watchers

import (
	"context"

	"github.com/frey788/heimdall/core/event"
)

type Emitter interface {
	Emit(ctx context.Context, e event.Event) error
}

type EmitterFunc func(ctx context.Context, e event.Event) error

func (f EmitterFunc) Emit(ctx context.Context, e event.Event) error {
	return f(ctx, e)
}

type NoopEmitter struct{}

func (NoopEmitter) Emit(context.Context, event.Event) error {
	return nil
}
