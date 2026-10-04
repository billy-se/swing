package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"vaine-backend/internal/auth"
	"github.com/redis/go-redis/v9"
)



type HandlerMiddleware struct {
	JwtAccessSecret string
	Redis            *redis.Client
}

func (hM *HandlerMiddleware) AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "[authMiddleware]: Missing authorization header", http.StatusUnauthorized)
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, "Invalid authorization header format", http.StatusUnauthorized)
			return
		}

		claims, err := auth.ParseToken(parts[1], hM.JwtAccessSecret)
		if err != nil {
			http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
			return
		}

		var userID int
		if userIDFloat, ok := claims["user_id"].(float64); ok {
			userID = int(userIDFloat)
		}

		ctx := context.WithValue(r.Context(), auth.UserIDKey, userID)
		ctx = context.WithValue(ctx, auth.ClaimsKey, claims)
		next(w, r.WithContext(ctx))
	}
}

func EnableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		} else {
			w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
		}
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, PATCH, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (h *HandlerMiddleware) refreshSessionMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userId := r.Context().Value(auth.UserIDKey).(int)
		activeSession := fmt.Sprintf("presence:user:%d", userId)

		h.Redis.Expire(context.Background(), activeSession, time.Second*30)

		next.ServeHTTP(w, r)
	}
}
