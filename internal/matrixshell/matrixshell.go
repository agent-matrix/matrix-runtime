// Package matrixshell installs and runs the real MatrixShell CLI
// (github.com/agent-matrix/MatrixShell) inside a dedicated Python virtual
// environment on the host, and executes commands in a sandbox working directory
// with the venv on PATH. No simulation: install, version and command output are
// all real.
package matrixshell

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/agent-matrix/matrix-runtime/internal/pyenv"
)

// RepoSpec is the pip/uv install spec for MatrixShell (not on PyPI yet).
const RepoSpec = "git+https://github.com/agent-matrix/MatrixShell.git@master"

// ErrBlocked is returned when a command is refused by the safety denylist.
var ErrBlocked = errors.New("refused by safety denylist")

// hard denylist — destructive operations refused even on explicit confirm.
var denyList = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bmkfs\b`), regexp.MustCompile(`(?i)\bdd\s+if=`),
	regexp.MustCompile(`(?i)\bfdisk\b`), regexp.MustCompile(`(?i)\bdiskpart\b`),
	regexp.MustCompile(`(?i)\bshutdown\b`), regexp.MustCompile(`(?i)\breboot\b`),
	regexp.MustCompile(`(?i)\bmkswap\b`), regexp.MustCompile(`:\(\)\s*\{`),
	regexp.MustCompile(`(?i)\brm\s+-rf\s+(/|~|\$HOME)(\s|$)`),
	regexp.MustCompile(`>\s*/dev/sd`),
}

// Denied reports whether a command is blocked by the safety denylist.
func Denied(command string) bool {
	for _, re := range denyList {
		if re.MatchString(command) {
			return true
		}
	}
	return false
}

// VenvDir is the MatrixShell sandbox venv directory under the data dir.
func VenvDir(dataDir string) string { return filepath.Join(dataDir, "venvs", "matrixshell") }

// SandboxDir is the working directory commands run in.
func SandboxDir(dataDir string) string { return filepath.Join(dataDir, "sandbox", "matrixshell") }

func env(dataDir string) *pyenv.Env { return &pyenv.Env{Dir: VenvDir(dataDir)} }

// Status describes the local MatrixShell installation.
type Status struct {
	Installed bool   `json:"installed"`
	Version   string `json:"version"`
	Venv      string `json:"venv"`
	Python    string `json:"python"`
	Sandbox   string `json:"sandbox"`
	Spec      string `json:"spec"`
}

// GetStatus inspects the sandbox venv and reports whether matrixsh is installed.
func GetStatus(ctx context.Context, dataDir string) Status {
	e := env(dataDir)
	st := Status{Venv: e.Dir, Sandbox: SandboxDir(dataDir), Spec: RepoSpec}
	if !e.Exists() {
		return st
	}
	st.Python = e.Python()
	if e.HasBin("matrixsh") {
		res, _ := e.Shell(ctx, SandboxDir(dataDir), `python -c "import importlib.metadata as m; print(m.version('matrixsh'))"`)
		if res.ExitCode == 0 {
			st.Installed = true
			st.Version = strings.TrimSpace(res.Stdout)
		}
	}
	return st
}

// Install creates the sandbox venv (if needed) and installs MatrixShell from
// git, streaming real output to emit. Returns the resulting status.
func Install(ctx context.Context, dataDir string, emit func(string)) (Status, error) {
	dir := VenvDir(dataDir)
	emit("creating python sandbox venv: " + dir)
	e, err := pyenv.Ensure(ctx, dir, emit)
	if err != nil {
		return Status{}, err
	}
	emit("installing MatrixShell (" + RepoSpec + ") — this is a real pip install")
	if err := e.Pip(ctx, emit, RepoSpec); err != nil {
		return Status{}, err
	}
	if err := os.MkdirAll(SandboxDir(dataDir), 0o755); err != nil {
		return Status{}, err
	}
	st := GetStatus(ctx, dataDir)
	if !st.Installed {
		return st, errors.New("matrixsh installed but its version could not be resolved")
	}
	emit("matrixsh " + st.Version + " ready in the sandbox")
	return st, nil
}

// Exec runs a command inside the sandbox venv (with matrixsh/python on PATH),
// in the sandbox working directory. Destructive commands are refused.
func Exec(ctx context.Context, dataDir, command string) (pyenv.Result, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return pyenv.Result{}, errors.New("empty command")
	}
	if Denied(command) {
		return pyenv.Result{}, ErrBlocked
	}
	e := env(dataDir)
	if !e.Exists() {
		return pyenv.Result{}, errors.New("MatrixShell sandbox is not installed yet")
	}
	return e.Shell(ctx, SandboxDir(dataDir), command)
}
