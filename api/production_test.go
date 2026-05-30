package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agent-matrix/matrix-runtime/internal/config"
	"github.com/agent-matrix/matrix-runtime/internal/jobs"
)

func get(t *testing.T, srv *Server, path, token string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec, out
}

func TestVersionAndReady(t *testing.T) {
	srv := testServer(t)
	rec, body := get(t, srv, "/v1/version", "")
	if rec.Code != http.StatusOK || body["name"] != "matrix-runtime" {
		t.Fatalf("version = %d %v", rec.Code, body)
	}
	rec, body = get(t, srv, "/v1/ready", "")
	// No store in testServer → not ready, with a store_unavailable warning.
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("ready status = %d, want 503 (no store)", rec.Code)
	}
	if _, ok := body["checks"].(map[string]any); !ok {
		t.Fatalf("ready missing checks: %v", body)
	}
	if body["ready"] != false {
		t.Errorf("ready = %v, want false", body["ready"])
	}
}

func TestReadyHealthyWithStore(t *testing.T) {
	srv := authServer(t) // has a store + temp data dir, local-dev
	rec, body := get(t, srv, "/v1/ready", "")
	if rec.Code != http.StatusOK || body["ready"] != true {
		t.Fatalf("ready = %d %v", rec.Code, body)
	}
	// local-dev should surface a local_dev_mode warning.
	ws, _ := body["warnings"].([]any)
	found := false
	for _, w := range ws {
		if m, ok := w.(map[string]any); ok && m["code"] == "local_dev_mode" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected local_dev_mode warning, got %v", ws)
	}
}

func TestProductionFailsClosedWithoutCreds(t *testing.T) {
	cfg := config.Defaults(config.ModeCustomerAgent) // production mode
	cfg.DataDir = t.TempDir()
	srv := NewServer(cfg, jobs.NewManager(cfg), nil)

	// Probes stay public.
	if rec, _ := get(t, srv, "/v1/health", ""); rec.Code != http.StatusOK {
		t.Fatalf("health should be public, got %d", rec.Code)
	}
	if rec, _ := get(t, srv, "/v1/version", ""); rec.Code != http.StatusOK {
		t.Fatalf("version should be public, got %d", rec.Code)
	}
	// A protected endpoint with no creds is rejected (fail-closed).
	if rec, _ := get(t, srv, "/v1/jobs", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("protected endpoint = %d, want 401", rec.Code)
	}
}

func TestOperatorAPITokenGate(t *testing.T) {
	cfg := config.Defaults(config.ModeCustomerAgent)
	cfg.DataDir = t.TempDir()
	cfg.APIToken = "s3cret-operator-token"
	srv := NewServer(cfg, jobs.NewManager(cfg), nil)

	if rec, _ := get(t, srv, "/v1/jobs", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("no token = %d, want 401", rec.Code)
	}
	if rec, _ := get(t, srv, "/v1/jobs", "wrong"); rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong token = %d, want 401", rec.Code)
	}
	if rec, _ := get(t, srv, "/v1/jobs", "s3cret-operator-token"); rec.Code == http.StatusUnauthorized {
		t.Fatalf("correct operator token should pass the auth gate, got 401")
	}
}

func TestMatrixShellGate(t *testing.T) {
	// Production mode → MatrixShell disabled by default → 403.
	cfg := config.Defaults(config.ModeCustomerAgent)
	cfg.DataDir = t.TempDir()
	cfg.APIToken = "tok"
	srv := NewServer(cfg, jobs.NewManager(cfg), nil)
	if rec, _ := get(t, srv, "/v1/matrixshell/status", "tok"); rec.Code != http.StatusForbidden {
		t.Fatalf("matrixshell (prod, disabled) = %d, want 403", rec.Code)
	}

	// Explicitly enabled → reachable (200).
	cfg2 := config.Defaults(config.ModeCustomerAgent)
	cfg2.DataDir = t.TempDir()
	cfg2.APIToken = "tok"
	cfg2.MatrixShellEnabled = true
	srv2 := NewServer(cfg2, jobs.NewManager(cfg2), nil)
	if rec, _ := get(t, srv2, "/v1/matrixshell/status", "tok"); rec.Code == http.StatusForbidden {
		t.Fatalf("matrixshell (enabled) should not be 403")
	}
}

func TestStorageEndpoint(t *testing.T) {
	srv := authServer(t) // has store + temp data dir, local-dev (no auth required)
	rec, body := get(t, srv, "/v1/system/storage", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("storage = %d", rec.Code)
	}
	if _, ok := body["areas"].(map[string]any); !ok {
		t.Fatalf("missing areas: %v", body)
	}
	if _, ok := body["total_bytes"]; !ok {
		t.Errorf("missing total_bytes")
	}
}

func TestRateLimit(t *testing.T) {
	cfg := config.Defaults(config.ModeLocalDev)
	cfg.DataDir = t.TempDir()
	cfg.RateLimitRPM = 3
	srv := NewServer(cfg, jobs.NewManager(cfg), nil)

	// GET probes are never limited.
	for i := 0; i < 10; i++ {
		if rec, _ := get(t, srv, "/v1/health", ""); rec.Code == http.StatusTooManyRequests {
			t.Fatalf("GET health should never be rate limited")
		}
	}
	// POSTs are limited after RateLimitRPM within the window.
	limited := false
	for i := 0; i < 6; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", jsonBody(`{"email":"a@b.io","password":"x"}`))
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)
		if rec.Code == http.StatusTooManyRequests {
			limited = true
			break
		}
	}
	if !limited {
		t.Errorf("expected a 429 after exceeding the rate limit")
	}
}
