package websocket

import (
	"sync"
)

type Hub struct {
	Clients    map[*Client]bool
	Broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	Mu         sync.Mutex
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.Mu.Lock()              //grab and lock
			h.Clients[client] = true //wait
			h.Mu.Unlock()            //unlock
		case client := <-h.unregister:
			h.Mu.Lock()
			if _, ok := h.Clients[client]; ok { //check if there is clients inside Hub
				delete(h.Clients, client) //delete
				close(client.Send)        //close
			}
			h.Mu.Unlock()
		case message := <-h.Broadcast:
			h.Mu.Lock()
			for client := range h.Clients {
				select {
				case client.Send <- message:
				default:
					close(client.Send)
					delete(h.Clients, client)
				}
			}
			h.Mu.Unlock()
		}
	}
}