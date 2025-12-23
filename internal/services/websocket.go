package services

import (
	"encoding/json"
	"log"
	"time"

	"polling-system/internal/models"
	"polling-system/pkg/websocket"
)

// WebSocketService handles WebSocket broadcasting logic
type WebSocketService struct {
	hub *websocket.Hub
}

// NewWebSocketService creates a new WebSocket service
func NewWebSocketService(hub *websocket.Hub) *WebSocketService {
	return &WebSocketService{
		hub: hub,
	}
}

// BroadcastVoteUpdate broadcasts vote updates to all clients connected to a poll
func (s *WebSocketService) BroadcastVoteUpdate(pollID uint, voteUpdate models.VoteUpdate) error {
	message := models.WSMessage{
		Type:      models.WSTypeVoteUpdate,
		PollID:    pollID,
		Data:      voteUpdate,
		Timestamp: time.Now(),
	}

	messageBytes, err := json.Marshal(message)
	if err != nil {
		log.Printf("Failed to marshal vote update message: %v", err)
		return err
	}

	s.hub.BroadcastToPoll(pollID, messageBytes)
	log.Printf("Broadcasted vote update for poll %d: option %d now has %d votes", 
		pollID, voteUpdate.OptionID, voteUpdate.VoteCount)
	
	return nil
}

// BroadcastPollUpdate broadcasts poll status updates to all clients connected to a poll
func (s *WebSocketService) BroadcastPollUpdate(pollID uint, pollUpdate models.PollUpdate) error {
	message := models.WSMessage{
		Type:      models.WSTypePollUpdate,
		PollID:    pollID,
		Data:      pollUpdate,
		Timestamp: time.Now(),
	}

	messageBytes, err := json.Marshal(message)
	if err != nil {
		log.Printf("Failed to marshal poll update message: %v", err)
		return err
	}

	s.hub.BroadcastToPoll(pollID, messageBytes)
	log.Printf("Broadcasted poll update for poll %d: status changed to %s", 
		pollID, pollUpdate.Status)
	
	return nil
}

// BroadcastPollClosed broadcasts poll closure notification to all clients
func (s *WebSocketService) BroadcastPollClosed(pollID uint, message string) error {
	wsMessage := models.WSMessage{
		Type:   models.WSTypePollClosed,
		PollID: pollID,
		Data: map[string]interface{}{
			"message":    message,
			"closed_at":  time.Now(),
		},
		Timestamp: time.Now(),
	}

	messageBytes, err := json.Marshal(wsMessage)
	if err != nil {
		log.Printf("Failed to marshal poll closed message: %v", err)
		return err
	}

	s.hub.BroadcastToPoll(pollID, messageBytes)
	log.Printf("Broadcasted poll closed notification for poll %d", pollID)
	
	return nil
}

// BroadcastError broadcasts error messages to all clients connected to a poll
func (s *WebSocketService) BroadcastError(pollID uint, errorMsg models.ErrorMessage) error {
	message := models.WSMessage{
		Type:      models.WSTypeError,
		PollID:    pollID,
		Data:      errorMsg,
		Timestamp: time.Now(),
	}

	messageBytes, err := json.Marshal(message)
	if err != nil {
		log.Printf("Failed to marshal error message: %v", err)
		return err
	}

	s.hub.BroadcastToPoll(pollID, messageBytes)
	log.Printf("Broadcasted error message for poll %d: %s", pollID, errorMsg.Message)
	
	return nil
}

// GetConnectedClientsCount returns the number of clients connected to a specific poll
func (s *WebSocketService) GetConnectedClientsCount(pollID uint) int {
	return s.hub.GetPollClientCount(pollID)
}

// BroadcastParticipationUpdate broadcasts participation statistics updates
func (s *WebSocketService) BroadcastParticipationUpdate(pollID uint, participationData map[string]interface{}) error {
	message := models.WSMessage{
		Type:      "participation_update",
		PollID:    pollID,
		Data:      participationData,
		Timestamp: time.Now(),
	}

	messageBytes, err := json.Marshal(message)
	if err != nil {
		log.Printf("Failed to marshal participation update message: %v", err)
		return err
	}

	s.hub.BroadcastToPoll(pollID, messageBytes)
	log.Printf("Broadcasted participation update for poll %d", pollID)
	
	return nil
}