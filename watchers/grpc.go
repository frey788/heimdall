package watchers

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/frey788/heimdall/core/event"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

func GRPCUnaryServerInterceptor(emitter Emitter, extraSensitiveMetadataKeys ...string) grpc.UnaryServerInterceptor {
	if emitter == nil {
		emitter = NoopEmitter{}
	}

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		emitGRPCEvent(emitter, ctx, grpcEventInput{
			Source:                     "watchers.grpc.server",
			Direction:                  event.DirectionInbound,
			FullMethod:                 info.FullMethod,
			RPCType:                    event.RPCTypeUnary,
			StatusCode:                 status.Code(err).String(),
			DurationMS:                 time.Since(start).Milliseconds(),
			RequestCount:               1,
			ResponseCount:              responseCount(err == nil),
			ExtraSensitiveMetadataKeys: extraSensitiveMetadataKeys,
		})
		return resp, err
	}
}

func GRPCStreamServerInterceptor(emitter Emitter, extraSensitiveMetadataKeys ...string) grpc.StreamServerInterceptor {
	if emitter == nil {
		emitter = NoopEmitter{}
	}

	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		start := time.Now()
		wrapped := &countingServerStream{ServerStream: stream}
		err := handler(srv, wrapped)

		rpcType := streamRPCType(info.IsClientStream, info.IsServerStream)
		emitGRPCEvent(emitter, stream.Context(), grpcEventInput{
			Source:                     "watchers.grpc.server",
			Direction:                  event.DirectionInbound,
			FullMethod:                 info.FullMethod,
			RPCType:                    rpcType,
			StatusCode:                 status.Code(err).String(),
			DurationMS:                 time.Since(start).Milliseconds(),
			RequestCount:               wrapped.recvCount,
			ResponseCount:              wrapped.sendCount,
			ExtraSensitiveMetadataKeys: extraSensitiveMetadataKeys,
		})

		return err
	}
}

func GRPCUnaryClientInterceptor(emitter Emitter, extraSensitiveMetadataKeys ...string) grpc.UnaryClientInterceptor {
	if emitter == nil {
		emitter = NoopEmitter{}
	}

	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		start := time.Now()
		err := invoker(ctx, method, req, reply, cc, opts...)

		emitGRPCEvent(emitter, ctx, grpcEventInput{
			Source:                     "watchers.grpc.client",
			Direction:                  event.DirectionOutbound,
			FullMethod:                 method,
			RPCType:                    event.RPCTypeUnary,
			StatusCode:                 status.Code(err).String(),
			DurationMS:                 time.Since(start).Milliseconds(),
			RequestCount:               1,
			ResponseCount:              responseCount(err == nil),
			ExtraSensitiveMetadataKeys: extraSensitiveMetadataKeys,
		})

		return err
	}
}

func GRPCStreamClientInterceptor(emitter Emitter, extraSensitiveMetadataKeys ...string) grpc.StreamClientInterceptor {
	if emitter == nil {
		emitter = NoopEmitter{}
	}

	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		start := time.Now()
		clientStream, err := streamer(ctx, desc, cc, method, opts...)
		if err != nil {
			emitGRPCEvent(emitter, ctx, grpcEventInput{
				Source:                     "watchers.grpc.client",
				Direction:                  event.DirectionOutbound,
				FullMethod:                 method,
				RPCType:                    streamRPCType(desc.ClientStreams, desc.ServerStreams),
				StatusCode:                 status.Code(err).String(),
				DurationMS:                 time.Since(start).Milliseconds(),
				ExtraSensitiveMetadataKeys: extraSensitiveMetadataKeys,
			})
			return nil, err
		}

		wrapped := &countingClientStream{
			ClientStream:               clientStream,
			emitter:                    emitter,
			ctx:                        ctx,
			fullMethod:                 method,
			rpcType:                    streamRPCType(desc.ClientStreams, desc.ServerStreams),
			start:                      start,
			extraSensitiveMetadataKeys: extraSensitiveMetadataKeys,
		}
		return wrapped, nil
	}
}

type countingServerStream struct {
	grpc.ServerStream
	recvCount uint64
	sendCount uint64
}

func (s *countingServerStream) RecvMsg(msg any) error {
	err := s.ServerStream.RecvMsg(msg)
	if err == nil {
		s.recvCount++
	}
	return err
}

func (s *countingServerStream) SendMsg(msg any) error {
	err := s.ServerStream.SendMsg(msg)
	if err == nil {
		s.sendCount++
	}
	return err
}

type countingClientStream struct {
	grpc.ClientStream
	emitter                    Emitter
	ctx                        context.Context
	fullMethod                 string
	rpcType                    event.RPCType
	start                      time.Time
	recvCount                  uint64
	sendCount                  uint64
	extraSensitiveMetadataKeys []string
	once                       sync.Once
}

