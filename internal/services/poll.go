package services

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"polling-system/internal/models"
	"polling-system/internal/repositories"
	"polling-system/pkg/qr"
)

// PollService handles poll business logic
type PollService struct {
	pollRepo  repositories.PollRepositoryInterface
	userRepo  repositories.UserRepositoryInterface
	qrService *qr.Service
	wsService *WebSocketService
}

// NewPollService creates a new poll service
func NewPollService(pollRepo repositories.PollRepositoryInterface, userRepo repositories.UserRepositoryInterface, qrService *qr.Service) *PollService {
	return &PollService{
		pollRepo:  pollRepo,
		userRepo:  userRepo,
		qrService: qrService,
		wsService: nil, // Will be set via SetWebSocketService
	}
}

// SetWebSocketService sets the WebSocket service for real-time broadcasting
func (s *PollService) SetWebSocketService(wsService *WebSocketService) {
	s.wsService = wsService
}

// CreatePollRequest represents a poll creation request
type CreatePollRequest struct {
	Title       string    `json:"title" validate:"required,min=3,max=200"`
	Description string    `json:"description" validate:"max=1000"`
	StartDate   time.Time `json:"start_date" validate:"required"`
	EndDate     time.Time `json:"end_date" validate:"required,gtfield=StartDate"`
	Options     []string  `json:"options" validate:"required,min=2,dive,required,min=1,max=500"`
}

// UpdatePollRequest represents a poll update request
type UpdatePollRequest struct {
	Title       *string    `json:"title,omitempty" validate:"omitempty,min=3,max=200"`
	Description *string    `json:"description,omitempty" validate:"omitempty,max=1000"`
	StartDate   *time.Time `json:"start_date,omitempty"`
	EndDate     *time.Time `json:"end_date,omitempty"`
	Options     []string   `json:"options,omitempty" validate:"omitempty,min=2,dive,required,min=1,max=500"`
}

// PollListFilter represents filters for listing polls
type PollListFilter struct {
	Status    *models.PollStatus `json:"status,omitempty"`
	CreatorID *uint              `json:"creator_id,omitempty"`
	IsActive  *bool              `json:"is_active,omitempty"`
	Limit     int                `json:"limit" validate:"min=1,max=100"`
	Offset    int                `json:"offset" validate:"min=0"`
}

// CreatePoll creates a new poll
func (s *PollService) CreatePoll(creatorID uint, req CreatePollRequest) (*models.Poll, error) {
	// Verify creator exists and is admin
	creator, err := s.userRepo.GetByID(creatorID)
	if err != nil {
		return nil, fmt.Errorf("creator not found: %w", err)
	}

	if !creator.IsAdmin() {
		return nil, errors.New("only admins can create polls")
	}

	// Validate dates
	if req.EndDate.Before(req.StartDate) {
		return nil, errors.New("end date must be after start date")
	}

	if req.StartDate.Before(time.Now()) {
		return nil, errors.New("start date cannot be in the past")
	}

	// Create poll
	poll := &models.Poll{
		Title:       req.Title,
		Description: req.Description,
		CreatedBy:   creatorID,
		StartDate:   req.StartDate,
		EndDate:     req.EndDate,
		Status:      models.PollStatusDraft,
		IsActive:    false,
	}

	if err := s.pollRepo.Create(poll); err != nil {
		return nil, fmt.Errorf("failed to create poll: %w", err)
	}

	// Create options
	for _, optionText := range req.Options {
		option := &models.Option{
			PollID:     poll.ID,
			OptionText: optionText,
			VoteCount:  0,
		}

		// Create option using transaction
		if err := s.pollRepo.Transaction(func(tx *gorm.DB) error {
			return tx.Create(option).Error
		}); err != nil {
			return nil, fmt.Errorf("failed to create option: %w", err)
		}
	}

	// Generate QR code and update poll with QR code URL
	if s.qrService != nil {
		qrCodeURL := s.qrService.GetQRCodeURL(poll.ID)
		poll.QRCodeURL = qrCodeURL
		
		// Save QR code to file system
		if _, err := s.qrService.SaveQRCode(poll.ID); err != nil {
			// Log error but don't fail poll creation
			fmt.Printf("Warning: failed to generate QR code for poll %d: %v\n", poll.ID, err)
		}
		
		// Update poll with QR code URL
		if err := s.pollRepo.Update(poll); err != nil {
			return nil, fmt.Errorf("failed to update poll with QR code URL: %w", err)
		}
	}

	// Reload poll with relations
	return s.pollRepo.GetByIDWithAll(poll.ID)
}

