package server

import (
	"context"
	"strings"
	"testing"

	"google.golang.org/grpc/test/bufconn"
)

func TestStartRejectsNilGRPCListener(t *testing.T) {
	srv := New(stubServerConfig())
	err := srv.Start(context.Background())
	if err == nil {
		t.Fatal("expected error when gRPC listener is nil")
	}
	if !strings.Contains(err.Error(), "missing gRPC listener") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestStartRejectsNilHTTPListener(t *testing.T) {
	cfg := stubServerConfig()
	cfg.GRPCListener = bufconn.Listen(bufconnSize)
	defer func() { _ = cfg.GRPCListener.Close() }()

	srv := New(cfg)
	err := srv.Start(context.Background())
	if err == nil {
		t.Fatal("expected error when HTTP listener is nil")
	}
	if !strings.Contains(err.Error(), "missing HTTP listener") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGrpcDialEndpoint(t *testing.T) {
	tests := []struct {
		name string
		addr string
		want string
	}{
		{"ipv4 wildcard", "0.0.0.0:8080", "passthrough:///127.0.0.1:8080"},
		{"ipv6 wildcard", "[::]:8080", "passthrough:///[::1]:8080"},
		{"empty host", ":8080", "passthrough:///127.0.0.1:8080"},
		{"ipv4 loopback", "127.0.0.1:8080", "passthrough:///127.0.0.1:8080"},
		{"ipv6 loopback", "[::1]:8080", "passthrough:///[::1]:8080"},
		{"ipv4 specific", "192.168.1.1:9090", "passthrough:///192.168.1.1:9090"},
		{"opaque (bufconn)", "bufconn", "passthrough:///bufconn"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := grpcDialEndpoint(tt.addr)
			if got != tt.want {
				t.Errorf("grpcDialEndpoint(%q) = %q, want %q", tt.addr, got, tt.want)
			}
		})
	}
}
