package repositories

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"polling-system/internal/models"
)

// PollRepositoryInterface defines poll-specific repository operations
type PollRepositoryInterface interface {
	BaseRepository[models.Poll]
	GetByIDWithRelations(id uint) (*models.Poll, error)
	GetByIDWithOptions(id uint) (*models.Poll, error)
	GetByIDWithVotes(id uint) (*models.Poll, error)
	GetByIDWithAll(id uint) (*models.Poll, error)
	UpdateStatus(pollID uint, status models.PollStatus) error
	SetActive(pollID uint, isActive bool) error
	ListByCreator(creatorID uint, limit, offset int) ([]*models.Poll, error)
	ListByStatus(status models.PollStatus, limit, offset int) ([]*models.Poll, error)
	ListWithFilters(limit, offset int, status *models.PollStatus, creatorID *uint, isActive *bool) ([]*models.Poll, error)
	CountWithFilters(status *models.PollStatus, creatorID *uint, isActive *bool) (int64, error)
	GetActivePolls() ([]*models.Poll, error)
	GetExpiredPolls() ([]*models.Poll, error)
	DeleteWithCascade(pollID uint) error
}

// PollRepository handles poll data access
type PollRepository struct {
	*BaseRepositoryImpl[models.Poll]
}

// NewPollRepository creates a new poll repository
func NewPollRepository(db *gorm.DB) PollRepositoryInterface {
	return &PollRepository{
		BaseRepositoryImpl: NewBaseRepository[models.Poll](db),
	}
}

// GetByIDWithRelations retrieves a poll with creator information
func (r *PollRepository) GetByIDWithRelations(id uint) (*models.Poll, error) {
	if id == 0 {
		return nil, errors.New("invalid ID: cannot be zero")
	}
	
	var poll models.Poll
	result := r.GetDB().Preload("Creator").First(&poll, id)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("poll with ID %d not found", id)
		}
		return nil, fmt.Errorf("failed to get poll by ID %d: %w", id, result.Error)
	}
	
	return &poll, nil
}

// GetByIDWithOptions retrieves a poll with its options
func (r *PollRepository) GetByIDWithOptions(id uint) (*models.Poll, error) {
	if id == 0 {
		return nil, errors.New("invalid ID: cannot be zero")
	}
	
	var poll models.Poll
	result := r.GetDB().Preload("Options").First(&poll, id)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("poll with ID %d not found", id)
		}
		return nil, fmt.Errorf("failed to get poll with options by ID %d: %w", id, result.Error)
	}
	
	return &poll, nil
}

// GetByIDWithVotes retrieves a poll with its votes
func (r *PollRepository) GetByIDWithVotes(id uint) (*models.Poll, error) {
	if id == 0 {
		return nil, errors.New("invalid ID: cannot be zero")
	}
	
	var poll models.Poll
	result := r.GetDB().Preload("Votes").Preload("Votes.User").Preload("Votes.Option").First(&poll, id)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("poll with ID %d not found", id)
		}
		return nil, fmt.Errorf("failed to get poll with votes by ID %d: %w", id, result.Error)
	}
	
	return &poll, nil
}

// GetByIDWithAll retrieves a poll with all related data
func (r *PollRepository) GetByIDWithAll(id uint) (*models.Poll, error) {
	if id == 0 {
		return nil, errors.New("invalid ID: cannot be zero")
	}
	
	var poll models.Poll
	result := r.GetDB().
		Preload("Creator").
		Preload("Options").
		Preload("Votes").
		Preload("Votes.User").
		Preload("Votes.Option").
		First(&poll, id)
	
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("poll with ID %d not found", id)
		}
		return nil, fmt.Errorf("failed to get poll with all relations by ID %d: %w", id, result.Error)
	}
	
	return &poll, nil
}

// UpdateStatus updates a poll's status
func (r *PollRepository) UpdateStatus(pollID uint, status models.PollStatus) error {
	if pollID == 0 {
		return errors.New("invalid poll ID: cannot be zero")
	}
	
	validStatuses := map[models.PollStatus]bool{
		models.PollStatusDraft:  true,
		models.PollStatusActive: true,
		models.PollStatusPaused: true,
		models.PollStatusClosed: true,
	}
	
	if !validStatuses[status] {
		return fmt.Errorf("invalid status: %s", status)
	}
	
	result := r.GetDB().Model(&models.Poll{}).Where("id = ?", pollID).Update("status", status)
	if result.Error != nil {
		return fmt.Errorf("failed to update poll status: %w", result.Error)
	}
	
	if result.RowsAffected == 0 {
		return fmt.Errorf("poll with ID %d not found", pollID)
	}
	
	return nil
}

// SetActive sets the active status of a poll
func (r *PollRepository) SetActive(pollID uint, isActive bool) error {
	if pollID == 0 {
		return errors.New("invalid poll ID: cannot be zero")
	}
	
	result := r.GetDB().Model(&models.Poll{}).Where("id = ?", pollID).Update("is_active", isActive)
	if result.Error != nil {
		return fmt.Errorf("failed to set poll active status: %w", result.Error)
	}
	
	if result.RowsAffected == 0 {
		return fmt.Errorf("poll with ID %d not found", pollID)
	}
	
	return nil
}

// ListByCreator retrieves polls created by a specific user
func (r *PollRepository) ListByCreator(creatorID uint, limit, offset int) ([]*models.Poll, error) {
	if creatorID == 0 {
		return nil, errors.New("invalid creator ID: cannot be zero")
	}
	if limit < 0 {
		return nil, errors.New("limit cannot be negative")
	}
	if offset < 0 {
		return nil, errors.New("offset cannot be negative")
	}
	
	var polls []*models.Poll
	result := r.GetDB().
		Where("created_by = ?", creatorID).
		Limit(limit).
		Offset(offset).
		Order("created_at DESC").
		Find(&polls)
	
	if result.Error != nil {
		return nil, fmt.Errorf("failed to list polls by creator: %w", result.Error)
	}
	
	return polls, nil
}