// GetPoll retrieves a poll by ID with all relations
func (s *PollService) GetPoll(pollID uint) (*models.Poll, error) {
	return s.pollRepo.GetByIDWithAll(pollID)
}

// GetPollWithOptions retrieves a poll with its options
func (s *PollService) GetPollWithOptions(pollID uint) (*models.Poll, error) {
	return s.pollRepo.GetByIDWithOptions(pollID)
}

// UpdatePoll updates an existing poll
func (s *PollService) UpdatePoll(pollID, userID uint, req UpdatePollRequest) (*models.Poll, error) {
	// Get existing poll
	poll, err := s.pollRepo.GetByIDWithRelations(pollID)
	if err != nil {
		return nil, fmt.Errorf("poll not found: %w", err)
	}

	// Check authorization
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	if !user.IsAdmin() && poll.CreatedBy != userID {
		return nil, errors.New("unauthorized: only poll creator or admin can update poll")
	}

	// Check if poll can be edited
	if !poll.CanBeEdited() {
		return nil, errors.New("poll cannot be edited in current status")
	}

	// Update fields
	if req.Title != nil {
		poll.Title = *req.Title
	}
	if req.Description != nil {
		poll.Description = *req.Description
	}
	if req.StartDate != nil {
		if req.StartDate.Before(time.Now()) {
			return nil, errors.New("start date cannot be in the past")
		}
		poll.StartDate = *req.StartDate
	}
	if req.EndDate != nil {
		if req.EndDate.Before(poll.StartDate) {
			return nil, errors.New("end date must be after start date")
		}
		poll.EndDate = *req.EndDate
	}

	// Update poll
	if err := s.pollRepo.Update(poll); err != nil {
		return nil, fmt.Errorf("failed to update poll: %w", err)
	}

	// Handle options update if provided
	if req.Options != nil {
		// This would require option repository operations
		// For now, we'll skip this part as it requires more complex logic
	}

	return s.pollRepo.GetByIDWithAll(pollID)
}

// DeletePoll deletes a poll and all related data
func (s *PollService) DeletePoll(pollID, userID uint) error {
	// Get poll
	poll, err := s.pollRepo.GetByIDWithRelations(pollID)
	if err != nil {
		return fmt.Errorf("poll not found: %w", err)
	}

	// Check authorization
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	if !user.IsAdmin() && poll.CreatedBy != userID {
		return errors.New("unauthorized: only poll creator or admin can delete poll")
	}

	// Delete with cascade
	return s.pollRepo.DeleteWithCascade(pollID)
}

// StartPoll starts a poll (changes status to active)
func (s *PollService) StartPoll(pollID, userID uint) (*models.Poll, error) {
	return s.changePollStatus(pollID, userID, models.PollStatusActive, true)
}

// PausePoll pauses a poll
func (s *PollService) PausePoll(pollID, userID uint) (*models.Poll, error) {
	return s.changePollStatus(pollID, userID, models.PollStatusPaused, false)
}

// StopPoll stops a poll (closes it permanently)
func (s *PollService) StopPoll(pollID, userID uint) (*models.Poll, error) {
	return s.changePollStatus(pollID, userID, models.PollStatusClosed, false)
}

