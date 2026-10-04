package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
	"vaine-backend/db"
	"vaine-backend/internal/config"
	"vaine-backend/internal/database"
	"vaine-backend/internal/middleware"
	"vaine-backend/internal/websocket"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"golang.org/x/time/rate"
)

type ClientManager struct {
	mu      sync.RWMutex
	clients map[string]*rate.Limiter
}

type App struct {
	DB               *sql.DB
	hub              *websocket.Hub
	wsTickets        map[string]WSTicket
	ticketMutex      sync.Mutex
	Redis            *redis.Client
	JwtAccessSecret  string
	JwtRefreshSecret string
	ClientManager    *ClientManager
}

type WSTicket struct {
	UserID    int
	ExpiresAt time.Time
}

func main() {
	//env
	/*if loadError := godotenv.Load(); loadError != nil {
		log.Fatalf("No .env found")
	}*/
	if _, err := os.Stat(".env"); err == nil {
		if loadError := godotenv.Load(); loadError != nil {
			log.Printf("Warning: Error loading .env file: %v", loadError)
		}
	}

	//database and migration setup
	cfg := config.Load()
	databaseConnection := database.ConnectDatabase()
	db.RunMigrations(databaseConnection)

	//websocket hub
	hub := websocket.NewHub()
	go hub.Run()

	//redis .env and setup
	redisAddr := os.Getenv("REDIS_URL")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	var opt *redis.Options
	var err error

	if strings.HasPrefix(redisAddr, "redis://") || strings.HasPrefix(redisAddr, "rediss://") {
		opt, err = redis.ParseURL(redisAddr)
		if err != nil {
			panic(err)
		}
	} else {
		opt = &redis.Options{
			Addr: redisAddr,
		}
	}

	rdb := redis.NewClient(opt)

	/*rdb := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: "",
		DB:       0,
	})*/

	//server struct
	app := &App{
		DB:               databaseConnection,
		hub:              hub,
		wsTickets:        make(map[string]WSTicket),
		Redis:            rdb,
		JwtAccessSecret:  os.Getenv("JWT_ACCESS"),
		JwtRefreshSecret: os.Getenv("JWT_REFRESH"),
		ClientManager: &ClientManager{
			clients: make(map[string]*rate.Limiter),
		},
	}
	if app.JwtAccessSecret == "" {
		log.Fatal("JWT_ACCESS environment variable is missing")
	}
	if app.JwtRefreshSecret == "" {
		log.Fatal("JWT_REFRESH environment variable is missing")
	}

	//endpoints setup
	handler := middleware.EnableCORS(app.SetupRouter())

	//server start
	log.Printf("Server running on port %s..\n", cfg.Port)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      handler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	if serverError := srv.ListenAndServe(); serverError != nil {
		log.Fatalf("Server failed to start: %v", serverError)
	}
}
