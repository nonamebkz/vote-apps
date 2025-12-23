package repositories

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"polling-system/internal/models"
)

// VoteRepositoryInterface defines vote-specific repository operations
type VoteRepositoryInterface interface {
	BaseRepository[models.Vote]
	CreateVote(userID, pollID, optionID uint) (*models.Vote, error)
	HasUserVoted(userID, pollID uint) (bool, error)
	GetUserVote(userID, pollID uint) (*models.Vote, error)
	GetVotesByPoll(pollID uint) ([]*models.Vote, error)
	GetVotesByUser(userID uint, limit, offset int) ([]*models.Vote, error)
	GetVoteCountByOption(optionID uint) (int64, error)
	GetVoteCountByPoll(pollID uint) (int64, error)
	GetVoteStatsByPoll(pollID uint) (map[uint]int64, error)
	GetUserVoteHistory(userID uint, limit, offset int) ([]*models.Vote, error)
	CountUserVotes(userID uint) (int64, error)
	DeleteVotesByPoll(pollID uint) error
	GetVotesWithDetails(pollID uint) ([]*models.Vote, error)
}

// VoteRepository handles vote data access
type VoteRepository struct {
	*BaseRepositoryImpl[models.Vote]
}

// NewVoteRepository creates a new vote repository
func NewVoteRepository(db *gorm.DB) VoteRepositoryInterface {
	return &VoteRepository{
		BaseRepositoryImpl: NewBaseRepository[models.Vote](db),
	}
}

// CreateVote creates a new vote with duplicate prevention
func (r *VoteRepository) CreateVote(userID, pollID, optionID uint) (*models.Vote, error) {
	if userID == 0 {
		return nil, errors.New("invalid user ID: cannot be zero")
	}
	if pollID == 0 {
		return nil, errors.New("invalid poll ID: cannot be zero")
	}
	if optionID == 0 {
		return nil, errors.New("invalid option ID: cannot be zero")
	}
	
	// Check if user has already voted on this poll
	hasVoted, err := r.HasUserVoted(userID, pollID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing vote: %w", err)
	}
	if hasVoted {
		return nil, errors.New("user has already voted on this poll")
	}
	
	// Verify that the option belongs to the poll
	var optionCount int64
	result := r.GetDB().Model(&models.Option{}).Where("id = ? AND poll_id = ?", optionID, pollID).Count(&optionCount)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to verify option: %w", result.Error)
	}
	if optionCount == 0 {
		return nil, errors.New("option does not belong to the specified poll")
	}
	
	vote := &models.Vote{
		UserID:   userID,
		PollID:   pollID,
		OptionID: optionID,
		VoteTime: time.Now(),
	}
	
	if err := r.Create(vote); err != nil {
		return nil, fmt.Errorf("failed to create vote: %w", err)
	}
	
	return vote, nil
}

// HasUserVoted checks if a user has already voted on a poll
func (r *VoteRepository) HasUserVoted(userID, pollID uint) (bool, error) {
	if userID == 0 {
		return false, errors.New("invalid user ID: cannot be zero")
	}
	if pollID == 0 {
		return false, errors.New("invalid poll ID: cannot be zero")
	}
	
	var count int64
	result := r.GetDB().Model(&models.Vote{}).Where("user_id = ? AND poll_id = ?", userID, pollID).Count(&count)
	if result.Error != nil {
		return false, fmt.Errorf("failed to check if user voted: %w", result.Error)
	}
	
	return count > 0, nil
}

// GetUserVote retrieves a user's vote for a specific poll
func (r *VoteRepository) GetUserVote(userID, pollID uint) (*models.Vote, error) {
	if userID == 0 {
		return nil, errors.New("invalid user ID: cannot be zero")
	}
	if pollID == 0 {
		return nil, errors.New("invalid poll ID: cannot be zero")
	}
	
	var vote models.Vote
	result := r.GetDB().Where("user_id = ? AND poll_id = ?", userID, pollID).First(&vote)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("vote not found for user %d on poll %d", userID, pollID)
		}
		return nil, fmt.Errorf("failed to get user vote: %w", result.Error)
	}
	
	return &vote, nil
}

// GetVotesByPoll retrieves all votes for a specific poll
func (r *VoteRepository) GetVotesByPoll(pollID uint) ([]*models.Vote, error) {
	if pollID == 0 {
		return nil, errors.New("invalid poll ID: cannot be zero")
	}
	
	var votes []*models.Vote
	result := r.GetDB().Where("poll_id = ?", pollID).Order("vote_time ASC").Find(&votes)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to get votes by poll: %w", result.Error)
	}
	
	return votes, nil
}

