package cache

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSafeModelKey(t *testing.T) {
	if got := SafeModelKey("Qwen", "Qwen2.5-7B-Instruct"); got != "Qwen--Qwen2.5-7B-Instruct" {
		t.Errorf("got %q", got)
	}
}

func TestModelDir(t *testing.T) {
	l := New("/var/lib/matrix-runtime")
	got := l.ModelDir("Qwen", "Qwen2.5-7B-Instruct")
	want := filepath.Join("/var/lib/matrix-runtime", "models", "huggingface", "Qwen--Qwen2.5-7B-Instruct")
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestEnsureDirsAndMetadata(t *testing.T) {
	dir := t.TempDir()
	l := New(dir)
	if err := l.EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	mdDir, err := l.WriteModelMetadata(ModelMetadata{Namespace: "Qwen", Name: "Q-7B", Revision: "main", Source: "huggingface"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(mdDir, "Qwen--Q-7B") {
		t.Errorf("unexpected model dir %q", mdDir)
	}
	meta, err := l.ReadModelMetadata("Qwen", "Q-7B")
	if err != nil {
		t.Fatal(err)
	}
	if meta.Revision != "main" {
		t.Errorf("got revision %q", meta.Revision)
	}
}
