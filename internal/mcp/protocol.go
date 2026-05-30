// Package mcp implements a minimal Model Context Protocol client over stdio.
//
// It speaks newline-delimited JSON-RPC 2.0 as used by the MCP stdio transport:
// initialize, the initialized notification, tools/list and tools/call.
package mcp

import "encoding/json"

// ProtocolVersion is the MCP protocol version advertised by the client.
const ProtocolVersion = "2024-11-05"

// request is an outgoing JSON-RPC request or notification (no id => notification).
type request struct {
	JSONRPC string `json:"jsonrpc"`
	ID      *int   `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// response is an incoming JSON-RPC message.
type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int            `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
	Method  string          `json:"method,omitempty"` // server-initiated requests/notifications
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *rpcError) Error() string { return e.Message }

// clientInfo identifies this client during initialize.
type clientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type initializeParams struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Capabilities    map[string]any `json:"capabilities"`
	ClientInfo      clientInfo     `json:"clientInfo"`
}

// Tool is a tool advertised by an MCP server.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema,omitempty"`
}

// toolsListResult is the result of a tools/list call. MCP servers use
// "inputSchema" (camelCase); we map it to the public InputSchema field.
type toolsListResult struct {
	Tools []struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		InputSchema json.RawMessage `json:"inputSchema"`
	} `json:"tools"`
}
