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

	mux.HandleFunc("POST /api/arguments", app.authMiddleware(app.handleCreateArgument))
	mux.HandleFunc("GET /api/arguments", app.handleGetArguments)
	mux.HandleFunc("POST /api/comments", app.authMiddleware(app.handleCreateComment))
	mux.HandleFunc("/ws", app.WebSocketHandler)

	mux.HandleFunc("/api/comments/fire", app.authMiddleware(app.handleFireReaction))
	mux.HandleFunc("GET /api/comments", app.handleGetComments)

	mux.HandleFunc("GET /api/user/profile", app.authMiddleware(app.handleGetProfile))

	mux.HandleFunc("GET /api/auth/me", app.authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		userID := r.Context().Value(userIDKey).(int)

		var id int
		var username string

		err := app.DB.QueryRow("SELECT id, username FROM users WHERE id = $1", userID).Scan(&id, &username)
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
		})
	}))

	mux.HandleFunc("GET /api/notifications", app.authMiddleware(app.handleGetNotifications))
	mux.HandleFunc("PATCH /api/notifications/{id}/read", app.authMiddleware(app.handleMarkNotificationRead))

	mux.HandleFunc("POST /api/ws-ticket", app.authMiddleware(app.handleGenerateWSTicket))

	handler := EnableCORS(mux)

	configuration := config.Load()
	log.Printf("Server running on port %s..\n", configuration.Port)
	if serverError := http.ListenAndServe(":"+configuration.Port, handler); serverError != nil {
		log.Fatalf("Server failed to start: %v", serverError)
	}
}
