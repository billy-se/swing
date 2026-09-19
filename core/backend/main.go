package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"
	"vaine-backend/config"
	"vaine-backend/db"

	"github.com/golang-jwt/jwt/v5"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

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
	mux.HandleFunc("POST /api/login", app.handleLogin)
	mux.HandleFunc("POST /api/refresh", app.handleRefresh)
	mux.HandleFunc("POST /api/viewer", app.handleViewerMode)

	mux.HandleFunc("POST /api/arguments", app.authMiddleware(app.handleCreateArgument))
	mux.HandleFunc("GET /api/arguments", app.handleGetArguments)
	mux.HandleFunc("POST /api/comments", app.authMiddleware(app.handleCreateComment))
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

	mux.HandleFunc("POST /api/logout", app.handleLogout)

	handler := EnableCORS(mux)

	configuration := config.Load()
	log.Printf("Server running on port %s..\n", configuration.Port)
	if serverError := http.ListenAndServe(":"+configuration.Port, handler); serverError != nil {
		log.Fatalf("Server failed to start: %v", serverError)
	}
}
