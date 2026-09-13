package main

import (
	"fmt"
    "encoding/json"
    "log"
    "net/http"
    "strconv"
    "strings"
    "time"
)

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
            var creatorID int64
            tx.QueryRow(`SELECT user_id FROM arguments WHERE id = $1`, argumentID).Scan(&creatorID)

            if err := a.ApplyConsensusDeflationTx(tx, req.CommentID, argumentID, creatorID, commentUserID); err != nil {
                println("Deflation Error Details:", err.Error())
                http.Error(w, "Failed to process consensus deflation: "+err.Error(), http.StatusInternalServerError)
                return
            }
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

    var ownerID int
    err = a.DB.QueryRow("SELECT user_id FROM arguments WHERE id = $1", input.ArgumentID).Scan(&ownerID)
    if err != nil {
        log.Printf("Failed to find argument owner: %v", err)
    }

    var notifID int
    var notifCreatedAt time.Time
    if err == nil && ownerID != userID {
        err = a.DB.QueryRow(`INSERT INTO notif (comment_id, user_id) VALUES ($1, $2) RETURNING id, create_at`, id, ownerID).Scan(&notifID, &notifCreatedAt)
        if err == nil {
            a.hub.mu.Lock()
            client, ok := a.hub.clients[ownerID]
            a.hub.mu.Unlock()

            if ok {
                notifMsg, _ := json.Marshal(map[string]interface{}{
                    "type": "NEW_NOTIFICATION",
                    "payload": map[string]interface{}{
                        "id": notifID,
                        "comment_id": id,
                        "created_at": notifCreatedAt.Format("2006-01-02 15:04:05"),
                    },
                })
                select {
                case client.send <- notifMsg:
                default:

                }
            }
        }
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