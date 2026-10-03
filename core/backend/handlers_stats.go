package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

type TimeSeriesPoint struct {
	TimeBucket string `json:"time_bucket"`
	Count      int    `json:"count"`
}

type ArgumentStatsResponse struct {
	ArgumentID    int               `json:"argument_id"`
	CreatedAt     time.Time         `json:"created_at"`
	AgeInSeconds  int64             `json:"age_in_seconds"`
	CommentVolume []TimeSeriesPoint `json:"comment_volume"`
	FireReactions []TimeSeriesPoint `json:"fire_reactions"`
}

func (a *App) handleShowStats(w http.ResponseWriter, r *http.Request) {
	argumentIDStr := r.URL.Query().Get("argument_id")
	if argumentIDStr == "" {
		http.Error(w, "Missing argument_id parameter", http.StatusBadRequest)
		return
	}

	argumentID, err := strconv.Atoi(argumentIDStr)
	if err != nil {
		http.Error(w, "Invalid argument_id format", http.StatusBadRequest)
		return
	}

	var stats ArgumentStatsResponse
	stats.ArgumentID = argumentID
	stats.CommentVolume = []TimeSeriesPoint{}
	stats.FireReactions = []TimeSeriesPoint{}

	queryAge := `
		SELECT created_at, EXTRACT(EPOCH FROM (NOW() - created_at))::BIGINT AS age_in_seconds
		FROM arguments 
		WHERE id = $1;
	`
	err = a.DB.QueryRow(queryAge, argumentID).Scan(&stats.CreatedAt, &stats.AgeInSeconds)
	if err == sql.ErrNoRows {
		http.Error(w, "Argument not found", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, "Database error (Age): "+err.Error(), http.StatusInternalServerError)
		return
	}

	queryCommentVol := `
		SELECT DATE_TRUNC('hour', created_at) AS time_bucket, COUNT(*) 
		FROM comments 
		WHERE argument_id = $1 
		GROUP BY time_bucket 
		ORDER BY time_bucket ASC;
	`
	rowsCV, err := a.DB.Query(queryCommentVol, argumentID)
	if err == nil {
		defer rowsCV.Close()
		for rowsCV.Next() {
			var p TimeSeriesPoint
			if err := rowsCV.Scan(&p.TimeBucket, &p.Count); err == nil {
				stats.CommentVolume = append(stats.CommentVolume, p)
			}
		}
		if err := rowsCV.Err(); err != nil {
			http.Error(w, "Database error (Comment Volume): "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	queryFire := `
		SELECT DATE_TRUNC('hour', cr.created_at) AS time_bucket, COUNT(*) 
		FROM comment_reactions cr
		JOIN comments c ON cr.comment_id = c.id
		WHERE c.argument_id = $1 AND cr.reaction_type = 'fire'
		GROUP BY time_bucket 
		ORDER BY time_bucket ASC;
	`
	rowsFire, err := a.DB.Query(queryFire, argumentID)
	if err == nil {
		defer rowsFire.Close()
		for rowsFire.Next() {
			var p TimeSeriesPoint
			if err := rowsFire.Scan(&p.TimeBucket, &p.Count); err == nil {
				stats.FireReactions = append(stats.FireReactions, p)
			}
		}
		if err := rowsFire.Err(); err != nil {
			http.Error(w, "Database error (Fire Reactions): "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(stats); err != nil {
		return
	}
}

type UserStatsResponse struct {
	UserID                 int `json:"user_id"`
	ArgumentsCount         int `json:"argumentsCount"`
	UniqueDiscussionsCount int `json:"uniqueDiscussionsCount"`
	FiresGivenCount        int `json:"firesGivenCount"`
}

func (a *App) handleUserStats(w http.ResponseWriter, r *http.Request) {
	userIdStr := r.URL.Query().Get("user_id")
	if userIdStr == "" {
		http.Error(w, "Missing user_id parameter", http.StatusBadRequest)
		return
	}

	userId, err := strconv.Atoi(userIdStr)
	if err != nil {
		http.Error(w, "Invalid user_id format", http.StatusBadRequest)
		return
	}

	var stats UserStatsResponse
	stats.UserID = userId

	queryArgs := `SELECT COUNT(*) FROM arguments WHERE user_id = $1;`
	err = a.DB.QueryRow(queryArgs, userId).Scan(&stats.ArgumentsCount)
	if err != nil {
		http.Error(w, "Database error (Arguments): "+err.Error(), http.StatusInternalServerError)
		return
	}

	queryDiscussions := `SELECT COUNT(DISTINCT argument_id) FROM comments WHERE user_id = $1;`
	err = a.DB.QueryRow(queryDiscussions, userId).Scan(&stats.UniqueDiscussionsCount)
	if err != nil {
		http.Error(w, "Database error (Discussions): "+err.Error(), http.StatusInternalServerError)
		return
	}

	queryFires := `SELECT COUNT(*) FROM comment_reactions WHERE user_id = $1 AND reaction_type = 'fire';`
	err = a.DB.QueryRow(queryFires, userId).Scan(&stats.FiresGivenCount)
	if err != nil {
		http.Error(w, "Database error (Fires Given): "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(stats); err != nil {
		return
	}
}
