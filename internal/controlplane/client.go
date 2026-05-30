// Package controlplane holds the (mostly designed, partly stubbed) outbound
// connection from matrix-runtime to MatrixHub Cloud. In the hybrid model the
// runtime lives in customer infrastructure and dials out to the cloud control
// plane, so no inbound firewall exposure is required.
package controlplane

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Client talks to MatrixHub Cloud over outbound HTTPS.
type Client struct {
	CloudURL  string
	JoinToken string
	RuntimeID string
	Workspace string
	HTTP      *http.Client
}

// New builds a control-plane client.
func New(cloudURL, joinToken, runtimeID, workspace string) *Client {
	return &Client{
		CloudURL:  strings.TrimRight(cloudURL, "/"),
		JoinToken: joinToken,
		RuntimeID: runtimeID,
		Workspace: workspace,
		HTTP:      &http.Client{Timeout: 30 * time.Second},
	}
}

// Heartbeat reports liveness to the cloud. Stubbed: it performs a single
// authenticated POST to /v1/runtimes/heartbeat and tolerates absence of the
// endpoint, since the MVP runs primarily via the direct HTTP API.
func (c *Client) Heartbeat(ctx context.Context, capabilities any) error {
	if c.CloudURL == "" || c.JoinToken == "" {
		return fmt.Errorf("control plane not configured")
	}
	body, _ := json.Marshal(map[string]any{
		"runtime_id":   c.RuntimeID,
		"workspace":    c.Workspace,
		"capabilities": capabilities,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.CloudURL+"/v1/runtimes/heartbeat", strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.JoinToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("heartbeat rejected: status %d", resp.StatusCode)
	}
	return nil
}
