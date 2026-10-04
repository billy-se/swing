package main

import (
	"context"
	"net/http"
	"time"
	"fmt"
	"log"

	"vaine-backend/internal/auth"
)



func (a *App) HandleHealthCheck(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := a.DB.PingContext(ctx); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Engine: online, Connection: lost"))
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Engine: online, DB: connected"))
}

func (a *App) HandleHeartbeat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cookie, err := r.Cookie("refresh_token_swing")
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	claims, err := auth.ParseRefreshToken(cookie.Value, a.JwtRefreshSecret)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var userId int
	if userIdFloat, ok := claims["user_id"].(float64); ok {
		userId = int(userIdFloat)
	} else {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var ctx = r.Context()

	activeSession := fmt.Sprintf("session_active:%d", userId)

	err = a.Redis.Set(ctx, activeSession, "active", time.Second*45).Err()
	if err != nil {
		log.Println("Redis heartbeat error:", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	/*_, err = a.DB.Exec("UPDATE sessions SET last_seen = NOW() WHERE token_hash = $1", cookie.Value)
	if err != nil {
		log.Println("Heartbeat DB Error:", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}*/

	w.WriteHeader(http.StatusOK)
}