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
	"strconv"

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

type ArgumentInput struct {
	Title   string `json:"title"`
	Content string `json:"content"`
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

type ArgumentResponse struct {
	ID         int             `json:"id"`
	Author     string          `json:"author"`
	Title      string          `json:"title"`
	Content    string          `json:"content"`
	LogicScore int             `json:"logic_score"`
	CreatedAt  string          `json:"created_at"`
	Comments   []*CommentInput `json:"comments"`
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

func (a *App) handleCreateArgument(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := r.Context().Value(userIDKey).(int)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var input ArgumentInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.Content == "" {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	var username string
	err := a.DB.QueryRow("SELECT username from users WHERE id = $1", userID).Scan(&username)
	if err != nil {
		log.Printf("Failed to fetch username for user %d: %v", userID, err)
		http.Error(w, "user profile error", http.StatusInternalServerError)
		return
	}

	var score int = 75
	var reviewContent string = ""
	var botCommentID int = 0
	var botCommentCreatedAt string = ""
	/*botResponseText, err := CallBotAgent(input.Title, input.Content)//needs to 
	//score here
	var reviewContent = "Automated security audit failed to generate review."
	var score int = 50
	if err == nil {
		var audit AuditResponse
		cleanedJSON := cleanJSONResponse(botResponseText)
		if json.Unmarshal([]byte(cleanedJSON), &audit) == nil {
			score = audit.Score
			reviewContent = audit.Review
		} else {
			reviewContent = botResponseText
		}
	} else {
		log.Printf("Bot agent call error: %v", err)
	}*/

	query := `
		INSERT INTO arguments (title, content, user_id, logic_score, author) 
		VALUES ($1, $2, $3, $4, $5) 
		RETURNING id, logic_score, created_at`

	var id int
	var logicScore int
	var createdAt string

	err = a.DB.QueryRow(query, input.Title, input.Content, userID, score, username).Scan(&id, &logicScore, &createdAt)
	if err != nil {
		log.Printf("Database insert error: %v", err)
		http.Error(w, "Failed to save argument", http.StatusInternalServerError)
		return
	}

	/*var botCommentID int
	var botCommentCreatedAt string
	if reviewContent != "" {
		commentQuery := `INSERT INTO comments (argument_id, user_id, author, content) VALUES ($1, NULL, $2, $3) RETURNING id, created_at`
		err = a.DB.QueryRow(commentQuery, id, "AI Auditor", reviewContent).Scan(&botCommentID, &botCommentCreatedAt)
		if err != nil {
			log.Printf("Failed to save bot review comment: %v", err)
		}
	}*/

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	var initialComments []interface{}// slice of empty array
	if reviewContent != "" {
		initialComments = []interface{}{
			map[string]interface{}{
				"id":        fmt.Sprintf("%d", botCommentID),
				"author":    "AI Auditor",
				"content":   reviewContent,
				"timestamp": botCommentCreatedAt,
				"replies":   []interface{}{},
			},
		}
	}

	newArg := map[string]interface{}{
		"id":          id,
		"author":      username,
		"title":       input.Title,
		"content":     input.Content,
		"logic_score": logicScore,
		"created_at":  createdAt,
		"comments":    initialComments,
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":     "Saved",
		"id":          id,
		"logic_score": logicScore,
		"author":      username,
		"created_at":  createdAt,
		"title": input.Title,
		"content": input.Content,
		"comments": initialComments,
	})

	msg, _ := json.Marshal(map[string]interface{}{
		"type":    "NEW_ARGUMENT",
		"payload": newArg,
	})
	a.hub.broadcast <- msg
}

/*botResponse, err := CallBotAgent(input.Content)
if err != nil {
	fmt.Println("Bot failed to respond:", err)
} else {
	commentQuery := `INSERT INTO comments (argument_id, content, user_id) VALUES ($1, $2, $3)`

	_, err = a.DB.Exec(commentQuery, id, botResponse, 0)
	if err != nil {
		log.Printf("Failed to save bot comment: %v", err)
	}
}*/

func (a *App) handleGetArguments(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    var currentUserID int64 = 0
    authHeader := r.Header.Get("Authorization")
    if authHeader != "" {
        parts := strings.SplitN(authHeader, " ", 2)
        if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
            if id, err := a.parseToken(parts[1]); err == nil {
                currentUserID = int64(id)
            }
        }
    }

    query := `
        SELECT id, user_id, title, content, logic_score, author, created_at 
        FROM arguments 
        ORDER BY created_at DESC
    `
    rows, err := a.DB.Query(query)
    if err != nil {
        log.Printf("Fetch error: %v", err)
        http.Error(w, "Failed to fetch arguments", http.StatusInternalServerError)
        return
    }
    defer rows.Close()

    arguments := []ArgumentResponse{}

    for rows.Next() {
        var arg ArgumentResponse
        var rawUserID sql.NullInt32
        var authorNull sql.NullString

        if err := rows.Scan(&arg.ID, &rawUserID, &arg.Title, &arg.Content, &arg.LogicScore, &authorNull, &arg.CreatedAt); err != nil {
            log.Printf("Scan error on argument row: %v", err)
            continue
        }

        if authorNull.Valid && authorNull.String != "" {
            arg.Author = authorNull.String
        } else {
            arg.Author = "ANONYMOUS_DEV"
        }

        commentQuery := `
            SELECT c.id, c.user_id, c.parent_id, c.content, c.author, c.created_at, c.score,
                   (SELECT COUNT(*) FROM comment_reactions cr WHERE cr.comment_id = c.id AND cr.reaction_type = 'fire') AS fire_count,
                   EXISTS(SELECT 1 FROM comment_reactions cr WHERE cr.comment_id = c.id AND cr.user_id = $2 AND cr.reaction_type = 'fire') AS user_has_fired
            FROM comments c 
            WHERE c.argument_id = $1 
            ORDER BY c.created_at ASC
        `
        commentRows, err := a.DB.Query(commentQuery, arg.ID, currentUserID)
        if err != nil {
            arg.Comments = []*CommentInput{}
            arguments = append(arguments, arg)
            continue
        }

        var flatComments []CommentInput
        for commentRows.Next() {
            var cID int64
            var cUserID sql.NullInt32
            var parentID sql.NullInt64
            var content string
            var authorNull sql.NullString
            var createdAt time.Time
            var score int
            var fireCount int64
            var userHasFired bool

            if err := commentRows.Scan(&cID, &cUserID, &parentID, &content, &authorNull, &createdAt, &score, &fireCount, &userHasFired); err == nil {
                var pID *int64
                if parentID.Valid {
                    val := parentID.Int64
                    pID = &val
                }

                var authorStr string
                if cUserID.Valid {
                    authorStr = authorNull.String
                } else {
                    authorStr = "BOT_REVIEWER"
                }

                var actualUserID int64
                if cUserID.Valid {
                    actualUserID = int64(cUserID.Int32)
                }

                formattedTime := createdAt.Format("2006-01-02 15:04:05")

                flatComments = append(flatComments, CommentInput{
                    ID:           fmt.Sprintf("%d", cID),
                    UserID:       actualUserID,
                    ParentID:     pID,
                    Author:       authorStr,
                    Content:      content,
                    Timestamp:    formattedTime,
                    Score:        score,
                    FireCount:    int(fireCount),
                    UserHasFired: userHasFired,
                    Replies:      []*CommentInput{},
                })
            } else {
                log.Printf("Comment scan error: %v", err)
            }
        }
        commentRows.Close()

        commentMap := make(map[string]*CommentInput)
        var rootComments []*CommentInput

        for i := range flatComments {
            commentMap[flatComments[i].ID] = &flatComments[i]
        }

        for i := range flatComments {
            comment := &flatComments[i]
            if comment.ParentID == nil {
                rootComments = append(rootComments, comment)
            } else {
                parentIDStr := fmt.Sprintf("%d", *comment.ParentID)
                if parent, exists := commentMap[parentIDStr]; exists {
                    parent.Replies = append(parent.Replies, comment)
                } else {
                    rootComments = append(rootComments, comment)
                }
            }
        }

        if rootComments == nil {
            arg.Comments = []*CommentInput{}
        } else {
            arg.Comments = rootComments
        }

        arguments = append(arguments, arg)
    }

    if arguments == nil {
        arguments = []ArgumentResponse{}
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(arguments)
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

	query := `INSERT INTO users (email, email_hash, password_hash, username) VALUES ($1, $2, $3, $4) RETURNING id, created_at`
	var id int
	var createdAt time.Time

	err = a.DB.QueryRow(query, secureEmail, decryptEmail, hashedPassword, name).Scan(&id, &createdAt)
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

type CommentInput struct {
    ID           string          `json:"id,omitempty"`
    ArgumentID   int64           `json:"argument_id"`
    UserID       int64           `json:"user_id"` 
    ParentID     *int64          `json:"parent_id,omitempty"`
    Content      string          `json:"content,omitempty"`
    Author       string          `json:"author,omitempty"`
    Timestamp    string          `json:"timestamp,omitempty"`
    Score        int             `json:"score"`
    FireCount    int             `json:"fire_count"`
    UserHasFired bool            `json:"user_has_fired"`
    Replies      []*CommentInput `json:"replies,omitempty"`
}

type FireReactionRequest struct{
	CommentID int64 `json:"comment_id"`
}

func (a *App) handleFireReaction(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    userIDVal := r.Context().Value(userIDKey) 
    if userIDVal == nil {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }
    
    var userID int64
    switch v := userIDVal.(type) {
    case int:
        userID = int64(v)
    case int64:
        userID = v
    case float64:
        userID = int64(v)
    default:
        http.Error(w, "Invalid user context", http.StatusUnauthorized)
        return
    }

    var req FireReactionRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid payload", http.StatusBadRequest)
        return
    }

    tx, err := a.DB.Begin()
    if err != nil {
        http.Error(w, "Database error", http.StatusInternalServerError)
        return
    }
    defer tx.Rollback()

    var argumentID, commentUserID int64
    var isTriggered bool
    err = tx.QueryRow(`
        SELECT c.argument_id, c.user_id, c.is_fire_triggered 
        FROM comments c WHERE c.id = $1`,
        req.CommentID,
    ).Scan(&argumentID, &commentUserID, &isTriggered)
    if err != nil {
        http.Error(w, "Comment not found", http.StatusNotFound)
        return
    }

    if userID == commentUserID {
        http.Error(w, "Cannot react to your own comment", http.StatusBadRequest)
        return
    }

    var exists bool
    err = tx.QueryRow(`
        SELECT EXISTS(
            SELECT 1 FROM comment_reactions 
            WHERE comment_id = $1 AND user_id = $2 AND reaction_type = 'fire'
        )`, req.CommentID, userID).Scan(&exists)
    if err != nil {
        http.Error(w, "Database error", http.StatusInternalServerError)
        return
    }

    if exists {
        _, err = tx.Exec(`
            DELETE FROM comment_reactions 
            WHERE comment_id = $1 AND user_id = $2 AND reaction_type = 'fire'`,
            req.CommentID, userID,
        )
    } else {
        _, err = tx.Exec(`
            INSERT INTO comment_reactions (comment_id, user_id, reaction_type) 
            VALUES ($1, $2, 'fire')`,
            req.CommentID, userID,
        )
    }

    if err != nil {
        println("DB Toggle Error:", err.Error())
        http.Error(w, "Failed to update reaction", http.StatusInternalServerError)
        return
    }

    if !exists && !isTriggered {
        var totalComments, fireVotes int64
        tx.QueryRow(`SELECT COUNT(*) FROM comments WHERE argument_id = $1`, argumentID).Scan(&totalComments)
        tx.QueryRow(`SELECT COUNT(*) FROM comment_reactions WHERE comment_id = $1 AND reaction_type = 'fire'`, req.CommentID).Scan(&fireVotes)

        if totalComments > 0 && (float64(fireVotes)/float64(totalComments)) >= 0.40 {
            tx.Exec(`UPDATE comments SET is_fire_triggered = TRUE WHERE id = $1`, req.CommentID)

            var creatorID int64
            tx.QueryRow(`SELECT user_id FROM arguments WHERE id = $1`, argumentID).Scan(&creatorID)

            tx.Exec(`UPDATE users SET logic_score = logic_score + 50 WHERE id = $1`, commentUserID)
            tx.Exec(`UPDATE users SET logic_score = logic_score - 10 WHERE id = $1`, creatorID)
            tx.Exec(`UPDATE arguments SET logic_score = logic_score - 10 WHERE id = $1`, argumentID)
            tx.Exec(`UPDATE comments SET score = score - 5 WHERE argument_id = $1 AND id != $2`, argumentID, req.CommentID)
        }
    }

    if err := tx.Commit(); err != nil {
        http.Error(w, "Transaction commit failed", http.StatusInternalServerError)
        return
    }

    var newFireCount int
    a.DB.QueryRow(`SELECT COUNT(*) FROM comment_reactions WHERE comment_id = $1 AND reaction_type = 'fire'`, req.CommentID).Scan(&newFireCount)

    if a.hub != nil && a.hub.broadcast != nil {
        broadcastData, _ := json.Marshal(map[string]interface{}{
            "type": "SWING_SCORE_UPDATE",
            "payload": map[string]interface{}{
                "comment_id":  req.CommentID,
                "argument_id": argumentID,
                "fire_count":  newFireCount,
				"user_id": userID,
            },
        })
        a.hub.broadcast <- broadcastData
    }

    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "swing scoring processed",
		"fire_count": newFireCount,
	})
}

