package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/baihua19941101/cdnManage/internal/config"
)

type TokenManager struct {
	config config.JWTConfig
}

type Claims struct {
	UserID       uint64 `json:"userId"`
	PlatformRole string `json:"platformRole"`
	jwt.RegisteredClaims
}

func NewTokenManager(cfg config.JWTConfig) *TokenManager {
	return &TokenManager{config: cfg}
}

func (m *TokenManager) Generate(userID uint64, platformRole string) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:       userID,
		PlatformRole: platformRole,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.config.Issuer,
			Subject:   fmt.Sprintf("%d", userID),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Duration(m.config.LifespanSeconds) * time.Second)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(m.config.Secret))
}

func (m *TokenManager) Parse(tokenString string) (*Claims, error) {
	parsed, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(m.config.Secret), nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := parsed.Claims.(*Claims)
	if !ok || !parsed.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return claims, nil
}
