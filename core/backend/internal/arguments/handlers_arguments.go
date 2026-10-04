package arguments

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/lib/pq"

	"vaine-backend/internal/auth"

	"github.com/redis/go-redis/v9"
)

type ArgumentInput struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

type ArgumentResponse struct {
	ID         int                `json:"id"`
	Author     string             `json:"author"`
	Title      string             `json:"title"`
	Content    string             `json:"content"`
	LogicScore int                `json:"logic_score"`
	PlanScore  float64            `json:"plan_score"`
	IsWatched  bool               `json:"is_watched"`
	CreatedAt  string             `json:"created_at"`
	Comments   []*CommentResponse `json:"comments"`
}

type CommentResponse struct {
	ID           string             `json:"id"`
	UserID       int64              `json:"user_id"`
	ParentID     *int64             `json:"parent_id"`
	Content      string             `json:"content"`
	Author       string             `json:"author"`
	Timestamp    string             `json:"timestamp"`
	Score        int                `json:"score"`
	FireCount    int                `json:"fire_count"`
	UserHasFired bool               `json:"user_has_fired"`
	Replies      []*CommentResponse `json:"replies,omitempty"`
}

type ArgumentHandler struct {
	DB              *sql.DB
	Redis           *redis.Client
	JwtAccessSecret string
	Broadcast       chan<- []byte
}

func NewArgumentHandler(db *sql.DB, redis *redis.Client, jwtSecret string, broadcast chan<- []byte) *ArgumentHandler {
	return &ArgumentHandler{
		DB:              db,
		Redis:           redis,
		JwtAccessSecret: jwtSecret,
		Broadcast:       broadcast,
	}
}

