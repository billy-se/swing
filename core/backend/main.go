package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
	"vaine-backend/config"
	"vaine-backend/db"

	"github.com/golang-jwt/jwt/v5"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"golang.org/x/time/rate"
)

type ClientManager struct {
	mu      sync.RWMutex
	clients map[string]*rate.Limiter
}

var manager = &ClientManager{
	clients: make(map[string]*rate.Limiter),
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

func RateLimitMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}

		limiter := manager.getLimiter(ip)

		if !limiter.Allow() {
			w.Header().Set("Retry-After", "1")
			http.Error(w, "Too Many Request - Slow down!", http.StatusTooManyRequests)
			return
		}

		next(w, r)
	}
}

type App struct {
	DB          *sql.DB
	hub         *Hub
	wsTickets   map[string]WSTicket
	ticketMutex sync.Mutex
}

type WSTicket struct {
	UserID    int
	ExpiresAt time.Time
}

func main() {

	if loadError := godotenv.Load(); loadError != nil {
		log.Fatalf("No .env found")
	}

	databaseConnection := config.ConnectDatabase()

	hub := NewHub()
	go hub.Run()

	app := &App{
		DB:        databaseConnection,
		hub:       hub,
		wsTickets: make(map[string]WSTicket),
	}

	db.RunMigrations(databaseConnection)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", app.handleHealthCheck)
	mux.HandleFunc("POST /api/register", app.handleRegister)
	mux.HandleFunc("POST /api/login", RateLimitMiddleware(app.handleLogin))
	mux.HandleFunc("POST /api/refresh", app.handleRefresh)
	mux.HandleFunc("POST /api/viewer", app.handleViewerMode)

	mux.HandleFunc("POST /api/arguments", app.authMiddleware(app.handleCreateArgument))
	mux.HandleFunc("GET /api/arguments", app.handleGetArguments)
	mux.HandleFunc("POST /api/comments", RateLimitMiddleware(app.authMiddleware(app.handleCreateComment)))

	mux.HandleFunc("/ws", app.WebSocketHandler)

	mux.HandleFunc("/api/comments/fire", app.authMiddleware(app.handleFireReaction))
	mux.HandleFunc("GET /api/comments", app.handleGetComments)

	mux.HandleFunc("GET /api/user/profile", app.authMiddleware(app.handleGetProfile))

	mux.HandleFunc("GET /api/auth/me", app.authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := r.Context().Value(claimsKey).(jwt.MapClaims)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		role, _ := claims["role"].(string)

		if role == "viewer" {
			username, _ := claims["username"].(string)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":       0,
				"username": username,
				"role":     "viewer",
			})
			return
		}

		userID := r.Context().Value(userIDKey).(int)

		var id int
		var username string
		var dbRole string

		err := app.DB.QueryRow("SELECT id, username, role FROM users WHERE id = $1", userID).Scan(&id, &username, &dbRole)
		if err != nil {
			if err == sql.ErrNoRows {
				http.Error(w, "User not found", http.StatusNotFound)
				return
			}
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":       id,
			"username": username,
			"role":     dbRole,
		})
	}))

	mux.HandleFunc("GET /api/notifications", app.authMiddleware(app.handleGetNotifications))
	mux.HandleFunc("PATCH /api/notifications/{id}/read", app.authMiddleware(app.handleMarkNotificationRead))

	mux.HandleFunc("POST /api/ws-ticket", app.authMiddleware(app.handleGenerateWSTicket))

	mux.HandleFunc("POST /api/logout", app.authMiddleware(app.handleLogout))

	mux.HandleFunc("GET /api/arguments/top", app.handleTopArguments)

	mux.HandleFunc("POST /api/watchlist", app.authMiddleware(app.handleCreateWatchlist))
	mux.HandleFunc("GET /api/watchlist", app.authMiddleware(app.handleGetWatchlist))

	mux.HandleFunc("POST /api/heartbeat", app.handleHeartbeat)

	handler := EnableCORS(mux)

	configuration := config.Load()
	log.Printf("Server running on port %s..\n", configuration.Port)

	srv := &http.Server{
		Addr:         ":" + configuration.Port,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	if serverError := srv.ListenAndServe(); serverError != nil {
		log.Fatalf("Server failed to start: %v", serverError)
	}
}
