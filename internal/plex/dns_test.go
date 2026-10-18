package plex

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPlexDNSFallbackIsStrictAndOnlyForDNSFailures(t *testing.T) {
	valid := "10-0-0-86.0123456789abcdef0123456789abcdef.plex.direct:32400"
	for _, tc := range []struct {
		name, address string
		failure       error
		want          int
	}{
		{"plex DNS", valid, &net.DNSError{IsNotFound: true}, 2},
		{"ordinary hostname", "example.com:443", &net.DNSError{}, 1},
		{"lookalike suffix", strings.TrimSuffix(valid, ":32400") + ".evil.test:32400", &net.DNSError{}, 1},
		{"invalid port", valid + ".evil.test", &net.DNSError{}, 1},
		{"invalid certificate id", "10-0-0-86.invalid.plex.direct:443", &net.DNSError{}, 1},
		{"invalid IP", "999-0-0-86.0123456789abcdef0123456789abcdef.plex.direct:443", &net.DNSError{}, 1},
		{"connection refused", valid, errors.New("connection refused"), 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			dial := plexDialContext(func(_ context.Context, _, address string) (net.Conn, error) {
				calls++
				if calls == 2 && address != "10.0.0.86:32400" {
					t.Fatalf("unexpected fallback %s", address)
				}
				return nil, tc.failure
			})
			_, _ = dial(context.Background(), "tcp", tc.address)
			if calls != tc.want {
				t.Fatalf("got %d calls, want %d", calls, tc.want)
			}
		})
	}
}

func TestPlexDNSFallbackPreservesTLSVerification(t *testing.T) {
	received := false
	server := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { received = true }))
	defer server.Close()
	_, port, _ := net.SplitHostPort(strings.TrimPrefix(server.URL, "https://"))
	host := "127-0-0-1.0123456789abcdef0123456789abcdef.plex.direct"
	client := NewClient("https://"+net.JoinHostPort(host, port), newID(), "client", "test")
	transport := client.http.Transport.(*http.Transport)
	transport.Proxy = nil
	dialer := &net.Dialer{}
	transport.DialContext = plexDialContext(func(ctx context.Context, network, address string) (net.Conn, error) {
		if strings.Contains(address, "plex.direct") {
			return nil, &net.DNSError{IsNotFound: true}
		}
		return dialer.DialContext(ctx, network, address)
	})
	_, err := client.Identity(context.Background())
	if ErrorCode(err) != "TLS" || received {
		t.Fatal("fallback must reject untrusted certificates before sending credentials")
	}
}

func TestPlexDNSFallbackIPv6(t *testing.T) {
	calls := 0
	dial := plexDialContext(func(_ context.Context, _, address string) (net.Conn, error) {
		calls++
		if calls == 2 && address != "[fd28:3585:4dd3:43eb:fe9d:5ff:fe07:27fa]:32400" {
			t.Fatalf("unexpected IPv6 target %s", address)
		}
		return nil, &net.DNSError{IsNotFound: true}
	})
	_, _ = dial(context.Background(), "tcp", "fd28-3585-4dd3-43eb-fe9d-05ff-fe07-27fa.0123456789abcdef0123456789abcdef.plex.direct:32400")
	if calls != 2 {
		t.Fatal("IPv6 fallback not attempted")
	}
}