func (aH *ArgumentHandler) HandleCreateArgument(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := r.Context().Value(auth.UserIDKey).(int)
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
	err := aH.DB.QueryRow("SELECT username from users WHERE id = $1", userID).Scan(&username)
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

	err = aH.DB.QueryRow(query, input.Title, input.Content, userID, score, username).Scan(&id, &logicScore, &createdAt)
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

	ctx := r.Context()

	//_ = a.Redis.Del(ctx, "arguments:feed", "argument:trending").Err()

	feedpattern := "arguments:feed:user:*"
	iterFeed := aH.Redis.Scan(ctx, 0, feedpattern, 0).Iterator()
	for iterFeed.Next(ctx) {
		_ = aH.Redis.Del(ctx, iterFeed.Val())
	}

	topPattern := "arguments:top:user:*"
	iterTop := aH.Redis.Scan(ctx, 0, topPattern, 0).Iterator()
	for iterTop.Next(ctx) {
		_ = aH.Redis.Del(ctx, iterTop.Val())
	}

	_ = aH.Redis.Del(ctx, "argument:trending").Err()

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
	aH.Broadcast <- msg
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

func (aH *ArgumentHandler) HandleGetArguments(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	page := 1
	limit := 20

	if pStr := r.URL.Query().Get("page"); pStr != "" {
		if p, err := strconv.Atoi(pStr); err == nil && p > 0 {
			page = p
		}
	}
	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	ctx := r.Context()

	if targetArgIdStr := r.URL.Query().Get("target_argument_id"); targetArgIdStr != "" {
		if targetID, err := strconv.ParseInt(targetArgIdStr, 10, 64); err == nil {
			var position int

			calcQuery := `
				SELECT COUNT(*)
				FROM arguments
				WHERE created_at >= (SELECT created_at FROM arguments WHERE id = $1)
			`

			err := aH.DB.QueryRowContext(ctx, calcQuery, targetID).Scan(&position)
			if err == nil && position > 0 {
				page = (position + limit - 1) / limit
			}
		}
	}
	offset := (page - 1) * limit

	var currentUserID int64 = 0
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {

			if claims, err := auth.ParseToken(parts[1], aH.JwtAccessSecret); err == nil {

				if userIDFloat, ok := claims["user_id"].(float64); ok {
					currentUserID = int64(userIDFloat)
				}
			}
		}
	}

	//ctx := r.Context()

	cacheKey := fmt.Sprintf("arguments:feed:user:%d:page:%d:limit:%d", currentUserID, page, limit)

	cachedData, err := aH.Redis.Get(ctx, cacheKey).Bytes()
	if err == nil && len(cachedData) > 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(cachedData)
		return
	}

	var totalCount int
	err = aH.DB.QueryRowContext(ctx, "SELECT COUNT (*) FROM arguments").Scan(&totalCount)
	if err != nil {
		log.Printf("Count error: %v", err)
		totalCount = 0
	}

	query := `
        SELECT a.id, a.user_id, a.title, a.content, a.logic_score, a.author, a.created_at,
		a.logic_score AS plan_score,
		CASE WHEN w.user_id IS NOT NULL THEN true ELSE false END AS is_watched
        FROM arguments a
		LEFT JOIN watchlist w ON w.argument_id = a.id AND w.user_id = $1
        ORDER BY a.created_at DESC
		LIMIT $2 OFFSET $3
    `
	rows, err := aH.DB.Query(query, currentUserID, limit, offset)
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
		commentRows, err := aH.DB.Query(commentQuery, arg.ID, currentUserID)
		if err != nil {
			arg.Comments = []*CommentResponse{}
			arguments = append(arguments, arg)
			continue
		}

		var flatComments []CommentResponse
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

				flatComments = append(flatComments, CommentResponse{
					ID:           fmt.Sprintf("%d", cID),
					UserID:       actualUserID,
					ParentID:     pID,
					Author:       authorStr,
					Content:      content,
					Timestamp:    formattedTime,
					Score:        score,
					FireCount:    int(fireCount),
					UserHasFired: userHasFired,
					Replies:      []*CommentResponse{},
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

		commentMap := make(map[string]*CommentResponse)
		var rootComments []*CommentResponse

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
			arg.Comments = []*CommentResponse{}
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

	type PaginatedResponse struct {
		Arguments []ArgumentResponse `json:"arguments"`
		Total     int                `json:"total"`
	}

	responsePayload := PaginatedResponse{
		Arguments: arguments,
		Total:     totalCount,
	}

	responseBytes, err := json.Marshal(responsePayload)
	if err != nil {
		log.Printf("JSON marshal error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	_ = aH.Redis.Set(ctx, cacheKey, responseBytes, 60*time.Second).Err()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(responseBytes)
	//json.NewEncoder(w).Encode(arguments)
}

func (aH *ArgumentHandler) HandleTopArguments(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	page := 1
	limit := 20

	if pStr := r.URL.Query().Get("page"); pStr != "" {
		if p, err := strconv.Atoi(pStr); err == nil && p > 0 {
			page = p
		}
	}
	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}
	offset := (page - 1) * limit

	var currentUserID int64 = 0
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			if claims, err := auth.ParseToken(parts[1], aH.JwtAccessSecret); err == nil {
				if userIDFloat, ok := claims["user_id"].(float64); ok {
					currentUserID = int64(userIDFloat)
				}
			}
		}
	}

	ctx := r.Context()

	cacheKey := fmt.Sprintf("arguments:top:user:%d:page:%d:limit:%d", currentUserID, page, limit)

	cachedData, err := aH.Redis.Get(ctx, cacheKey).Bytes()
	if err == nil && len(cachedData) > 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(cachedData)
		return
	}

	var totalCount int
	err = aH.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM arguments").Scan(&totalCount)
	if err != nil {
		log.Printf("Count error: %v", err)
		totalCount = 0
	}

	query := `
        SELECT 
            a.id, a.user_id, a.title, a.content, a.logic_score, a.author, a.created_at, a.logic_score AS plan_score,
			CASE WHEN w.user_id IS NOT NULL THEN true ELSE false END AS is_watched
        FROM arguments a
		LEFT JOIN watchlist w ON w.argument_id = a.id AND w.user_id = $1
        ORDER BY a.logic_score DESC
		LIMIT $2 OFFSET $3
    `

	rows, err := aH.DB.Query(query, currentUserID, limit, offset)
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

		commentRows, err := aH.DB.Query(commentQuery, arg.ID, currentUserID)
		if err != nil {
			arg.Comments = []*CommentResponse{}
			arguments = append(arguments, arg)
			continue
		}

		var flatComments []CommentResponse
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

				flatComments = append(flatComments, CommentResponse{
					ID:           fmt.Sprintf("%d", cID),
					UserID:       actualUserID,
					ParentID:     pID,
					Author:       authorStr,
					Content:      content,
					Timestamp:    formattedTime,
					Score:        score,
					FireCount:    int(fireCount),
					UserHasFired: userHasFired,
					Replies:      []*CommentResponse{},
				})
			}
		}
		if err := commentRows.Err(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		commentRows.Close()

		commentMap := make(map[string]*CommentResponse)
		var rootComments []*CommentResponse

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
			arg.Comments = []*CommentResponse{}
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

	type PaginatedResponse struct {
		Arguments []ArgumentResponse `json:"arguments"`
		Total     int                `json:"total"`
	}

	responsePayload := PaginatedResponse{
		Arguments: arguments,
		Total:     totalCount,
	}

	responseBytes, err := json.Marshal(responsePayload)
	if err != nil {
		log.Printf("JSON marshal error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	_ = aH.Redis.Set(ctx, cacheKey, responseBytes, 60*time.Second).Err()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(responseBytes)
	//json.NewEncoder(w).Encode(arguments)
}

func (aH *ArgumentHandler) HandleCreateWatchlist(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userIDVal := r.Context().Value(auth.UserIDKey)
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
	err := aH.DB.QueryRow(`
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
		_, err = aH.DB.Exec(`
			DELETE FROM watchlist
			WHERE user_id = $1 AND argument_id = $2`,
			userID, req.ArgumentID,
		)
		actionStatus = "unwatched"
	} else {
		_, err = aH.DB.Exec(`
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

	ctx := r.Context()

	feedPattern := fmt.Sprintf("arguments:feed:user:%d:*", userID)
	iterFeed := aH.Redis.Scan(ctx, 0, feedPattern, 0).Iterator()
	for iterFeed.Next(ctx) {
		_ = aH.Redis.Del(ctx, iterFeed.Val())
	}

	topPattern := fmt.Sprintf("arguments:top:user:%d:*", userID)
	iterTop := aH.Redis.Scan(ctx, 0, topPattern, 0).Iterator()
	for iterTop.Next(ctx) {
		_ = aH.Redis.Del(ctx, iterTop.Val())
	}

	watchlistPattern := fmt.Sprintf("arguments:watchlist:user:%d:*", userID)
	iterWatchlist := aH.Redis.Scan(ctx, 0, watchlistPattern, 0).Iterator()
	for iterWatchlist.Next(ctx) {
		_ = aH.Redis.Del(ctx, iterWatchlist.Val())
	}

	_ = aH.Redis.Del(ctx,
		//fmt.Sprintf("arguments:feed:user:%d", userID),
		//fmt.Sprintf("arguments:top:user:%d", userID),
		//fmt.Sprintf("arguments:watchlist:user:%d", userID),
		fmt.Sprintf("argument:detail:%d", req.ArgumentID),
	).Err()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": actionStatus,
	})
}

func (aH *ArgumentHandler) HandleGetWatchlist(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userIDVal := r.Context().Value(auth.UserIDKey)
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

	page := 1
	limit := 20
	if pStr := r.URL.Query().Get("page"); pStr != "" {
		if p, err := strconv.Atoi(pStr); err == nil && p > 0 {
			page = p
		}
	}
	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}
	offset := (page - 1) * limit

	ctx := r.Context()

	cacheKey := fmt.Sprintf("arguments:watchlist:user:%d:page:%d:limit:%d", userID, page, limit)

	cachedData, err := aH.Redis.Get(ctx, cacheKey).Bytes()
	if err == nil && len(cachedData) > 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(cachedData)
		return
	}

	var totalCount int
	err = aH.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM watchlist WHERE user_id = $1", userID).Scan(&totalCount)
	if err != nil {
		log.Printf("Count error: %v", err)
		totalCount = 0
	}

	query := `
        SELECT
            a.id, a.user_id, a.title, a.content, a.logic_score, a.author, a.created_at, a.logic_score AS plan_score, true AS is_watched
        FROM arguments a
        JOIN watchlist w ON w.argument_id = a.id
        WHERE w.user_id = $1
        ORDER BY w.created_at DESC
		LIMIT $2 OFFSET $3
    `
	rows, err := aH.DB.Query(query, userID, limit, offset)
	if err != nil {
		log.Printf("Watchlist fetch error: %v", err)
		http.Error(w, "Failed to fetch watchlist", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	var argIDs []int64

	var arguments []ArgumentResponse

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

		arguments = append(arguments, arg)
		argIDs = append(argIDs, int64(arg.ID))

		/*commentQuery := `
		            SELECT c.id, c.user_id, c.parent_id, c.content, c.author, c.created_at, c.score,
		                   (SELECT COUNT(*) FROM comment_reactions cr WHERE cr.comment_id = c.id AND cr.reaction_type = 'fire') AS fire_count,
		                   EXISTS(SELECT 1 FROM comment_reactions cr WHERE cr.comment_id = c.id AND cr.user_id = $2 AND cr.reaction_type = 'fire') AS user_has_fired
		            FROM comments c
		            WHERE c.argument_id = $1
		            ORDER BY c.created_at ASC
		        `
				commentRows, err := a.DB.Query(commentQuery, arg.ID, userID)
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
					http.Error(w, "Error iterating comment rows", http.StatusInternalServerError)
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

				arguments = append(arguments, arg)*/
	}

	if err := rows.Err(); err != nil {
		http.Error(w, "Error iterating argument rows", http.StatusInternalServerError)
		return
	}

	if arguments == nil {
		arguments = []ArgumentResponse{}
	}

	if len(argIDs) > 0 {
		commentQuery := `
            SELECT c.id, c.argument_id, c.user_id, c.parent_id, c.content, c.author, c.created_at, c.score,
                   (SELECT COUNT(*) FROM comment_reactions cr WHERE cr.comment_id = c.id AND cr.reaction_type = 'fire') AS fire_count,
                   EXISTS(SELECT 1 FROM comment_reactions cr WHERE cr.comment_id = c.id AND cr.user_id = $2 AND cr.reaction_type = 'fire') AS user_has_fired
            FROM comments c 
            WHERE c.argument_id = ANY($1) 
            ORDER BY c.argument_id, c.created_at ASC
        `

		commentRows, err := aH.DB.Query(commentQuery, pq.Array(argIDs), userID)
		if err == nil {
			defer commentRows.Close()

			commentsByArg := make(map[int64][]CommentResponse)

			for commentRows.Next() {
				var cID int64
				var argID int64
				var cUserID sql.NullInt32
				var parentID sql.NullInt64
				var content string
				var cAuthorNull sql.NullString
				var createdAt time.Time
				var score int
				var fireCount int64
				var userHasFired bool

				if err := commentRows.Scan(&cID, &argID, &cUserID, &parentID, &content, &cAuthorNull, &createdAt, &score, &fireCount, &userHasFired); err == nil {
					var pID *int64
					if parentID.Valid {
						val := parentID.Int64
						pID = &val
					}

					authorStr := "BOT_REVIEWER"
					if cUserID.Valid && cAuthorNull.Valid {
						authorStr = cAuthorNull.String
					}

					var actualUserID int64
					if cUserID.Valid {
						actualUserID = int64(cUserID.Int32)
					}

					flatComments := CommentResponse{
						ID:           fmt.Sprintf("%d", cID),
						UserID:       actualUserID,
						ParentID:     pID,
						Author:       authorStr,
						Content:      content,
						Timestamp:    createdAt.Format("2006-01-02 15:04:05"),
						Score:        score,
						FireCount:    int(fireCount),
						UserHasFired: userHasFired,
						Replies:      []*CommentResponse{},
					}

					commentsByArg[argID] = append(commentsByArg[argID], flatComments)
				}
			}
			if err := commentRows.Err(); err != nil {
				log.Printf("Comment rows iteration error: %v", err)
				http.Error(w, "Error iterating comment rows", http.StatusInternalServerError)
				return
			}

			for i := range arguments {
				flatList := commentsByArg[int64(arguments[i].ID)]
				if len(flatList) == 0 {
					arguments[i].Comments = []*CommentResponse{}
					continue
				}

				commentMap := make(map[string]*CommentResponse)
				var rootComments []*CommentResponse

				for j := range flatList {
					commentMap[flatList[j].ID] = &flatList[j]
				}

				for j := range flatList {
					comment := &flatList[j]
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

				arguments[i].Comments = rootComments
			}
		}
	}

	type PaginatedResponse struct {
		Arguments []ArgumentResponse `json:"watchlist"`
		Total     int                `json:"total"`
	}

	responsePayload := PaginatedResponse{
		Arguments: arguments,
		Total:     totalCount,
	}

	responseBytes, err := json.Marshal(responsePayload)
	if err != nil {
		log.Printf("JSON marshal error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	_ = aH.Redis.Set(ctx, cacheKey, responseBytes, 60*time.Second).Err()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(responseBytes)
	//json.NewEncoder(w).Encode(arguments)
}

func (aH *ArgumentHandler) HandleGetArgument(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := r.PathValue("id")
	argumentID, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid argument ID", http.StatusBadRequest)
		return
	}

	var currentUserID int64 = 0
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			if claims, err := auth.ParseToken(parts[1], aH.JwtAccessSecret); err == nil {
				if userIDFloat, ok := claims["user_id"].(float64); ok {
					currentUserID = int64(userIDFloat)
				}
			}
		}
	}

	ctx := r.Context()

	query := `
		SELECT a.id, a.user_id, a.title, a.content, a.logic_score, a.author, a.created_at,
		a.logic_score AS plan_score,
		CASE WHEN w.user_id IS NOT NULL THEN true ELSE false END AS is_watched
		FROM arguments a
		LEFT JOIN watchlist w ON w.argument_id = a.id AND w.user_id = $1
		WHERE a.id = $2
	`

	var arg ArgumentResponse
	var rawUserID sql.NullInt32
	var authorNull sql.NullString

	err = aH.DB.QueryRowContext(ctx, query, currentUserID, argumentID).Scan(
		&arg.ID, &rawUserID, &arg.Title, &arg.Content, &arg.LogicScore, &authorNull, &arg.CreatedAt, &arg.PlanScore, &arg.IsWatched,
	)

	if err == sql.ErrNoRows {
		http.Error(w, "Argument not found", http.StatusNotFound)
		return
	} else if err != nil {
		log.Printf("Fetch single argument error: %v", err)
		http.Error(w, "Failed to fetch argument", http.StatusInternalServerError)
		return
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
	commentRows, err := aH.DB.QueryContext(ctx, commentQuery, argumentID, currentUserID)
	if err != nil {
		arg.Comments = []*CommentResponse{}
	} else {
		defer commentRows.Close()
		var flatComments []CommentResponse

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

				flatComments = append(flatComments, CommentResponse{
					ID:           fmt.Sprintf("%d", cID),
					UserID:       actualUserID,
					ParentID:     pID,
					Author:       authorStr,
					Content:      content,
					Timestamp:    createdAt.Format("2006-01-02 15:04:05"),
					Score:        score,
					FireCount:    int(fireCount),
					UserHasFired: userHasFired,
					Replies:      []*CommentResponse{},
				})
			}
		}
		if err := commentRows.Err(); err != nil {
			log.Printf("Comment rows iteration error: %v", err)
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		commentMap := make(map[string]*CommentResponse)
		var rootComments []*CommentResponse

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
			arg.Comments = []*CommentResponse{}
		} else {
			arg.Comments = rootComments
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(arg)
}
