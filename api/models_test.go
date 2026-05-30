package api

import (
	"net/http"
	"testing"
)

// authedToken signs up a user on the given server and returns its session token.
func authedToken(t *testing.T, srv *Server) string {
	t.Helper()
	rec, body := do(t, srv, http.MethodPost, "/v1/auth/signup", "", `{"name":"Mo","email":"mo@acme.io","password":"hunter2x"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("signup %d", rec.Code)
	}
	return body["token"].(string)
}

func TestModelProfilesRequireAuth(t *testing.T) {
	srv := authServer(t)
	rec, _ := do(t, srv, http.MethodGet, "/v1/model-profiles", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth, got %d", rec.Code)
	}
}

func TestImportListAttachProfile(t *testing.T) {
	srv := authServer(t)
	tok := authedToken(t, srv)

	// import a profile
	rec, body := do(t, srv, http.MethodPost, "/v1/model-profiles", tok,
		`{"source_type":"huggingface","provider":"Hugging Face","external_id":"deepseek-ai/DeepSeek-V3","display_name":"deepseek-ai/DeepSeek-V3","task":"text-generation"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("import status %d: %v", rec.Code, body)
	}
	prof := body["profile"].(map[string]any)
	if prof["status"] != "profile_only" {
		t.Errorf("status = %v", prof["status"])
	}
	pid := prof["id"].(string)

	// list profiles
	rec, body = do(t, srv, http.MethodGet, "/v1/model-profiles", tok, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list status %d", rec.Code)
	}
	if len(body["profiles"].([]any)) != 1 {
		t.Fatalf("expected 1 profile, got %v", body["profiles"])
	}

	// attach -> creates installation + a job
	rec, body = do(t, srv, http.MethodPost, "/v1/model-profiles/"+pid+"/attach", tok,
		`{"runtimeId":"acme-prod-runtime","installMode":"pull_from_source","servingEngine":"vLLM"}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("attach status %d: %v", rec.Code, body)
	}
	if body["job_id"] == nil || body["installation_id"] == nil {
		t.Fatalf("attach missing ids: %v", body)
	}

	// installation should now appear in the runtime cache
	rec, body = do(t, srv, http.MethodGet, "/v1/model-installations", tok, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("installations status %d", rec.Code)
	}
	ins := body["installations"].([]any)
	if len(ins) != 1 {
		t.Fatalf("expected 1 installation, got %d", len(ins))
	}
	got := ins[0].(map[string]any)
	if got["runtime_id"] != "acme-prod-runtime" || got["serving_engine"] != "vLLM" {
		t.Errorf("installation fields: %v", got)
	}

	// cancel the background job so it doesn't linger past the test
	if id, _ := got["job_id"].(string); id != "" {
		_, _ = do(t, srv, http.MethodDelete, "/v1/jobs/"+id, tok, "")
	}
}
