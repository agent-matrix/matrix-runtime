package mcp

import (
	"context"
	"encoding/json"
	"sync"
)

// Session is a live, initialised MCP server that can answer tools/list and
// tools/call requests for the duration of a sandbox. It is safe for concurrent
// use by API handlers while the owning job keeps it alive.
type Session struct {
	client *Client

	mu    sync.RWMutex
	tools []Tool
	ready bool
}

// NewSession wraps a connected client.
func NewSession(c *Client) *Session { return &Session{client: c} }

// Bootstrap initialises the server and caches its tool list.
func (s *Session) Bootstrap(ctx context.Context) ([]Tool, error) {
	if _, err := s.client.Initialize(ctx); err != nil {
		return nil, err
	}
	tools, err := s.client.ListTools(ctx)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.tools = tools
	s.ready = true
	s.mu.Unlock()
	return tools, nil
}

// Tools returns the cached tool list.
func (s *Session) Tools() []Tool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Tool, len(s.tools))
	copy(out, s.tools)
	return out
}

// Ready reports whether the session completed bootstrap.
func (s *Session) Ready() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ready
}

// CallTool proxies a tools/call to the live server.
func (s *Session) CallTool(ctx context.Context, name string, args map[string]any) (json.RawMessage, error) {
	return s.client.CallTool(ctx, name, args)
}

// Close terminates the underlying server process.
func (s *Session) Close() {
	if s.client != nil {
		s.client.Close()
	}
}
