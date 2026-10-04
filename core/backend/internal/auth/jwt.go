package auth

import (
	"errors"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
)

func ParseToken(tokenString string, secretKey string) (jwt.MapClaims, error) {
	if secretKey == "" {
		return nil, errors.New("[ParseToken]: JWT_ACCESS environment variable is missing")
	}

	//fmt.Printf("[DEBUG] ParseToken using secret (len: %d, start: %s)\n", len(h.JwtAccessSecret), h.JwtAccessSecret[:3])

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(secretKey), nil
	})
	if err != nil {
		fmt.Println("[ParseToken]: JWT Parse Error Details:", err)
		return nil, err
	}
	if !token.Valid {
		return nil, fmt.Errorf("invalid or expired token")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}
	/*userIDFloat, ok := claims["user_id"].(float64)
	if !ok {
		return nil, fmt.Errorf("invalid user ID in token")
	}*/
	return claims, nil
}