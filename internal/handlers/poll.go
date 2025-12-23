package handlers

import (
	"net/http"
	"strconv"
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
	_ = GetQueryParam(r, "status") // status filter (unused for now)
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
	_, ok := middleware.GetUserFromContext(r) // claims (unused for now)
	if !ok {
		WriteError(w, http.StatusUnauthorized, "USER_NOT_FOUND", "User not found in context", nil)
		return
	}

	// List polls based on user role
	// TODO: Implement ListAllPolls and ListAccessiblePolls methods in PollService
	// For now, return a placeholder error
	WriteError(w, http.StatusNotImplemented, "NOT_IMPLEMENTED", "Poll listing not yet implemented", nil)
	return
	
	/*
	var polls []*models.Poll
	var total int64
	var err error

	if claims.Role == string(models.RoleAdmin) {
		// Admin can see all polls
		polls, total, err = h.pollService.ListAllPolls(status, page, limit)
	} else {
		// Voters can only see active polls they can participate in
		polls, total, err = h.pollService.ListAccessiblePolls(claims.UserID, status, page, limit)
	}

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
	*/
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

	// Create poll
	// TODO: Implement CreatePoll method with proper signature in PollService
	// For now, return a placeholder error
	WriteError(w, http.StatusNotImplemented, "NOT_IMPLEMENTED", "Poll creation not yet implemented", nil)
	return
	
	/*
	poll, err := h.pollService.CreatePoll(claims.UserID, req.Title, req.Description, req.Options, req.StartDate, req.EndDate)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "CREATE_FAILED", "Failed to create poll", map[string]string{"error": err.Error()})
		return
	}

	WriteSuccess(w, poll, "Poll created successfully")
	*/
}

// GetPoll handles getting a specific poll
func (h *PollHandler) GetPoll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET method is allowed", nil)
		return
	}

	// Extract poll ID from path
	_, err := GetPathParamUint(r, "id") // pollID (unused for now)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_POLL_ID", "Invalid poll ID", nil)
		return
	}

	// Get user from context
	_, ok := middleware.GetUserFromContext(r) // claims (unused for now)
	if !ok {
		WriteError(w, http.StatusUnauthorized, "USER_NOT_FOUND", "User not found in context", nil)
		return
	}

	// Get poll with options and vote counts
	// TODO: Implement GetPollWithDetails method in PollService
	// For now, return a placeholder error
	WriteError(w, http.StatusNotImplemented, "NOT_IMPLEMENTED", "Poll details not yet implemented", nil)
	return
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

	// Update poll
	// TODO: Implement UpdatePoll method with proper signature in PollService
	// For now, return a placeholder error
	WriteError(w, http.StatusNotImplemented, "NOT_IMPLEMENTED", "Poll update not yet implemented", nil)
	_ = pollID // unused for now
	return
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
	// TODO: Implement DeletePoll method in PollService
	// For now, return a placeholder error
	WriteError(w, http.StatusNotImplemented, "NOT_IMPLEMENTED", "Poll deletion not yet implemented", nil)
	_ = pollID // unused for now
	return
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
	// TODO: Implement StartPoll method in PollService
	// For now, return a placeholder error
	WriteError(w, http.StatusNotImplemented, "NOT_IMPLEMENTED", "Poll start not yet implemented", nil)
	_ = pollID // unused for now
	return

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
	// TODO: Implement PausePoll method in PollService
	// For now, return a placeholder error
	WriteError(w, http.StatusNotImplemented, "NOT_IMPLEMENTED", "Poll pause not yet implemented", nil)
	_ = pollID // unused for now
	return

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
	// TODO: Implement StopPoll method in PollService
	// For now, return a placeholder error
	WriteError(w, http.StatusNotImplemented, "NOT_IMPLEMENTED", "Poll stop not yet implemented", nil)
	_ = pollID // unused for now
	return

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