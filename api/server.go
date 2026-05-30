// Package api exposes the matrix-runtime HTTP API: health, capabilities, jobs
// and the sandbox compatibility aliases used by MatrixHub.
package api

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/agent-matrix/matrix-runtime/internal/config"
	"github.com/agent-matrix/matrix-runtime/internal/email"
	"github.com/agent-matrix/matrix-runtime/internal/jobs"
	"github.com/agent-matrix/matrix-runtime/internal/store"
	"github.com/agent-matrix/matrix-runtime/web"
)

// Server wires the job manager, user store, email sender and config to routes.
type Server struct {
	cfg     *config.Config
	manager *jobs.Manager
	store   *store.Store
	email   *email.Sender
	limiter *rateLimiter
}

// NewServer builds a Server. store may be nil if the user database could not be
// opened; auth endpoints then return 503.
func NewServer(cfg *config.Config, mgr *jobs.Manager, st *store.Store) *Server {
	s := &Server{cfg: cfg, manager: mgr, store: st, email: email.NewFromEnv()}
	if cfg.RateLimitRPM > 0 {
		s.limiter = newRateLimiter(cfg.RateLimitRPM)
	}
	return s
}

// Handler returns the configured HTTP handler with all routes registered.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /v1/health", s.handleHealth)
	mux.HandleFunc("GET /v1/ready", s.handleReady)
	mux.HandleFunc("GET /v1/version", s.handleVersion)
	mux.HandleFunc("GET /v1/capabilities", s.handleCapabilities)
	mux.HandleFunc("GET /v1/runtimes", s.handleListRuntimes)
	mux.HandleFunc("GET /v1/catalog", s.handleCatalog)
	mux.HandleFunc("GET /v1/policies", s.handlePolicies)

	// Multitenant auth (users + sessions, SQLite or Postgres/Neon).
	mux.HandleFunc("POST /v1/auth/signup", s.handleSignup)
	mux.HandleFunc("POST /v1/auth/login", s.handleLogin)
	mux.HandleFunc("GET /v1/auth/me", s.handleMe)
	mux.HandleFunc("POST /v1/auth/logout", s.handleLogout)
	// Password recovery + email verification (delivered via Resend).
	mux.HandleFunc("POST /v1/auth/forgot", s.handleForgotPassword)
	mux.HandleFunc("POST /v1/auth/reset", s.handleResetPassword)
	mux.HandleFunc("POST /v1/auth/verify", s.handleVerifyEmail)

	// Hosted control plane: runtimes, join tokens, BYO provider creds, usage.
	mux.HandleFunc("GET /v1/cloud/runtimes", s.handleCloudListRuntimes)
	mux.HandleFunc("POST /v1/cloud/runtimes/register", s.handleCloudRegisterRuntime)
	mux.HandleFunc("POST /v1/cloud/runtimes/heartbeat", s.handleCloudHeartbeat)
	mux.HandleFunc("GET /v1/cloud/join-tokens", s.handleCloudListJoinTokens)
	mux.HandleFunc("POST /v1/cloud/join-tokens", s.handleCloudMintJoinToken)
	mux.HandleFunc("GET /v1/cloud/providers", s.handleCloudListProviders)
	mux.HandleFunc("POST /v1/cloud/providers", s.handleCloudSetProvider)
	mux.HandleFunc("GET /v1/cloud/usage", s.handleCloudUsage)
	mux.HandleFunc("GET /v1/cloud/audit", s.handleCloudAudit)

	mux.HandleFunc("GET /v1/model-sources/huggingface/search", s.handleHFSearch)
	mux.HandleFunc("POST /v1/model-sources/resolve", s.handleResolveSource)
	mux.HandleFunc("GET /v1/model-profiles", s.handleListProfiles)
	mux.HandleFunc("POST /v1/model-profiles", s.handleImportProfile)
	mux.HandleFunc("POST /v1/model-profiles/{id}/attach", s.handleAttachProfile)
	mux.HandleFunc("GET /v1/model-installations", s.handleListInstallations)

	// MatrixShell — real local Python sandbox (install / status / exec).
	mux.HandleFunc("GET /v1/matrixshell/status", s.handleMatrixShellStatus)
	mux.HandleFunc("POST /v1/matrixshell/install", s.handleMatrixShellInstall)
	mux.HandleFunc("POST /v1/matrixshell/exec", s.handleMatrixShellExec)

	mux.HandleFunc("POST /v1/jobs", s.handleCreateJob)
	mux.HandleFunc("GET /v1/jobs", s.handleListJobs)
	mux.HandleFunc("GET /v1/jobs/{job_id}", s.handleGetJob)
	mux.HandleFunc("GET /v1/jobs/{job_id}/events", s.handleJobEvents)
	mux.HandleFunc("DELETE /v1/jobs/{job_id}", s.handleDeleteJob)

	mux.HandleFunc("POST /v1/sandbox/sessions", s.handleCreateSandbox)
	mux.HandleFunc("GET /v1/sandbox/sessions/{session_id}", s.handleGetSandbox)
	mux.HandleFunc("GET /v1/sandbox/sessions/{session_id}/events", s.handleSandboxEvents)
	mux.HandleFunc("GET /v1/sandbox/sessions/{session_id}/tools", s.handleSandboxTools)
	mux.HandleFunc("POST /v1/sandbox/sessions/{session_id}/tools/call", s.handleSandboxToolCall)
	mux.HandleFunc("DELETE /v1/sandbox/sessions/{session_id}", s.handleDeleteSandbox)

	// API docs (OpenAPI spec + a self-contained viewer), public.
	mux.HandleFunc("GET /openapi.yaml", s.handleOpenAPISpec)
	mux.HandleFunc("GET /docs", s.handleDocs)

	// System: storage usage.
	mux.HandleFunc("GET /v1/system/storage", s.handleStorage)

	// Enterprise console (static SPA) served from the embedded web assets.
	mux.Handle("/", s.consoleHandler())

	// Outermost: rate limiting (protects auth + writes); then auth.
	return s.withRateLimit(s.withAuth(mux))
}

// consoleHandler serves the embedded console, falling back to index.html for
// client-side routes (single-page app).
func (s *Server) consoleHandler() http.Handler {
	assets := web.Static()
	fileServer := http.FileServer(http.FS(assets))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Never let the SPA shadow the API namespace.
		if strings.HasPrefix(r.URL.Path, "/v1/") {
			http.NotFound(w, r)
			return
		}
		// Serve the asset when it exists; otherwise hand back the app shell.
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p == "" {
			p = "index.html"
		}
		if f, err := assets.Open(p); err == nil {
			_ = f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/"
		fileServer.ServeHTTP(w, r2)
	})
}

// Run starts the HTTP server and blocks until ctx is cancelled, then performs
// a graceful shutdown.
// Run binds addr and serves until ctx is cancelled. Kept for compatibility;
// prefer Serve with a pre-bound listener (e.g. from a port-fallback search).
func (s *Server) Run(ctx context.Context, addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return s.Serve(ctx, ln)
}

// Serve serves HTTP on the given listener until ctx is cancelled, then performs
// a graceful shutdown.
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	srv := &http.Server{
		Handler:           s.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ln) }()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}

// writeJSON serialises v as JSON with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes a structured JSON error.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg, "status": status})
}
