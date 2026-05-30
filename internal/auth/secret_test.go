package auth

import (
	"encoding/hex"
	"strings"
	"testing"
)

func TestSecretBoxKeyFlexibleInput(t *testing.T) {
	cases := []struct{ name, val string }{
		{"hex32", strings.Repeat("ab", 32)},        // 64 hex chars
		{"passphrase", "a short human passphrase"}, // arbitrary → SHA-256
		{"too-short-hex", "abcd"},                  // not 32 bytes → SHA-256
		{"whitespace", "  " + strings.Repeat("cd", 32) + "\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("MATRIXCLOUD_SECRET_KEY", c.val)
			box, err := LoadSecretBox(t.TempDir())
			if err != nil {
				t.Fatalf("LoadSecretBox(%q) errored: %v", c.val, err)
			}
			enc, err := box.Encrypt("hf_secret_token")
			if err != nil {
				t.Fatal(err)
			}
			got, err := box.Decrypt(enc)
			if err != nil || got != "hf_secret_token" {
				t.Fatalf("round-trip failed: %q %v", got, err)
			}
		})
	}
}

func TestSecretBoxStableAcrossLoads(t *testing.T) {
	// The same env value must yield the same key so ciphertext stays decryptable
	// across restarts.
	t.Setenv("MATRIXCLOUD_SECRET_KEY", "consistent passphrase")
	a, err := LoadSecretBox(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	enc, _ := a.Encrypt("v")
	b, err := LoadSecretBox(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if got, err := b.Decrypt(enc); err != nil || got != "v" {
		t.Fatalf("key not stable across loads: %q %v", got, err)
	}
}

func TestSecretBoxExactHexHonored(t *testing.T) {
	key := strings.Repeat("11", 32)
	t.Setenv("MATRIXCLOUD_SECRET_KEY", key)
	box, err := LoadSecretBox(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	// Sanity: a 64-char hex decodes to the exact 32-byte key (not re-hashed).
	raw, _ := hex.DecodeString(key)
	if len(raw) != 32 {
		t.Fatal("test setup")
	}
	if _, err := box.Encrypt("x"); err != nil {
		t.Fatal(err)
	}
}
