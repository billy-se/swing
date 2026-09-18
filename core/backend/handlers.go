package main

import (
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
	"vaine-backend/utils"

	"github.com/coder/websocket"
	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const userIDKey contextKey = "user_id"

//var jwtSecret = []byte(os.Getenv("JWTSECRET"))

func (a *App) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
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

		tokenString := parts[1]

		userID, err := a.parseToken(tokenString)
		if err != nil {
			http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), userIDKey, userID)
		next(w, r.WithContext(ctx))
	}
}

/*func generateJWT(userID int) (string, error) {
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
}*/

func (a *App) parseToken(tokenString string) (int, error) {
	jwtParseSigningKey := os.Getenv("JWT_ACCESS")
	if jwtParseSigningKey == "" {
		return 0, errors.New("[parseToken]: JWT_ACCESS environment variable is missing")
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(jwtParseSigningKey), nil
	})
	if err != nil {
		fmt.Println("[parseToken]: JWT Parse Error Details:", err)
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

func generateAccessToken(userID int) (string, error) {
	jwtAccessSigningKey := os.Getenv("JWT_ACCESS")
	if jwtAccessSigningKey == "" {
		return "", errors.New("JWT_ACCESS environment variable is missing")
	}

	claims := jwt.MapClaims{
		"user_id": userID,
		"type":    "access",
		"exp":     time.Now().Add(time.Minute * 15).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(jwtAccessSigningKey))
}

func generateRefreshToken(userID int) (string, error) {
	jwtRefreshSigningKey := os.Getenv("JWT_REFRESH")
	if jwtRefreshSigningKey == "" {
		return "", errors.New("JWT_REFRESH environment variable is missing")
	}

	claims := jwt.MapClaims{
		"user_id": userID,
		"type":    "refresh",
		"exp":     time.Now().Add(time.Hour * 24 * 7).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(jwtRefreshSigningKey))
}

func (a *App) handleRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cookie, err := r.Cookie("refresh_token_swing")
	if err != nil {
		if err == http.ErrNoCookie {
			http.Error(w, "Unauthorized: No refresh token cookie", http.StatusUnauthorized)
			return
		}
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	refreshTokenString := cookie.Value

	jwtRefresh := os.Getenv("JWT_REFRESH")
	if jwtRefresh == "" {
		http.Error(w, "[handleRefresh]: Cant find JWT_REFRESH", 0)
	}

	token, err := jwt.Parse(refreshTokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(jwtRefresh), nil
	})

	if err != nil || !token.Valid {
		http.Error(w, "Unauthorized: Invalid or expired refresh token", http.StatusUnauthorized)
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		http.Error(w, "Unauthorized: Invalid token claims", http.StatusUnauthorized)
		return
	}

	if tokenType, ok := claims["type"].(string); !ok || tokenType != "refresh" {
		http.Error(w, "Unauthorized: Invalid token type", http.StatusUnauthorized)
		return
	}

	userIDFloat, ok := claims["user_id"].(float64)
	if !ok {
		http.Error(w, "Unauthorized: Invalid user ID", http.StatusUnauthorized)
		return
	}
	userID := int(userIDFloat)

	newAccessToken, err := generateAccessToken(userID)
	if err != nil {
		http.Error(w, "Failed to generate new access token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"access_token": newAccessToken,
	})
}

func generateRandomString(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func (a *App) handleGenerateWSTicket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := r.Context().Value(userIDKey).(int)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	ticket, err := generateRandomString(16)
	if err != nil {
		http.Error(w, "Failed to generate ticket", http.StatusInternalServerError)
		return
	}

	a.ticketMutex.Lock()
	a.wsTickets[ticket] = WSTicket{
		UserID:    userID,
		ExpiresAt: time.Now().Add(30 * time.Second),
	}
	a.ticketMutex.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"ticket": ticket,
	})
}

func (a *App) handleRefreshToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cookie, err := r.Cookie("refresh_token_swing")
	if err != nil {
		http.Error(w, "Missing refresh token cookie", http.StatusUnauthorized)
		return
	}

	userID, err := a.parseToken(cookie.Value)
	if err != nil {
		http.Error(w, "Invalid or expired refresh token", http.StatusUnauthorized)
		return
	}

	newAccessToken, err := generateAccessToken(userID)
	if err != nil {
		http.Error(w, "Failed to generate access token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"access_token": newAccessToken,
	})
}

type RegisterInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type User struct {
	Id             int    `json:"id"`
	Email          string `json:"email"`
	HashedPassword string `json:"-"`
	Username       string `json:"username"`
	Role           string `json:"role"`
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
	fmt.Println("DEBUG: handleGetProfile was hit! userId =", r.Context().Value(userIDKey))
	userId := r.Context().Value(userIDKey)

	if userId == nil || userId == 0 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id":          0,
			"username":    "VIEWER",
			"logic_score": 0,
			"role":        "viewer",
		})
		return
	}

	var username string
	var logicScore int
	var role string
	query := `SELECT username, logic_score, role FROM users WHERE id = $1`

	err := a.DB.QueryRowContext(r.Context(), query, userId).Scan(&username, &logicScore, &role)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"id":          userId,
		"username":    username,
		"logic_score": logicScore,
		"role":        role,
	})
}

