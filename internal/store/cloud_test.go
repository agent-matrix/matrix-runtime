package store

import (
	"testing"
	"time"
)

func TestCloudTablesCoexistWithExistingData(t *testing.T) {
	s := newTestStore(t)
	// existing tables still work
	u, err := s.Signup("Neo", "neo@zion.io", "redpill1", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateProfile(ModelProfile{WorkspaceID: u.WorkspaceID, SourceType: "huggingface", Provider: "Hugging Face", ExternalID: "x/y", DisplayName: "x/y"}); err != nil {
		t.Fatalf("existing model_profiles broken: %v", err)
	}
	ws := u.WorkspaceID

	// join token → register runtime → heartbeat
	jt, secret, err := s.MintJoinToken(ws, u.ID, "my space", 1, time.Hour)
	if err != nil || jt.ID == "" {
		t.Fatalf("mint join token: %v", err)
	}
	gotWS, err := s.RedeemJoinToken(secret)
	if err != nil || gotWS != ws {
		t.Fatalf("redeem: ws=%s err=%v", gotWS, err)
	}
	if _, err := s.RedeemJoinToken(secret); err == nil {
		t.Error("single-use join token redeemed twice")
	}

	rt, token, err := s.RegisterRuntime(Runtime{WorkspaceID: ws, Name: "maya-space", Mode: "hf-space", Kind: "hf-space", HFSpace: "maya/matrixcloud", Caps: []string{"mcp.test"}})
	if err != nil || token == "" {
		t.Fatalf("register runtime: %v", err)
	}
	if _, err := s.HeartbeatRuntime(token, "online", []string{"mcp.test", "model.inspect"}); err != nil {
		t.Fatalf("heartbeat: %v", err)
	}
	if _, err := s.HeartbeatRuntime("bogus", "online", nil); err != ErrNotFound {
		t.Errorf("expected ErrNotFound for bad token, got %v", err)
	}
	list, err := s.ListRuntimes(ws, time.Minute)
	if err != nil || len(list) != 1 || list[0].Status != "online" || len(list[0].Caps) != 2 {
		t.Fatalf("ListRuntimes = %+v err=%v", list, err)
	}
	_ = rt

	// provider credential is encrypted at rest and recoverable server-side
	if _, err := s.SetProviderCredential(ws, u.ID, "huggingface", "default", "hf_supersecret_token", map[string]any{"default_model": "Qwen/Qwen2.5-7B-Instruct"}); err != nil {
		t.Fatalf("set credential: %v", err)
	}
	creds, _ := s.ListProviderCredentials(ws)
	if len(creds) != 1 || creds[0].Hint == "" || creds[0].Hint == "hf_supersecret_token" {
		t.Fatalf("credential should expose only a hint, got %+v", creds)
	}
	sec, err := s.ProviderSecret(ws, "huggingface", "default")
	if err != nil || sec != "hf_supersecret_token" {
		t.Fatalf("decrypt provider secret failed: %q %v", sec, err)
	}

	// usage metering
	for i := 0; i < 3; i++ {
		_ = s.RecordUsage(ws, u.ID, rt.ID, "model.inspect", 1, nil)
	}
	used, err := s.UsageSince(ws, time.Now().Add(-time.Hour))
	if err != nil || used["model.inspect"] != 3 {
		t.Fatalf("usage = %v err=%v", used, err)
	}
}
