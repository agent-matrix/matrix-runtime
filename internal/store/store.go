// Package store is the SQLite-backed persistence layer for multitenant users,
// workspaces (tenants) and sessions. It uses the pure-Go modernc.org/sqlite
// driver so the binary stays statically linkable with CGO disabled.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"

	"github.com/agent-matrix/matrix-runtime/internal/auth"
)

// Common errors.
var (
	ErrEmailTaken   = errors.New("an account with this email already exists")
	ErrInvalidLogin = errors.New("invalid email or password")
	ErrNotFound     = errors.New("not found")
)

// SessionTTL is how long an issued session token remains valid.
const SessionTTL = 30 * 24 * time.Hour

// Store wraps a SQL database (SQLite by default, or PostgreSQL/Neon for the
// hosted control plane). On Postgres all objects live in a dedicated schema so
// MatrixCloud never collides with other apps sharing the instance.
type Store struct {
	db     *sql.DB
	box    *auth.SecretBox
	pg     bool
	schema string
	tblRe  *regexp.Regexp
}

// pgTables are the tables MatrixCloud owns. On Postgres every reference to them
// is schema-qualified (qualify) so we never depend on search_path — which
// Neon's connection pooler does not preserve — and never collide with other
// apps that share the database (e.g. admin.matrixhub.io's own `users` table).
var pgTables = []string{
	"model_runtime_installations", "runtime_join_tokens", "provider_credentials",
	"email_verifications", "password_resets", "model_profiles", "usage_events",
	"audit_events", "workspaces", "sessions", "runtimes", "users",
}

func compileTableRe() *regexp.Regexp {
	return regexp.MustCompile(`\b(` + strings.Join(pgTables, "|") + `)\b`)
}

// qualify rewrites bare table names to "<schema>.<table>" for Postgres. Word
// boundaries keep it from touching columns/index names (e.g. workspace_id,
// idx_users_workspace). No-op for SQLite.
func (s *Store) qualify(q string) string {
	if !s.pg || s.tblRe == nil {
		return q
	}
	return s.tblRe.ReplaceAllString(q, s.schema+`.${1}`)
}

// rb rewrites "?" placeholders to "$N" for Postgres; no-op for SQLite.
func (s *Store) rb(q string) string {
	if !s.pg {
		return q
	}
	var b strings.Builder
	n := 0
	for i := 0; i < len(q); i++ {
		if q[i] == '?' {
			n++
			b.WriteByte('$')
			b.WriteString(strconv.Itoa(n))
		} else {
			b.WriteByte(q[i])
		}
	}
	return b.String()
}

func (s *Store) exec(q string, a ...any) (sql.Result, error) {
	return s.db.Exec(s.rb(s.qualify(q)), a...)
}
func (s *Store) query(q string, a ...any) (*sql.Rows, error) {
	return s.db.Query(s.rb(s.qualify(q)), a...)
}
func (s *Store) queryRow(q string, a ...any) *sql.Row { return s.db.QueryRow(s.rb(s.qualify(q)), a...) }

// User is a row in the users table joined with its workspace.
type User struct {
	ID            string    `json:"id"`
	WorkspaceID   string    `json:"workspace_id"`
	WorkspaceName string    `json:"workspace"`
	WorkspaceSlug string    `json:"workspace_slug"`
	Name          string    `json:"name"`
	Email         string    `json:"email"`
	Role          string    `json:"role"`
	CreatedAt     time.Time `json:"created_at"`
}

// Workspace is a tenant.
type Workspace struct {
	ID        string
	Name      string
	Slug      string
	CreatedAt time.Time
}

// Open opens (creating if needed) the SQLite database at path and applies the
// schema. A WAL journal and busy timeout keep concurrent access smooth.
func Open(path string) (*Store, error) {
	dsn := "file:" + path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1) // serialise writes; modernc sqlite is happiest single-writer
	box, err := auth.LoadSecretBox(filepath.Dir(path))
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("secret key: %w", err)
	}
	s := &Store{db: db, box: box}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// OpenPostgres opens a PostgreSQL/Neon database (e.g. for cloud.matrixhub.io).
// All MatrixCloud objects are created in and resolved from `schema` (default
// "matrixcloud") so the instance can be shared with other apps without
// collisions. secretDir holds the at-rest encryption key.
func OpenPostgres(dsn, schema, secretDir string) (*Store, error) {
	if schema == "" {
		schema = "matrixcloud"
	}
	// Pin the search_path so unqualified table names resolve into our schema.
	if !strings.Contains(dsn, "search_path=") {
		sep := "?"
		if strings.Contains(dsn, "?") {
			sep = "&"
		}
		dsn += sep + "search_path=" + schema
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	box, err := auth.LoadSecretBox(secretDir)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("secret key: %w", err)
	}
	s := &Store{db: db, box: box, pg: true, schema: schema, tblRe: compileTableRe()}
	if _, err := db.Exec(`CREATE SCHEMA IF NOT EXISTS ` + quoteIdent(schema)); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("create schema %q: %w", schema, err)
	}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

// Ping verifies the database is reachable (used by the readiness probe).
func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

