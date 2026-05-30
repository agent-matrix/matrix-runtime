package store

import "testing"

func TestQualifyPostgres(t *testing.T) {
	s := &Store{pg: true, schema: "matrixcloud", tblRe: compileTableRe()}
	cases := []struct{ in, want string }{
		{
			`SELECT u.id FROM users u JOIN workspaces w ON w.id = u.workspace_id WHERE u.email = ?`,
			`SELECT u.id FROM matrixcloud.users u JOIN matrixcloud.workspaces w ON w.id = u.workspace_id WHERE u.email = ?`,
		},
		{
			`CREATE INDEX IF NOT EXISTS idx_users_workspace ON users(workspace_id)`,
			`CREATE INDEX IF NOT EXISTS idx_users_workspace ON matrixcloud.users(workspace_id)`,
		},
		{
			`INSERT INTO model_runtime_installations (id,model_profile_id) VALUES (?,?)`,
			`INSERT INTO matrixcloud.model_runtime_installations (id,model_profile_id) VALUES (?,?)`,
		},
		{
			`LEFT JOIN model_profiles p ON p.id = i.model_profile_id`,
			`LEFT JOIN matrixcloud.model_profiles p ON p.id = i.model_profile_id`,
		},
		{
			`SELECT * FROM runtimes WHERE id IN (SELECT workspace_id FROM runtime_join_tokens)`,
			`SELECT * FROM matrixcloud.runtimes WHERE id IN (SELECT workspace_id FROM matrixcloud.runtime_join_tokens)`,
		},
		{
			`CREATE TABLE IF NOT EXISTS sessions (token TEXT, user_id TEXT REFERENCES users(id))`,
			`CREATE TABLE IF NOT EXISTS matrixcloud.sessions (token TEXT, user_id TEXT REFERENCES matrixcloud.users(id))`,
		},
	}
	for _, c := range cases {
		if got := s.qualify(c.in); got != c.want {
			t.Errorf("qualify mismatch\n in:   %s\n got:  %s\n want: %s", c.in, got, c.want)
		}
	}
}

func TestQualifyNoopForSQLite(t *testing.T) {
	s := &Store{pg: false}
	q := `SELECT * FROM users JOIN workspaces`
	if got := s.qualify(q); got != q {
		t.Errorf("SQLite qualify should be a no-op, got %q", got)
	}
}
