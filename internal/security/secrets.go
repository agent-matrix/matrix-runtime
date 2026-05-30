package security

import (
	"fmt"
	"regexp"
	"strings"
)

// secretRef matches a secret reference such as ${secret:db_password} or
// secret://workspace/key. References are allowed; raw values are not.
var secretRef = regexp.MustCompile(`^\s*(\$\{secret:[^}]+\}|secret://\S+)\s*$`)

// looksLikeSecretKey flags environment-style keys whose name implies they
// carry sensitive material.
var looksLikeSecretKey = regexp.MustCompile(`(?i)(secret|token|password|passwd|api[_-]?key|private[_-]?key|credential)`)

// CheckNoRawSecrets rejects any env entry that appears to carry a raw secret
// value. For the MVP, mcp.test must never receive real user secrets; only
// secret references are permitted (and only for future mcp.run/agent.run).
func CheckNoRawSecrets(env map[string]string) error {
	for k, v := range env {
		if !looksLikeSecretKey.MatchString(k) {
			continue
		}
		if v == "" {
			continue
		}
		if IsSecretRef(v) {
			continue
		}
		return fmt.Errorf("env %q appears to contain a raw secret value; pass a secret reference instead", k)
	}
	return nil
}

// IsSecretRef reports whether v is a secret reference rather than a raw value.
func IsSecretRef(v string) bool {
	return secretRef.MatchString(strings.TrimSpace(v))
}
