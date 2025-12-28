package handlers

import (
	"encoding/json" // Added for UnmarshalJSON
	"fmt"           // Added for UnmarshalJSON
	"net/http"
	"strconv"
	"strings"
	"time"

	"polling-system/internal/middleware"
	"polling-system/internal/models"
	"polling-system/internal/services"
)

// PollHandler handles poll related requests
type PollHandler struct {
	pollService *services.PollService
	wsHub       *services.WebSocketService
}

// NewPollHandler creates a new poll handler
func NewPollHandler(pollService *services.PollService, wsHub *services.WebSocketService) *PollHandler {
	return &PollHandler{
		pollService: pollService,
		wsHub:       wsHub,
	}
}

// CreatePollRequest represents a poll creation request
type CreatePollRequest struct {
	Title       string    `json:"title" validate:"required,min=1,max=200"`
	Description string    `json:"description" validate:"max=1000"`
	Options     []string  `json:"options" validate:"required,min=2,max=10"`
	StartDate   time.Time `json:"start_date" validate:"required"`
	EndDate     time.Time `json:"end_date" validate:"required"`
}

// UnmarshalJSON implements custom JSON unmarshaling for CreatePollRequest to handle multiple date formats
func (r *CreatePollRequest) UnmarshalJSON(data []byte) error {
	type Alias CreatePollRequest
	aux := struct {
		StartDate string `json:"start_date"`
		EndDate   string `json:"end_date"`
		*Alias
	}{
		Alias: (*Alias)(r),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	parseTime := func(s string) (time.Time, error) {
		if s == "" {
			return time.Time{}, nil
		}
		// Try RFC3339 first (standard)
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			return t, nil
		}
		// Try "2006-01-02T15:04" (HTML datetime-local format)
		if t, err := time.Parse("2006-01-02T15:04", s); err == nil {
			return t, nil
		}
		// Try "2006-01-02 15:04"
		if t, err := time.Parse("2006-01-02 15:04", s); err == nil {
			return t, nil
		}
		// Try date only
		if t, err := time.Parse("2006-01-02", s); err == nil {
			return t, nil
		}
		return time.Time{}, fmt.Errorf("invalid time format: %s", s)
	}

	var err error
	r.StartDate, err = parseTime(aux.StartDate)
	if err != nil {
		return fmt.Errorf("invalid start_date: %w", err)
	}

	r.EndDate, err = parseTime(aux.EndDate)
	if err != nil {
		return fmt.Errorf("invalid end_date: %w", err)
	}

	return nil
}

// UpdatePollRequest represents a poll update request
type UpdatePollRequest struct {
	Title       *string    `json:"title,omitempty" validate:"omitempty,min=1,max=200"`
	Description *string    `json:"description,omitempty" validate:"omitempty,max=1000"`
	Options     []string   `json:"options,omitempty" validate:"omitempty,min=2,max=10"`
	StartDate   *time.Time `json:"start_date,omitempty"`
	EndDate     *time.Time `json:"end_date,omitempty"`
}

// ListPolls handles listing polls
func (h *PollHandler) ListPolls(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET method is allowed", nil)
		return
	}

	// Get query parameters for filtering and pagination
	statusStr := GetQueryParam(r, "status")
	page := GetQueryParamInt(r, "page", 1)
	limit := GetQueryParamInt(r, "limit", 10)

	// Validate pagination parameters
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	// Get user from context
	claims, ok := middleware.GetUserFromContext(r)
	if !ok {
		WriteError(w, http.StatusUnauthorized, "USER_NOT_FOUND", "User not found in context", nil)
		return
	}

	// Build filter
	filter := services.PollListFilter{
		Limit:  limit,
		Offset: (page - 1) * limit,
	}

	if statusStr != "" {
		status := models.PollStatus(statusStr)
		filter.Status = &status
	}

	// If user is voter, they can only see active polls
	if claims.Role == string(models.RoleVoter) {
		active := true
		filter.IsActive = &active
		activeStatus := models.PollStatusActive
		filter.Status = &activeStatus
	}

	// List polls using service
	polls, total, err := h.pollService.ListPolls(filter)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "LIST_FAILED", "Failed to list polls", map[string]string{"error": err.Error()})
		return
	}

	// Calculate pagination info
	totalPages := (total + int64(limit) - 1) / int64(limit)

	WriteSuccess(w, map[string]interface{}{
		"polls": polls,
		"pagination": map[string]interface{}{
			"page":        page,
			"limit":       limit,
			"total":       total,
			"total_pages": totalPages,
		},
	}, "Polls retrieved successfully")
}

