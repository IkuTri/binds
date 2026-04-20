package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// Event is a server-sent event pushed to WebSocket clients.
type Event struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
	Target  string      `json:"-"`
}

// Hub manages active WebSocket connections and event broadcasting.
type Hub struct {
	mu      sync.RWMutex
	clients map[*wsClient]bool
}

type wsClient struct {
	conn  *websocket.Conn
	agent string
	send  chan []byte
}

func newHub() *Hub {
	return &Hub{clients: make(map[*wsClient]bool)}
}

func (h *Hub) register(c *wsClient) {
	h.mu.Lock()
	h.clients[c] = true
	h.mu.Unlock()
	log.Printf("ws: %s connected (%d clients)", c.agent, len(h.clients))
}

func (h *Hub) unregister(c *wsClient) {
	h.mu.Lock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		close(c.send)
	}
	h.mu.Unlock()
}

// Broadcast sends an event to all connected clients, or to a specific agent if Target is set.
func (h *Hub) Broadcast(evt *Event) {
	data, err := json.Marshal(evt)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		if evt.Target != "" && c.agent != evt.Target {
			continue
		}
		select {
		case c.send <- data:
		default:
		}
	}
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	agent, err := s.authenticate(r)
	if err != nil {
		jsonError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		log.Printf("ws accept: %v", err)
		return
	}

	c := &wsClient{
		conn:  conn,
		agent: agent,
		send:  make(chan []byte, 64),
	}

	s.hub.register(c)
	defer s.hub.unregister(c)

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	go func() {
		defer conn.Close(websocket.StatusNormalClosure, "")
		for {
			select {
			case msg, ok := <-c.send:
				if !ok {
					return
				}
				wctx, wcancel := context.WithTimeout(ctx, 10*time.Second)
				err := conn.Write(wctx, websocket.MessageText, msg)
				wcancel()
				if err != nil {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	for {
		_, _, err := conn.Read(ctx)
		if err != nil {
			cancel()
			return
		}
	}
}
