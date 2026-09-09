// Package github: PAT encryption at rest (AES-256-GCM, key = GH_ENC_KEY env,
// 32 bytes base64) and the create-issue REST client (10s timeout, one retry
// on 5xx). Tokens never appear in errors or logs.
package github

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

// ParseEncKey decodes the GH_ENC_KEY env value (32 bytes base64 → AES-256).
// Empty input yields a nil key (feature off); anything else that doesn't
// decode to 32 bytes is an error.
func ParseEncKey(s string) (*[32]byte, error) {
	if s == "" {
		return nil, nil
	}
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("not valid base64")
	}
	if len(raw) != 32 {
		return nil, fmt.Errorf("decoded %d bytes, want 32", len(raw))
	}
	var key [32]byte
	copy(key[:], raw)
	return &key, nil
}

func newGCM(key *[32]byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// EncryptToken seals the PAT as nonce || ciphertext+tag.
func EncryptToken(key *[32]byte, token string) ([]byte, error) {
	if key == nil {
		return nil, errors.New("GH_ENC_KEY is not configured")
	}
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, []byte(token), nil), nil
}

// DecryptToken opens a nonce-prefixed ciphertext.
func DecryptToken(key *[32]byte, ct []byte) (string, error) {
	if key == nil {
		return "", errors.New("GH_ENC_KEY is not configured")
	}
	gcm, err := newGCM(key)
	if err != nil {
		return "", err
	}
	if len(ct) < gcm.NonceSize()+gcm.Overhead() {
		return "", errors.New("ciphertext too short")
	}
	pt, err := gcm.Open(nil, ct[:gcm.NonceSize()], ct[gcm.NonceSize():], nil)
	if err != nil {
		return "", errors.New("ciphertext authentication failed")
	}
	return string(pt), nil
}
