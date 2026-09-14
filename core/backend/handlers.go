package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
	"vaine-backend/utils"
	"strings"
	"os"
	"math/rand"
	"sync"

	"github.com/golang-jwt/jwt/v5"
	"github.com/coder/websocket"
)

type contextKey string

const userIDKey contextKey = "user_id"

//var jwtSecret = []byte(os.Getenv("JWTSECRET"))

func (a *App) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("Auth Header received:", r.Header.Get("Authorization"))
        authHeader := r.Header.Get("Authorization")
        if authHeader == "" {
            http.Error(w, "Missing authorization header", http.StatusUnauthorized)
            return
        }

        parts := strings.SplitN(authHeader, " ", 2)
        if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
            http.Error(w, "Invalid authorization format", http.StatusUnauthorized)
            return
        }

        tokenString := parts[1]
        userID, err := a.parseToken(tokenString)
        if err != nil {
            http.Error(w, err.Error(), http.StatusUnauthorized)
            return
        }

        ctx := context.WithValue(r.Context(), userIDKey, userID)
        next(w, r.WithContext(ctx))
    }
}

func generateJWT(userID int) (string, error) {
    secret := os.Getenv("JWTSECRET")
    if secret == "" {
        secret = "default_fallback_secret"
    }

    claims := jwt.MapClaims{
        "user_id": userID,
        "exp":     time.Now().Add(time.Hour * 24 * 7).Unix(),
    }

    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString([]byte(secret))
}

type RegisterInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type User struct {
	Id             int
	Email          string
	HashedPassword string
}

func (a *App) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	if err := a.DB.Ping(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Engine: online, Connection: lost"))
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Engine: online, DB: connected"))
}

func cleanJSONResponse(input string) string {
	input = strings.TrimSpace(input)
	input = strings.TrimPrefix(input, "```json")
	input = strings.TrimPrefix(input, "```")
	input = strings.TrimSuffix(input, "```")
	return strings.TrimSpace(input)
}

func (a *App) handleGetProfile(w http.ResponseWriter, r *http.Request) {
    // 1. Get the user ID from the request context (assuming your auth middleware sets it)
    userId := r.Context().Value(userIDKey) 
    if userId == nil {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }

    var username string
    var logicScore int
    query := `SELECT username, logic_score FROM users WHERE id = $1`
    
    err := a.DB.QueryRowContext(r.Context(), query, userId).Scan(&username, &logicScore)
    if err != nil {
        http.Error(w, "User not found", http.StatusNotFound)
        return
    }

    // 3. Send the data back as JSON
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]any{
        "username":    username,
        "logic_score": logicScore,
    })
}

var w1 = []string{"Mine", "Spare", "South", "Hum", "Rode"}
var w2 = []string{"Peak", "Jar", "Ink", "Leap", "Up"}

func generateNames() string{
	random := rand.New(rand.NewSource(time.Now().UnixNano()))

	wo1 := w1[random.Intn(len(w1))]
	wo2 := w2[random.Intn(len(w2))]
	randomNum := random.Intn(900) + 100			
	
	return fmt.Sprintf("%s%s%d", wo1, wo2, randomNum)
}