// CreatePoll handles creating a new poll
func (h *PollHandler) CreatePoll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST method is allowed", nil)
		return
	}

	// Get user from context
	claims, ok := middleware.GetUserFromContext(r)
	if !ok {
		WriteError(w, http.StatusUnauthorized, "USER_NOT_FOUND", "User not found in context", nil)
		return
	}

	// Only admins can create polls
	if claims.Role != string(models.RoleAdmin) {
		WriteError(w, http.StatusForbidden, "ADMIN_REQUIRED", "Only admins can create polls", nil)
		return
	}

	var req CreatePollRequest
	if err := ParseJSONBody(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON format", map[string]string{"error": err.Error()})
		return
	}

	// Validate required fields
	if req.Title == "" {
		WriteError(w, http.StatusBadRequest, "MISSING_TITLE", "Poll title is required", nil)
		return
	}
	if len(req.Options) < 2 {
		WriteError(w, http.StatusBadRequest, "INSUFFICIENT_OPTIONS", "At least 2 options are required", nil)
		return
	}
	if len(req.Options) > 10 {
		WriteError(w, http.StatusBadRequest, "TOO_MANY_OPTIONS", "Maximum 10 options allowed", nil)
		return
	}
	if req.StartDate.IsZero() {
		WriteError(w, http.StatusBadRequest, "MISSING_START_DATE", "Start date is required", nil)
		return
	}
	if req.EndDate.IsZero() {
		WriteError(w, http.StatusBadRequest, "MISSING_END_DATE", "End date is required", nil)
		return
	}
	if req.EndDate.Before(req.StartDate) {
		WriteError(w, http.StatusBadRequest, "INVALID_DATE_RANGE", "End date must be after start date", nil)
		return
	}

	// Convert to service request
	serviceReq := services.CreatePollRequest{
		Title:       req.Title,
		Description: req.Description,
		Options:     req.Options,
		StartDate:   req.StartDate,
		EndDate:     req.EndDate,
	}

	// Create poll
	poll, err := h.pollService.CreatePoll(claims.UserID, serviceReq)
	if err != nil {
		// Differentiate between validation/business logic errors (400) and actual server errors (500)
		status := http.StatusBadRequest
		// Simple check for internal failures - in a larger app we might use custom error types
		if strings.Contains(err.Error(), "database") || strings.Contains(err.Error(), "failed to update") {
			status = http.StatusInternalServerError
		}
		WriteError(w, status, "CREATE_FAILED", err.Error(), nil)
		return
	}

	WriteSuccess(w, poll, "Poll created successfully")
}

// GetPoll handles getting a specific poll
func (h *PollHandler) GetPoll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET method is allowed", nil)
		return
	}

	// Extract poll ID from path
	pollID, err := GetPathParamUint(r, "id")
	if err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_POLL_ID", "Invalid poll ID", nil)
		return
	}

	// Get user from context (already checked by middleware)
	_, ok := middleware.GetUserFromContext(r)
	if !ok {
		WriteError(w, http.StatusUnauthorized, "USER_NOT_FOUND", "User not found in context", nil)
		return
	}

	// Get poll with options and vote counts
	poll, err := h.pollService.GetPoll(pollID)
	if err != nil {
		WriteError(w, http.StatusNotFound, "POLL_NOT_FOUND", "Poll not found", map[string]string{"error": err.Error()})
		return
	}

	WriteSuccess(w, poll, "Poll details retrieved successfully")
}

