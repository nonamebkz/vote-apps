package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"polling-system/internal/services"
)

// QRHandler handles QR code related requests
type QRHandler struct {
	pollService *services.PollService
}

// NewQRHandler creates a new QR handler
func NewQRHandler(pollService *services.PollService) *QRHandler {
	return &QRHandler{
		pollService: pollService,
	}
}

// GetQRCode handles serving QR code images
func (h *QRHandler) GetQRCode(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	pollIDStr, exists := vars["poll_id"]
	if !exists {
		http.Error(w, "Poll ID is required", http.StatusBadRequest)
		return
	}

	pollID, err := strconv.ParseUint(pollIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid poll ID", http.StatusBadRequest)
		return
	}

	// Generate QR code
	qrData, err := h.pollService.GenerateQRCode(uint(pollID))
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to generate QR code: %v", err), http.StatusInternalServerError)
		return
	}

	// Set appropriate headers
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Length", strconv.Itoa(len(qrData)))
	w.Header().Set("Cache-Control", "public, max-age=3600") // Cache for 1 hour

	// Write QR code data
	if _, err := w.Write(qrData); err != nil {
		http.Error(w, "Failed to write QR code", http.StatusInternalServerError)
		return
	}
}

// GetQRCodeFile handles serving QR code files from storage
func (h *QRHandler) GetQRCodeFile(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	pollIDStr, exists := vars["poll_id"]
	if !exists {
		http.Error(w, "Poll ID is required", http.StatusBadRequest)
		return
	}

	pollID, err := strconv.ParseUint(pollIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid poll ID", http.StatusBadRequest)
		return
	}

	// Verify poll exists
	_, err = h.pollService.GetPoll(uint(pollID))
	if err != nil {
		http.Error(w, "Poll not found", http.StatusNotFound)
		return
	}

	// Get QR code file path (this would need QR service access)
	// For now, generate QR code on demand
	qrData, err := h.pollService.GenerateQRCode(uint(pollID))
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to generate QR code: %v", err), http.StatusInternalServerError)
		return
	}

	// Set appropriate headers
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=\"poll_%d_qr.png\"", pollID))
	w.Header().Set("Cache-Control", "public, max-age=3600")

	// Write QR code data
	if _, err := w.Write(qrData); err != nil {
		http.Error(w, "Failed to write QR code", http.StatusInternalServerError)
		return
	}
}

// RegenerateQRCode handles regenerating QR code for a poll
func (h *QRHandler) RegenerateQRCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	vars := mux.Vars(r)
	pollIDStr, exists := vars["poll_id"]
	if !exists {
		http.Error(w, "Poll ID is required", http.StatusBadRequest)
		return
	}

	pollID, err := strconv.ParseUint(pollIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid poll ID", http.StatusBadRequest)
		return
	}

	// Regenerate QR code
	if err := h.pollService.RegenerateQRCode(uint(pollID)); err != nil {
		http.Error(w, fmt.Sprintf("Failed to regenerate QR code: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("QR code regenerated successfully"))
}

// GetVotingURL handles getting the voting URL for a poll
func (h *QRHandler) GetVotingURL(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	pollIDStr, exists := vars["poll_id"]
	if !exists {
		http.Error(w, "Poll ID is required", http.StatusBadRequest)
		return
	}

	pollID, err := strconv.ParseUint(pollIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid poll ID", http.StatusBadRequest)
		return
	}

	// Get voting URL
	votingURL, err := h.pollService.GetVotingURL(uint(pollID))
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get voting URL: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"voting_url": "%s"}`, votingURL)
}