func (a *App) handleCreateComment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := r.Context().Value(userIDKey).(int)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var input CommentInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.Content == "" {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	var author string
	err := a.DB.QueryRow("SELECT username FROM users WHERE id = $1", userID).Scan(&author)
	if err != nil {
		http.Error(w, "User not found", http.StatusInternalServerError)
		return
	}

	query := `
        INSERT INTO comments (argument_id, parent_id, user_id, content, author)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING id, created_at`

	var id int64
	var createdAt time.Time

	err = a.DB.QueryRow(query, input.ArgumentID, input.ParentID, userID, input.Content, author).Scan(&id, &createdAt)
	if err != nil {
		log.Printf("Database insert error: %v", err)
		http.Error(w, "Failed to save comment", http.StatusInternalServerError)
		return
	}

	formattedTime := createdAt.Format("2006-01-02 15:04:05")

	newComment := map[string]interface{}{
        "id":          fmt.Sprintf("%d", id),
        "argument_id": input.ArgumentID,
        "parent_id":   input.ParentID,
        "content":     input.Content,
        "author":      author,
        "timestamp":   formattedTime,
        "user_id":     userID,
        "replies":     []interface{}{},
    }

	msg, _ := json.Marshal(map[string]interface{}{
		"type":    "NEW_COMMENT",
		"payload": newComment,
	})
	a.hub.broadcast <- msg

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":    "Comment saved",
		"id":         id,
		"created_at": formattedTime,
		"author":     author,
		"user_id": userID,
	})
}

