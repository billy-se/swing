package notification

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"vaine-backend/internal/auth"
	"github.com/redis/go-redis/v9"
)

type NotificationHandler struct {
	Redis           *redis.Client
	DB              *sql.DB
}

func (nH *NotificationHandler) HandleGetNotifications(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(auth.UserIDKey).(int)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	//under constructiom
	ctx := r.Context()
	cacheKey := fmt.Sprintf("user:notifications:%d", userID)

	cacheData, err := nH.Redis.Get(ctx, cacheKey).Bytes()
	if err == nil && len(cacheData) > 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(cacheData)
		return
	}

	rows, err := nH.DB.Query(`SELECT id, argument_id, comment_id, type, content, created_at, is_read FROM notif WHERE user_id = $1 ORDER BY created_at DESC LIMIT 20`, userID)
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

	responsePayload := map[string]interface{}{
		"notifications": notifications,
	}

	responseBytes, err := json.Marshal(responsePayload)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	_ = nH.Redis.Set(ctx, cacheKey, responseBytes, 60*time.Second).Err()

	w.Header().Set("Content-Type", "application/json")
	w.Write(responseBytes)
	/*json.NewEncoder(w).Encode(map[string]interface{}{
		"notifications": notifications,
	})*/
}

func (nH *NotificationHandler) HandleMarkNotificationRead(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(auth.UserIDKey).(int)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	notifID := r.PathValue("id")
	if notifID == "" {
		http.Error(w, "Invalid notification ID", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	_, err := nH.DB.Exec(`UPDATE notif SET is_read = TRUE WHERE id = $1 AND user_id = $2`, notifID, userID)
	if err != nil {
		http.Error(w, "Failed to update notification", http.StatusInternalServerError)
		return
	}

	cacheKey := fmt.Sprintf("user:notifications:%d", userID)
	_ = nH.Redis.Del(ctx, cacheKey).Err()

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Marked as read"})
}
