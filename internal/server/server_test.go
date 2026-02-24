package server

import "testing"

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
