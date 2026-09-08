package main

import (
	//"bytes"
	//"encoding/json"
	//"fmt"
	//"net/http"
	//"os"
	"database/sql"
	"log"
)

/*type AuditResponse struct {
	Score  int    `json:"score"`
	Review string `json:"review"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type ChatResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
}

func CallBotAgent(title, content string) (string, error) {
	apiKey := os.Getenv("API_KEY")
	if apiKey == "" {
		return `{"score": 85, "review": "Automated audit mock: Solid initial argument and clean logic."}`, nil
	}

	prompt := fmt.Sprintf("Audit this argument:\nTitle: %s\nContent: %s", title, content)

	reqBody := ChatRequest{
		Model: "gpt-4o-mini",
		Messages: []Message{
			{Role: "system", Content: "You are a strict logical auditor. Respond strictly in JSON format with keys 'score' (int 0-100) and 'review' (string)."},
			{Role: "user", Content: prompt},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", err
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no response choices returned from LLM")
	}

	return chatResp.Choices[0].Message.Content, nil
}*/

func CalculateFinalScore(db *sql.DB, argumentID int64, baseAIScore int) (int, error) {
	query := `
		SELECT COUNT(*) 
		FROM comments 
		WHERE argument_id = $1 AND LENGTH(content) >= 40
	`
	
	var validCommentCount int
	err := db.QueryRow(query, argumentID).Scan(&validCommentCount)
	if err != nil {
		log.Printf("Failed to count comments for scoring: %v", err)
		return baseAIScore, nil
	}

	bonus := validCommentCount * 2
	if bonus > 20 {
		bonus = 20
	}

	finalScore := baseAIScore + bonus

	if finalScore > 100 {
		finalScore = 100
	}

	return finalScore, nil
}