// Package auth: session JWT (HS256, 12h) and password hashing for kica.
package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const (
	// CookieName is the httpOnly session cookie per docs/architecture.md.
	CookieName = "kica_session"
	// SessionTTL is the JWT lifetime; login refreshes it.
	SessionTTL = 12 * time.Hour
)

var ErrInvalidToken = errors.New("invalid token")

// SignToken returns an HS256 JWT with sub=user id, exp=now+12h.
func SignToken(secret, userID string) (string, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Subject:   userID,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(SessionTTL)),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

// ParseToken verifies signature+expiry and returns the subject (user id).
func ParseToken(secret, token string) (string, error) {
	t, err := jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil || !t.Valid {
		return "", ErrInvalidToken
	}
	sub, err := t.Claims.GetSubject()
	if err != nil || sub == "" {
		return "", ErrInvalidToken
	}
	return sub, nil
}

// HashPassword bcrypts with the default cost.
func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	return string(b), err
}

// CheckPassword compares a bcrypt hash.
func CheckPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}