func (s *Store) migrate() error {
	// Execute statements individually: pgx's extended protocol rejects
	// multi-statement Exec, and it keeps errors precise on both drivers.
	for _, stmt := range splitStatements(schema) {
		if _, err := s.db.Exec(s.qualify(stmt)); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	return nil
}

// splitStatements strips line comments and splits a DDL script on ";".
func splitStatements(sqlText string) []string {
	var sb strings.Builder
	for _, line := range strings.Split(sqlText, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		sb.WriteString(line)
		sb.WriteByte('\n')
	}
	var out []string
	for _, p := range strings.Split(sb.String(), ";") {
		if strings.TrimSpace(p) != "" {
			out = append(out, p)
		}
	}
	return out
}

// CountUsers returns the number of registered users.
func (s *Store) CountUsers() (int, error) {
	var n int
	err := s.queryRow(`SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

// Signup creates a workspace (tenant) and an owner user, returning the user.
func (s *Store) Signup(name, email, password, workspaceName string) (*User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	var exists int
	if err := s.queryRow(`SELECT COUNT(*) FROM users WHERE email = ?`, email).Scan(&exists); err != nil {
		return nil, err
	}
	if exists > 0 {
		return nil, ErrEmailTaken
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	if strings.TrimSpace(workspaceName) == "" {
		workspaceName = defaultWorkspaceName(name, email)
	}
	ws := Workspace{ID: auth.NewID("ws_"), Name: workspaceName, Slug: slugify(workspaceName), CreatedAt: now}
	// Ensure unique slug.
	ws.Slug = s.uniqueSlug(ws.Slug)

	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(s.rb(s.qualify(`INSERT INTO workspaces(id,name,slug,created_at) VALUES(?,?,?,?)`)),
		ws.ID, ws.Name, ws.Slug, now.Format(time.RFC3339)); err != nil {
		return nil, err
	}
	u := &User{ID: auth.NewID("usr_"), WorkspaceID: ws.ID, WorkspaceName: ws.Name, WorkspaceSlug: ws.Slug,
		Name: strings.TrimSpace(name), Email: email, Role: "Owner", CreatedAt: now}
	if _, err := tx.Exec(s.rb(s.qualify(`INSERT INTO users(id,workspace_id,name,email,password_hash,role,created_at) VALUES(?,?,?,?,?,?,?)`)),
		u.ID, u.WorkspaceID, u.Name, u.Email, hash, u.Role, now.Format(time.RFC3339)); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return u, nil
}

// Login verifies credentials and returns the user.
func (s *Store) Login(email, password string) (*User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	var hash string
	u := &User{}
	var created string
	row := s.queryRow(`
		SELECT u.id,u.workspace_id,u.name,u.email,u.role,u.password_hash,u.created_at,w.name,w.slug
		FROM users u JOIN workspaces w ON w.id = u.workspace_id WHERE u.email = ?`, email)
	if err := row.Scan(&u.ID, &u.WorkspaceID, &u.Name, &u.Email, &u.Role, &hash, &created, &u.WorkspaceName, &u.WorkspaceSlug); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrInvalidLogin
		}
		return nil, err
	}
	if !auth.VerifyPassword(password, hash) {
		return nil, ErrInvalidLogin
	}
	u.CreatedAt, _ = time.Parse(time.RFC3339, created)
	return u, nil
}

// CreateSession issues a session token for a user.
func (s *Store) CreateSession(userID string) (string, error) {
	token, err := auth.NewToken()
	if err != nil {
		return "", err
	}
	now := time.Now().UTC()
	_, err = s.exec(`INSERT INTO sessions(token,user_id,created_at,expires_at) VALUES(?,?,?,?)`,
		token, userID, now.Format(time.RFC3339), now.Add(SessionTTL).Format(time.RFC3339))
	return token, err
}

// UserBySession resolves a session token to its user, enforcing expiry.
func (s *Store) UserBySession(token string) (*User, error) {
	if token == "" {
		return nil, ErrNotFound
	}
	u := &User{}
	var created, expires string
	row := s.queryRow(`
		SELECT u.id,u.workspace_id,u.name,u.email,u.role,u.created_at,w.name,w.slug,s.expires_at
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		JOIN workspaces w ON w.id = u.workspace_id
		WHERE s.token = ?`, token)
	if err := row.Scan(&u.ID, &u.WorkspaceID, &u.Name, &u.Email, &u.Role, &created, &u.WorkspaceName, &u.WorkspaceSlug, &expires); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if exp, err := time.Parse(time.RFC3339, expires); err == nil && time.Now().After(exp) {
		_ = s.DeleteSession(token)
		return nil, ErrNotFound
	}
	u.CreatedAt, _ = time.Parse(time.RFC3339, created)
	return u, nil
}

// DeleteSession removes a single session (logout).
func (s *Store) DeleteSession(token string) error {
	_, err := s.exec(`DELETE FROM sessions WHERE token = ?`, token)
	return err
}

// DeleteUserSessions removes every session for a user (logout everywhere).
func (s *Store) DeleteUserSessions(userID string) error {
	_, err := s.exec(`DELETE FROM sessions WHERE user_id = ?`, userID)
	return err
}

func (s *Store) uniqueSlug(base string) string {
	slug := base
	for i := 1; ; i++ {
		var n int
		_ = s.queryRow(`SELECT COUNT(*) FROM workspaces WHERE slug = ?`, slug).Scan(&n)
		if n == 0 {
			return slug
		}
		slug = fmt.Sprintf("%s-%d", base, i)
	}
}

func defaultWorkspaceName(name, email string) string {
	if n := strings.TrimSpace(name); n != "" {
		return n + "'s workspace"
	}
	if at := strings.IndexByte(email, '@'); at > 0 {
		return email[:at] + "'s workspace"
	}
	return "Workspace"
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "workspace"
	}
	return out
}
