// Package auth does password hashing and signed login tokens using only the
// Go standard library.
//
// Note for production: most teams use well-tested libraries for this
// (golang.org/x/crypto/bcrypt or argon2, and golang-jwt/jwt). It is written
// by hand here so the whole flow is easy to read and has no extra dependencies.
package auth

import (
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const pbkdf2Iterations = 600_000 // OWASP's recommendation for PBKDF2-HMAC-SHA256

var enc = base64.RawURLEncoding

// HashPassword returns "pbkdf2-sha256$<iterations>$<salt>$<hash>".
// A fresh random salt means two users with the same password get different hashes.
func HashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, pbkdf2Iterations, 32)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("pbkdf2-sha256$%d$%s$%s", pbkdf2Iterations, enc.EncodeToString(salt), enc.EncodeToString(key)), nil
}

// CheckPassword re-derives the key from the stored salt and compares in constant time.
func CheckPassword(password, stored string) bool {
	parts := strings.Split(stored, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha256" {
		return false
	}
	iter, err := strconv.Atoi(parts[1])
	if err != nil || iter <= 0 {
		return false
	}
	salt, err1 := enc.DecodeString(parts[2])
	want, err2 := enc.DecodeString(parts[3])
	if err1 != nil || err2 != nil {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, password, salt, iter, len(want))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(got, want) == 1
}

// ---- tokens (JWT, HS256) ----

type claims struct {
	Sub int64 `json:"sub"` // user id
	Exp int64 `json:"exp"` // unix seconds
}

var ErrInvalidToken = errors.New("invalid or expired token")

const jwtHeader = `{"alg":"HS256","typ":"JWT"}`

func sign(secret []byte, signingInput string) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(signingInput))
	return enc.EncodeToString(mac.Sum(nil))
}

// NewToken makes a token for userID that is valid for ttl.
func NewToken(secret []byte, userID int64, ttl time.Duration) (string, error) {
	payload, err := json.Marshal(claims{Sub: userID, Exp: time.Now().Add(ttl).Unix()})
	if err != nil {
		return "", err
	}
	input := enc.EncodeToString([]byte(jwtHeader)) + "." + enc.EncodeToString(payload)
	return input + "." + sign(secret, input), nil
}

// ParseToken checks the signature and expiry and returns the user id.
func ParseToken(secret []byte, token string) (int64, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return 0, ErrInvalidToken
	}
	header, err := enc.DecodeString(parts[0])
	if err != nil || string(header) != jwtHeader { // only accept the algorithm we issue
		return 0, ErrInvalidToken
	}
	gotSig, err := enc.DecodeString(parts[2])
	if err != nil {
		return 0, ErrInvalidToken
	}
	wantSig, _ := enc.DecodeString(sign(secret, parts[0]+"."+parts[1]))
	if !hmac.Equal(gotSig, wantSig) {
		return 0, ErrInvalidToken
	}
	raw, err := enc.DecodeString(parts[1])
	if err != nil {
		return 0, ErrInvalidToken
	}
	var c claims
	if err := json.Unmarshal(raw, &c); err != nil || c.Sub == 0 || time.Now().Unix() >= c.Exp {
		return 0, ErrInvalidToken
	}
	return c.Sub, nil
}