// ListByStatus retrieves polls with a specific status
func (r *PollRepository) ListByStatus(status models.PollStatus, limit, offset int) ([]*models.Poll, error) {
	validStatuses := map[models.PollStatus]bool{
		models.PollStatusDraft:  true,
		models.PollStatusActive: true,
		models.PollStatusPaused: true,
		models.PollStatusClosed: true,
	}
	
	if !validStatuses[status] {
		return nil, fmt.Errorf("invalid status: %s", status)
	}
	if limit < 0 {
		return nil, errors.New("limit cannot be negative")
	}
	if offset < 0 {
		return nil, errors.New("offset cannot be negative")
	}
	
	var polls []*models.Poll
	result := r.GetDB().
		Where("status = ?", status).
		Limit(limit).
		Offset(offset).
		Order("created_at DESC").
		Find(&polls)
	
	if result.Error != nil {
		return nil, fmt.Errorf("failed to list polls by status: %w", result.Error)
	}
	
	return polls, nil
}

// ListWithFilters retrieves polls with optional filters
func (r *PollRepository) ListWithFilters(limit, offset int, status *models.PollStatus, creatorID *uint, isActive *bool) ([]*models.Poll, error) {
	if limit < 0 {
		return nil, errors.New("limit cannot be negative")
	}
	if offset < 0 {
		return nil, errors.New("offset cannot be negative")
	}
	
	query := r.GetDB().Limit(limit).Offset(offset).Order("created_at DESC")
	
	if status != nil {
		validStatuses := map[models.PollStatus]bool{
			models.PollStatusDraft:  true,
			models.PollStatusActive: true,
			models.PollStatusPaused: true,
			models.PollStatusClosed: true,
		}
		if !validStatuses[*status] {
			return nil, fmt.Errorf("invalid status filter: %s", *status)
		}
		query = query.Where("status = ?", *status)
	}
	
	if creatorID != nil {
		if *creatorID == 0 {
			return nil, errors.New("invalid creator ID filter: cannot be zero")
		}
		query = query.Where("created_by = ?", *creatorID)
	}
	
	if isActive != nil {
		query = query.Where("is_active = ?", *isActive)
	}
	
	var polls []*models.Poll
	result := query.Find(&polls)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to list polls with filters: %w", result.Error)
	}
	
	return polls, nil
}

// CountWithFilters counts polls with optional filters
func (r *PollRepository) CountWithFilters(status *models.PollStatus, creatorID *uint, isActive *bool) (int64, error) {
	query := r.GetDB().Model(&models.Poll{})
	
	if status != nil {
		validStatuses := map[models.PollStatus]bool{
			models.PollStatusDraft:  true,
			models.PollStatusActive: true,
			models.PollStatusPaused: true,
			models.PollStatusClosed: true,
		}
		if !validStatuses[*status] {
			return 0, fmt.Errorf("invalid status filter: %s", *status)
		}
		query = query.Where("status = ?", *status)
	}
	
	if creatorID != nil {
		if *creatorID == 0 {
			return 0, errors.New("invalid creator ID filter: cannot be zero")
		}
		query = query.Where("created_by = ?", *creatorID)
	}
	
	if isActive != nil {
		query = query.Where("is_active = ?", *isActive)
	}
	
	var count int64
	result := query.Count(&count)
	if result.Error != nil {
		return 0, fmt.Errorf("failed to count polls with filters: %w", result.Error)
	}
	
	return count, nil
}

// GetActivePolls retrieves all currently active polls
func (r *PollRepository) GetActivePolls() ([]*models.Poll, error) {
	var polls []*models.Poll
	now := time.Now()
	
	result := r.GetDB().
		Where("status = ? AND is_active = ? AND start_date <= ? AND end_date > ?", 
			models.PollStatusActive, true, now, now).
		Order("created_at DESC").
		Find(&polls)
	
	if result.Error != nil {
		return nil, fmt.Errorf("failed to get active polls: %w", result.Error)
	}
	
	return polls, nil
}

// GetExpiredPolls retrieves polls that have passed their end date
func (r *PollRepository) GetExpiredPolls() ([]*models.Poll, error) {
	var polls []*models.Poll
	now := time.Now()
	
	result := r.GetDB().
		Where("end_date <= ? AND status != ?", now, models.PollStatusClosed).
		Order("end_date ASC").
		Find(&polls)
	
	if result.Error != nil {
		return nil, fmt.Errorf("failed to get expired polls: %w", result.Error)
	}
	
	return polls, nil
}

// DeleteWithCascade deletes a poll and all its related data
func (r *PollRepository) DeleteWithCascade(pollID uint) error {
	if pollID == 0 {
		return errors.New("invalid poll ID: cannot be zero")
	}
	
	return r.Transaction(func(tx *gorm.DB) error {
		// Delete votes first
		if err := tx.Where("poll_id = ?", pollID).Delete(&models.Vote{}).Error; err != nil {
			return fmt.Errorf("failed to delete votes: %w", err)
		}
		
		// Delete options
		if err := tx.Where("poll_id = ?", pollID).Delete(&models.Option{}).Error; err != nil {
			return fmt.Errorf("failed to delete options: %w", err)
		}
		
		// Delete poll
		result := tx.Delete(&models.Poll{}, pollID)
		if result.Error != nil {
			return fmt.Errorf("failed to delete poll: %w", result.Error)
		}
		
		if result.RowsAffected == 0 {
			return fmt.Errorf("poll with ID %d not found", pollID)
		}
		
		return nil
	})
}