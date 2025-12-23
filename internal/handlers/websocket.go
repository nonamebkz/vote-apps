package handlers

import (
	"net/http"

	"polling-system/pkg/websocket"
)

// WebSocketHandler handles WebSocket connections
type WebSocketHandler struct {
	hub *websocket.Hub
}

// NewWebSocketHandler creates a new WebSocket handler
func NewWebSocketHandler(hub *websocket.Hub) *WebSocketHandler {
	return &WebSocketHandler{
		hub: hub,
	}
}

// HandleConnection handles WebSocket connection requests
func (h *WebSocketHandler) HandleConnection(w http.ResponseWriter, r *http.Request) {
	h.hub.HandleWebSocket(w, r)
}

// GetHub returns the WebSocket hub for use by other services
func (h *WebSocketHandler) GetHub() *websocket.Hub {
	return h.hub
}