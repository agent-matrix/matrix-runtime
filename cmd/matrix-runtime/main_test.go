package main

import (
	"net"
	"testing"
)

func TestListenWithFallback_SkipsBusyPort(t *testing.T) {
	// Occupy a port, then ask listenWithFallback to start there.
	occ, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = occ.Close() }()
	busy := occ.Addr().(*net.TCPAddr).Port

	ln, got, err := listenWithFallback(busy, 20)
	if err != nil {
		t.Fatalf("listenWithFallback: %v", err)
	}
	defer func() { _ = ln.Close() }()

	if got == busy {
		t.Fatalf("expected fallback to a different port, got the busy one %d", busy)
	}
	if got <= busy {
		t.Errorf("expected a higher port than %d, got %d", busy, got)
	}
	if ln.Addr().(*net.TCPAddr).Port != got {
		t.Errorf("listener port %d != reported %d", ln.Addr().(*net.TCPAddr).Port, got)
	}
}

func TestListenWithFallback_FreePort(t *testing.T) {
	// Find a free port, release it, then bind it via the helper.
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	free := probe.Addr().(*net.TCPAddr).Port
	_ = probe.Close()

	ln, got, err := listenWithFallback(free, 20)
	if err != nil {
		t.Fatalf("listenWithFallback: %v", err)
	}
	defer func() { _ = ln.Close() }()
	if got != free {
		t.Errorf("expected to bind the free port %d, got %d", free, got)
	}
}