func (s *countingClientStream) SendMsg(msg any) error {
	err := s.ClientStream.SendMsg(msg)
	if err == nil {
		s.sendCount++
		return nil
	}
	s.emit(status.Code(err).String())
	return err
}

func (s *countingClientStream) RecvMsg(msg any) error {
	err := s.ClientStream.RecvMsg(msg)
	if err == nil {
		s.recvCount++
		return nil
	}

	if errors.Is(err, io.EOF) {
		s.emit(codes.OK.String())
		return err
	}

	s.emit(status.Code(err).String())
	return err
}

func (s *countingClientStream) CloseSend() error {
	err := s.ClientStream.CloseSend()
	if err != nil {
		s.emit(status.Code(err).String())
	}
	return err
}

func (s *countingClientStream) emit(statusCode string) {
	s.once.Do(func() {
		emitGRPCEvent(s.emitter, s.ctx, grpcEventInput{
			Source:                     "watchers.grpc.client",
			Direction:                  event.DirectionOutbound,
			FullMethod:                 s.fullMethod,
			RPCType:                    s.rpcType,
			StatusCode:                 statusCode,
			DurationMS:                 time.Since(s.start).Milliseconds(),
			RequestCount:               s.sendCount,
			ResponseCount:              s.recvCount,
			ExtraSensitiveMetadataKeys: s.extraSensitiveMetadataKeys,
		})
	})
}

type grpcEventInput struct {
	Source                     string
	Direction                  event.Direction
	FullMethod                 string
	RPCType                    event.RPCType
	StatusCode                 string
	DurationMS                 int64
	RequestCount               uint64
	ResponseCount              uint64
	ExtraSensitiveMetadataKeys []string
}

func emitGRPCEvent(emitter Emitter, ctx context.Context, input grpcEventInput) {
	e, err := event.NewEvent(event.EventTypeRequest, input.Source)
	if err != nil {
		return
	}

	service, method := parseFullMethod(input.FullMethod)
	metadataValues := event.RedactMetadata(metadataMap(ctx), input.ExtraSensitiveMetadataKeys...)
	deadline := deadlineFromContext(ctx)

	e.Transport = event.TransportGRPC
	e.Direction = input.Direction
	e.DurationMS = input.DurationMS
	e.Status = input.StatusCode
	e.TraceID = metadataFirstValue(metadataValues, "x-trace-id")
	e.RequestID = metadataFirstValue(metadataValues, "x-request-id")
	e.GRPC = &event.GRPCContext{
		Service:       service,
		Method:        method,
		FullMethod:    input.FullMethod,
		RPCType:       input.RPCType,
		StatusCode:    input.StatusCode,
		Peer:          peerAddress(ctx),
		Deadline:      deadline,
		Metadata:      metadataValues,
		RequestCount:  input.RequestCount,
		ResponseCount: input.ResponseCount,
	}

	if err := e.Validate(); err != nil {
		return
	}

	_ = emitter.Emit(ctx, e)
}

func parseFullMethod(fullMethod string) (service string, method string) {
	trimmed := strings.TrimPrefix(fullMethod, "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return trimmed, "unknown"
}

func streamRPCType(clientStream, serverStream bool) event.RPCType {
	switch {
	case clientStream && serverStream:
		return event.RPCTypeBidiStream
	case clientStream:
		return event.RPCTypeClientStream
	case serverStream:
		return event.RPCTypeServerStream
	default:
		return event.RPCTypeUnary
	}
}

func responseCount(ok bool) uint64 {
	if ok {
		return 1
	}
	return 0
}

func metadataMap(ctx context.Context) map[string][]string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		md, ok = metadata.FromOutgoingContext(ctx)
		if !ok {
			return map[string][]string{}
		}
	}

	result := make(map[string][]string, len(md))
	for key, values := range md {
		copied := make([]string, len(values))
		copy(copied, values)
		result[key] = copied
	}

	return result
}

func metadataFirstValue(metadataValues map[string][]string, key string) string {
	values, ok := metadataValues[strings.ToLower(key)]
	if !ok || len(values) == 0 {
		values, ok = metadataValues[key]
		if !ok || len(values) == 0 {
			return ""
		}
	}
	return values[0]
}

func peerAddress(ctx context.Context) string {
	p, ok := peer.FromContext(ctx)
	if !ok || p.Addr == nil {
		return ""
	}
	return p.Addr.String()
}

func deadlineFromContext(ctx context.Context) *time.Time {
	deadline, ok := ctx.Deadline()
	if !ok {
		return nil
	}
	deadline = deadline.UTC()
	return &deadline
}
