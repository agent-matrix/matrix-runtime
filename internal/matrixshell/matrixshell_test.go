package matrixshell

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestDenied(t *testing.T) {
	blocked := []string{"mkfs.ext4 /dev/sda", "dd if=/dev/zero of=/dev/sda", "shutdown now", "rm -rf /", ":(){ :|:& };:"}
	for _, c := range blocked {
		if !Denied(c) {
			t.Errorf("expected %q to be denied", c)
		}
	}
	allowed := []string{"ls -la", "python --version", "matrixsh --help", "pip list", "echo hi", "rm -rf ./build"}
	for _, c := range allowed {
		if Denied(c) {
			t.Errorf("did not expect %q to be denied", c)
		}
	}
}

func TestStatusNotInstalled(t *testing.T) {
	dir := t.TempDir()
	st := GetStatus(context.Background(), dir)
	if st.Installed {
		t.Error("expected not installed on a fresh dir")
	}
	if st.Spec == "" || st.Venv == "" || st.Sandbox == "" {
		t.Error("status paths/spec should be populated")
	}
}

// TestInstallAndExec performs a REAL install of MatrixShell from git into a
// sandbox venv and runs commands in it. Opt-in (network + ~1 min) via
// MATRIXSHELL_E2E=1 so it never slows the default test suite / CI.
func TestInstallAndExec(t *testing.T) {
	if os.Getenv("MATRIXSHELL_E2E") != "1" {
		t.Skip("set MATRIXSHELL_E2E=1 to run the real install integration test")
	}
	dir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	var lines []string
	st, err := Install(ctx, dir, func(s string) { lines = append(lines, s) })
	if err != nil {
		t.Fatalf("install: %v\n%s", err, strings.Join(lines, "\n"))
	}
	if !st.Installed || st.Version == "" {
		t.Fatalf("expected installed with version, got %+v", st)
	}
	t.Logf("installed matrixsh %s", st.Version)

	// real exec inside the sandbox venv
	res, err := Exec(ctx, dir, "matrixsh --help")
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != 0 || !strings.Contains(res.Stdout+res.Stderr, "matrixsh") {
		t.Fatalf("matrixsh --help: code=%d out=%q err=%q", res.ExitCode, res.Stdout, res.Stderr)
	}

	// a real shell command in the sandbox
	res, _ = Exec(ctx, dir, "python -c \"print(6*7)\"")
	if strings.TrimSpace(res.Stdout) != "42" {
		t.Fatalf("python exec returned %q", res.Stdout)
	}

	// denylist enforced
	if _, err := Exec(ctx, dir, "shutdown now"); err != ErrBlocked {
		t.Fatalf("expected ErrBlocked, got %v", err)
	}
}
