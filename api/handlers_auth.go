package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/agent-matrix/matrix-runtime/internal/store"
)

var emailRe = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

// userJSON is the public shape of a user returned to the console.
func userJSON(u *store.User) map[string]any {
	return map[string]any{
		"id":             u.ID,
		"name":           u.Name,
		"email":          u.Email,
		"role":           u.Role,
		"workspace":      u.WorkspaceName,
		"workspace_slug": u.WorkspaceSlug,
		"workspace_id":   u.WorkspaceID,
	}
}

func (s *Server) requireStore(w http.ResponseWriter) bool {
	if s.store == nil {
		writeError(w, http.StatusServiceUnavailable, "user store is not available")
		return false
	}
	return true
}

func (s *Server) handleSignup(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	var req struct{ Name, Email, Password, Workspace string }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	req.Email = strings.TrimSpace(req.Email)
	if !emailRe.MatchString(req.Email) {
		writeError(w, http.StatusBadRequest, "enter a valid email address")
		return
	}
	if len(req.Password) < 6 {
		writeError(w, http.StatusBadRequest, "password must be at least 6 characters")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	u, err := s.store.Signup(req.Name, req.Email, req.Password, req.Workspace)
	if err != nil {
		if errors.Is(err, store.ErrEmailTaken) {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "could not create account")
		return
	}
	token, err := s.store.CreateSession(u.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not start session")
		return
	}
	s.audit(r, u.WorkspaceID, u.ID, "user.signup", u.Email, "success", nil)
	// Best-effort welcome + verification email (never blocks signup success).
	if verifyTok, verr := s.store.CreateEmailVerification(u.ID, 24*time.Hour); verr == nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			_ = s.email.SendWelcome(ctx, u.Email, u.Name, verifyTok)
		}()
	}
	writeJSON(w, http.StatusCreated, map[string]any{"token": token, "user": userJSON(u)})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	var req struct{ Email, Password string }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	u, err := s.store.Login(req.Email, req.Password)
	if err != nil {
		s.audit(r, "", strings.ToLower(strings.TrimSpace(req.Email)), "user.login", req.Email, "failure", nil)
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}
	token, err := s.store.CreateSession(u.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not start session")
		return
	}
	s.audit(r, u.WorkspaceID, u.ID, "user.login", u.Email, "success", nil)
	writeJSON(w, http.StatusOK, map[string]any{"token": token, "user": userJSON(u)})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	u, err := s.store.UserBySession(bearer(r))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": userJSON(u)})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	token := bearer(r)
	all := r.URL.Query().Get("all") == "true"
	if all {
		if u, err := s.store.UserBySession(token); err == nil {
			_ = s.store.DeleteUserSessions(u.ID)
		}
	}
	_ = s.store.DeleteSession(token)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func bearer(r *http.Request) string {
	return strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
}

// handleForgotPassword always responds 200 (to avoid leaking which emails are
// registered). When the email matches a user, a reset link is emailed via
// Resend with a single-use, 1-hour token.
func (s *Server) handleForgotPassword(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	var req struct{ Email string }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	const ok = "If an account exists for that email, a reset link is on its way."
	u, err := s.store.UserByEmail(req.Email)
	if err != nil {
		// Don't reveal absence; respond identically.
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": ok})
		return
	}
	token, err := s.store.CreatePasswordReset(u.ID, time.Hour)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not start password reset")
		return
	}
	if err := s.email.SendPasswordReset(r.Context(), u.Email, token); err != nil {
		writeError(w, http.StatusBadGateway, "could not send reset email")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": ok})
}

// handleResetPassword consumes a reset token and sets a new password.
func (s *Server) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	var req struct{ Token, Password string }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := s.store.ResetPassword(strings.TrimSpace(req.Token), req.Password); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "Your password has been updated. Please sign in."})
}

// handleVerifyEmail consumes an email-verification token.
func (s *Server) handleVerifyEmail(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	var req struct{ Token string }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if _, err := s.store.VerifyEmail(strings.TrimSpace(req.Token)); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "Email verified."})
}
