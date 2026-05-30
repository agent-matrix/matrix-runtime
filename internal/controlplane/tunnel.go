package controlplane

import (
	"context"
	"errors"
)

// ErrTunnelNotImplemented marks the future control channel as unavailable.
var ErrTunnelNotImplemented = errors.New("control-plane tunnel not implemented in this build")

// Tunnel is the future outbound control channel: the runtime opens an outbound
// WebSocket/HTTPS connection to matrix-hub, which pushes jobs; the runtime
// executes them and streams logs/results back. Designed now, stubbed for the
// MVP where the direct HTTP API is sufficient.
type Tunnel struct {
	client *Client
}

// NewTunnel constructs a tunnel bound to a control-plane client.
func NewTunnel(c *Client) *Tunnel { return &Tunnel{client: c} }

// Run would maintain the outbound control channel. Stubbed for the MVP.
func (t *Tunnel) Run(_ context.Context) error {
	return ErrTunnelNotImplemented
}
