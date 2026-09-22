package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

type ArgumentInput struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

type ArgumentResponse struct {
	ID         int             `json:"id"`
	Author     string          `json:"author"`
	Title      string          `json:"title"`
	Content    string          `json:"content"`
	LogicScore int             `json:"logic_score"`
	PlanScore  float64         `json:"plan_score"`
	IsWatched  bool            `json:"is_watched"`
	CreatedAt  string          `json:"created_at"`
	Comments   []*CommentInput `json:"comments"`
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

	var score int = 0
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

	var initialComments []interface{} // slice of empty array
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
		"title":       input.Title,
		"content":     input.Content,
		"comments":    initialComments,
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

			if claims, err := a.parseToken(parts[1]); err == nil {

				if userIDFloat, ok := claims["user_id"].(float64); ok {
					currentUserID = int64(userIDFloat)
				}
			}
		}
	}

	query := `
        SELECT a.id, a.user_id, a.title, a.content, a.logic_score, a.author, a.created_at,
		a.logic_score AS plan_score,
		CASE WHEN w.user_id IS NOT NULL THEN true ELSE false END AS is_watched
        FROM arguments a
		LEFT JOIN watchlist w ON w.argument_id = a.id AND w.user_id = $1
        ORDER BY a.created_at DESC
    `
	rows, err := a.DB.Query(query, currentUserID)
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

		if err := rows.Scan(&arg.ID, &rawUserID, &arg.Title, &arg.Content, &arg.LogicScore, &authorNull, &arg.CreatedAt, &arg.PlanScore, &arg.IsWatched); err != nil {
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

		if err := commentRows.Err(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
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

	if err := rows.Err(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if arguments == nil {
		arguments = []ArgumentResponse{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(arguments)
}

func (a *App) handleTopArguments(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var currentUserID int64 = 0
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			if claims, err := a.parseToken(parts[1]); err == nil {
				if userIDFloat, ok := claims["user_id"].(float64); ok {
					currentUserID = int64(userIDFloat)
				}
			}
		}
	}

	query := `
        SELECT 
            a.id, a.user_id, a.title, a.content, a.logic_score, a.author, a.created_at, a.logic_score AS plan_score,
			CASE WHEN w.user_id IS NOT NULL THEN true ELSE false END AS is_watched
        FROM arguments a
		LEFT JOIN watchlist w ON w.argument_id = a.id AND w.user_id = $1
        ORDER BY a.logic_score DESC
    `

	rows, err := a.DB.Query(query, currentUserID)
	if err != nil {
		log.Printf("Top arguments fetch error: %v", err)
		http.Error(w, "Failed to fetch top arguments", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	arguments := []ArgumentResponse{}

	for rows.Next() {
		var arg ArgumentResponse
		var rawUserID sql.NullInt32
		var authorNull sql.NullString

		if err := rows.Scan(&arg.ID, &rawUserID, &arg.Title, &arg.Content, &arg.LogicScore, &authorNull, &arg.CreatedAt, &arg.PlanScore, &arg.IsWatched); err != nil {
			log.Printf("Scan error on top argument row: %v", err)
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
			var cAuthorNull sql.NullString
			var createdAt time.Time
			var score int
			var fireCount int64
			var userHasFired bool

			if err := commentRows.Scan(&cID, &cUserID, &parentID, &content, &cAuthorNull, &createdAt, &score, &fireCount, &userHasFired); err == nil {
				var pID *int64
				if parentID.Valid {
					val := parentID.Int64
					pID = &val
				}

				var authorStr string
				if cUserID.Valid {
					authorStr = cAuthorNull.String
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
			}
		}
		if err := commentRows.Err(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
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
	if err := rows.Err(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if arguments == nil {
		arguments = []ArgumentResponse{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(arguments)
}

func (a *App) handleCreateWatchlist(w http.ResponseWriter, r *http.Request) {
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

	var req struct {
		ArgumentID int64 `json:"argument_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	var exists bool
	err := a.DB.QueryRow(`
		SELECT EXISTS(
			SELECT 1 from watchlist
			WHERE user_id = $1 AND argument_id = $2
		)`, userID, req.ArgumentID).Scan(&exists)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	var actionStatus string
	if exists {
		_, err = a.DB.Exec(`
			DELETE FROM watchlist
			WHERE user_id = $1 AND argument_id = $2`,
			userID, req.ArgumentID,
		)
		actionStatus = "unwatched"
	} else {
		_, err = a.DB.Exec(`
			INSERT INTO watchlist (user_id, argument_id)
			VALUES ($1,$2)`,
			userID, req.ArgumentID,
		)
		actionStatus = "watched"
	}

	if err != nil {
		http.Error(w, "Failed to update watchlist", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": actionStatus,
	})
}

func (a *App) handleGetWatchlist(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
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

	query := `
		SELECT
			a.id, a.user_id, a.title, a.content, a.logic_score, a.author, a.created_at, a.logic_score AS plan_score, true AS is_watched
		FROM arguments a
		JOIN watchlist w ON w.argument_id = a.id
		WHERE w.user_id = $1
		ORDER BY a.logic_score DESC
	`
	rows, err := a.DB.Query(query, userID)
	if err != nil {
		log.Printf("Watchlist fetch error: %v", err)
		http.Error(w, "Failed to fetch watchlist", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	arguments := []ArgumentResponse{}

	for rows.Next() {
		var arg ArgumentResponse
		var rawUserID sql.NullInt32
		var authorNull sql.NullString

		if err := rows.Scan(&arg.ID, &rawUserID, &arg.Title, &arg.Content, &arg.LogicScore, &authorNull, &arg.CreatedAt, &arg.PlanScore, &arg.IsWatched); err != nil {
			log.Printf("Scan error on watchlist row: %v", err)
			continue
		}

		if authorNull.Valid && authorNull.String != "" {
			arg.Author = authorNull.String
		} else {
			arg.Author = "ANONYMOUS_DEV"
		}

		arg.Comments = []*CommentInput{}
		arguments = append(arguments, arg)
	}

	if err := rows.Err(); err != nil {
		http.Error(w, "Error iterating argument rows", http.StatusInternalServerError)
		return
	}

	if arguments == nil {
		arguments = []ArgumentResponse{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(arguments)
}
