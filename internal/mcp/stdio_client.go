package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
)

// Client is a JSON-RPC client speaking to an MCP server over the child
// process's stdin/stdout. A background reader dispatches responses to pending
// callers keyed by request id; server-initiated notifications are ignored.
type Client struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader

	mu      sync.Mutex
	nextID  int
	pending map[int]chan response

	encMu sync.Mutex // serialises writes to stdin

	closeOnce sync.Once
	readErr   error
}

// newClient wires a client to an already-started command's pipes and starts
// the reader loop.
func newClient(cmd *exec.Cmd, stdin io.WriteCloser, stdout io.Reader) *Client {
	c := &Client{
		cmd:     cmd,
		stdin:   stdin,
		stdout:  bufio.NewReaderSize(stdout, 1<<20),
		pending: make(map[int]chan response),
	}
	go c.readLoop()
	return c
}

func (c *Client) readLoop() {
	for {
		line, err := c.stdout.ReadBytes('\n')
		if len(line) > 0 {
			var msg response
			if jErr := json.Unmarshal(line, &msg); jErr == nil {
				c.dispatch(msg)
			}
		}
		if err != nil {
			c.mu.Lock()
			c.readErr = err
			for id, ch := range c.pending {
				close(ch)
				delete(c.pending, id)
			}
			c.mu.Unlock()
			return
		}
	}
}

func (c *Client) dispatch(msg response) {
	if msg.ID == nil {
		// Server-initiated notification or request: ignored for the MVP.
		return
	}
	c.mu.Lock()
	ch, ok := c.pending[*msg.ID]
	if ok {
		delete(c.pending, *msg.ID)
	}
	c.mu.Unlock()
	if ok {
		ch <- msg
		close(ch)
	}
}

// call sends a request and waits for the matching response or ctx expiry.
func (c *Client) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	c.mu.Lock()
	if c.readErr != nil {
		c.mu.Unlock()
		return nil, fmt.Errorf("mcp server closed: %w", c.readErr)
	}
	c.nextID++
	id := c.nextID
	ch := make(chan response, 1)
	c.pending[id] = ch
	c.mu.Unlock()

	if err := c.write(request{JSONRPC: "2.0", ID: &id, Method: method, Params: params}); err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, err
	}

	select {
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, fmt.Errorf("mcp %s timed out: %w", method, ctx.Err())
	case msg, ok := <-ch:
		if !ok {
			return nil, fmt.Errorf("mcp %s: server closed before responding", method)
		}
		if msg.Error != nil {
			return nil, fmt.Errorf("mcp %s error %d: %s", method, msg.Error.Code, msg.Error.Message)
		}
		return msg.Result, nil
	}
}

// notify sends a notification (no id, no response expected).
func (c *Client) notify(method string, params any) error {
	return c.write(request{JSONRPC: "2.0", Method: method, Params: params})
}

func (c *Client) write(r request) error {
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	c.encMu.Lock()
	defer c.encMu.Unlock()
	_, err = c.stdin.Write(b)
	return err
}

// Close terminates the underlying process and releases pipes.
func (c *Client) Close() {
	c.closeOnce.Do(func() {
		_ = c.stdin.Close()
		if c.cmd != nil && c.cmd.Process != nil {
			_ = c.cmd.Process.Kill()
			_ = c.cmd.Wait()
		}
	})
}