// UpdatePoll handles updating a poll
func (h *PollHandler) UpdatePoll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only PUT method is allowed", nil)
		return
	}

	// Extract poll ID from path
	pollID, err := GetPathParamUint(r, "id")
	if err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_POLL_ID", "Invalid poll ID", nil)
		return
	}

	// Get user from context
	claims, ok := middleware.GetUserFromContext(r)
	if !ok {
		WriteError(w, http.StatusUnauthorized, "USER_NOT_FOUND", "User not found in context", nil)
		return
	}

	// Only admins can update polls
	if claims.Role != string(models.RoleAdmin) {
		WriteError(w, http.StatusForbidden, "ADMIN_REQUIRED", "Only admins can update polls", nil)
		return
	}

	var req UpdatePollRequest
	if err := ParseJSONBody(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON format", map[string]string{"error": err.Error()})
		return
	}

	// Validate date range if both dates are provided
	if req.StartDate != nil && req.EndDate != nil && req.EndDate.Before(*req.StartDate) {
		WriteError(w, http.StatusBadRequest, "INVALID_DATE_RANGE", "End date must be after start date", nil)
		return
	}

	// Convert to service request
	serviceReq := services.UpdatePollRequest{
		Title:       req.Title,
		Description: req.Description,
		Options:     req.Options,
		StartDate:   req.StartDate,
		EndDate:     req.EndDate,
	}

	// Update poll
	poll, err := h.pollService.UpdatePoll(pollID, claims.UserID, serviceReq)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "UPDATE_FAILED", "Failed to update poll", map[string]string{"error": err.Error()})
		return
	}

	WriteSuccess(w, poll, "Poll updated successfully")
}

// DeletePoll handles deleting a poll
func (h *PollHandler) DeletePoll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only DELETE method is allowed", nil)
		return
	}

	// Extract poll ID from path
	pollID, err := GetPathParamUint(r, "id")
	if err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_POLL_ID", "Invalid poll ID", nil)
		return
	}

	// Get user from context
	claims, ok := middleware.GetUserFromContext(r)
	if !ok {
		WriteError(w, http.StatusUnauthorized, "USER_NOT_FOUND", "User not found in context", nil)
		return
	}

	// Only admins can delete polls
	if claims.Role != string(models.RoleAdmin) {
		WriteError(w, http.StatusForbidden, "ADMIN_REQUIRED", "Only admins can delete polls", nil)
		return
	}

	// Delete poll
	if err := h.pollService.DeletePoll(pollID, claims.UserID); err != nil {
		WriteError(w, http.StatusInternalServerError, "DELETE_FAILED", "Failed to delete poll", map[string]string{"error": err.Error()})
		return
	}

	WriteSuccess(w, nil, "Poll deleted successfully")
}

// StartPoll handles starting a poll
func (h *PollHandler) StartPoll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST method is allowed", nil)
		return
	}

	// Extract poll ID from path
	pollID, err := GetPathParamUint(r, "id")
	if err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_POLL_ID", "Invalid poll ID", nil)
		return
	}

	// Get user from context
	claims, ok := middleware.GetUserFromContext(r)
	if !ok {
		WriteError(w, http.StatusUnauthorized, "USER_NOT_FOUND", "User not found in context", nil)
		return
	}

	// Only admins can start polls
	if claims.Role != string(models.RoleAdmin) {
		WriteError(w, http.StatusForbidden, "ADMIN_REQUIRED", "Only admins can start polls", nil)
		return
	}

	// Start poll
	poll, err := h.pollService.StartPoll(pollID, claims.UserID)
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "database") || strings.Contains(err.Error(), "failed to update") {
			status = http.StatusInternalServerError
		}
		WriteError(w, status, "START_FAILED", err.Error(), nil)
		return
	}

	WriteSuccess(w, poll, "Poll started successfully")

	// Notify WebSocket clients about poll status change
	// TODO: Implement BroadcastPollStatusUpdate method in WebSocketService
	// if h.wsHub != nil {
	//     h.wsHub.BroadcastPollStatusUpdate(pollID, string(poll.Status))
	// }
}

