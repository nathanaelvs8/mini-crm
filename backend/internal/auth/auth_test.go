package auth

import (
	"strings"
	"testing"
	"time"
)

func TestPasswordHashAndCheck(t *testing.T) {
	h1, err := HashPassword("correct horse")
	if err != nil {
		t.Fatal(err)
	}
	h2, _ := HashPassword("correct horse")
	if h1 == h2 {
		t.Error("same password should produce different hashes (random salt)")
	}
	if !CheckPassword("correct horse", h1) {
		t.Error("right password rejected")
	}
	if CheckPassword("wrong horse", h1) {
		t.Error("wrong password accepted")
	}
	if CheckPassword("x", "not-a-hash") {
		t.Error("garbage hash accepted")
	}
}

func TestTokenRoundTripAndTampering(t *testing.T) {
	secret := []byte("secret-secret-secret")
	tok, err := NewToken(secret, 42, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	id, err := ParseToken(secret, tok)
	if err != nil || id != 42 {
		t.Fatalf("round trip: id=%d err=%v", id, err)
	}
	if _, err := ParseToken([]byte("other-secret-other"), tok); err == nil {
		t.Error("token signed with a different secret was accepted")
	}

	// change the payload (user id) but keep the old signature
	parts := strings.Split(tok, ".")
	forged, _ := NewToken(secret, 7, time.Hour)
	forgedParts := strings.Split(forged, ".")
	if _, err := ParseToken(secret, parts[0]+"."+forgedParts[1]+"."+parts[2]); err == nil {
		t.Error("token with swapped payload was accepted")
	}

	expired, _ := NewToken(secret, 42, -time.Minute)
	if _, err := ParseToken(secret, expired); err == nil {
		t.Error("expired token was accepted")
	}
	if _, err := ParseToken(secret, "a.b"); err == nil {
		t.Error("malformed token was accepted")
	}
}
