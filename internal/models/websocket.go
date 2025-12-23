package models

import "time"

// WSMessageType represents the type of WebSocket message
type WSMessageType string

const (
	WSTypeVoteUpdate    WSMessageType = "vote_update"
	WSTypePollUpdate    WSMessageType = "poll_update"
	WSTypePollClosed    WSMessageType = "poll_closed"
	WSTypeError         WSMessageType = "error"
	WSTypeConnection    WSMessageType = "connection"
	WSTypeDisconnection WSMessageType = "disconnection"
)

// WSMessage represents a WebSocket message
type WSMessage struct {
	Type      WSMessageType `json:"type"`
	PollID    uint          `json:"poll_id"`
	Data      interface{}   `json:"data"`
	Timestamp time.Time     `json:"timestamp"`
}

// VoteUpdate represents vote update data for WebSocket
type VoteUpdate struct {
	OptionID     uint    `json:"option_id"`
	OptionText   string  `json:"option_text"`
	VoteCount    int     `json:"vote_count"`
	TotalVotes   int     `json:"total_votes"`
	Percentage   float64 `json:"percentage"`
	UserID       uint    `json:"user_id,omitempty"`
	Username     string  `json:"username,omitempty"`
}

// PollUpdate represents poll status update data for WebSocket
type PollUpdate struct {
	PollID      uint       `json:"poll_id"`
	Status      PollStatus `json:"status"`
	IsActive    bool       `json:"is_active"`
	TotalVotes  int        `json:"total_votes"`
	Message     string     `json:"message,omitempty"`
}

// ConnectionInfo represents connection information
type ConnectionInfo struct {
	UserID       uint   `json:"user_id"`
	Username     string `json:"username"`
	ConnectedAt  time.Time `json:"connected_at"`
	TotalClients int    `json:"total_clients"`
}

// ErrorMessage represents error information for WebSocket
type ErrorMessage struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}