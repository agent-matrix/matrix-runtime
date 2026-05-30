package jobs

import (
	"context"

	"github.com/agent-matrix/matrix-runtime/internal/matrixshell"
)

// handleMatrixShellInstall creates the local Python sandbox venv and installs
// the real MatrixShell CLI into it, streaming pip/venv output over SSE.
func handleMatrixShellInstall(ctx context.Context, m *Manager, j *Job) error {
	j.Emit("queue", EvOK, "Preparing MatrixShell sandbox", nil)
	st, err := matrixshell.Install(ctx, m.Config().DataDir, func(line string) {
		j.Emit("install", EvRunning, line, nil)
	})
	if err != nil {
		return err
	}
	j.setResult(map[string]any{
		"installed": st.Installed, "version": st.Version,
		"venv": st.Venv, "sandbox": st.Sandbox, "python": st.Python,
	})
	j.Emit("ready", EvOK, "MatrixShell "+st.Version+" installed in sandbox", nil)
	return nil
}
