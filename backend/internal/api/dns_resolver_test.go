package api

import (
	"context"
	"errors"
	"net"
	"testing"
)

func notFoundErr(host string) error {
	return &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
}

func TestChainResolvers_FirstSucceeds(t *testing.T) {
	calls := 0
	lookups := []cnameLookupFunc{
		func(_ context.Context, _ string) (string, error) {
			calls++
			return "target.example.", nil
		},
		func(_ context.Context, _ string) (string, error) {
			t.Fatal("second resolver should not be called")
			return "", nil
		},
	}
	cname, err := chainResolvers(lookups)(context.Background(), "host.example")
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if cname != "target.example." {
		t.Fatalf("cname = %q", cname)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestChainResolvers_NotFoundStopsImmediately(t *testing.T) {
	second := false
	lookups := []cnameLookupFunc{
		func(_ context.Context, host string) (string, error) {
			return "", notFoundErr(host)
		},
		func(_ context.Context, _ string) (string, error) {
			second = true
			return "target.example.", nil
		},
	}
	_, err := chainResolvers(lookups)(context.Background(), "host.example")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if second {
		t.Fatal("must not fall back to next resolver on a definitive NXDOMAIN")
	}
}

func TestChainResolvers_NetworkErrorFallsBack(t *testing.T) {
	lookups := []cnameLookupFunc{
		func(_ context.Context, _ string) (string, error) {
			return "", errors.New("dial timeout")
		},
		func(_ context.Context, _ string) (string, error) {
			return "target.example.", nil
		},
	}
	cname, err := chainResolvers(lookups)(context.Background(), "host.example")
	if err != nil {
		t.Fatalf("err = %v, want nil (fallback should succeed)", err)
	}
	if cname != "target.example." {
		t.Fatalf("cname = %q", cname)
	}
}

func TestChainResolvers_AllFail(t *testing.T) {
	lookups := []cnameLookupFunc{
		func(_ context.Context, _ string) (string, error) { return "", errors.New("timeout 1") },
		func(_ context.Context, _ string) (string, error) { return "", errors.New("timeout 2") },
	}
	_, err := chainResolvers(lookups)(context.Background(), "host.example")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "timeout 2" {
		t.Fatalf("err = %v, want the last resolver's error", err)
	}
}