// GetVotesByUser retrieves votes by a specific user with pagination
func (r *VoteRepository) GetVotesByUser(userID uint, limit, offset int) ([]*models.Vote, error) {
	if userID == 0 {
		return nil, errors.New("invalid user ID: cannot be zero")
	}
	if limit < 0 {
		return nil, errors.New("limit cannot be negative")
	}
	if offset < 0 {
		return nil, errors.New("offset cannot be negative")
	}
	
	var votes []*models.Vote
	result := r.GetDB().
		Where("user_id = ?", userID).
		Limit(limit).
		Offset(offset).
		Order("vote_time DESC").
		Find(&votes)
	
	if result.Error != nil {
		return nil, fmt.Errorf("failed to get votes by user: %w", result.Error)
	}
	
	return votes, nil
}

// GetVoteCountByOption returns the number of votes for a specific option
func (r *VoteRepository) GetVoteCountByOption(optionID uint) (int64, error) {
	if optionID == 0 {
		return 0, errors.New("invalid option ID: cannot be zero")
	}
	
	var count int64
	result := r.GetDB().Model(&models.Vote{}).Where("option_id = ?", optionID).Count(&count)
	if result.Error != nil {
		return 0, fmt.Errorf("failed to count votes by option: %w", result.Error)
	}
	
	return count, nil
}

// GetVoteCountByPoll returns the total number of votes for a poll
func (r *VoteRepository) GetVoteCountByPoll(pollID uint) (int64, error) {
	if pollID == 0 {
		return 0, errors.New("invalid poll ID: cannot be zero")
	}
	
	var count int64
	result := r.GetDB().Model(&models.Vote{}).Where("poll_id = ?", pollID).Count(&count)
	if result.Error != nil {
		return 0, fmt.Errorf("failed to count votes by poll: %w", result.Error)
	}
	
	return count, nil
}

// GetVoteStatsByPoll returns vote statistics grouped by option for a poll
func (r *VoteRepository) GetVoteStatsByPoll(pollID uint) (map[uint]int64, error) {
	if pollID == 0 {
		return nil, errors.New("invalid poll ID: cannot be zero")
	}
	
	type VoteStat struct {
		OptionID uint  `json:"option_id"`
		Count    int64 `json:"count"`
	}
	
	var stats []VoteStat
	result := r.GetDB().
		Model(&models.Vote{}).
		Select("option_id, COUNT(*) as count").
		Where("poll_id = ?", pollID).
		Group("option_id").
		Find(&stats)
	
	if result.Error != nil {
		return nil, fmt.Errorf("failed to get vote stats by poll: %w", result.Error)
	}
	
	statsMap := make(map[uint]int64)
	for _, stat := range stats {
		statsMap[stat.OptionID] = stat.Count
	}
	
	return statsMap, nil
}

// GetUserVoteHistory retrieves user's vote history with poll and option details
func (r *VoteRepository) GetUserVoteHistory(userID uint, limit, offset int) ([]*models.Vote, error) {
	if userID == 0 {
		return nil, errors.New("invalid user ID: cannot be zero")
	}
	if limit < 0 {
		return nil, errors.New("limit cannot be negative")
	}
	if offset < 0 {
		return nil, errors.New("offset cannot be negative")
	}
	
	var votes []*models.Vote
	result := r.GetDB().
		Preload("Poll").
		Preload("Option").
		Where("user_id = ?", userID).
		Limit(limit).
		Offset(offset).
		Order("vote_time DESC").
		Find(&votes)
	
	if result.Error != nil {
		return nil, fmt.Errorf("failed to get user vote history: %w", result.Error)
	}
	
	return votes, nil
}

// CountUserVotes returns the total number of votes by a user
func (r *VoteRepository) CountUserVotes(userID uint) (int64, error) {
	if userID == 0 {
		return 0, errors.New("invalid user ID: cannot be zero")
	}
	
	var count int64
	result := r.GetDB().Model(&models.Vote{}).Where("user_id = ?", userID).Count(&count)
	if result.Error != nil {
		return 0, fmt.Errorf("failed to count user votes: %w", result.Error)
	}
	
	return count, nil
}

// DeleteVotesByPoll deletes all votes for a specific poll
func (r *VoteRepository) DeleteVotesByPoll(pollID uint) error {
	if pollID == 0 {
		return errors.New("invalid poll ID: cannot be zero")
	}
	
	result := r.GetDB().Where("poll_id = ?", pollID).Delete(&models.Vote{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete votes by poll: %w", result.Error)
	}
	
	return nil
}

// GetVotesWithDetails retrieves votes for a poll with user and option details
func (r *VoteRepository) GetVotesWithDetails(pollID uint) ([]*models.Vote, error) {
	if pollID == 0 {
		return nil, errors.New("invalid poll ID: cannot be zero")
	}
	
	var votes []*models.Vote
	result := r.GetDB().
		Preload("User").
		Preload("Option").
		Where("poll_id = ?", pollID).
		Order("vote_time ASC").
		Find(&votes)
	
	if result.Error != nil {
		return nil, fmt.Errorf("failed to get votes with details: %w", result.Error)
	}
	
	return votes, nil
}