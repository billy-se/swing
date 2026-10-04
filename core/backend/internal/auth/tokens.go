package auth

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func ParseRefreshToken(tokenString string, secretKey string) (jwt.MapClaims, error) {
	if secretKey == "" {
		return nil, errors.New("[parseRefreshToken]: JWT_REFRESH environment variable is missing")
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(secretKey), nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, fmt.Errorf("invalid or expired refresh token")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}
	return claims, nil
}

func (h *Handler) generateAccessToken(userID int, userName string) (string, error) {
	if h.JwtAccessSecret == "" {
		return "", errors.New("JWT_ACCESS environment variable is missing")
	}

	claims := jwt.MapClaims{
		"username": userName,
		"user_id":  userID,
		"type":     "access",
		"exp":      time.Now().Add(time.Minute * 15).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.JwtAccessSecret))
}

func (h *Handler) generateRefreshToken(userID int, userName string) (string, error) {
	if h.JwtRefreshSecret == "" {
		return "", errors.New("JWT_REFRESH environment variable is missing")
	}

	fmt.Printf("[DEBUG] generateAccessToken using secret (len: %d, start: %s)\n", len(h.JwtAccessSecret), h.JwtAccessSecret[:3])

	claims := jwt.MapClaims{
		"username": userName,
		"user_id":  userID,
		"type":     "refresh",
		"exp":      time.Now().Add(time.Hour * 24 * 7).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.JwtRefreshSecret))
}

func generateViewerAccessToken(username string) (string, error) {
	jwtAccessSigningKey := os.Getenv("JWT_ACCESS")
	if jwtAccessSigningKey == "" {
		return "", errors.New("JWT_ACCESS environment variable is missing")
	}

	claims := jwt.MapClaims{
		"user_id":  0,
		"username": username,
		"role":     "viewer",
		"type":     "access",
		"exp":      time.Now().Add(time.Hour * 24).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(jwtAccessSigningKey))
}
