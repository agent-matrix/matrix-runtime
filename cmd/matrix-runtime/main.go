// Command matrix-runtime is the execution plane for Matrix Cloud. It serves an
// HTTP API for running MCP sandboxes, inspecting models and (in future)
// agents/tools, and can join a MatrixHub Cloud control plane.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/agent-matrix/matrix-runtime/api"
	"github.com/agent-matrix/matrix-runtime/internal/config"
	"github.com/agent-matrix/matrix-runtime/internal/controlplane"
	"github.com/agent-matrix/matrix-runtime/internal/jobs"
	"github.com/agent-matrix/matrix-runtime/internal/store"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "join" {
		if err := runJoin(os.Args[2:]); err != nil {
			log.Fatalf("join: %v", err)
		}
		return
	}
	if err := runServe(os.Args[1:]); err != nil {
		log.Fatalf("matrix-runtime: %v", err)
	}
}

func runServe(args []string) error {
	fs := flag.NewFlagSet("matrix-runtime", flag.ExitOnError)
	mode := fs.String("mode", "", "runtime mode: cloud-worker|customer-agent|hf-space|local-dev")
	port := fs.Int("port", 0, "HTTP port (overrides MATRIX_RUNTIME_PORT)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg := config.FromEnv(*mode)
	if *port != 0 {
		cfg.Port = *port
	}
	if err := cfg.Validate(); err != nil {
		return err
	}

	// Loud, actionable warnings for unsafe production configuration. These do not
	// abort startup (the readiness probe also reports them) but they should be
	// impossible to miss in logs.
	if cfg.IsProduction() {
		if cfg.APIToken == "" {
			log.Printf("WARNING: MATRIX_RUNTIME_API_TOKEN is not set in %s mode — the API is unauthenticated for non-session callers", cfg.Mode)
		}
		if cfg.DatabaseURL == "" {
			log.Printf("WARNING: using SQLite in %s mode — set MATRIX_RUNTIME_DATABASE_URL (Postgres) for multi-user/HA deployments", cfg.Mode)
		}
	}
	if cfg.MatrixShellEnabled {
		log.Printf("WARNING: MatrixShell is ENABLED — it executes commands in a local sandbox (disable with MATRIX_SHELL_ENABLED=false)")
	}

	mgr := jobs.NewManager(cfg)
	defer mgr.Shutdown()

	if err := mgr.Layout().EnsureDirs(); err != nil {
		log.Printf("warning: could not create data dirs under %s: %v", cfg.DataDir, err)
	}

	// Open the multitenant user store. When a PostgreSQL/Neon URL is configured
	// (hosted control plane, e.g. cloud.matrixhub.io) use it and isolate all
	// objects in cfg.DBSchema; otherwise fall back to a local SQLite file. A
	// failure here is non-fatal: the console loads but auth endpoints return 503.
	var (
		st     *store.Store
		err    error
		dbDesc string
	)
	if cfg.DatabaseURL != "" {
		st, err = store.OpenPostgres(cfg.DatabaseURL, cfg.DBSchema, cfg.DataDir)
		dbDesc = fmt.Sprintf("postgres (schema %q)", cfg.DBSchema)
	} else {
		st, err = store.Open(cfg.DBPath)
		dbDesc = cfg.DBPath
	}
	if err != nil {
		log.Printf("warning: could not open user store (%s): %v", dbDesc, err)
	} else {
		defer func() { _ = st.Close() }()
		mgr.SetInstallStore(st)
		log.Printf("user store ready: %s", dbDesc)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Background retention: purge old terminal jobs, scratch dirs and logs.
	mgr.StartJanitor(ctx)

	// Bind the configured port; if it's busy, walk forward to the next free one.
	ln, boundPort, err := listenWithFallback(cfg.Port, 50)
	if err != nil {
		return err
	}
	if boundPort != cfg.Port {
		log.Printf("port %d is in use — using %d instead", cfg.Port, boundPort)
	}
	cfg.Port = boundPort

	log.Printf("matrix-runtime %s (commit %s, built %s) starting: mode=%s runtime_id=%s addr=:%d data_dir=%s",
		config.Version, config.Commit, config.Date, cfg.Mode, cfg.EffectiveRuntimeID(), boundPort, cfg.DataDir)
	log.Printf("console + API ready: http://localhost:%d", boundPort)

	srv := api.NewServer(cfg, mgr, st)
	if err := srv.Serve(ctx, ln); err != nil && err.Error() != "http: Server closed" {
		return err
	}
	log.Printf("matrix-runtime stopped")
	return nil
}

// listenWithFallback binds the first free TCP port in [start, start+maxTries),
// returning the listener and the port it bound. A port already in use is
// skipped; other errors (e.g. permission) are returned.
func listenWithFallback(start, maxTries int) (net.Listener, int, error) {
	var lastErr error
	for p := start; p < start+maxTries && p <= 65535; p++ {
		ln, err := net.Listen("tcp", ":"+strconv.Itoa(p))
		if err == nil {
			return ln, p, nil
		}
		lastErr = err
		if errors.Is(err, syscall.EADDRINUSE) {
			continue
		}
		// Non "in use" error on the very first try is fatal; otherwise keep trying.
		if p == start {
			return nil, 0, err
		}
	}
	return nil, 0, fmt.Errorf("no free port in range %d-%d: %w", start, start+maxTries-1, lastErr)
}

func runJoin(args []string) error {
	fs := flag.NewFlagSet("join", flag.ExitOnError)
	cloudURL := fs.String("cloud-url", "", "MatrixHub Cloud URL")
	token := fs.String("token", "", "runtime join token")
	runtimeID := fs.String("runtime-id", "", "optional runtime id")
	workspace := fs.String("workspace", "", "optional workspace")
	if err := fs.Parse(args); err != nil {
		return err
	}
	path, err := controlplane.WriteJoinConfig(controlplane.JoinConfig{
		CloudURL:  *cloudURL,
		JoinToken: *token,
		RuntimeID: *runtimeID,
		Workspace: *workspace,
	})
	if err != nil {
		return err
	}
	fmt.Printf("Wrote join configuration to %s\n", path)
	return nil
}
