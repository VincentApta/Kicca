package github

import (
	"bytes"
	"encoding/base64"
	"testing"
)

func TestEncKeyRoundTrip(t *testing.T) {
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = byte(i)
	}
	key, err := ParseEncKey(base64.StdEncoding.EncodeToString(raw))
	if err != nil || key == nil {
		t.Fatalf("ParseEncKey: %v %v", key, err)
	}
	ct, err := EncryptToken(key, "ghp_tok")
	if err != nil {
		t.Fatalf("EncryptToken: %v", err)
	}
	if bytes.Contains(ct, []byte("ghp_tok")) {
		t.Fatal("ciphertext contains plaintext token")
	}
	got, err := DecryptToken(key, ct)
	if err != nil || got != "ghp_tok" {
		t.Fatalf("DecryptToken: %q %v", got, err)
	}
	// wrong key fails closed
	other := ParseEncKeyMust(t)
	if _, err := DecryptToken(other, ct); err == nil {
		t.Fatal("decrypt with wrong key succeeded")
	}
}

func ParseEncKeyMust(t *testing.T) *[32]byte {
	t.Helper()
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = byte(255 - i)
	}
	key, err := ParseEncKey(base64.StdEncoding.EncodeToString(raw))
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func TestParseEncKeyRejectsBadInput(t *testing.T) {
	if k, err := ParseEncKey(""); k != nil || err != nil {
		t.Fatalf("empty: %v %v", k, err)
	}
	if _, err := ParseEncKey("not-base64!!"); err == nil {
		t.Fatal("bad base64 accepted")
	}
	if _, err := ParseEncKey(base64.StdEncoding.EncodeToString([]byte("short"))); err == nil {
		t.Fatal("wrong length accepted")
	}
}
