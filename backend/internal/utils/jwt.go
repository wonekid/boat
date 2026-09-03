package utils

import (
	"errors"
	"time"

	"boat/internal/config"

	"github.com/golang-jwt/jwt/v5"
)

// Claims JWT 载荷
type Claims struct {
	UserID   uint   `json:"userId"`
	Username string `json:"username"`
	Mfa      bool   `json:"mfa"` // true 表示仅完成密码、待 MFA 校验的临时令牌
	jwt.RegisteredClaims
}

// GenerateToken 签发 Token
func GenerateToken(userID uint, username string) (string, error) {
	cfg := config.Global.JWT
	expire := time.Duration(cfg.ExpireHours) * time.Hour
	claims := Claims{
		UserID:   userID,
		Username: username,
		Mfa:      false,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expire)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "boat-ops",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.Secret))
}

// GenerateMFAToken 签发仅完成密码、待 MFA 校验的短期临时令牌（5 分钟有效）
func GenerateMFAToken(userID uint, username string) (string, error) {
	cfg := config.Global.JWT
	claims := Claims{
		UserID:   userID,
		Username: username,
		Mfa:      true,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "boat-ops",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.Secret))
}

// ParseToken 解析 Token
func ParseToken(tokenStr string) (*Claims, error) {
	cfg := config.Global.JWT
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("非预期签名算法")
		}
		return []byte(cfg.Secret), nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	return nil, errors.New("无效令牌")
}
