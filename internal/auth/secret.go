package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
)

// SecretBox encrypts small secrets (e.g. BYO provider tokens) at rest with
// AES-256-GCM. The key is loaded from MATRIXCLOUD_SECRET_KEY (hex or base64, 32
// bytes) or generated once and persisted to <dir>/secret.key (mode 0600).
type SecretBox struct {
	gcm cipher.AEAD
}

// LoadSecretBox resolves the encryption key and returns a SecretBox.
func LoadSecretBox(dir string) (*SecretBox, error) {
	key, err := loadOrCreateKey(dir)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &SecretBox{gcm: gcm}, nil
}

func loadOrCreateKey(dir string) ([]byte, error) {
	if env := os.Getenv("MATRIXCLOUD_SECRET_KEY"); env != "" {
		if b, err := hex.DecodeString(env); err == nil && len(b) == 32 {
			return b, nil
		}
		if b, err := base64.StdEncoding.DecodeString(env); err == nil && len(b) == 32 {
			return b, nil
		}
		return nil, errors.New("MATRIXCLOUD_SECRET_KEY must be 32 bytes (hex or base64)")
	}
	path := filepath.Join(dir, "secret.key")
	if b, err := os.ReadFile(path); err == nil {
		if raw, err := hex.DecodeString(string(b)); err == nil && len(raw) == 32 {
			return raw, nil
		}
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err == nil {
		_ = os.WriteFile(path, []byte(hex.EncodeToString(key)), 0o600)
	}
	return key, nil
}

// Encrypt returns base64(nonce||ciphertext) for the given plaintext.
func (s *SecretBox) Encrypt(plaintext string) (string, error) {
	nonce := make([]byte, s.gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	ct := s.gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ct), nil
}

// Decrypt reverses Encrypt.
func (s *SecretBox) Decrypt(enc string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		return "", err
	}
	ns := s.gcm.NonceSize()
	if len(raw) < ns {
		return "", errors.New("ciphertext too short")
	}
	pt, err := s.gcm.Open(nil, raw[:ns], raw[ns:], nil)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}

// Hint returns a non-secret hint (last 4 chars) for display.
func Hint(secret string) string {
	if len(secret) <= 4 {
		return "••••"
	}
	return "••••" + secret[len(secret)-4:]
}
