package api

import (
	"net/http"
	"testing"
)

func TestCloudRuntimeAndProviderFlow(t *testing.T) {
	srv := authServer(t)

	// Create a workspace owner and grab a session token.
	rec, body := do(t, srv, http.MethodPost, "/v1/auth/signup", "", `{"name":"Maya","email":"maya@acme.io","password":"hunter22"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("signup %d: %v", rec.Code, body)
	}
	token, _ := body["token"].(string)

	// Mint a join token.
	rec, body = do(t, srv, http.MethodPost, "/v1/cloud/join-tokens", token, `{"label":"my space","max_uses":1,"ttl_minutes":60}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("mint join token %d: %v", rec.Code, body)
	}
	secret, _ := body["secret"].(string)
	if secret == "" {
		t.Fatal("expected a join-token secret")
	}

	// Register a runtime using the join token (no session — simulates a remote Space).
	rec, body = do(t, srv, http.MethodPost, "/v1/cloud/runtimes/register", "",
		`{"join_token":"`+secret+`","name":"maya-space","kind":"hf-space","hf_space":"maya/matrixcloud","caps":["mcp.test"]}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("register runtime %d: %v", rec.Code, body)
	}
	runtimeToken, _ := body["runtime_token"].(string)
	if runtimeToken == "" {
		t.Fatal("expected a runtime token")
	}

	// The join token is single-use now.
	rec, _ = do(t, srv, http.MethodPost, "/v1/cloud/runtimes/register", "",
		`{"join_token":"`+secret+`","name":"again"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("reused join token should 401, got %d", rec.Code)
	}

	// Heartbeat with the runtime token.
	rec, _ = do(t, srv, http.MethodPost, "/v1/cloud/runtimes/heartbeat", runtimeToken, `{"status":"online","caps":["mcp.test","model.inspect"]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("heartbeat %d", rec.Code)
	}
	// Bad runtime token rejected.
	rec, _ = do(t, srv, http.MethodPost, "/v1/cloud/runtimes/heartbeat", "bogus", `{"status":"online"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("bad heartbeat token should 401, got %d", rec.Code)
	}

	// The owner sees the runtime online.
	rec, body = do(t, srv, http.MethodGet, "/v1/cloud/runtimes", token, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list runtimes %d", rec.Code)
	}
	rts, _ := body["runtimes"].([]any)
	if len(rts) != 1 {
		t.Fatalf("expected 1 runtime, got %d", len(rts))
	}

	// Store a BYO Hugging Face credential; only a hint is ever returned.
	rec, body = do(t, srv, http.MethodPost, "/v1/cloud/providers", token,
		`{"provider":"huggingface","label":"default","secret":"hf_supersecret9","meta":{"default_model":"Qwen/Qwen2.5-7B-Instruct"}}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("set provider %d: %v", rec.Code, body)
	}
	rec, body = do(t, srv, http.MethodGet, "/v1/cloud/providers", token, "")
	provs, _ := body["providers"].([]any)
	if len(provs) != 1 {
		t.Fatalf("expected 1 provider, got %v", body)
	}
	p0, _ := provs[0].(map[string]any)
	if hint, _ := p0["hint"].(string); hint == "" || hint == "hf_supersecret9" {
		t.Errorf("provider must expose only a hint, got %v", p0["hint"])
	}

	// Cloud endpoints require auth.
	rec, _ = do(t, srv, http.MethodGet, "/v1/cloud/runtimes", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("unauth list should 401, got %d", rec.Code)
	}
}

func TestAuditLogRecordsActions(t *testing.T) {
	srv := authServer(t)
	_, body := do(t, srv, http.MethodPost, "/v1/auth/signup", "", `{"name":"Auditor","email":"audit@acme.io","password":"hunter22"}`)
	token, _ := body["token"].(string)

	// Generate a couple of auditable actions.
	do(t, srv, http.MethodPost, "/v1/cloud/join-tokens", token, `{"label":"x","max_uses":1}`)
	do(t, srv, http.MethodPost, "/v1/cloud/providers", token, `{"provider":"huggingface","secret":"hf_abc1234"}`)

	rec, b := do(t, srv, http.MethodGet, "/v1/cloud/audit", token, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("audit list %d: %v", rec.Code, b)
	}
	events, _ := b["events"].([]any)
	actions := map[string]bool{}
	for _, e := range events {
		if m, ok := e.(map[string]any); ok {
			actions[m["action"].(string)] = true
		}
	}
	for _, want := range []string{"user.signup", "runtime.join_token.created", "provider.credential.added"} {
		if !actions[want] {
			t.Errorf("missing audit action %q (got %v)", want, actions)
		}
	}
	// Unauthenticated access is rejected.
	if rec, _ := do(t, srv, http.MethodGet, "/v1/cloud/audit", "", ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("unauth audit = %d, want 401", rec.Code)
	}
}
