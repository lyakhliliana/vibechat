package jwt

import (
	"errors"
	"fmt"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"vibechat/internal/domain"
)

type claims struct {
	UserID uuid.UUID `json:"uid"`
	gojwt.RegisteredClaims
}

type JWTManager struct {
	cfg Config
}

func New(cfg Config) *JWTManager {
	return &JWTManager{cfg: cfg}
}

func (m *JWTManager) GenerateAccessToken(userID uuid.UUID) (string, error) {
	return m.sign(userID, m.cfg.AccessSecret, m.cfg.AccessTokenTTL.Duration)
}

func (m *JWTManager) GenerateRefreshToken(userID uuid.UUID) (string, error) {
	return m.sign(userID, m.cfg.RefreshSecret, m.cfg.RefreshTokenTTL.Duration)
}

func (m *JWTManager) ValidateAccessToken(token string) (uuid.UUID, error) {
	return m.parse(token, m.cfg.AccessSecret)
}

func (m *JWTManager) ValidateRefreshToken(token string) (uuid.UUID, error) {
	return m.parse(token, m.cfg.RefreshSecret)
}

func (m *JWTManager) sign(userID uuid.UUID, secret string, ttl time.Duration) (string, error) {
	c := &claims{
		UserID: userID,
		RegisteredClaims: gojwt.RegisteredClaims{
			ExpiresAt: gojwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  gojwt.NewNumericDate(time.Now()),
			ID:        uuid.New().String(),
		},
	}
	token, err := gojwt.NewWithClaims(gojwt.SigningMethodHS256, c).SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("jwt: sign: %w", err)
	}
	return token, nil
}

func (m *JWTManager) parse(tokenStr, secret string) (uuid.UUID, error) {
	token, err := gojwt.ParseWithClaims(tokenStr, &claims{}, func(t *gojwt.Token) (any, error) {
		if _, ok := t.Method.(*gojwt.SigningMethodHMAC); !ok {
			return nil, domain.ErrInvalidToken
		}
		return []byte(secret), nil
	})
	if err != nil {
		if errors.Is(err, gojwt.ErrTokenExpired) {
			return uuid.Nil, domain.ErrExpiredToken
		}
		return uuid.Nil, domain.ErrInvalidToken
	}

	c, ok := token.Claims.(*claims)
	if !ok || !token.Valid {
		return uuid.Nil, domain.ErrInvalidToken
	}
	return c.UserID, nil
}
