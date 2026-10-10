package encrypt

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"testing"
)

func newTestKey(t *testing.T) string {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("failed to generate test key: %v", err)
	}
	return base64.StdEncoding.EncodeToString(key)
}

func newTestEncryptor(t *testing.T) *Encryptor {
	t.Helper()
	e, err := New(newTestKey(t))
	if err != nil {
		t.Fatalf("failed to create encryptor: %v", err)
	}
	return e
}

func TestNew_InvalidBase64(t *testing.T) {
	if _, err := New("not-valid-base64!!!"); err == nil {
		t.Error("expected error for non-base64 key")
	}
}

func TestNew_InvalidKeyLength(t *testing.T) {
	short := base64.StdEncoding.EncodeToString([]byte("too-short"))
	if _, err := New(short); err == nil {
		t.Error("expected error for key shorter than 32 bytes")
	}
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	e := newTestEncryptor(t)

	plaintext := []byte(`{"access_key_id":"AKIATEST","secret_access_key":"hidden","region":"us-east-1"}`)
	ciphertext, err := e.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("failed to encrypt: %v", err)
	}
	if bytes.Equal(ciphertext, plaintext) {
		t.Error("ciphertext should differ from plaintext")
	}

	decrypted, err := e.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("failed to decrypt: %v", err)
	}
	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("round-trip mismatch: got %q, want %q", decrypted, plaintext)
	}
}

func TestEncrypt_RoundTripEmptyPlaintext(t *testing.T) {
	e := newTestEncryptor(t)

	ciphertext, err := e.Encrypt([]byte{})
	if err != nil {
		t.Fatalf("failed to encrypt empty plaintext: %v", err)
	}

	decrypted, err := e.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("failed to decrypt empty plaintext: %v", err)
	}
	if len(decrypted) != 0 {
		t.Errorf("expected empty plaintext, got %q", decrypted)
	}
}

func TestEncrypt_UsesRandomNonce(t *testing.T) {
	e := newTestEncryptor(t)

	plaintext := []byte("same input")
	first, err := e.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("failed to encrypt: %v", err)
	}
	second, err := e.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("failed to encrypt: %v", err)
	}

	if bytes.Equal(first, second) {
		t.Error("encrypting the same plaintext twice should produce different ciphertexts")
	}
}

func TestDecrypt_TamperedCiphertext(t *testing.T) {
	e := newTestEncryptor(t)

	ciphertext, err := e.Encrypt([]byte("sensitive data"))
	if err != nil {
		t.Fatalf("failed to encrypt: %v", err)
	}

	// Flip a byte in the middle of the ciphertext (after the nonce).
	tampered := make([]byte, len(ciphertext))
	copy(tampered, ciphertext)
	tampered[len(tampered)/2] ^= 0xFF

	if _, err := e.Decrypt(tampered); err == nil {
		t.Error("expected error when decrypting tampered ciphertext")
	}
}

func TestDecrypt_WrongKey(t *testing.T) {
	first, err := New(newTestKey(t))
	if err != nil {
		t.Fatalf("failed to create encryptor: %v", err)
	}
	second, err := New(newTestKey(t))
	if err != nil {
		t.Fatalf("failed to create encryptor: %v", err)
	}

	ciphertext, err := first.Encrypt([]byte("sensitive data"))
	if err != nil {
		t.Fatalf("failed to encrypt: %v", err)
	}

	if _, err := second.Decrypt(ciphertext); err == nil {
		t.Error("expected error when decrypting with wrong key")
	}
}

func TestDecrypt_CiphertextTooShort(t *testing.T) {
	e := newTestEncryptor(t)

	if _, err := e.Decrypt([]byte("short")); err == nil {
		t.Error("expected error for ciphertext shorter than the nonce size")
	}
}
