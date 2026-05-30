package store

// schema is the SQLite DDL applied on Open. It is idempotent.
const schema = `
CREATE TABLE IF NOT EXISTS workspaces (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    slug        TEXT NOT NULL UNIQUE,
    created_at  TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS users (
    id             TEXT PRIMARY KEY,
    workspace_id   TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name           TEXT NOT NULL,
    email          TEXT NOT NULL UNIQUE,
    password_hash  TEXT NOT NULL,
    role           TEXT NOT NULL DEFAULT 'Member',
    created_at     TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_users_workspace ON users(workspace_id);

CREATE TABLE IF NOT EXISTS sessions (
    token       TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at  TEXT NOT NULL,
    expires_at  TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);

CREATE TABLE IF NOT EXISTS model_profiles (
    id            TEXT PRIMARY KEY,
    workspace_id  TEXT NOT NULL,
    source_type   TEXT NOT NULL,
    source_uri    TEXT NOT NULL DEFAULT '',
    provider      TEXT NOT NULL DEFAULT '',
    external_id   TEXT NOT NULL DEFAULT '',
    display_name  TEXT NOT NULL DEFAULT '',
    task          TEXT NOT NULL DEFAULT '',
    library       TEXT NOT NULL DEFAULT '',
    license       TEXT NOT NULL DEFAULT '',
    tags_json     TEXT NOT NULL DEFAULT '[]',
    metadata_json TEXT NOT NULL DEFAULT '{}',
    status        TEXT NOT NULL DEFAULT 'profile_only',
    created_at    TEXT NOT NULL,
    updated_at    TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_profiles_workspace ON model_profiles(workspace_id);

CREATE TABLE IF NOT EXISTS model_runtime_installations (
    id               TEXT PRIMARY KEY,
    workspace_id     TEXT NOT NULL,
    model_profile_id TEXT NOT NULL,
    runtime_id       TEXT NOT NULL,
    install_mode     TEXT NOT NULL DEFAULT 'pull_from_source',
    serving_engine   TEXT NOT NULL DEFAULT '',
    status           TEXT NOT NULL DEFAULT 'queued',
    progress         INTEGER NOT NULL DEFAULT 0,
    local_path       TEXT NOT NULL DEFAULT '',
    endpoint_url     TEXT NOT NULL DEFAULT '',
    job_id           TEXT NOT NULL DEFAULT '',
    created_at       TEXT NOT NULL,
    updated_at       TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_installs_workspace ON model_runtime_installations(workspace_id);

-- ===========================================================================
-- Hosted MatrixCloud (cloud.matrixhub.io): per-workspace runtimes, join
-- tokens, BYO provider credentials and usage metering. All additive.
-- ===========================================================================

-- A registered runtime/sandbox plane that belongs to a workspace. A user's
-- duplicated Hugging Face Space registers itself here (outbound) and is then
-- managed centrally from the control plane.
CREATE TABLE IF NOT EXISTS runtimes (
    id            TEXT PRIMARY KEY,             -- rt_xxxx
    workspace_id  TEXT NOT NULL,
    name          TEXT NOT NULL,
    mode          TEXT NOT NULL DEFAULT 'customer-agent',
    kind          TEXT NOT NULL DEFAULT 'self-hosted', -- hf-space | self-hosted | local
    url           TEXT NOT NULL DEFAULT '',     -- e.g. https://user-space.hf.space
    hf_space      TEXT NOT NULL DEFAULT '',     -- "owner/space" for HF-space runtimes
    region        TEXT NOT NULL DEFAULT '',
    caps_json     TEXT NOT NULL DEFAULT '[]',
    status        TEXT NOT NULL DEFAULT 'pending', -- pending | online | idle | offline
    token_hash    TEXT NOT NULL DEFAULT '',     -- hashed runtime token (heartbeat auth)
    version       TEXT NOT NULL DEFAULT '',
    last_seen_at  TEXT NOT NULL DEFAULT '',
    created_at    TEXT NOT NULL,
    updated_at    TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_runtimes_workspace ON runtimes(workspace_id);

-- Single/limited-use tokens a workspace mints so a runtime can join.
CREATE TABLE IF NOT EXISTS runtime_join_tokens (
    id           TEXT PRIMARY KEY,              -- jt_xxxx
    workspace_id TEXT NOT NULL,
    token_hash   TEXT NOT NULL UNIQUE,          -- hash of the mxrt_ token
    label        TEXT NOT NULL DEFAULT '',
    created_by   TEXT NOT NULL DEFAULT '',
    max_uses     INTEGER NOT NULL DEFAULT 1,
    uses         INTEGER NOT NULL DEFAULT 0,
    revoked      INTEGER NOT NULL DEFAULT 0,
    expires_at   TEXT NOT NULL DEFAULT '',
    created_at   TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_jointokens_workspace ON runtime_join_tokens(workspace_id);

-- BYO model-provider credentials (Hugging Face token, OpenAI-compatible, …),
-- AES-GCM encrypted at rest. This is how a free user plugs in their own HF
-- account to use HF inference inside MatrixCloud.
CREATE TABLE IF NOT EXISTS provider_credentials (
    id           TEXT PRIMARY KEY,              -- pc_xxxx
    workspace_id TEXT NOT NULL,
    provider     TEXT NOT NULL,                 -- huggingface | openai | ollama | vllm | anthropic
    label        TEXT NOT NULL DEFAULT 'default',
    secret_enc   TEXT NOT NULL DEFAULT '',      -- AES-GCM ciphertext (never plaintext)
    hint         TEXT NOT NULL DEFAULT '',      -- last-4 display hint
    meta_json    TEXT NOT NULL DEFAULT '{}',    -- {base_url, default_model, …}
    created_by   TEXT NOT NULL DEFAULT '',
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL,
    UNIQUE(workspace_id, provider, label)
);
CREATE INDEX IF NOT EXISTS idx_providers_workspace ON provider_credentials(workspace_id);

-- Append-only metering for free-plan limits and analytics.
CREATE TABLE IF NOT EXISTS usage_events (
    id           TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    user_id      TEXT NOT NULL DEFAULT '',
    runtime_id   TEXT NOT NULL DEFAULT '',
    kind         TEXT NOT NULL,                 -- model.inspect | sandbox.start | llm.tokens | model.attach | matrixshell.exec
    quantity     INTEGER NOT NULL DEFAULT 1,
    meta_json    TEXT NOT NULL DEFAULT '{}',
    created_at   TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_usage_workspace ON usage_events(workspace_id, kind);

-- ===========================================================================
-- Account email flows (Resend): password resets and email verification.
-- Tokens are stored hashed; only single-use, time-bounded rows are honored.
-- ===========================================================================
CREATE TABLE IF NOT EXISTS password_resets (
    id          TEXT PRIMARY KEY,                 -- pr_xxxx
    user_id     TEXT NOT NULL,
    token_hash  TEXT NOT NULL UNIQUE,
    expires_at  TEXT NOT NULL,
    used_at     TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_resets_user ON password_resets(user_id);

CREATE TABLE IF NOT EXISTS email_verifications (
    id          TEXT PRIMARY KEY,                 -- ev_xxxx
    user_id     TEXT NOT NULL,
    token_hash  TEXT NOT NULL UNIQUE,
    expires_at  TEXT NOT NULL,
    used_at     TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_verifs_user ON email_verifications(user_id);

-- ===========================================================================
-- Audit log — append-only record of sensitive actions for traceability.
-- ===========================================================================
CREATE TABLE IF NOT EXISTS audit_events (
    id           TEXT PRIMARY KEY,                 -- au_xxxx
    workspace_id TEXT NOT NULL DEFAULT '',
    actor        TEXT NOT NULL DEFAULT '',         -- user_id or runtime_id
    action       TEXT NOT NULL,                    -- e.g. sandbox.started
    target       TEXT NOT NULL DEFAULT '',
    ip           TEXT NOT NULL DEFAULT '',
    status       TEXT NOT NULL DEFAULT 'success',  -- success | failure
    meta_json    TEXT NOT NULL DEFAULT '{}',
    created_at   TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_audit_workspace ON audit_events(workspace_id, created_at);
`