func (a *App) handleGetComments(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    argIDStr := r.URL.Query().Get("argument_id")
    if argIDStr == "" {
        http.Error(w, "Missing argument_id", http.StatusBadRequest)
        return
    }

    argumentID, err := strconv.ParseInt(argIDStr, 10, 64)
    if err != nil {
        http.Error(w, "Invalid argument_id format", http.StatusBadRequest)
        return
    }

    var currentUserID int64 = 0
    authHeader := r.Header.Get("Authorization")
    if authHeader != "" {
        parts := strings.SplitN(authHeader, " ", 2)
        if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
            if id, err := a.parseToken(parts[1]); err == nil {
                currentUserID = int64(id)
            }
        }
    }

    query := `
        SELECT c.id, c.content, c.user_id, c.argument_id, c.score, c.is_fire_triggered,
               (SELECT COUNT(*) FROM comment_reactions cr WHERE cr.comment_id = c.id AND cr.reaction_type = 'fire') AS fire_count,
               EXISTS(SELECT 1 FROM comment_reactions cr WHERE cr.comment_id = c.id AND cr.user_id = $2 AND cr.reaction_type = 'fire') AS user_has_fired
        FROM comments c 
        WHERE c.argument_id = $1
        ORDER BY c.created_at ASC
    `

    rows, err := a.DB.Query(query, argumentID, currentUserID)
    if err != nil {
        http.Error(w, "Database error", http.StatusInternalServerError)
        return
    }
    defer rows.Close()

    type CommentResponse struct {
        ID              int64  `json:"id"`
        Content         string `json:"content"`
        UserID          int64  `json:"user_id"`
        ArgumentID      int64  `json:"argument_id"`
        Score           int    `json:"score"`
        IsFireTriggered bool   `json:"is_fire_triggered"`
        FireCount       int64  `json:"fire_count"`
        UserHasFired    bool   `json:"user_has_fired"`
    }

    var comments []CommentResponse
    for rows.Next() {
        var c CommentResponse
        if err := rows.Scan(&c.ID, &c.Content, &c.UserID, &c.ArgumentID, &c.Score, &c.IsFireTriggered, &c.FireCount, &c.UserHasFired); err != nil {
            continue 
        }
        comments = append(comments, c)
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(comments)
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
}

type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	mu         sync.Mutex
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
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

    client := &Client{
        connection: connection,
        send:       make(chan []byte, 256),
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

func (h *Hub) Run() {
	for {
		select {
		case client := <- h.register:
			h.mu.Lock()//grab and lock
			h.clients[client] = true//wait
			h.mu.Unlock()//unlock
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {//check if there is clients inside Hub
				delete(h.clients, client)//delete
				close(client.send)//close
			}
			h.mu.Unlock()
		case message := <-h.broadcast:
			h.mu.Lock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
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