// PausePoll handles pausing a poll
func (h *PollHandler) PausePoll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST method is allowed", nil)
		return
	}

	// Extract poll ID from path
	pollID, err := GetPathParamUint(r, "id")
	if err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_POLL_ID", "Invalid poll ID", nil)
		return
	}

	// Get user from context
	claims, ok := middleware.GetUserFromContext(r)
	if !ok {
		WriteError(w, http.StatusUnauthorized, "USER_NOT_FOUND", "User not found in context", nil)
		return
	}

	// Only admins can pause polls
	if claims.Role != string(models.RoleAdmin) {
		WriteError(w, http.StatusForbidden, "ADMIN_REQUIRED", "Only admins can pause polls", nil)
		return
	}

	// Pause poll
	poll, err := h.pollService.PausePoll(pollID, claims.UserID)
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "database") || strings.Contains(err.Error(), "failed to update") {
			status = http.StatusInternalServerError
		}
		WriteError(w, status, "PAUSE_FAILED", err.Error(), nil)
		return
	}

	WriteSuccess(w, poll, "Poll paused successfully")

	// Notify WebSocket clients about poll status change
	// TODO: Implement BroadcastPollStatusUpdate method in WebSocketService
	// if h.wsHub != nil {
	//     h.wsHub.BroadcastPollStatusUpdate(pollID, string(poll.Status))
	// }
}

// StopPoll handles stopping a poll
func (h *PollHandler) StopPoll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST method is allowed", nil)
		return
	}

	// Extract poll ID from path
	pollID, err := GetPathParamUint(r, "id")
	if err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_POLL_ID", "Invalid poll ID", nil)
		return
	}

	// Get user from context
	claims, ok := middleware.GetUserFromContext(r)
	if !ok {
		WriteError(w, http.StatusUnauthorized, "USER_NOT_FOUND", "User not found in context", nil)
		return
	}

	// Only admins can stop polls
	if claims.Role != string(models.RoleAdmin) {
		WriteError(w, http.StatusForbidden, "ADMIN_REQUIRED", "Only admins can stop polls", nil)
		return
	}

	// Stop poll
	poll, err := h.pollService.StopPoll(pollID, claims.UserID)
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "database") || strings.Contains(err.Error(), "failed to update") {
			status = http.StatusInternalServerError
		}
		WriteError(w, status, "STOP_FAILED", err.Error(), nil)
		return
	}

	WriteSuccess(w, poll, "Poll stopped successfully")

	// Notify WebSocket clients about poll status change
	// TODO: Implement BroadcastPollStatusUpdate method in WebSocketService
	// if h.wsHub != nil {
	//     h.wsHub.BroadcastPollStatusUpdate(pollID, string(poll.Status))
	// }
}

// GenerateQR handles QR code generation
func (h *PollHandler) GenerateQR(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET method is allowed", nil)
		return
	}

	// Extract poll ID from path
	pollID, err := GetPathParamUint(r, "id")
	if err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_POLL_ID", "Invalid poll ID", nil)
		return
	}

	// Get user from context
	claims, ok := middleware.GetUserFromContext(r)
	if !ok {
		WriteError(w, http.StatusUnauthorized, "USER_NOT_FOUND", "User not found in context", nil)
		return
	}

	// Only admins can generate QR codes
	if claims.Role != string(models.RoleAdmin) {
		WriteError(w, http.StatusForbidden, "ADMIN_REQUIRED", "Only admins can generate QR codes", nil)
		return
	}

	// Generate QR code
	qrData, err := h.pollService.GenerateQRCode(pollID)
	if err != nil {
		if err.Error() == "poll not found" {
			WriteError(w, http.StatusNotFound, "POLL_NOT_FOUND", "Poll not found", nil)
			return
		}
		WriteError(w, http.StatusInternalServerError, "QR_GENERATION_FAILED", "Failed to generate QR code", map[string]string{"error": err.Error()})
		return
	}

	// Return QR code as image
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Disposition", "inline; filename=poll_"+strconv.Itoa(int(pollID))+"_qr.png")
	w.Write(qrData)
}
