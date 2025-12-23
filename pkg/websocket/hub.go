package websocket

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"polling-system/internal/models"
	"polling-system/pkg/auth"
)

// Hub maintains the set of active clients and broadcasts messages to the clients
type Hub struct {
	// Registered clients per poll
	clients map[uint]map[*Client]bool

	// Mutex for thread-safe access to clients map
	clientsMutex sync.RWMutex

	// Inbound messages from the clients
	broadcast chan BroadcastMessage

	// Register requests from the clients
	register chan *Client

	// Unregister requests from clients
	unregister chan *Client

	// Channel to stop the hub
	stop chan bool
}

// BroadcastMessage represents a message to be broadcasted to specific poll clients
type BroadcastMessage struct {
	PollID  uint
	Message []byte
}

// Client represents a WebSocket client
type Client struct {
	hub      *Hub
	conn     *websocket.Conn
	send     chan []byte
	pollID   uint
	userID   uint
	username string
	role     string
	
	// Connection metadata
	connectedAt time.Time
	lastPing    time.Time
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow connections from any origin - should be configured properly in production
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer
	maxMessageSize = 512
)

// NewHub creates a new WebSocket hub
func NewHub() *Hub {
	return &Hub{
		clients:      make(map[uint]map[*Client]bool),
		clientsMutex: sync.RWMutex{},
		broadcast:    make(chan BroadcastMessage, 256),
		register:     make(chan *Client),
		unregister:   make(chan *Client),
		stop:         make(chan bool),
	}
}

// Run starts the hub
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.registerClient(client)

		case client := <-h.unregister:
			h.unregisterClient(client)

		case broadcastMsg := <-h.broadcast:
			h.broadcastToPoll(broadcastMsg.PollID, broadcastMsg.Message)

		case <-h.stop:
			log.Println("WebSocket hub stopping...")
			return
		}
	}
}

// registerClient registers a new client to the hub
func (h *Hub) registerClient(client *Client) {
	h.clientsMutex.Lock()
	defer h.clientsMutex.Unlock()

	if h.clients[client.pollID] == nil {
		h.clients[client.pollID] = make(map[*Client]bool)
	}
	h.clients[client.pollID][client] = true
	
	log.Printf("Client registered: User %s (ID: %d) connected to poll %d. Total clients for poll: %d", 
		client.username, client.userID, client.pollID, len(h.clients[client.pollID]))

	// Send connection confirmation message
	connectionInfo := models.ConnectionInfo{
		UserID:       client.userID,
		Username:     client.username,
		ConnectedAt:  client.connectedAt,
		TotalClients: len(h.clients[client.pollID]),
	}

	wsMessage := models.WSMessage{
		Type:      models.WSTypeConnection,
		PollID:    client.pollID,
		Data:      connectionInfo,
		Timestamp: time.Now(),
	}

	if msgBytes, err := json.Marshal(wsMessage); err == nil {
		select {
		case client.send <- msgBytes:
		default:
			close(client.send)
			delete(h.clients[client.pollID], client)
		}
	}
}

// unregisterClient unregisters a client from the hub
func (h *Hub) unregisterClient(client *Client) {
	h.clientsMutex.Lock()
	defer h.clientsMutex.Unlock()

	if clients, ok := h.clients[client.pollID]; ok {
		if _, ok := clients[client]; ok {
			delete(clients, client)
			close(client.send)
			
			log.Printf("Client unregistered: User %s (ID: %d) disconnected from poll %d. Remaining clients: %d", 
				client.username, client.userID, client.pollID, len(clients))

			// Clean up empty poll rooms
			if len(clients) == 0 {
				delete(h.clients, client.pollID)
				log.Printf("Poll room %d cleaned up (no remaining clients)", client.pollID)
			}
		}
	}
}

// broadcastToPoll broadcasts a message to all clients connected to a specific poll
func (h *Hub) broadcastToPoll(pollID uint, message []byte) {
	h.clientsMutex.RLock()
	clients := h.clients[pollID]
	h.clientsMutex.RUnlock()

	if clients == nil {
		log.Printf("No clients connected to poll %d for broadcast", pollID)
		return
	}

	log.Printf("Broadcasting message to %d clients for poll %d", len(clients), pollID)

	for client := range clients {
		select {
		case client.send <- message:
		default:
			// Client's send channel is full, close it and remove from hub
			h.clientsMutex.Lock()
			close(client.send)
			delete(clients, client)
			h.clientsMutex.Unlock()
			log.Printf("Removed unresponsive client for user %d from poll %d", client.userID, pollID)
		}
	}
}

// BroadcastToPoll sends a message to all clients connected to a specific poll
func (h *Hub) BroadcastToPoll(pollID uint, message []byte) {
	select {
	case h.broadcast <- BroadcastMessage{PollID: pollID, Message: message}:
	default:
		log.Printf("Broadcast channel full, dropping message for poll %d", pollID)
	}
}

// GetPollClientCount returns the number of clients connected to a specific poll
func (h *Hub) GetPollClientCount(pollID uint) int {
	h.clientsMutex.RLock()
	defer h.clientsMutex.RUnlock()
	
	if clients, ok := h.clients[pollID]; ok {
		return len(clients)
	}
	return 0
}

// Stop gracefully stops the hub
func (h *Hub) Stop() {
	close(h.stop)
}

// HandleWebSocket handles WebSocket connections
func (h *Hub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Extract poll ID from URL
	vars := mux.Vars(r)
	pollIDStr, ok := vars["poll_id"]
	if !ok {
		http.Error(w, "Poll ID is required", http.StatusBadRequest)
		return
	}

	pollID, err := strconv.ParseUint(pollIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid poll ID", http.StatusBadRequest)
		return
	}

	// Extract and validate JWT token
	tokenString := r.URL.Query().Get("token")
	if tokenString == "" {
		// Try to get token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
			tokenString = strings.TrimPrefix(authHeader, "Bearer ")
		}
	}

	if tokenString == "" {
		http.Error(w, "Authentication token is required", http.StatusUnauthorized)
		return
	}

	claims, err := auth.ValidateToken(tokenString)
	if err != nil {
		http.Error(w, "Invalid authentication token", http.StatusUnauthorized)
		return
	}

	// Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	// Create new client
	client := &Client{
		hub:         h,
		conn:        conn,
		send:        make(chan []byte, 256),
		pollID:      uint(pollID),
		userID:      claims.UserID,
		username:    claims.Username,
		role:        claims.Role,
		connectedAt: time.Now(),
		lastPing:    time.Now(),
	}

	// Register client with hub
	client.hub.register <- client

	// Start goroutines for reading and writing
	go client.writePump()
	go client.readPump()
}

// writePump pumps messages from the hub to the websocket connection
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued chat messages to the current websocket message
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// readPump pumps messages from the websocket connection to the hub
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.lastPing = time.Now()
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error for user %d: %v", c.userID, err)
			}
			break
		}

		// Handle incoming messages (ping/pong, heartbeat, etc.)
		log.Printf("Received message from user %d in poll %d: %s", c.userID, c.pollID, string(message))
		
		// For now, we just log incoming messages
		// In the future, this could handle client-side events like typing indicators, etc.
	}
}