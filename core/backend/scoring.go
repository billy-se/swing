package main

import (
	"database/sql"
	"fmt"
	"math"
)

const (
	MajorityThresholdRatio = 0.40
	CreatorPenaltyRate     = 0.02
	CompetitorPenaltyRate  = 0.05
)

func HasReachedMajority(commentFires, totalFires int) bool {
	if totalFires <= 0 {
		return false
	}
	ratio := float64(commentFires) / float64(totalFires)
	return ratio >= MajorityThresholdRatio
}

func CalculateCreatorPenalty(currentScore int) int {
	if currentScore <= 0 {
		return 0
	}
	penalty := int(math.Round(float64(currentScore) * CreatorPenaltyRate))
	if penalty < 1 {
		return 1
	}
	return penalty
}

func CalculateCompetitorPenalty(currentScore int) int {
	if currentScore <= 0 {
		return 0
	}
	penalty := int(math.Round(float64(currentScore) * CompetitorPenaltyRate))
	if penalty < 1 {
		return 1
	}
	return penalty
}

func (a *App) ApplyConsensusDeflationTx(tx *sql.Tx, commentID int64, argumentID int64, creatorID int64, commentUserID int64) error {
	_, err := tx.Exec(`UPDATE comments SET is_fire_triggered = TRUE WHERE id = $1`, commentID)
	if err != nil {
		return fmt.Errorf("failed to trigger comment: %w", err)
	}

	var creatorScore int
	err = tx.QueryRow(`SELECT logic_score FROM users WHERE id = $1 FOR UPDATE`, creatorID).Scan(&creatorScore)
	if err == nil && creatorScore > 0 {
		penalty := CalculateCreatorPenalty(creatorScore)
		newScore := max(0, creatorScore-penalty)

		_, err = tx.Exec(`UPDATE users SET logic_score = $1 WHERE id = $2`, newScore, creatorID)
		if err != nil {
			return fmt.Errorf("failed to update creator score: %w", err)
		}

		_, err = tx.Exec(
			`INSERT INTO score_events (user_id, comment_id, delta_score, event_type) VALUES ($1, $2, $3, $4)`,
			creatorID, commentID, -penalty, "CREATOR_PENALTY",
		)
		if err != nil {
			return fmt.Errorf("failed to log creator penalty: %w", err)
		}
	}

	rows, err := tx.Query(`SELECT DISTINCT user_id FROM comments WHERE argument_id = $1 AND id != $2`, argumentID, commentID)
	if err != nil {
		return fmt.Errorf("failed to query competitors: %w", err)
	}

	var compUserIDs []int64
	for rows.Next() {
		var compUserID int64
		if err := rows.Scan(&compUserID); err != nil {
			continue
		}
		if compUserID != commentUserID {
			compUserIDs = append(compUserIDs, compUserID)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("row iteration error: %w", err)
	}
	rows.Close()

	for _, compUserID := range compUserIDs {
		var compScore int
		err = tx.QueryRow(`SELECT logic_score FROM users WHERE id = $1 FOR UPDATE`, compUserID).Scan(&compScore)
		if err == nil && compScore > 0 {
			penalty := CalculateCompetitorPenalty(compScore)
			newScore := max(0, compScore-penalty)

			_, err = tx.Exec(`UPDATE users SET logic_score = $1 WHERE id = $2`, newScore, compUserID)
			if err != nil {
				return fmt.Errorf("failed to update competitor score: %w", err)
			}

			_, err = tx.Exec(
				`INSERT INTO score_events (user_id, comment_id, delta_score, event_type) VALUES ($1, $2, $3, $4)`,
				compUserID, commentID, -penalty, "COMPETITOR_PENALTY",
			)
			if err != nil {
				return fmt.Errorf("failed to log competitor penalty: %w", err)
			}
		}
	}

	_, err = tx.Exec(`UPDATE users SET logic_score = logic_score + 50 WHERE id = $1`, commentUserID)
	if err != nil {
		return fmt.Errorf("failed to reward comment user: %w", err)
	}

	return nil
}