// changePollStatus changes the status of a poll
func (s *PollService) changePollStatus(pollID, userID uint, newStatus models.PollStatus, isActive bool) (*models.Poll, error) {
	// Get poll
	poll, err := s.pollRepo.GetByIDWithRelations(pollID)
	if err != nil {
		return nil, fmt.Errorf("poll not found: %w", err)
	}

	// Check authorization
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	if !user.IsAdmin() && poll.CreatedBy != userID {
		return nil, errors.New("unauthorized: only poll creator or admin can change poll status")
	}

	// Validate status transition
	if err := s.validateStatusTransition(poll.Status, newStatus); err != nil {
		return nil, err
	}

	// For starting a poll, check if it's time
	if newStatus == models.PollStatusActive {
		now := time.Now()
		if now.Before(poll.StartDate) {
			return nil, errors.New("cannot start poll before start date")
		}
		if now.After(poll.EndDate) {
			return nil, errors.New("cannot start poll after end date")
		}
	}

	// Store old status for broadcasting
	oldStatus := poll.Status

	// Update status
	if err := s.pollRepo.UpdateStatus(pollID, newStatus); err != nil {
		return nil, fmt.Errorf("failed to update poll status: %w", err)
	}

	// Update active flag
	if err := s.pollRepo.SetActive(pollID, isActive); err != nil {
		return nil, fmt.Errorf("failed to update poll active status: %w", err)
	}

	// Get updated poll
	updatedPoll, err := s.pollRepo.GetByIDWithAll(pollID)
	if err != nil {
		return nil, fmt.Errorf("failed to get updated poll: %w", err)
	}

	// Broadcast status change via WebSocket if service is available
	if s.wsService != nil {
		go s.broadcastPollStatusChange(pollID, oldStatus, newStatus, isActive)
	}

	return updatedPoll, nil
}

// broadcastPollStatusChange broadcasts poll status changes to WebSocket clients
func (s *PollService) broadcastPollStatusChange(pollID uint, oldStatus, newStatus models.PollStatus, isActive bool) {
	// Get current vote count for the poll
	voteCount := int64(0)
	if voteRepo, ok := interface{}(s.pollRepo).(interface{ GetVoteCount(uint) (int64, error) }); ok {
		if count, err := voteRepo.GetVoteCount(pollID); err == nil {
			voteCount = count
		}
	}

	// Create poll update message
	pollUpdate := models.PollUpdate{
		PollID:     pollID,
		Status:     newStatus,
		IsActive:   isActive,
		TotalVotes: int(voteCount),
	}

	// Add appropriate message based on status change
	switch newStatus {
	case models.PollStatusActive:
		pollUpdate.Message = "Poll has been started and is now accepting votes"
	case models.PollStatusPaused:
		pollUpdate.Message = "Poll has been paused - voting is temporarily disabled"
	case models.PollStatusClosed:
		pollUpdate.Message = "Poll has been closed - voting is no longer available"
		// Also send poll closed notification
		if err := s.wsService.BroadcastPollClosed(pollID, "This poll has been closed by an administrator"); err != nil {
			fmt.Printf("Failed to broadcast poll closed notification: %v\n", err)
		}
	}

	// Broadcast the status update
	if err := s.wsService.BroadcastPollUpdate(pollID, pollUpdate); err != nil {
		fmt.Printf("Failed to broadcast poll status update: %v\n", err)
	}
}

// validateStatusTransition validates if a status transition is allowed
func (s *PollService) validateStatusTransition(currentStatus, newStatus models.PollStatus) error {
	validTransitions := map[models.PollStatus][]models.PollStatus{
		models.PollStatusDraft:  {models.PollStatusActive},
		models.PollStatusActive: {models.PollStatusPaused, models.PollStatusClosed},
		models.PollStatusPaused: {models.PollStatusActive, models.PollStatusClosed},
		models.PollStatusClosed: {}, // No transitions from closed
	}

	allowedStatuses, exists := validTransitions[currentStatus]
	if !exists {
		return fmt.Errorf("invalid current status: %s", currentStatus)
	}

	for _, allowed := range allowedStatuses {
		if newStatus == allowed {
			return nil
		}
	}

	return fmt.Errorf("invalid status transition from %s to %s", currentStatus, newStatus)
}

