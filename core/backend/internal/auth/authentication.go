package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"

	mrand "math/rand/v2"

	gonanoid "github.com/matoous/go-nanoid/v2"
)

type contextKey string

const UserIDKey contextKey = "user_id"
const ClaimsKey contextKey = "claims"

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

// helpers
var (
	cookieSecure   = getCookieSecure()
	cookieSameSite = getCookieSameSite()
)

func getCookieSecure() bool {
	secure, _ := strconv.ParseBool(os.Getenv("COOKIE_SECURE"))
	return secure
}

func getCookieSameSite() http.SameSite {
	switch os.Getenv("COOKIE_SAMESITE") {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}

func SetCookie(w http.ResponseWriter, name, value string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   cookieSecure,
		SameSite: cookieSameSite,
		Expires:  expiresAt,
	})
}

func ClearCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   cookieSecure,
		SameSite: cookieSameSite,
		MaxAge:   -1,
	})
}

type Handler struct {
	DB               *sql.DB
	JwtRefreshSecret string
	JwtAccessSecret  string
	Redis            *redis.Client
}

func (h *Handler) HandleRegister(w http.ResponseWriter, r *http.Request) {

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
	encryptEmail := GenerateBlindIndex(email, []byte(os.Getenv("keyAesGo")))

	var existingID int
	err = h.DB.QueryRow("SELECT id FROM users WHERE email_hash = $1", encryptEmail).Scan(&existingID)

	if errors.Is(err, sql.ErrNoRows) {
		secureEmail, err := AesGo(input.Email)
		if err != nil {
			log.Printf("Email AES error: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		hashedPassword, err := HashPassword(input.Password)
		if err != nil {
			log.Printf("Password hashing error: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		name := generateNames()

		var defaultScore = 1000

		query := `INSERT INTO users (email, email_hash, password_hash, username, logic_score, role) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id, created_at`
		var id int
		var createdAt time.Time
		defaultRole := "user"

		err = h.DB.QueryRow(query, secureEmail, encryptEmail, hashedPassword, name, defaultScore, defaultRole).Scan(&id, &createdAt)
		if err != nil {
			log.Printf("Database insert errorrr: %v", err)
			http.Error(w, "Email might already be taken", http.StatusBadRequest)
			return
		}

		fmt.Printf("Created -> Name: %s, ID: %d\n", name, id)

		w.WriteHeader(http.StatusCreated)
		fmt.Fprintf(w, `{"message": "User registered successfully", "id": %d}`, id)
		return
	}

	if err == nil {
		http.Error(w, "An account with this email already exists", http.StatusConflict)
		return
	} else {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	/*securedEmail, err := utils.AesPy(input.Email)
	if err != nil {
		log.Printf("Email AES error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}*/
}

func (h *Handler) HandleLogin(w http.ResponseWriter, r *http.Request) {

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
	decryptEmail := GenerateBlindIndex(email, []byte(os.Getenv("keyAesGo")))

	ctx1, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	//var username string
	var user User
	query := "SELECT id, email, password_hash, username, role FROM users WHERE email_hash = $1"

	//creds.Email
	err = h.DB.QueryRowContext(ctx1, query, decryptEmail).Scan(&user.Id, &user.Email, &user.HashedPassword, &user.Username, &user.Role)

	if err != nil {
		if ctx1.Err() == context.DeadlineExceeded {
			http.Error(w, "Request timed out, please try again", http.StatusGatewayTimeout)
			return
		}
		if err == sql.ErrNoRows {
			http.Error(w, "Invalid username or password", http.StatusUnauthorized)
			return
		}
		fmt.Println("DB Error: ", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	IsValid := CheckPassword(creds.Password, user.HashedPassword)
	if !IsValid {
		http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		return
	}

	//under construnction

	/*var ctx = context.Background()
	activeSession := fmt.Sprintf("session_lock:%d", user.Id)

	success, err := a.Redis.SetNX(ctx, activeSession, "active", time.Second*30).Result()

	if err != nil {
		log.Println("Redis error setting presence:", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if !success {
		http.Error(w, "Someone is using this account", http.StatusConflict)
		return
	}*/

	fmt.Printf("In -> Name: %s, ID: %d\n", user.Username, user.Id)

	/*
		val, err := a.Redis.Get(ctx, activeSession).Result()
		if err == nil && val != "" {
			http.Error(w, "Someone is using this account", http.StatusConflict)
			return
		}*/

	/*var activeSessionCount int //redis here
	err = a.DB.QueryRow(
		"SELECT COUNT(*) FROM sessions WHERE user_id = $1 AND expires_at > NOW() AND last_seen > NOW() - INTERVAL '30 seconds'",
		user.Id,
	).Scan(&activeSessionCount)

	if activeSessionCount > 0 {
		http.Error(w, "Someone is using this account", http.StatusConflict)
		return
	}*/

	/*_, err = a.DB.Exec("DELETE FROM sessions WHERE user_id = $1 AND last_seen <= NOW() - INTERVAL '30 seconds'", user.Id)
	if err != nil {
		log.Println("Failed to clear stale sessions:", err)
	}*/

	//generate short-lived access token
	accessToken, err := h.generateAccessToken(user.Id, user.Username)
	if err != nil {
		log.Println("JWT Generation Error:", err)

		http.Error(w, "Failed to generate access token", http.StatusInternalServerError)
		return
	}

	//generate long-lived refresh token
	refreshToken, err := h.generateRefreshToken(user.Id, user.Username)
	if err != nil {
		log.Println("JWT Generation Error:", err)

		http.Error(w, "Failed to generate refresh token", http.StatusInternalServerError)
		return
	}

	ctx := context.Background()
	activeSessionKey := fmt.Sprintf("session_active:%d", user.Id)

	exists, err := h.Redis.Exists(ctx, activeSessionKey).Result()
	if err != nil {
		log.Println("Redis error checking session:", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if exists > 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(`{"error": "Account is already logged in somewhere."}`))
		return
	}

	err = h.Redis.Set(ctx, activeSessionKey, "active", time.Second*45).Err()
	if err != nil {
		log.Println("Redis error setting active session lock:", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	//redisKey := fmt.Sprintf("user:refresh_token:%d", user.Id)

	//refreshKey := fmt.Sprintf("session:refresh:%d", user.Id)
	expiresAt := time.Now().Add(time.Hour * 24 * 7) //redis here

	/*err = a.Redis.Set(ctx, redisKey, refreshToken, time.Hour*24*7).Err()
	if err != nil {
		log.Println("Redis error saving session:", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}*/

	//remove this
	/*err = a.Redis.Set(ctx, activeSession, "active", time.Second*30).Err()
	if err != nil {
		log.Println("Redis error setting presense")
		return
	}*/

	/*_, err = a.DB.Exec(
		"INSERT INTO sessions (user_id, token_hash, expires_at) VALUES ($1, $2, $3)",
		user.Id, refreshToken, expiresAt,
	)
	if err != nil {
		log.Println("Database error saving refresh token:", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}*/

	/*http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token_swing",
		Value:    refreshToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: sameSiteMode,
		Expires:  expiresAt,
	})*/

	SetCookie(w, "refresh_token_swing", refreshToken, expiresAt)

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

func HandleViewerMode(w http.ResponseWriter, r *http.Request) {

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Only Post is allowed", http.StatusMethodNotAllowed)
		return
	}

	name := generateNames()

	accessToken, err := generateViewerAccessToken(name)
	if err != nil {
		http.Error(w, "Failed to generate viewer session", http.StatusInternalServerError)
		return
	}

	/*http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token_swing",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})*/

	ClearCookie(w, "refresh_token_swing")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"username":     name,
		"role":         "viewer",
		"access_token": accessToken,
	})
}

func (h *Handler) HandleRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cookie, err := r.Cookie("refresh_token_swing")
	if err != nil {
		if err == http.ErrNoCookie {
			http.Error(w, "[handleRefresh]: Unauthorized: No refresh token cookie", http.StatusUnauthorized)
			return
		}
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	refreshTokenString := cookie.Value

	if h.JwtRefreshSecret == "" {
		http.Error(w, "[handleRefresh]: Cant find JWT_REFRESH", 0)
		return
	}

	token, err := jwt.Parse(refreshTokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(h.JwtRefreshSecret), nil
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

	userUsernameString, ok := claims["username"].(string)
	if !ok {
		http.Error(w, "Invalid usernmae in the token claims", http.StatusUnauthorized)
		return
	}
	username := userUsernameString

	newAccessToken, err := h.generateAccessToken(userID, username)
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

func (h *Handler) HandleRefreshToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cookie, err := r.Cookie("refresh_token_swing")
	if err != nil {
		http.Error(w, "Missing refresh token cookie", http.StatusUnauthorized)
		return
	}

	claims, err := ParseToken(cookie.Value, h.JwtAccessSecret)
	if err != nil {
		http.Error(w, "Invalid or expired refresh token", http.StatusUnauthorized)
		return
	}

	userIDFloat, ok := claims["user_id"].(float64)
	if !ok {
		http.Error(w, "Invalid user ID in the token claims", http.StatusUnauthorized)
	}
	userID := int(userIDFloat)

	refreshToken := cookie.Value

	var expiresAt time.Time
	query := "SELECT user_id, expires_at FROM sessions WHERE token_hash = $1"
	err = h.DB.QueryRow(query, refreshToken).Scan(&userID, &expiresAt)

	if err == sql.ErrNoRows || time.Now().After(expiresAt) {
		http.Error(w, "Invalid or expires refresh_token (Session terminated)", http.StatusUnauthorized)
		return
	} else if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	userUsernameString, ok := claims["username"].(string)
	if !ok {
		http.Error(w, "Invalid usernmae in the token claims", http.StatusUnauthorized)
		return
	}
	username := userUsernameString

	newAccessToken, err := h.generateAccessToken(userID, username)

	if err != nil {
		http.Error(w, "Failed to generate access token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"access_token": newAccessToken,
	})
}

func GenerateRandomString(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func (h *Handler) HandleGetProfile(w http.ResponseWriter, r *http.Request) {

	userId := r.Context().Value(UserIDKey)

	fmt.Printf("DEBUG: Raw userID from context: %v (type: %T)\n", userId, userId)

	claims, _ := r.Context().Value(ClaimsKey).(jwt.MapClaims)

	var contextUsername string
	if claims != nil {
		if uname, ok := claims["username"].(string); ok {
			contextUsername = uname
		}
	}

	if userId == nil || userId == 0 {
		usernameToUse := "VIEWER"
		if contextUsername != "" {
			usernameToUse = contextUsername
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id":          0,
			"username":    usernameToUse,
			"logic_score": 0,
			"role":        "viewer",
		})
		return
	}

	var username string
	var logicScore int
	var role string
	query := `SELECT username, logic_score, role FROM users WHERE id = $1`

	err := h.DB.QueryRowContext(r.Context(), query, userId).Scan(&username, &logicScore, &role)
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

	wo1 := w1[mrand.IntN(len(w1))]
	wo2 := w2[mrand.IntN(len(w2))]

	suffix, err := gonanoid.Generate("abcdefghijklmnopqrstuvwxyz0123456789", 4)
	if err != nil {
		suffix = fmt.Sprintf("%d", mrand.IntN(9000)+1000)
	}

	return fmt.Sprintf("%s%s%s", wo1, wo2, suffix)
}

func (h *Handler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	userID, ok := r.Context().Value(UserIDKey).(int)

	if !ok || userID <= 0 {
		cookie, err := r.Cookie("refresh_token_swing")
		if err == nil && cookie.Value != "" {
			claims, err := ParseToken(cookie.Value, h.JwtAccessSecret)
			if err == nil {
				if userIdFloat, ok := claims["user_id"].(float64); ok {
					userID = int(userIdFloat)
				}
			}
		}
	}

	if userID > 0 {
		presenceKey := fmt.Sprintf("presence:user:%d", userID)
		refreshKey := fmt.Sprintf("session:refresh:%d", userID)

		err := h.Redis.Del(ctx, presenceKey, refreshKey).Err()
		if err != nil {
			log.Println("Redis error clearing session on logout:", err)
		}

		/*_, err := a.DB.Exec("DELETE FROM sessions WHERE user_id = $1", userID)
		if err != nil {
			log.Println("Database error clearing sessions on logout:", err)
		}*/
	} /* else {
		cookie, err := r.Cookie("refresh_token_swing")
		if err == nil {
			a.DB.Exec("DELETE FROM sessions WHERE token_hash = $1", cookie.Value)
		}
	}*/

	claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims)

	fmt.Printf("DEBUG CLAIMS: %v (type: %T)\n", claims, claims)

	var username string
	if ok {
		if name, exist := claims["username"]; exist {
			username = name.(string)
		}
	}

	fmt.Printf("Out -> Name: %s, ID: %d\n", username, userID)

	/*http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token_swing",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
	})*/
	ClearCookie(w, "refresh_token_swing")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Logged out successfully"})
}
