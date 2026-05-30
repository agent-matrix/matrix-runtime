// Package security provides command validation and limit helpers for the
// runtime. The goal for the MVP is a "safe-enough" guard for verified MCP
// servers and short-lived sandboxes, not perfect isolation.
package security

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// allowedPrefixes are the only program names a start command may invoke.
var allowedPrefixes = map[string]bool{
	"npx":     true,
	"uvx":     true,
	"pipx":    true,
	"python":  true,
	"python3": true,
	"node":    true,
}

// blockedTokens are exact tokens that may never appear in a command.
var blockedTokens = map[string]bool{
	"sudo":      true,
	"docker":    true,
	"apt":       true,
	"apt-get":   true,
	"apk":       true,
	"yum":       true,
	"dnf":       true,
	"systemctl": true,
	"mkfs":      true,
	"mount":     true,
	"curl":      true,
	"wget":      true,
	"sh":        true,
	"bash":      true,
	"zsh":       true,
	"eval":      true,
}

// shellMetacharacters are forbidden anywhere in the raw command string. They
// enable chaining, redirects, command substitution or backgrounding.
const shellMetacharacters = "&|;<>`$()\n\r"

// ValidateCommand checks a raw start command and returns its tokens when
// accepted. It rejects shell chaining, redirects, backgrounding and any
// program outside the allow-list.
func ValidateCommand(raw string) ([]string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, fmt.Errorf("empty start command")
	}
	if i := strings.IndexAny(trimmed, shellMetacharacters); i >= 0 {
		return nil, fmt.Errorf("command contains forbidden shell metacharacter %q", trimmed[i])
	}
	tokens, err := SplitCommand(trimmed)
	if err != nil {
		return nil, err
	}
	if len(tokens) == 0 {
		return nil, fmt.Errorf("empty start command")
	}
	base := filepath.Base(tokens[0])
	if !allowedPrefixes[base] {
		return nil, fmt.Errorf("program %q is not allowed (permitted: npx, uvx, pipx, python, python3, node)", base)
	}
	for _, t := range tokens {
		if blockedTokens[filepath.Base(t)] {
			return nil, fmt.Errorf("command contains blocked token %q", t)
		}
	}
	return tokens, nil
}

// SplitCommand tokenises a command line, honouring single and double quotes.
// It is intentionally simple: shell metacharacters are rejected upstream, so
// no escaping, expansion or globbing is performed.
func SplitCommand(s string) ([]string, error) {
	var tokens []string
	var cur strings.Builder
	var quote rune
	inToken := false

	flush := func() {
		if inToken {
			tokens = append(tokens, cur.String())
			cur.Reset()
			inToken = false
		}
	}

	for _, r := range s {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
			inToken = true
		case r == ' ' || r == '\t':
			flush()
		default:
			inToken = true
			cur.WriteRune(r)
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("unbalanced quote in command")
	}
	flush()
	return tokens, nil
}

// AllowedPrograms returns the sorted list of permitted start-command programs.
func AllowedPrograms() []string {
	out := make([]string, 0, len(allowedPrefixes))
	for k := range allowedPrefixes {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// BlockedTokens returns the sorted list of tokens refused anywhere in a command.
func BlockedTokens() []string {
	out := make([]string, 0, len(blockedTokens))
	for k := range blockedTokens {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
