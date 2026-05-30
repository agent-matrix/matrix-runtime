package store

import (
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestSignupLoginSession(t *testing.T) {
	s := newTestStore(t)

	u, err := s.Signup("Maya Chen", "Maya@Acme.io", "secret123", "")
	if err != nil {
		t.Fatal(err)
	}
	if u.Role != "Owner" || u.Email != "maya@acme.io" {
		t.Errorf("unexpected user %+v", u)
	}
	if u.WorkspaceSlug == "" || u.WorkspaceID == "" {
		t.Error("expected a workspace to be created")
	}

	// duplicate email
	if _, err := s.Signup("X", "maya@acme.io", "secret123", ""); err != ErrEmailTaken {
		t.Errorf("expected ErrEmailTaken, got %v", err)
	}

	// wrong password
	if _, err := s.Login("maya@acme.io", "nope"); err != ErrInvalidLogin {
		t.Errorf("expected ErrInvalidLogin, got %v", err)
	}

	// correct login (case-insensitive email)
	lu, err := s.Login("MAYA@acme.io", "secret123")
	if err != nil {
		t.Fatal(err)
	}
	if lu.ID != u.ID {
		t.Errorf("login returned different user")
	}

	// session lifecycle
	tok, err := s.CreateSession(u.ID)
	if err != nil {
		t.Fatal(err)
	}
	su, err := s.UserBySession(tok)
	if err != nil || su.ID != u.ID {
		t.Fatalf("UserBySession failed: %v", err)
	}
	if err := s.DeleteSession(tok); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UserBySession(tok); err != ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestUniqueWorkspaceSlug(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.Signup("Acme", "a@x.io", "secret123", "Acme"); err != nil {
		t.Fatal(err)
	}
	u2, err := s.Signup("Acme Two", "b@x.io", "secret123", "Acme")
	if err != nil {
		t.Fatal(err)
	}
	if u2.WorkspaceSlug == "acme" {
		t.Error("expected a de-duplicated workspace slug")
	}
}