func (a *App) handleRegister(w http.ResponseWriter, r *http.Request) {

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var input RegisterInput
	err := json.NewDecoder(r.Body).Decode(&input)
	if err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	if input.Email == "" || input.Password == "" {
		http.Error(w, "Email and password are required", http.StatusBadRequest)
		return
	}

	email := strings.ToLower(strings.TrimSpace(input.Email))
	decryptEmail := utils.GenerateBlindIndex(email, []byte(os.Getenv("keyAesGo")))

	/*securedEmail, err := utils.AesPy(input.Email)
	if err != nil {
		log.Printf("Email AES error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}*/
	secureEmail, err := utils.AesGo(input.Email)
	if err != nil {
		log.Printf("Email AES error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	hashedPassword, err := utils.HashPassword(input.Password)
	if err != nil {
		log.Printf("Password hashing error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	
	name := generateNames()
	fmt.Println("DEBUG: Generate ->", name)

    var defaultScore = 1000

	query := `INSERT INTO users (email, email_hash, password_hash, username, logic_score) VALUES ($1, $2, $3, $4, $5) RETURNING id, created_at`
	var id int
	var createdAt time.Time

	err = a.DB.QueryRow(query, secureEmail, decryptEmail, hashedPassword, name, defaultScore).Scan(&id, &createdAt)
	if err != nil {
		log.Printf("Database insert errorrr: %v", err)
		http.Error(w, "Email might already be taken", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
	fmt.Fprintf(w, `{"message": "User registered successfully", "id": %d}`, id)
}

func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var creds RegisterInput
	err := json.NewDecoder(r.Body).Decode(&creds)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	email := strings.ToLower(strings.TrimSpace(creds.Email))
	decryptEmail := utils.GenerateBlindIndex(email, []byte(os.Getenv("keyAesGo")))

	var username string
	var user User
	//WHERE email = 1$ (earlier)
	query := "SELECT id, email, password_hash, username FROM users WHERE email_hash = $1"


	//creds.Email
	err = a.DB.QueryRow(query, decryptEmail).Scan(&user.Id, &user.Email, &user.HashedPassword, &username)

	if err == sql.ErrNoRows {
		http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		return
	} else if err != nil {
		fmt.Println("DB Error: ", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	IsValid := utils.CheckPassword(creds.Password, user.HashedPassword)
	if !IsValid {
		http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		return
	}

	tokenString, err := generateJWT(user.Id)
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Login successful!",
		"token":   tokenString,
		"username": username,
	})
}

func EnableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, PATCH, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("X-Content-Type-Options", "nosniff")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

type Client struct{
	connection *websocket.Conn
	send chan []byte
    userId int
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[int]*Client),
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

func (a *App) WebSocketHandler(w http.ResponseWriter, r *http.Request) {
    token := r.URL.Query().Get("token")

    if token != "" && token != "null" && token != "undefined" {
        _, err := a.parseToken(token)
        if err != nil {
            log.Printf("WebSocket viewer mode failed: %v", err)
        }
    }

    connection, err := websocket.Accept(w, r, &websocket.AcceptOptions{
        OriginPatterns: []string{"localhost:*", "127.0.0.1:*"},
    })
    if err != nil {
        log.Println("Failed to upgrade connection", err)
        return
    }

    defer connection.Close(websocket.StatusNormalClosure, "Session ended")

    claims, err := a.parseToken(token)
    userId := claims

    client := &Client{
        connection: connection,
        send:       make(chan []byte, 256),
        userId: userId,
    }

    a.hub.register <- client
    defer func() {
        a.hub.unregister <- client
    }()

    ctx := r.Context()

    go func() {
        for message := range client.send {
            err := connection.Write(ctx, websocket.MessageText, message)
            if err != nil {
                break
            }
        }
    }()

    for {
        _, _, err := connection.Read(ctx)
        if err != nil {
            log.Println("Client disconnected", err)
            break
        }
    }
}

type Hub struct {
	clients    map[int]*Client
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	mu         sync.Mutex
}

func (h *Hub) Run() {
	for {
		select {
		case client := <- h.register:
			h.mu.Lock()//grab and lock
			h.clients[client.userId] = client//wait
			h.mu.Unlock()//unlock
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client.userId]; ok {//check if there is clients inside Hub
				delete(h.clients, client.userId)//delete
				close(client.send)//close
			}
			h.mu.Unlock()
		case message := <-h.broadcast:
			h.mu.Lock()
			for userId, client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, userId)
				}
			}
			h.mu.Unlock()
		}
	}
}

func (a *App) parseToken(tokenString string) (int, error) {
    secret := os.Getenv("JWTSECRET")
    if secret == "" {
        secret = "default_fallback_secret"
    }

    token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
        if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, fmt.Errorf("unexpected signing method")
        }
        return []byte(secret), nil
    })
    if err != nil {
        fmt.Println("JWT Parse Error Details:", err)
        return 0, err
    }
    if !token.Valid {
        return 0, fmt.Errorf("invalid or expired token")
    }
    claims, ok := token.Claims.(jwt.MapClaims)
    if !ok {
        return 0, fmt.Errorf("invalid token claims")
    }
    userIDFloat, ok := claims["user_id"].(float64)
    if !ok {
        return 0, fmt.Errorf("invalid user ID in token")
    }
    return int(userIDFloat), nil
}

func (a *App) handleGetNotifications(w http.ResponseWriter, r *http.Request){
    userID, ok := r.Context().Value(userIDKey).(int)
    if !ok {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }

    rows, err := a.DB.Query(`SELECT id, argument_id, comment_id, type, content, created_at, is_read FROM notif WHERE user_id = $1 ORDER BY created_at DESC LIMIT 20`, userID)
    if err != nil {
        http.Error(w, "Failed to fetch notifications", http.StatusInternalServerError)
        return
    }
    defer rows.Close()

    notifications := []map[string]interface{}{}
    for rows.Next() {
        var id int
        var argID sql.NullInt64
        var commentID sql.NullInt64
        var notifType sql.NullString
        var content sql.NullString
        var createdAt time.Time
        var isRead bool

        if err := rows.Scan(&id, &argID, &commentID, &notifType, &content, &createdAt, &isRead); err != nil {
            continue
        }

        notifMap := map[string]interface{}{
            "id":         id,
            "type":       notifType.String,
            "content":    content.String,
            "created_at": createdAt.Format("2006-01-02 15:04:05"),
            "is_read":    isRead,
        }

        if argID.Valid {
            notifMap["argument_id"] = argID.Int64
        }
        if commentID.Valid {
            notifMap["comment_id"] = commentID.Int64
        }

        notifications = append(notifications, notifMap)
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "notifications": notifications,
    })
}

func (a *App) handleMarkNotificationRead(w http.ResponseWriter, r *http.Request) {
    userID, ok := r.Context().Value(userIDKey).(int)
    if !ok {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }

    notifID := r.PathValue("id")
    if notifID == "" {
        http.Error(w, "Invalid notification ID", http.StatusBadRequest)
        return
    }

    _, err := a.DB.Exec(`UPDATE notif SET is_read = TRUE WHERE id = $1 AND user_id = $2`, notifID, userID)
    if err != nil {
        http.Error(w, "Failed to update notification", http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{"message": "Marked as read"})
}