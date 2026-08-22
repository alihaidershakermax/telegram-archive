package factory

import "testing"

func TestTokenCipherRoundTrip(t *testing.T) {
	cipher, err := NewTokenCipher("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("new cipher: %v", err)
	}
	ciphertext, nonce, err := cipher.Encrypt("123456:ABC_def-123")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if ciphertext == "123456:ABC_def-123" || nonce == "" {
		t.Fatal("token was not encrypted")
	}
	got, err := cipher.Decrypt(ciphertext, nonce)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if got != "123456:ABC_def-123" {
		t.Fatalf("unexpected plaintext %q", got)
	}
}

func TestTokenCipherRejectsInvalidKey(t *testing.T) {
	if _, err := NewTokenCipher("too-short"); err == nil {
		t.Fatal("expected invalid key error")
	}
}

func TestValidateTokenShape(t *testing.T) {
	valid := "123456:ABC_def-12345678901234567890"
	if err := validateTokenShape(valid); err != nil {
		t.Fatalf("valid token rejected: %v", err)
	}
	for _, token := range []string{"", "no-colon", "123:bad space"} {
		if err := validateTokenShape(token); err == nil {
			t.Fatalf("expected token %q to be rejected", token)
		}
	}
}
