package main

import (
	"database/sql"
	"log"
	"net/http"
	"vaine-backend/config"
	"vaine-backend/db"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type App struct {
	DB *sql.DB
	hub *Hub
}

func main() {

	if loadError := godotenv.Load(); loadError != nil {
		log.Fatalf("No .env found")
	}

	databaseConnection := config.ConnectDatabase()

	hub := NewHub()
	go hub.Run()

	app := &App{DB: databaseConnection, hub: hub,}

	db.RunMigrations(databaseConnection)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", app.handleHealthCheck)
	mux.HandleFunc("POST /api/register", app.handleRegister)
	mux.HandleFunc("POST /api/login", app.handleLogin)

	mux.HandleFunc("POST /api/arguments", app.authMiddleware(app.handleCreateArgument))
	mux.HandleFunc("GET /api/arguments", app.handleGetArguments)
	mux.HandleFunc("POST /api/comments", app.authMiddleware(app.handleCreateComment))
	mux.HandleFunc("/ws", app.WebSocketHandler)

	mux.HandleFunc("/api/comments/fire", app.authMiddleware(app.handleFireReaction))
	mux.HandleFunc("GET /api/comments", app.handleGetComments)

	mux.HandleFunc("GET /api/user/profile", app.authMiddleware(app.handleGetProfile))

	mux.HandleFunc("GET /api/notifications", app.authMiddleware(app.handleGetNotifications))
	mux.HandleFunc("PATCH /api/notifications/{id}/read", app.authMiddleware(app.handleMarkNotificationRead))
	
	handler := EnableCORS(mux)

	configuration := config.Load()
	log.Printf("Server running on port %s..\n", configuration.Port)
	if serverError := http.ListenAndServe(":"+configuration.Port, handler); serverError != nil {
		log.Fatalf("Server failed to start: %v", serverError)
	}
}
