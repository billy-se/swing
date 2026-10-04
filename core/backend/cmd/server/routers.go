package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/time/rate"

	"vaine-backend/internal/arguments"
	"vaine-backend/internal/auth"
	"vaine-backend/internal/comments"
	"vaine-backend/internal/middleware"
	"vaine-backend/internal/websocket"
	"vaine-backend/internal/notification"
	"vaine-backend/internal/stats"

)

func (app *App) SetupRouter() http.Handler {
	mux := http.NewServeMux()

	loginLimiter := RateLimiter(app.Redis, 5, 10*time.Second)

	authHandler := &auth.Handler{
		DB: app.DB,
		Redis: app.Redis,
		JwtAccessSecret: app.JwtAccessSecret,
		JwtRefreshSecret: app.JwtRefreshSecret,
	}

	mw := &middleware.HandlerMiddleware{
		JwtAccessSecret: app.JwtAccessSecret,
		Redis:           app.Redis,
	}

	argumentHandler := arguments.NewArgumentHandler(app.DB, app.Redis, app.JwtAccessSecret, app.hub.Broadcast)

	commentHandler := &comments.CommentHandler{
		DB:                         app.DB,
		Redis:                      app.Redis,
		JwtAccessSecret:            app.JwtAccessSecret,
		Broadcast:                  app.hub.Broadcast,
		UpdateArgumentLogicScoreFn: argumentHandler.UpdateArgumentLogicScore,
		Hub:                        app.hub,
	}

	websocketHandler := websocket.NewWebSocketHandler(app.hub, app.JwtAccessSecret)

	notificationHandler := &notification.NotificationHandler{
		Redis         : app.Redis,
	DB              : app.DB,
	}

	statsHandler := stats.HandlerStats{
		DB              : app.DB,
	}

	//public routes
	mux.HandleFunc("GET /api/health", app.HandleHealthCheck)
	mux.HandleFunc("POST /api/register", authHandler.HandleRegister)
	mux.HandleFunc("POST /api/login", loginLimiter(authHandler.HandleLogin))
	mux.HandleFunc("POST /api/refresh", authHandler.HandleRefresh)
	mux.HandleFunc("POST /api/viewer", auth.HandleViewerMode)
	mux.HandleFunc("GET /api/arguments", argumentHandler.HandleGetArguments)
	mux.HandleFunc("GET /api/arguments/{id}", argumentHandler.HandleGetArgument)
	mux.HandleFunc("GET /api/arguments/top", argumentHandler.HandleTopArguments)
	mux.HandleFunc("GET /api/comments", commentHandler.HandleGetComments)
	mux.HandleFunc("POST /api/heartbeat", app.HandleHeartbeat)
	mux.HandleFunc("/ws", websocketHandler.WebSocketHandler)

	//protected routes
	mux.HandleFunc("POST /api/arguments", mw.AuthMiddleware(argumentHandler.HandleCreateArgument))
	mux.HandleFunc("POST /api/comments", mw.AuthMiddleware(app.RateLimitMiddleware(commentHandler.HandleCreateComment)))
	mux.HandleFunc("POST /api/comments/fire", mw.AuthMiddleware(commentHandler. HandleFireReaction))
	mux.HandleFunc("GET /api/user/profile", mw.AuthMiddleware(authHandler.HandleGetProfile))
	mux.HandleFunc("GET /api/notifications", mw.AuthMiddleware(notificationHandler.HandleGetNotifications))
	mux.HandleFunc("PATCH /api/notifications/{id}/read", mw.AuthMiddleware(notificationHandler.HandleMarkNotificationRead))
	mux.HandleFunc("POST /api/ws-ticket", websocketHandler.HandleGenerateWSTicket)
	mux.HandleFunc("POST /api/logout", mw.AuthMiddleware(authHandler.HandleLogout))
	mux.HandleFunc("POST /api/watchlist", mw.AuthMiddleware(argumentHandler.HandleCreateWatchlist))
	mux.HandleFunc("GET /api/watchlist", mw.AuthMiddleware(argumentHandler.HandleGetWatchlist))
	mux.HandleFunc("GET /api/stats", mw.AuthMiddleware(statsHandler.HandleShowStats))
	mux.HandleFunc("GET /api/user/stats", mw.AuthMiddleware(statsHandler.HandleUserStats))

	return mux
}

func (cm *ClientManager) getLimiter(ip string) *rate.Limiter {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	limiter, exists := cm.clients[ip]
	if !exists {
		limiter = rate.NewLimiter(rate.Every(time.Second/5), 10)
		cm.clients[ip] = limiter
	}
	return limiter
}

func (app *App) RateLimitMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := r.Context().Value(auth.UserIDKey).(int)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		userIdKey := fmt.Sprintf("user_%d", userID)

		limiter := app.ClientManager.getLimiter(userIdKey)

		if !limiter.Allow() {
			w.Header().Set("Retry-After", "1")
			http.Error(w, "Too Many Request - Slow down!", http.StatusTooManyRequests)
			return
		}

		next(w, r)
	}
}

func RateLimiter(rdb *redis.Client, limit int, window time.Duration) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			ctx := context.Background()

			clientIP := r.RemoteAddr
			rateKey := fmt.Sprintf("rate:login:%s", clientIP)

			count, err := rdb.Incr(ctx, rateKey).Result()
			if err != nil {
				next(w, r)
				return
			}

			if count == 1 {
				rdb.Expire(ctx, rateKey, window)
			}

			if int(count) > limit {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"error": "Your sending too much. Please try again later."}`))
				return
			}

			next(w, r)
		}
	}
}
