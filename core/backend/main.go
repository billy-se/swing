/*package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"

	_ "github.com/lib/pq"
)

type App struct {
	DB *sql.DB
}

func main() {
	dbConnStr := os.Getenv("DB_CONN_STR")
	if dbConnStr == "" {
		dbConnStr = "user=postgres password=password dbname=postgres sslmode=disable"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	db, err := sql.Open("postgres", dbConnStr)
	if err != nil {
		log.Fatalf("Database connection error: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("Database unreachable: %v", err)
	}
	fmt.Println("Sucessfull connected to PostgreSQL.")

	app := &App{DB: db}
	if err := app.initDatabase(); err != nil {
		log.Fatalf("Failed to initialize schema: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", app.handleHealthCheck)

	fmt.Printf("Server running on port %s...\n", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

func (a *App) initDatabase() error {
	query := `
	CREATE TABLE IF NOT EXISTS arguments (
	id SERIAL PRIMARY KEY,
	content TEXT NOT NULL,
	merit_score INT DEFAULT 0,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`

	_, err := a.DB.Exec(query)
	if err != nil {
		return err
	}
	fmt.Println("Database schema initialized.")
	return nil
}

func (a *App) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Engine is online, DB is connected and structured cleanly."))
}*/

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

	if err := godotenv.Load(); err != nil {
		log.Fatalf("No .env found")
	}

	dbConnect := config.ConnectDatabase()

	hub := NewHub()
	go hub.Run()

	app := &App{DB: dbConnect, hub: hub,}

	db.RunMigrations(dbConnect)

	/*if err := app.initDatabase(); err != nil {
		log.Fatalf("Failed to initialize schema: %v", err)
	}*/

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

	//mux.HandleFunc("POST /api/arguments", app.handleCreateArgument)
	//mux.HandleFunc("POST /api/jwt", app.handleJWT)
	//mux.HandleFunc("GET /api/dashboard", app.handleDashboard)
	
	handler := EnableCORS(mux)

	cfg := config.Load()
	log.Printf("Server running on port %s..\n", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, handler); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

/*OLD WAY
func (a *App) initDatabase() error {
	query := `
	CREATE TABLE IF NOT EXISTS arguments (
	id SERIAL PRIMARY KEY,
	content TEXT NOT NULL,
	merit_score INT DEFAULT 0,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`

	_, err := a.DB.Exec(query)
	if err != nil {
		return err
	}
	log.Println("Database schema verified/initialized.")
	return nil
}*/
