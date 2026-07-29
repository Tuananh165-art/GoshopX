package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rasadov/EcommerceAPI/account/config"
)

type JWTCustomClaims struct {
	UserID uint64 `json:"user_id"`
	jwt.RegisteredClaims
}

func GenerateToken(userID uint64) (string, error) {
	claims := &JWTCustomClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    config.Issuer,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(config.SecretKey))
}

func ValidateToken(encodedToken string) (*jwt.Token, error) {
	token, err := jwt.ParseWithClaims(
		encodedToken,
		&JWTCustomClaims{},
		func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return []byte(config.SecretKey), nil
		},
	)
	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*JWTCustomClaims); ok && token.Valid {
		if claims.Issuer != config.Issuer {
			return nil, errors.New("invalid Issuer in token")
		}
		return token, nil
	}

	return nil, errors.New("invalid token claims")
}
