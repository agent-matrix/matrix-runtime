# Authentication & multitenancy

MatrixCloud accounts are persisted in a real **SQLite** database, replacing the
design prototype's localStorage demo. The driver is the pure-Go
`modernc.org/sqlite`, so the binary stays statically linkable with `CGO_ENABLED=0`.

## Data model

```
workspaces (tenants)   users                          sessions
-----------            ---------------------------    -----------------------
id (PK)                id (PK)                        token (PK)
name                   workspace_id  -> workspaces    user_id -> users
slug (unique)          name                           created_at
created_at             email (unique, lowercased)     expires_at  (30 days)
                       password_hash                  
                       role          (Owner|Member)
                       created_at
```

- **Signup** creates a workspace (tenant) and an `Owner` user, then issues a
  session. Workspace slugs are de-duplicated automatically.
- Passwords are hashed with **PBKDF2-HMAC-SHA256** (120k iterations, per-user
  random salt) using only the standard library — no external crypto dependency.
- Sessions are random 32-byte bearer tokens with a 30-day expiry. Logout deletes
  the current session; `?all=true` deletes every session for the user.

## Endpoints

| Method | Path               | Auth            | Purpose                          |
|--------|--------------------|-----------------|----------------------------------|
| POST   | `/v1/auth/signup`  | public          | Create workspace + owner account |
| POST   | `/v1/auth/login`   | public          | Authenticate, start a session    |
| GET    | `/v1/auth/me`      | session bearer  | Current user                     |
| POST   | `/v1/auth/logout`  | session bearer  | End session (`?all=true` for all)|

Auth endpoints are always public — they are exempt from the operator
`MATRIX_RUNTIME_API_TOKEN` (which protects the rest of `/v1`). User sessions and
the operator token are independent mechanisms.

```bash
# sign up (creates a workspace + owner, returns a token)
curl -s -X POST localhost:8080/v1/auth/signup \
  -H 'Content-Type: application/json' \
  -d '{"name":"Maya Chen","email":"maya@acme.io","password":"secret123"}'

# use the token
curl -s localhost:8080/v1/auth/me -H "Authorization: Bearer <token>"
```

## Configuration

| Variable                 | Default                       | Meaning                  |
|--------------------------|-------------------------------|--------------------------|
| `MATRIX_RUNTIME_DB_PATH` | `<data-dir>/matrixcloud.db`   | SQLite database path     |

The database is opened on startup; if it cannot be opened the console still
loads but auth endpoints return `503`.

## Scope & roadmap

This iteration delivers persistent, multitenant **accounts and workspaces**.
Per-workspace scoping of jobs/sandboxes, member invitations, roles/RBAC and SSO
are natural next steps and are intentionally out of scope here.
