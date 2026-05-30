package hf

import (
	"fmt"
	"strings"
)

// ParseRef parses a model identifier into a Ref. Accepted forms:
//
//	hf:Qwen/Qwen2.5-7B-Instruct
//	hf:mistralai/Mistral-7B-Instruct-v0.3
//	Qwen/Qwen2.5-7B-Instruct        (hf: prefix optional)
//
// A revision override (e.g. a branch, tag or commit) may be supplied; when
// empty it defaults to "main". A revision embedded in the id via "@" takes
// precedence only when no explicit override is given.
func ParseRef(model, revisionOverride string) (Ref, error) {
	id := strings.TrimSpace(model)
	if id == "" {
		return Ref{}, fmt.Errorf("empty model identifier")
	}
	id = strings.TrimPrefix(id, "hf:")
	id = strings.TrimPrefix(id, "huggingface:")

	embeddedRev := ""
	if at := strings.LastIndex(id, "@"); at >= 0 {
		embeddedRev = id[at+1:]
		id = id[:at]
	}

	parts := strings.Split(id, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return Ref{}, fmt.Errorf("invalid model id %q (want namespace/name)", model)
	}

	rev := revisionOverride
	if rev == "" {
		rev = embeddedRev
	}
	if rev == "" {
		rev = "main"
	}

	return Ref{
		Source:    "huggingface",
		Namespace: parts[0],
		Name:      parts[1],
		Revision:  rev,
	}, nil
}
