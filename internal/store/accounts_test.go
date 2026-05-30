package store

import (
	"testing"
	"time"
)

func TestPasswordResetFlow(t *testing.T) {
	s := newTestStore(t)
	u, err := s.Signup("Trinity", "trinity@zion.io", "original1", "")
	if err != nil {
		t.Fatal(err)
	}
	// A live session exists.
	tok, err := s.CreateSession(u.ID)
	if err != nil {
		t.Fatal(err)
	}

	// Lookups are case/space-insensitive and don't leak absence.
	if got, err := s.UserByEmail("  Trinity@Zion.io "); err != nil || got.ID != u.ID {
		t.Fatalf("UserByEmail = %v err=%v", got, err)
	}
	if _, err := s.UserByEmail("nobody@nowhere.io"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	secret, err := s.CreatePasswordReset(u.ID, time.Hour)
	if err != nil || secret == "" {
		t.Fatalf("create reset: %v", err)
	}

	// Too-short password is rejected.
	if err := s.ResetPassword(secret, "short"); err == nil {
		t.Error("expected short-password rejection")
	}
	// Valid reset succeeds, logs the user out everywhere, and lets them log in.
	if err := s.ResetPassword(secret, "brandnew9"); err != nil {
		t.Fatalf("reset: %v", err)
	}
	if _, err := s.UserBySession(tok); err != ErrNotFound {
		t.Error("old session should be invalidated after password reset")
	}
	if _, err := s.Login("trinity@zion.io", "brandnew9"); err != nil {
		t.Fatalf("login with new password: %v", err)
	}
	if _, err := s.Login("trinity@zion.io", "original1"); err == nil {
		t.Error("old password should no longer work")
	}
	// Single-use: the same token can't be replayed.
	if err := s.ResetPassword(secret, "another99"); err == nil {
		t.Error("reset token should be single-use")
	}
}

func TestEmailVerificationFlow(t *testing.T) {
	s := newTestStore(t)
	u, err := s.Signup("Morpheus", "morpheus@zion.io", "redpill1", "")
	if err != nil {
		t.Fatal(err)
	}
	secret, err := s.CreateEmailVerification(u.ID, time.Hour)
	if err != nil || secret == "" {
		t.Fatalf("create verification: %v", err)
	}
	got, err := s.VerifyEmail(secret)
	if err != nil || got != u.ID {
		t.Fatalf("verify: got=%s err=%v", got, err)
	}
	// Idempotent: verifying again returns the same user, no error.
	if got, err := s.VerifyEmail(secret); err != nil || got != u.ID {
		t.Fatalf("re-verify should be idempotent: got=%s err=%v", got, err)
	}
	if _, err := s.VerifyEmail("mxev_bogus"); err == nil {
		t.Error("bogus token should fail")
	}
}
