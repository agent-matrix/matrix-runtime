package store

import "testing"

func TestModelProfilesAndInstallations(t *testing.T) {
	s := newTestStore(t)
	u, err := s.Signup("Neo", "neo@zion.io", "redpill1", "")
	if err != nil {
		t.Fatal(err)
	}
	ws := u.WorkspaceID

	p, err := s.CreateProfile(ModelProfile{
		WorkspaceID: ws, SourceType: "huggingface", Provider: "Hugging Face",
		ExternalID: "deepseek-ai/DeepSeek-V3", DisplayName: "deepseek-ai/DeepSeek-V3",
		Task: "text-generation", Library: "transformers", License: "mit",
		Tags: []string{"moe"}, Metadata: map[string]any{"gpu": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.Status != "profile_only" {
		t.Errorf("default status = %q", p.Status)
	}

	list, err := s.ListProfiles(ws)
	if err != nil || len(list) != 1 || list[0].Tags[0] != "moe" {
		t.Fatalf("ListProfiles = %v err=%v", list, err)
	}

	in, err := s.CreateInstallation(ModelInstallation{
		WorkspaceID: ws, ModelProfileID: p.ID, RuntimeID: "rt_gpu_01",
		InstallMode: "pull_from_source", ServingEngine: "vLLM",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateInstallation(in.ID, "downloading", 43, "", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateInstallation(in.ID, "ready", 100, "/var/lib/x", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.SetProfileStatus(p.ID, "ready"); err != nil {
		t.Fatal(err)
	}

	ins, err := s.ListInstallations(ws)
	if err != nil || len(ins) != 1 {
		t.Fatalf("ListInstallations = %v err=%v", ins, err)
	}
	got := ins[0]
	if got.Status != "ready" || got.Progress != 100 || got.LocalPath != "/var/lib/x" {
		t.Errorf("installation not updated: %+v", got)
	}
	if got.ModelName != "deepseek-ai/DeepSeek-V3" || got.Provider != "Hugging Face" {
		t.Errorf("join fields missing: %+v", got)
	}

	pr, err := s.GetProfile(ws, p.ID)
	if err != nil || pr.Status != "ready" {
		t.Fatalf("GetProfile status = %v err=%v", pr, err)
	}
}
