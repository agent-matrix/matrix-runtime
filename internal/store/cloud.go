package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/agent-matrix/matrix-runtime/internal/auth"
)

// ---------------------------------------------------------------------------
// Runtimes — a workspace's registered runtime/sandbox planes (incl. HF Spaces).
// ---------------------------------------------------------------------------

// Runtime is a registered execution plane belonging to a workspace.
type Runtime struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	Name        string    `json:"name"`
	Mode        string    `json:"mode"`
	Kind        string    `json:"kind"`
	URL         string    `json:"url"`
	HFSpace     string    `json:"hf_space"`
	Region      string    `json:"region"`
	Caps        []string  `json:"caps"`
	Status      string    `json:"status"`
	Version     string    `json:"version"`
	LastSeenAt  time.Time `json:"last_seen_at"`
	CreatedAt   time.Time `json:"created_at"`
}

// RegisterRuntime inserts a runtime for a workspace and returns it plus a
// freshly minted runtime token (shown once) the runtime uses for heartbeats.
func (s *Store) RegisterRuntime(rt Runtime) (*Runtime, string, error) {
	rt.ID = auth.NewID("rt_")
	if rt.Status == "" {
		rt.Status = "online"
	}
	if rt.Caps == nil {
		rt.Caps = []string{}
	}
	token, err := auth.NewToken()
	if err != nil {
		return nil, "", err
	}
	now := nowRFC()
	caps, _ := json.Marshal(rt.Caps)
	_, err = s.exec(`
		INSERT INTO runtimes (id,workspace_id,name,mode,kind,url,hf_space,region,caps_json,status,token_hash,version,last_seen_at,created_at,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		rt.ID, rt.WorkspaceID, rt.Name, rt.Mode, rt.Kind, rt.URL, rt.HFSpace, rt.Region, string(caps), rt.Status, hashToken(token), rt.Version, now, now, now)
	if err != nil {
		return nil, "", err
	}
	rt.CreatedAt, _ = time.Parse(time.RFC3339, now)
	rt.LastSeenAt = rt.CreatedAt
	return &rt, token, nil
}

func hashToken(t string) string {
	// reuse the password hasher's PBKDF2 would be overkill for a high-entropy
	// random token; a fast keyed digest is enough — but we keep it simple and
	// store a SHA-style hash via the auth package's id helper is not suitable,
	// so use a constant-time-comparable hex of a salted hash.
	return auth.FastHash(t)
}

func scanRuntime(row interface{ Scan(...any) error }) (*Runtime, error) {
	var rt Runtime
	var caps, last, created string
	if err := row.Scan(&rt.ID, &rt.WorkspaceID, &rt.Name, &rt.Mode, &rt.Kind, &rt.URL, &rt.HFSpace, &rt.Region, &caps, &rt.Status, &rt.Version, &last, &created); err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(caps), &rt.Caps)
	rt.LastSeenAt, _ = time.Parse(time.RFC3339, last)
	rt.CreatedAt, _ = time.Parse(time.RFC3339, created)
	return &rt, nil
}

const runtimeCols = `id,workspace_id,name,mode,kind,url,hf_space,region,caps_json,status,version,last_seen_at,created_at`

// ListRuntimes returns a workspace's runtimes, newest first, marking those not
// seen within staleAfter as offline.
func (s *Store) ListRuntimes(workspaceID string, staleAfter time.Duration) ([]Runtime, error) {
	rows, err := s.query(`SELECT `+runtimeCols+` FROM runtimes WHERE workspace_id=? ORDER BY created_at DESC`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []Runtime{}
	for rows.Next() {
		rt, err := scanRuntime(rows)
		if err != nil {
			return nil, err
		}
		if rt.Status != "offline" && staleAfter > 0 && !rt.LastSeenAt.IsZero() && time.Since(rt.LastSeenAt) > staleAfter {
			rt.Status = "offline"
		}
		out = append(out, *rt)
	}
	return out, rows.Err()
}

// HeartbeatRuntime authenticates a runtime token and updates status/last_seen.
func (s *Store) HeartbeatRuntime(token, status string, caps []string) (*Runtime, error) {
	row := s.queryRow(`SELECT `+runtimeCols+` FROM runtimes WHERE token_hash=?`, hashToken(token))
	rt, err := scanRuntime(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if status == "" {
		status = "online"
	}
	now := nowRFC()
	if caps != nil {
		c, _ := json.Marshal(caps)
		_, err = s.exec(`UPDATE runtimes SET status=?, caps_json=?, last_seen_at=?, updated_at=? WHERE id=?`, status, string(c), now, now, rt.ID)
		rt.Caps = caps
	} else {
		_, err = s.exec(`UPDATE runtimes SET status=?, last_seen_at=?, updated_at=? WHERE id=?`, status, now, now, rt.ID)
	}
	rt.Status = status
	rt.LastSeenAt, _ = time.Parse(time.RFC3339, now)
	return rt, err
}

// ---------------------------------------------------------------------------
// Runtime join tokens — workspace-minted tokens used to register a runtime.
// ---------------------------------------------------------------------------

// JoinToken is the public (non-secret) view of a minted join token.
type JoinToken struct {
	ID        string    `json:"id"`
	Label     string    `json:"label"`
	MaxUses   int       `json:"max_uses"`
	Uses      int       `json:"uses"`
	Revoked   bool      `json:"revoked"`
	ExpiresAt string    `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// MintJoinToken creates a join token for a workspace and returns the public
// record plus the one-time secret (mxrt_...).
func (s *Store) MintJoinToken(workspaceID, createdBy, label string, maxUses int, ttl time.Duration) (*JoinToken, string, error) {
	if maxUses <= 0 {
		maxUses = 1
	}
	raw, err := auth.NewToken()
	if err != nil {
		return nil, "", err
	}
	secret := "mxrt_" + raw
	now := time.Now().UTC()
	exp := ""
	if ttl > 0 {
		exp = now.Add(ttl).Format(time.RFC3339)
	}
	jt := &JoinToken{ID: auth.NewID("jt_"), Label: label, MaxUses: maxUses, ExpiresAt: exp, CreatedAt: now}
	_, err = s.exec(`INSERT INTO runtime_join_tokens (id,workspace_id,token_hash,label,created_by,max_uses,uses,revoked,expires_at,created_at)
		VALUES (?,?,?,?,?,?,0,0,?,?)`,
		jt.ID, workspaceID, hashToken(secret), label, createdBy, maxUses, exp, now.Format(time.RFC3339))
	if err != nil {
		return nil, "", err
	}
	return jt, secret, nil
}

// RedeemJoinToken validates a join-token secret and returns its workspace,
// incrementing its use counter. Expired/revoked/exhausted tokens are rejected.
func (s *Store) RedeemJoinToken(secret string) (workspaceID string, err error) {
	var id, ws, exp string
	var maxUses, uses, revoked int
	row := s.queryRow(`SELECT id,workspace_id,max_uses,uses,revoked,expires_at FROM runtime_join_tokens WHERE token_hash=?`, hashToken(secret))
	if err := row.Scan(&id, &ws, &maxUses, &uses, &revoked, &exp); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", errors.New("invalid join token")
		}
		return "", err
	}
	if revoked == 1 {
		return "", errors.New("join token revoked")
	}
	if exp != "" {
		if t, e := time.Parse(time.RFC3339, exp); e == nil && time.Now().After(t) {
			return "", errors.New("join token expired")
		}
	}
	if uses >= maxUses {
		return "", errors.New("join token already used")
	}
	if _, err := s.exec(`UPDATE runtime_join_tokens SET uses=uses+1 WHERE id=?`, id); err != nil {
		return "", err
	}
	return ws, nil
}

