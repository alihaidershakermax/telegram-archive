package factory

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
)

// TokenCipher encrypts managed bot tokens with AES-256-GCM.
type TokenCipher struct {
	key []byte
}

// NewTokenCipher accepts a 64-character hex key, a base64-encoded 32-byte key,
// or a raw 32-byte key. Ambiguous or weak keys are rejected.
func NewTokenCipher(raw string) (*TokenCipher, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("factory encryption key is empty")
	}

	var key []byte
	if len(raw) == 64 {
		decoded, err := hex.DecodeString(raw)
		if err == nil {
			key = decoded
		}
	}
	if len(key) == 0 {
		if decoded, err := base64.StdEncoding.DecodeString(raw); err == nil && len(decoded) == 32 {
			key = decoded
		}
	}
	if len(key) == 0 && len(raw) == 32 {
		key = []byte(raw)
	}
	if len(key) != 32 {
		return nil, errors.New("factory encryption key must decode to exactly 32 bytes")
	}
	return &TokenCipher{key: append([]byte(nil), key...)}, nil
}

func (c *TokenCipher) aead() (cipher.AEAD, error) {
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// Encrypt returns base64 ciphertext and nonce, suitable for MongoDB fields.
func (c *TokenCipher) Encrypt(plaintext string) (ciphertext, nonce string, err error) {
	aead, err := c.aead()
	if err != nil {
		return "", "", err
	}
	nonceBytes := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonceBytes); err != nil {
		return "", "", fmt.Errorf("generate nonce: %w", err)
	}
	sealed := aead.Seal(nil, nonceBytes, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(sealed), base64.StdEncoding.EncodeToString(nonceBytes), nil
}

// Decrypt reverses Encrypt.
func (c *TokenCipher) Decrypt(ciphertext, nonce string) (string, error) {
	aead, err := c.aead()
	if err != nil {
		return "", err
	}
	sealed, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", errors.New("invalid ciphertext encoding")
	}
	nonceBytes, err := base64.StdEncoding.DecodeString(nonce)
	if err != nil || len(nonceBytes) != aead.NonceSize() {
		return "", errors.New("invalid nonce encoding")
	}
	plaintext, err := aead.Open(nil, nonceBytes, sealed, nil)
	if err != nil {
		return "", errors.New("token decryption failed")
	}
	return string(plaintext), nil
}

// TokenHash creates a stable non-reversible identifier for duplicate checks.
func TokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func validateTokenShape(token string) error {
	token = strings.TrimSpace(token)
	if len(token) < 20 || len(token) > 256 {
		return errors.New("token length is invalid")
	}
	colon := strings.IndexByte(token, ':')
	if colon <= 0 || colon == len(token)-1 {
		return errors.New("token format is invalid")
	}
	for _, r := range token[colon+1:] {
		if !(r >= 'A' && r <= 'Z') && !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9') && r != '_' && r != '-' {
			return errors.New("token contains unsupported characters")
		}
	}
	return nil
}