// ListPolls lists polls with filters
func (s *PollService) ListPolls(filter PollListFilter) ([]*models.Poll, int64, error) {
	// Set default limit if not provided
	if filter.Limit == 0 {
		filter.Limit = 20
	}

	polls, err := s.pollRepo.ListWithFilters(filter.Limit, filter.Offset, filter.Status, filter.CreatorID, filter.IsActive)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list polls: %w", err)
	}

	count, err := s.pollRepo.CountWithFilters(filter.Status, filter.CreatorID, filter.IsActive)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count polls: %w", err)
	}

	return polls, count, nil
}

// GetActivePolls returns all currently active polls
func (s *PollService) GetActivePolls() ([]*models.Poll, error) {
	return s.pollRepo.GetActivePolls()
}

// GetExpiredPolls returns polls that have expired
func (s *PollService) GetExpiredPolls() ([]*models.Poll, error) {
	return s.pollRepo.GetExpiredPolls()
}

// GetPollsByCreator returns polls created by a specific user
func (s *PollService) GetPollsByCreator(creatorID uint, limit, offset int) ([]*models.Poll, error) {
	if limit == 0 {
		limit = 20
	}
	return s.pollRepo.ListByCreator(creatorID, limit, offset)
}

// CanUserAccessPoll checks if a user can access a poll
func (s *PollService) CanUserAccessPoll(userID, pollID uint) (bool, error) {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return false, fmt.Errorf("user not found: %w", err)
	}

	poll, err := s.pollRepo.GetByID(pollID)
	if err != nil {
		return false, fmt.Errorf("poll not found: %w", err)
	}

	// Admins can access any poll
	if user.IsAdmin() {
		return true, nil
	}

	// Poll creators can access their polls
	if poll.CreatedBy == userID {
		return true, nil
	}

	// Active voters can access active polls
	if user.CanVote() && poll.CanAcceptVotes() {
		return true, nil
	}

	return false, nil
}

// GenerateQRCode generates a QR code for a poll
func (s *PollService) GenerateQRCode(pollID uint) ([]byte, error) {
	if s.qrService == nil {
		return nil, errors.New("QR service not available")
	}
	
	// Verify poll exists
	_, err := s.pollRepo.GetByID(pollID)
	if err != nil {
		return nil, fmt.Errorf("poll not found: %w", err)
	}
	
	return s.qrService.GenerateQRCode(pollID)
}

// GetQRCodeURL returns the QR code URL for a poll
func (s *PollService) GetQRCodeURL(pollID uint) (string, error) {
	if s.qrService == nil {
		return "", errors.New("QR service not available")
	}
	
	// Verify poll exists
	_, err := s.pollRepo.GetByID(pollID)
	if err != nil {
		return "", fmt.Errorf("poll not found: %w", err)
	}
	
	return s.qrService.GetQRCodeURL(pollID), nil
}

// GetVotingURL returns the voting URL for a poll
func (s *PollService) GetVotingURL(pollID uint) (string, error) {
	if s.qrService == nil {
		return "", errors.New("QR service not available")
	}
	
	// Verify poll exists
	_, err := s.pollRepo.GetByID(pollID)
	if err != nil {
		return "", fmt.Errorf("poll not found: %w", err)
	}
	
	return s.qrService.GenerateVotingURL(pollID), nil
}

// RegenerateQRCode regenerates the QR code for a poll
func (s *PollService) RegenerateQRCode(pollID uint) error {
	if s.qrService == nil {
		return errors.New("QR service not available")
	}
	
	// Verify poll exists
	poll, err := s.pollRepo.GetByID(pollID)
	if err != nil {
		return fmt.Errorf("poll not found: %w", err)
	}
	
	// Delete existing QR code if it exists
	if err := s.qrService.DeleteQRCode(pollID); err != nil {
		return fmt.Errorf("failed to delete existing QR code: %w", err)
	}
	
	// Generate new QR code
	if _, err := s.qrService.SaveQRCode(pollID); err != nil {
		return fmt.Errorf("failed to generate new QR code: %w", err)
	}
	
	// Update poll with new QR code URL
	qrCodeURL := s.qrService.GetQRCodeURL(pollID)
	poll.QRCodeURL = qrCodeURL
	
	return s.pollRepo.Update(poll)
}