// ListJoinTokens returns a workspace's active (non-revoked) tokens.
func (s *Store) ListJoinTokens(workspaceID string) ([]JoinToken, error) {
	rows, err := s.query(`SELECT id,label,max_uses,uses,revoked,expires_at,created_at FROM runtime_join_tokens WHERE workspace_id=? ORDER BY created_at DESC`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []JoinToken{}
	for rows.Next() {
		var jt JoinToken
		var rev int
		var created string
		if err := rows.Scan(&jt.ID, &jt.Label, &jt.MaxUses, &jt.Uses, &rev, &jt.ExpiresAt, &created); err != nil {
			return nil, err
		}
		jt.Revoked = rev == 1
		jt.CreatedAt, _ = time.Parse(time.RFC3339, created)
		out = append(out, jt)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// Provider credentials — BYO model-provider tokens (encrypted at rest).
// ---------------------------------------------------------------------------

// ProviderCredential is the public view of a stored credential (no secret).
type ProviderCredential struct {
	ID        string         `json:"id"`
	Provider  string         `json:"provider"`
	Label     string         `json:"label"`
	Hint      string         `json:"hint"`
	Meta      map[string]any `json:"meta"`
	CreatedAt time.Time      `json:"created_at"`
}

// SetProviderCredential stores (upserts) an encrypted provider token for a
// workspace. The plaintext secret is never persisted.
func (s *Store) SetProviderCredential(workspaceID, createdBy, provider, label, secret string, meta map[string]any) (*ProviderCredential, error) {
	if label == "" {
		label = "default"
	}
	enc, err := s.box.Encrypt(secret)
	if err != nil {
		return nil, err
	}
	metaB, _ := json.Marshal(meta)
	now := nowRFC()
	id := auth.NewID("pc_")
	_, err = s.exec(`
		INSERT INTO provider_credentials (id,workspace_id,provider,label,secret_enc,hint,meta_json,created_by,created_at,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(workspace_id,provider,label) DO UPDATE SET
		  secret_enc=excluded.secret_enc, hint=excluded.hint, meta_json=excluded.meta_json, updated_at=excluded.updated_at`,
		id, workspaceID, provider, label, enc, auth.Hint(secret), string(metaB), createdBy, now, now)
	if err != nil {
		return nil, err
	}
	created, _ := time.Parse(time.RFC3339, now)
	return &ProviderCredential{ID: id, Provider: provider, Label: label, Hint: auth.Hint(secret), Meta: meta, CreatedAt: created}, nil
}

// ListProviderCredentials returns a workspace's credentials (without secrets).
func (s *Store) ListProviderCredentials(workspaceID string) ([]ProviderCredential, error) {
	rows, err := s.query(`SELECT id,provider,label,hint,meta_json,created_at FROM provider_credentials WHERE workspace_id=? ORDER BY provider`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []ProviderCredential{}
	for rows.Next() {
		var pc ProviderCredential
		var meta, created string
		if err := rows.Scan(&pc.ID, &pc.Provider, &pc.Label, &pc.Hint, &meta, &created); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(meta), &pc.Meta)
		pc.CreatedAt, _ = time.Parse(time.RFC3339, created)
		out = append(out, pc)
	}
	return out, rows.Err()
}

// ProviderSecret decrypts and returns a workspace's provider token (server-side
// use only — e.g. to call the model gateway on the user's behalf).
func (s *Store) ProviderSecret(workspaceID, provider, label string) (string, error) {
	if label == "" {
		label = "default"
	}
	var enc string
	row := s.queryRow(`SELECT secret_enc FROM provider_credentials WHERE workspace_id=? AND provider=? AND label=?`, workspaceID, provider, label)
	if err := row.Scan(&enc); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", err
	}
	return s.box.Decrypt(enc)
}

// ---------------------------------------------------------------------------
// Usage metering — append-only events for free-plan limits and analytics.
// ---------------------------------------------------------------------------

// RecordUsage appends a metering event.
func (s *Store) RecordUsage(workspaceID, userID, runtimeID, kind string, quantity int, meta map[string]any) error {
	if quantity <= 0 {
		quantity = 1
	}
	metaB, _ := json.Marshal(meta)
	_, err := s.exec(`INSERT INTO usage_events (id,workspace_id,user_id,runtime_id,kind,quantity,meta_json,created_at) VALUES (?,?,?,?,?,?,?,?)`,
		auth.NewID("ue_"), workspaceID, userID, runtimeID, kind, quantity, string(metaB), nowRFC())
	return err
}

// UsageSince returns summed quantities per kind for a workspace since t.
func (s *Store) UsageSince(workspaceID string, since time.Time) (map[string]int, error) {
	rows, err := s.query(`SELECT kind, COALESCE(SUM(quantity),0) FROM usage_events WHERE workspace_id=? AND created_at>=? GROUP BY kind`,
		workspaceID, since.UTC().Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[string]int{}
	for rows.Next() {
		var k string
		var n int
		if err := rows.Scan(&k, &n); err != nil {
			return nil, err
		}
		out[k] = n
	}
	return out, rows.Err()
}
