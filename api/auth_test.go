package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/agent-matrix/matrix-runtime/internal/config"
	"github.com/agent-matrix/matrix-runtime/internal/jobs"
	"github.com/agent-matrix/matrix-runtime/internal/store"
)

func authServer(t *testing.T) *Server {
	t.Helper()
	cfg := config.Defaults(config.ModeLocalDev)
	cfg.DataDir = t.TempDir()
	st, err := store.Open(filepath.Join(t.TempDir(), "auth.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return NewServer(cfg, jobs.NewManager(cfg), st)
}

func do(t *testing.T, srv *Server, method, path, token, body string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	var rdr *httptest.ResponseRecorder = httptest.NewRecorder()
	req := httptest.NewRequest(method, path, jsonBody(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	srv.Handler().ServeHTTP(rdr, req)
	var out map[string]any
	_ = json.Unmarshal(rdr.Body.Bytes(), &out)
	return rdr, out
}

func TestAuthFlow(t *testing.T) {
	srv := authServer(t)

	// signup
	rec, body := do(t, srv, http.MethodPost, "/v1/auth/signup", "", `{"name":"Neo","email":"neo@zion.io","password":"redpill1"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("signup status %d: %v", rec.Code, body)
	}
	token, _ := body["token"].(string)
	if token == "" {
		t.Fatal("expected a session token")
	}
	user, _ := body["user"].(map[string]any)
	if user["role"] != "Owner" || user["workspace"] == "" {
		t.Errorf("unexpected user %v", user)
	}

	// me with token
	rec, body = do(t, srv, http.MethodGet, "/v1/auth/me", token, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("me status %d", rec.Code)
	}

	// me without token -> 401
	rec, _ = do(t, srv, http.MethodGet, "/v1/auth/me", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("me-no-token status %d, want 401", rec.Code)
	}

	// duplicate signup -> 409
	rec, _ = do(t, srv, http.MethodPost, "/v1/auth/signup", "", `{"name":"X","email":"neo@zion.io","password":"redpill1"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("dup signup status %d, want 409", rec.Code)
	}

	// wrong login -> 401
	rec, _ = do(t, srv, http.MethodPost, "/v1/auth/login", "", `{"email":"neo@zion.io","password":"nope"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("bad login status %d, want 401", rec.Code)
	}

	// correct login -> 200
	rec, body = do(t, srv, http.MethodPost, "/v1/auth/login", "", `{"email":"neo@zion.io","password":"redpill1"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("login status %d", rec.Code)
	}

	// logout invalidates the session
	rec, _ = do(t, srv, http.MethodPost, "/v1/auth/logout", token, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("logout status %d", rec.Code)
	}
	rec, _ = do(t, srv, http.MethodGet, "/v1/auth/me", token, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("me after logout status %d, want 401", rec.Code)
	}
}

func TestAuthUnavailableWithoutStore(t *testing.T) {
	cfg := config.Defaults(config.ModeLocalDev)
	cfg.DataDir = t.TempDir()
	srv := NewServer(cfg, jobs.NewManager(cfg), nil)
	rec, _ := do(t, srv, http.MethodPost, "/v1/auth/login", "", `{"email":"a@b.io","password":"x"}`)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 without store, got %d", rec.Code)
	}
}
