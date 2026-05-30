// Package pyenv manages real Python virtual environments on the host: creating
// them (uv when available, else python -m venv), installing packages, and
// running commands inside them. It backs the runtime's local Python sandbox
// (e.g. for MatrixShell).
package pyenv

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Env is a Python virtual environment rooted at Dir.
type Env struct {
	Dir string
}

func binSubdir() string {
	if runtime.GOOS == "windows" {
		return "Scripts"
	}
	return "bin"
}

func exeSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

// Python returns the venv's python interpreter path.
func (e *Env) Python() string { return filepath.Join(e.Dir, binSubdir(), "python"+exeSuffix()) }

// Bin returns the path to a console script installed in the venv.
func (e *Env) Bin(name string) string { return filepath.Join(e.Dir, binSubdir(), name+exeSuffix()) }

// Exists reports whether the venv has been created.
func (e *Env) Exists() bool {
	_, err := os.Stat(e.Python())
	return err == nil
}

// HasBin reports whether a console script exists in the venv.
func (e *Env) HasBin(name string) bool {
	_, err := os.Stat(e.Bin(name))
	return err == nil
}

func hasUv() bool { _, err := exec.LookPath("uv"); return err == nil }

func hostPython() string {
	for _, c := range []string{"python3", "python"} {
		if _, err := exec.LookPath(c); err == nil {
			return c
		}
	}
	return "python3"
}

// Ensure creates the venv if it does not exist, streaming command output to
// emit (line by line). It prefers uv (fast) and falls back to python -m venv.
func Ensure(ctx context.Context, dir string, emit func(string)) (*Env, error) {
	e := &Env{Dir: dir}
	if e.Exists() {
		return e, nil
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return nil, err
	}
	var cmd *exec.Cmd
	if hasUv() {
		cmd = exec.CommandContext(ctx, "uv", "venv", dir)
	} else {
		cmd = exec.CommandContext(ctx, hostPython(), "-m", "venv", dir)
	}
	if err := runStream(cmd, emit); err != nil {
		return nil, err
	}
	if !e.Exists() {
		return nil, errors.New("venv creation reported success but interpreter is missing")
	}
	return e, nil
}

// Pip installs packages into the venv (uv pip when available, else venv pip),
// streaming output to emit.
func (e *Env) Pip(ctx context.Context, emit func(string), args ...string) error {
	var cmd *exec.Cmd
	if hasUv() {
		a := append([]string{"pip", "install", "--python", e.Python()}, args...)
		cmd = exec.CommandContext(ctx, "uv", a...)
	} else {
		a := append([]string{"-m", "pip", "install"}, args...)
		cmd = exec.CommandContext(ctx, e.Python(), a...)
	}
	return runStream(cmd, emit)
}

// Result is the outcome of a sandbox command.
type Result struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
}

// Shell runs a shell command inside workDir with the venv's bin on PATH and
// VIRTUAL_ENV set, capturing stdout/stderr/exit. It uses bash on unix.
func (e *Env) Shell(ctx context.Context, workDir, command string) (Result, error) {
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return Result{}, err
	}
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", command)
	} else {
		cmd = exec.CommandContext(ctx, "bash", "-lc", command)
	}
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(),
		"VIRTUAL_ENV="+e.Dir,
		"PATH="+filepath.Join(e.Dir, binSubdir())+string(os.PathListSeparator)+os.Getenv("PATH"),
	)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	return Result{Stdout: out.String(), Stderr: errb.String(), ExitCode: exitCode(err)}, nil
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	return -1
}

func runStream(cmd *exec.Cmd, emit func(string)) error {
	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw
	if err := cmd.Start(); err != nil {
		_ = pw.Close()
		return err
	}
	done := make(chan struct{})
	go func() {
		sc := bufio.NewScanner(pr)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			line := strings.TrimRight(sc.Text(), "\r")
			if emit != nil && line != "" {
				emit(line)
			}
		}
		close(done)
	}()
	err := cmd.Wait()
	_ = pw.Close()
	<-done
	return err
}
