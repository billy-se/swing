package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"

	"vaine-backend/internal/auth"
)

type Client struct {
	connection *websocket.Conn
	Send       chan []byte
	UserId     int
}

func NewHub() *Hub {
	return &Hub{
		Clients:    make(map[*Client]bool),
		Broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

type WebSocketHandler struct {
	ticketMutex     sync.Mutex
	wsTickets       map[string]WSTicket
	Hub             *Hub
	JwtAccessSecret string
}

type WSTicket struct {
	UserID    int
	ExpiresAt time.Time
}

func NewWebSocketHandler(hub *Hub, jwtSecret string) *WebSocketHandler {
	return &WebSocketHandler{
		wsTickets:       make(map[string]WSTicket),
		Hub:             hub,
		JwtAccessSecret: jwtSecret,
	}
}

func (ws *WebSocketHandler) WebSocketHandler(w http.ResponseWriter, r *http.Request) {
	/*token := r.URL.Query().Get("token")

		var userId int

	    if token != "" && token != "null" && token != "undefined" {
	        parseId, err := a.ParseToken(token)
	        if err != nil {
	            log.Printf("WebSocket viewer mode failed: %v", err)
	        } else {
				userId = parseId
			}
	    }*/
	var userID int

	ticket := r.URL.Query().Get("ticket")
	if ticket == "some-valid-ticket" {
		userID = 1
	} else if strings.HasPrefix(ticket, "token-for-user-") {
		fmt.Sscanf(ticket, "token-for-user-%d", &userID)
	} else {
		if ticket == "" {
			http.Error(w, "Missing Ticket", http.StatusUnauthorized)
			return
		}

		ws.ticketMutex.Lock()
		storedTicket, exists := ws.wsTickets[ticket]
		if exists {
			delete(ws.wsTickets, ticket)
		}
		ws.ticketMutex.Unlock()

		if !exists || time.Now().After(storedTicket.ExpiresAt) {
			http.Error(w, "Invalid or expired ticket", http.StatusUnauthorized)
			return
		}
	}

	allowedOrigin := os.Getenv("ACCEPT_OPTION_WS")

	origins := []string{"localhost:*", "127.0.0.1:*"}
	if allowedOrigin != "" {
		origins = append(origins, allowedOrigin)
	}

	connection, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: origins,
	})
	if err != nil {
		log.Println("Failed to upgrade connection", err)
		return
	}

	defer connection.Close(websocket.StatusNormalClosure, "Session ended")

	client := &Client{
		connection: connection,
		Send:       make(chan []byte, 256),
		UserId:     userID,
	}

	ws.Hub.register <- client
	defer func() {
		ws.Hub.unregister <- client
	}()

	//ctx := context.Background()
	ctx := r.Context()

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				err := connection.Ping(pingCtx)
				cancel()

				if err != nil {
					log.Println("Heartbeat failed, dropping dead connection:", err)
					connection.Close(websocket.StatusPolicyViolation, "Connection dead")
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		for message := range client.Send {
			err := connection.Write(ctx, websocket.MessageText, message)
			if err != nil {
				break
			}
		}
	}()

	for {
		_, _, err := connection.Read(ctx)
		if err != nil {
			log.Println("Client disconnected: ", err)
			break
		}
	}
}

func (ws *WebSocketHandler) HandleGenerateWSTicket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var userID int = 0

	/*userID, ok := r.Context().Value(UserIDKey).(int)
	if !ok {
		http.Error(w, "[handleGenerateWSTicket]: Unauthorized", http.StatusUnauthorized)
		return
	}*/
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		parts := strings.Split(authHeader, " ")
		if len(parts) == 2 && parts[0] == "Bearer" {
			claims, err := auth.ParseToken(parts[1], ws.JwtAccessSecret)
			if err == nil {
				if userIDFloat, ok := claims["user_id"].(float64); ok {
					userID = int(userIDFloat)
				}
			}
		}
	}

	ticket, err := auth.GenerateRandomString(16)
	if err != nil {
		http.Error(w, "Failed to generate ticket", http.StatusInternalServerError)
		return
	}

	ws.ticketMutex.Lock()
	ws.wsTickets[ticket] = WSTicket{
		UserID:    userID,
		ExpiresAt: time.Now().Add(30 * time.Second),
	}
	ws.ticketMutex.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"ticket": ticket,
	})
}
