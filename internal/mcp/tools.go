package mcp

import (
	"context"
	"encoding/json"
)

// Initialize performs the MCP initialize handshake and sends the initialized
// notification. It returns the raw server result for diagnostics.
func (c *Client) Initialize(ctx context.Context) (json.RawMessage, error) {
	res, err := c.call(ctx, "initialize", initializeParams{
		ProtocolVersion: ProtocolVersion,
		Capabilities:    map[string]any{},
		ClientInfo:      clientInfo{Name: "matrix-runtime", Version: "0.1.0"},
	})
	if err != nil {
		return nil, err
	}
	// Per the spec, the client sends an initialized notification after the
	// initialize response. Errors here are non-fatal for tools/list.
	_ = c.notify("notifications/initialized", map[string]any{})
	return res, nil
}

// ListTools requests the server's tool catalogue.
func (c *Client) ListTools(ctx context.Context) ([]Tool, error) {
	res, err := c.call(ctx, "tools/list", map[string]any{})
	if err != nil {
		return nil, err
	}
	var parsed toolsListResult
	if err := json.Unmarshal(res, &parsed); err != nil {
		return nil, err
	}
	tools := make([]Tool, 0, len(parsed.Tools))
	for _, t := range parsed.Tools {
		tools = append(tools, Tool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		})
	}
	return tools, nil
}

// CallTool invokes a tool with the given arguments and returns the raw result.
func (c *Client) CallTool(ctx context.Context, name string, args map[string]any) (json.RawMessage, error) {
	if args == nil {
		args = map[string]any{}
	}
	return c.call(ctx, "tools/call", map[string]any{
		"name":      name,
		"arguments": args,
	})
}
