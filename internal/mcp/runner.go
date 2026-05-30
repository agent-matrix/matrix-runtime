package mcp

import (
	"context"
	"fmt"
	"io"
	"os/exec"
)

// StartOptions configure launching an MCP server process.
type StartOptions struct {
	Args    []string  // validated command tokens; Args[0] is the program
	WorkDir string    // temporary working directory
	Env     []string  // process environment (os.Environ() style)
	Stderr  io.Writer // captures the server's stderr for logs
}

// Start launches the MCP server process over stdio and returns a connected
// Client. The process is bound to ctx: cancelling ctx kills the process.
func Start(ctx context.Context, opts StartOptions) (*Client, error) {
	if len(opts.Args) == 0 {
		return nil, fmt.Errorf("no command to start")
	}
	cmd := exec.CommandContext(ctx, opts.Args[0], opts.Args[1:]...)
	cmd.Dir = opts.WorkDir
	cmd.Env = opts.Env

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	if opts.Stderr != nil {
		cmd.Stderr = opts.Stderr
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start mcp server: %w", err)
	}
	return newClient(cmd, stdin, stdout), nil
}