var w1 = []string{"Mine", "Spare", "South", "Hum", "Rode"}
var w2 = []string{"Peak", "Jar", "Ink", "Leap", "Up"}

func generateNames() string {
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

	var existingID int
	err = a.DB.QueryRow("SELECT id FROM users WHERE email_hash = $1", decryptEmail).Scan(&existingID)
	if err == nil {
		http.Error(w, "An account with this email already exists", http.StatusConflict)
		return
	}

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

	query := `INSERT INTO users (email, email_hash, password_hash, username, logic_score, role) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id, created_at`
	var id int
	var createdAt time.Time
	defaultRole := "user"

	err = a.DB.QueryRow(query, secureEmail, decryptEmail, hashedPassword, name, defaultScore, defaultRole).Scan(&id, &createdAt)
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

	//var username string
	var user User
	query := "SELECT id, email, password_hash, username, role FROM users WHERE email_hash = $1"

	//creds.Email
	err = a.DB.QueryRow(query, decryptEmail).Scan(&user.Id, &user.Email, &user.HashedPassword, &user.Username, &user.Role)

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

	//generate short-lived access token
	accessToken, err := generateAccessToken(user.Id)
	if err != nil {
		log.Println("JWT Generation Error:", err)

		http.Error(w, "Failed to generate access token", http.StatusInternalServerError)
		return
	}

	//generate long-lived refresh token
	refreshToken, err := generateRefreshToken(user.Id)
	if err != nil {
		log.Println("JWT Generation Error:", err)

		http.Error(w, "Failed to generate refresh token", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token_swing",
		Value:    refreshToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(time.Hour * 24 * 7),
	})

	/*tokenString, err := generateJWT(user.Id) // old method
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}*/

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Login successful!",
		//"token":   tokenString,
		"access_token": accessToken,
		"username":     user.Username, //username,
		"role":         user.Role,
	})
}

func EnableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
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

type Client struct {
	connection *websocket.Conn
	send       chan []byte
	userId     int
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[int]*Client),
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

/*var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}*/

func (a *App) WebSocketHandler(w http.ResponseWriter, r *http.Request) {
	/*token := r.URL.Query().Get("token")

		var userId int

	    if token != "" && token != "null" && token != "undefined" {
	        parseId, err := a.parseToken(token)
	        if err != nil {
	            log.Printf("WebSocket viewer mode failed: %v", err)
	        } else {
				userId = parseId
			}
	    }*/

	ticket := r.URL.Query().Get("ticket")
	if ticket == "" {
		http.Error(w, "Missing Ticket", http.StatusUnauthorized)
		return
	}

	a.ticketMutex.Lock()
	storedTicket, exists := a.wsTickets[ticket]
	if exists {
		delete(a.wsTickets, ticket)
	}
	a.ticketMutex.Unlock()

	if !exists || time.Now().After(storedTicket.ExpiresAt) {
		http.Error(w, "Invalid or expired ticket", http.StatusUnauthorized)
		return
	}

	connection, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"localhost:*", "127.0.0.1:*"},
	})
	if err != nil {
		log.Println("Failed to upgrade connection", err)
		return
	}

	defer connection.Close(websocket.StatusNormalClosure, "Session ended")

	client := &Client{
		connection: connection,
		send:       make(chan []byte, 256),
		userId:     storedTicket.UserID,
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
		case client := <-h.register:
			h.mu.Lock()                       //grab and lock
			h.clients[client.userId] = client //wait
			h.mu.Unlock()                     //unlock
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client.userId]; ok { //check if there is clients inside Hub
				delete(h.clients, client.userId) //delete
				close(client.send)               //close
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

func (a *App) handleGetNotifications(w http.ResponseWriter, r *http.Request) {
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

	if err := rows.Err(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
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

func (a *App) handleViewerMode(w http.ResponseWriter, r *http.Request) {

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Only Post is alloewd", http.StatusMethodNotAllowed)
		return
	}

	name := generateNames()

	accessToken, err := generateViewerAccessToken(name)
	if err != nil {
		http.Error(w, "Failed to generate viewer session", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token_swing",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"username":     name,
		"role":         "viewer",
		"access_token": accessToken,
	})
}

func generateViewerAccessToken(username string) (string, error) {
	jwtAccessSigningKey := os.Getenv("JWT_ACCESS")
	if jwtAccessSigningKey == "" {
		return "", errors.New("JWT_ACCESS environment variable is missing")
	}

	claims := jwt.MapClaims{
		"user_id":  0,
		"username": username,
		"role":     "viewer",
		"type":     "access",
		"exp":      time.Now().Add(time.Minute * 5).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(jwtAccessSigningKey))
}
