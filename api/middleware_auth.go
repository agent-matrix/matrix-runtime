package api

import (
	"net/http"
	"strings"
)

// alwaysOpen is the set of endpoints reachable without any credentials:
// liveness/readiness/version probes and the capabilities advert.
func alwaysOpen(path string) bool {
	switch path {
	case "/v1/health", "/v1/ready", "/v1/version", "/v1/capabilities":
		return true
	}
	return false
}

// withAuth authenticates API requests. Two credential types are accepted and
// either is sufficient:
//
//   - a valid user session bearer token (multitenant console), or
//   - the operator API token (self-hosted single-tenant runtime), when one is
//     configured via MATRIX_RUNTIME_API_TOKEN.
//
// Public probes, the user-auth endpoints, runtime onboarding (which carries its
// own join/runtime-token auth), and static console assets are always allowed.
// In production modes a request with no valid credential is rejected
// (fail-closed); in local-dev it is permitted for convenience.
func (s *Server) withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Public probes + non-API (console assets).
		if alwaysOpen(path) || !strings.HasPrefix(path, "/v1/") {
			next.ServeHTTP(w, r)
			return
		}
		// User auth endpoints (login/signup/forgot/reset/verify) and runtime
		// onboarding (own-token auth) are always reachable.
		if strings.HasPrefix(path, "/v1/auth/") ||
			path == "/v1/cloud/runtimes/register" || path == "/v1/cloud/runtimes/heartbeat" {
			next.ServeHTTP(w, r)
			return
		}

		token := bearer(r)

		// 1) Valid user session → allow (multitenant console).
		if s.store != nil && token != "" {
			if _, err := s.store.UserBySession(token); err == nil {
				next.ServeHTTP(w, r)
				return
			}
		}
		// 2) Operator API token configured → require an exact match.
		if s.cfg.APIToken != "" {
			if token == s.cfg.APIToken {
				next.ServeHTTP(w, r)
				return
			}
			writeError(w, http.StatusUnauthorized, "missing or invalid bearer token")
			return
		}
		// 3) No operator token configured: fail-closed in production, allow in dev.
		if s.cfg.IsProduction() {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		next.ServeHTTP(w, r)
	})
}
