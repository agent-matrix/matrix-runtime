package hf

import "net/http"

// applyAuth attaches a bearer token to the request when a token is configured.
func applyAuth(req *http.Request, token string) {
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
}
