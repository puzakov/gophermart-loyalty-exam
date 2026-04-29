package auth

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type TokenManager struct {
	secret    []byte
	accessTTL time.Duration
}

type AccessClaims struct {
	jwt.RegisteredClaims
}

func NewTokenManager(secret string, accessTTL time.Duration) (*TokenManager, error) {
	if secret == "" {
		return nil, errors.New("JWT_SECRET is required")
	}
	return &TokenManager{
		secret:    []byte(secret),
		accessTTL: accessTTL,
	}, nil
}

func (m *TokenManager) NewAccessToken(userID int64, now time.Time) (string, time.Time, error) {
	exp := now.Add(m.accessTTL)
	jti, err := randomTokenID()
	if err != nil {
		return "", time.Time{}, err
	}
	claims := AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   itoa64(userID),
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}

	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := tok.SignedString(m.secret)
	return s, exp, err
}

func randomTokenID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (m *TokenManager) ParseAccessToken(tokenString string) (userID int64, err error) {
	tok, err := jwt.ParseWithClaims(tokenString, &AccessClaims{}, func(token *jwt.Token) (any, error) {
		return m.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Name}))
	if err != nil {
		return 0, err
	}
	claims, ok := tok.Claims.(*AccessClaims)
	if !ok || !tok.Valid {
		return 0, errors.New("invalid token")
	}
	return atoi64(claims.Subject)
}
