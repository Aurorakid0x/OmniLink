package myjwt

import (
	"OmniLink/internal/config"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type CustomClaims struct {
	Uuid     string `json:"uuid"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

func GenerateToken(uuid string, username string) (string, error) {
	conf := config.GetConfig()
	key := conf.JwtConfig.Key
	if key == "" {
		return "", errors.New("jwt key is empty")
	}

	expireHours := conf.JwtConfig.ExpireHours
	if expireHours <= 0 {
		expireHours = 24
	}

	issuer := conf.JwtConfig.Issuer
	if issuer == "" {
		issuer = conf.MainConfig.AppName
	}

	claims := CustomClaims{
		Uuid:     uuid,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(expireHours) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    issuer,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(key))
}

func ParseToken(tokenString string) (*CustomClaims, error) {
	conf := config.GetConfig()
	key := conf.JwtConfig.Key
	if key == "" {
		return nil, errors.New("jwt key is empty")
	}

	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(key), nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*CustomClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}
