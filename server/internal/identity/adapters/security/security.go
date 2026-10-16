// Package security implements password hashing (bcrypt) and session-bound
// access tokens (HS256 JWT carrying the session ID).
package security

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type Bcrypt struct{ Cost int }

func (b Bcrypt) Hash(pw string) (string, error) {
	cost := b.Cost
	if cost == 0 {
		cost = 12
	}
	h, err := bcrypt.GenerateFromPassword([]byte(pw), cost)
	return string(h), err
}

func (Bcrypt) Compare(hash, pw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}

const issuer = "paytrack"

type JWT struct {
	Secret []byte
	TTL    time.Duration
}

type claims struct {
	TenantID  string `json:"tid"`
	SessionID string `json:"sid"`
	jwt.RegisteredClaims
}

func (j JWT) Issue(userID, tenantID, sessionID string) (string, time.Time, error) {
	exp := time.Now().Add(j.TTL)
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims{
		TenantID: tenantID, SessionID: sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: userID, Issuer: issuer,
			IssuedAt: jwt.NewNumericDate(time.Now()), ExpiresAt: jwt.NewNumericDate(exp),
		},
	})
	s, err := tok.SignedString(j.Secret)
	return s, exp, err
}

func (j JWT) Verify(token string) (string, string, string, error) {
	var c claims
	_, err := jwt.ParseWithClaims(token, &c, func(*jwt.Token) (any, error) { return j.Secret, nil },
		jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer(issuer), jwt.WithExpirationRequired())
	if err != nil {
		return "", "", "", err
	}
	if c.Subject == "" || c.TenantID == "" || c.SessionID == "" {
		return "", "", "", errors.New("token missing subject, tenant or session")
	}
	return c.Subject, c.TenantID, c.SessionID, nil
}
