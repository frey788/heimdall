package watchers

import (
	"testing"

	"github.com/frey788/heimdall/core/event"
)

func TestParseFullMethod(t *testing.T) {
	service, method := parseFullMethod("/users.UserService/GetProfile")
	if service != "users.UserService" {
		t.Fatalf("expected service users.UserService, got %s", service)
	}
	if method != "GetProfile" {
		t.Fatalf("expected method GetProfile, got %s", method)
	}
}

func TestStreamRPCType(t *testing.T) {
	tests := []struct {
		name         string
		clientStream bool
		serverStream bool
		want         event.RPCType
	}{
		{name: "unary", clientStream: false, serverStream: false, want: event.RPCTypeUnary},
		{name: "client-stream", clientStream: true, serverStream: false, want: event.RPCTypeClientStream},
		{name: "server-stream", clientStream: false, serverStream: true, want: event.RPCTypeServerStream},
		{name: "bidi-stream", clientStream: true, serverStream: true, want: event.RPCTypeBidiStream},
	}

	for _, tc := range tests {
		got := streamRPCType(tc.clientStream, tc.serverStream)
		if got != tc.want {
			t.Fatalf("%s: expected %s, got %s", tc.name, tc.want, got)
		}
	}
}
