package services

import (
	"errors"
	"fmt"
	"log"
	"time"

	"polling-system/internal/models"
	"polling-system/internal/repositories"
)

// VoteService handles voting business logic
type VoteService struct {
	voteRepo      repositories.VoteRepositoryInterface
	pollRepo      repositories.PollRepositoryInterface
	userRepo      repositories.UserRepositoryInterface
	wsService     *WebSocketService
}

// NewVoteService creates a new vote service
func NewVoteService(voteRepo repositories.VoteRepositoryInterface, pollRepo repositories.PollRepositoryInterface, userRepo repositories.UserRepositoryInterface) *VoteService {
	return &VoteService{
		voteRepo:  voteRepo,
		pollRepo:  pollRepo,
		userRepo:  userRepo,
		wsService: nil, // Will be set via SetWebSocketService
	}
}

// SetWebSocketService sets the WebSocket service for real-time broadcasting
func (s *VoteService) SetWebSocketService(wsService *WebSocketService) {
	s.wsService = wsService
}

// SubmitVoteRequest represents a vote submission request
type SubmitVoteRequest struct {
	PollID   uint `json:"poll_id" validate:"required"`
	OptionID uint `json:"option_id" validate:"required"`
}

// VoteHistoryFilter represents filters for vote history
type VoteHistoryFilter struct {
	UserID uint `json:"user_id" validate:"required"`
	Limit  int  `json:"limit" validate:"min=1,max=100"`
	Offset int  `json:"offset" validate:"min=0"`
}

// PollStatistics represents poll voting statistics
type PollStatistics struct {
	PollID           uint                    `json:"poll_id"`
	TotalVotes       int64                   `json:"total_votes"`
	TotalUsers       int64                   `json:"total_users"`
	ParticipationRate float64                `json:"participation_rate"`
	OptionStats      []OptionStatistic       `json:"option_stats"`
	VoteDistribution map[uint]VoteDistribution `json:"vote_distribution"`
}

// OptionStatistic represents statistics for a single option
type OptionStatistic struct {
	OptionID    uint    `json:"option_id"`
	OptionText  string  `json:"option_text"`
	VoteCount   int64   `json:"vote_count"`
	Percentage  float64 `json:"percentage"`
}

// VoteDistribution represents vote distribution data
type VoteDistribution struct {
	Count      int64   `json:"count"`
	Percentage float64 `json:"percentage"`
}

// SubmitVote submits a vote with anti-double vote logic
func (s *VoteService) SubmitVote(userID uint, req SubmitVoteRequest) (*models.Vote, error) {
	// Validate user
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	if !user.CanVote() {
		return nil, errors.New("user is not authorized to vote")
	}

	// Validate poll
	poll, err := s.pollRepo.GetByIDWithOptions(req.PollID)
	if err != nil {
		return nil, fmt.Errorf("poll not found: %w", err)
	}

	if !poll.CanAcceptVotes() {
		return nil, errors.New("poll is not accepting votes")
	}

	// Check if user has already voted on this poll (anti-double vote)
	hasVoted, err := s.voteRepo.HasUserVoted(userID, req.PollID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing vote: %w", err)
	}
	if hasVoted {
		return nil, errors.New("user has already voted on this poll")
	}

	// Validate option belongs to poll
	var selectedOption *models.Option
	for _, option := range poll.Options {
		if option.ID == req.OptionID {
			selectedOption = &option
			break
		}
	}
	if selectedOption == nil {
		return nil, errors.New("invalid option for this poll")
	}

	// Create vote
	vote, err := s.voteRepo.CreateVote(userID, req.PollID, req.OptionID)
	if err != nil {
		return nil, fmt.Errorf("failed to create vote: %w", err)
	}

	// Broadcast vote update via WebSocket if service is available
	if s.wsService != nil {
		go s.broadcastVoteUpdate(req.PollID, req.OptionID, selectedOption.OptionText, user.Username)
	}

	return vote, nil
}

// broadcastVoteUpdate broadcasts the vote update to WebSocket clients
func (s *VoteService) broadcastVoteUpdate(pollID, optionID uint, optionText, username string) {
	// Get updated statistics
	stats, err := s.GetPollStatistics(pollID)
	if err != nil {
		log.Printf("Failed to get poll statistics for WebSocket broadcast: %v", err)
		return
	}

	// Find the updated option statistics
	var updatedOption *OptionStatistic
	for _, optionStat := range stats.OptionStats {
		if optionStat.OptionID == optionID {
			updatedOption = &optionStat
			break
		}
	}

	if updatedOption == nil {
		log.Printf("Could not find option statistics for option %d", optionID)
		return
	}

	// Create vote update message
	voteUpdate := models.VoteUpdate{
		OptionID:     optionID,
		OptionText:   optionText,
		VoteCount:    int(updatedOption.VoteCount),
		TotalVotes:   int(stats.TotalVotes),
		Percentage:   updatedOption.Percentage,
		Username:     username,
	}

	// Broadcast the update
	if err := s.wsService.BroadcastVoteUpdate(pollID, voteUpdate); err != nil {
		log.Printf("Failed to broadcast vote update: %v", err)
	}

	// Also broadcast participation update
	participationData := map[string]interface{}{
		"poll_id":            pollID,
		"total_votes":        stats.TotalVotes,
		"participation_rate": stats.ParticipationRate,
		"connected_clients":  s.wsService.GetConnectedClientsCount(pollID),
		"updated_at":         time.Now(),
	}

	if err := s.wsService.BroadcastParticipationUpdate(pollID, participationData); err != nil {
		log.Printf("Failed to broadcast participation update: %v", err)
	}
}

// GetUserVote retrieves a user's vote for a specific poll
func (s *VoteService) GetUserVote(userID, pollID uint) (*models.Vote, error) {
	// Validate user
	_, err := s.userRepo.GetByID(userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	// Validate poll
	_, err = s.pollRepo.GetByID(pollID)
	if err != nil {
		return nil, fmt.Errorf("poll not found: %w", err)
	}

	return s.voteRepo.GetUserVote(userID, pollID)
}

// HasUserVoted checks if a user has voted on a poll
func (s *VoteService) HasUserVoted(userID, pollID uint) (bool, error) {
	return s.voteRepo.HasUserVoted(userID, pollID)
}

// GetVoteHistory retrieves a user's voting history
func (s *VoteService) GetVoteHistory(filter VoteHistoryFilter) ([]*models.Vote, int64, error) {
	// Validate user
	_, err := s.userRepo.GetByID(filter.UserID)
	if err != nil {
		return nil, 0, fmt.Errorf("user not found: %w", err)
	}

	// Set default limit if not provided
	if filter.Limit == 0 {
		filter.Limit = 20
	}

	votes, err := s.voteRepo.GetUserVoteHistory(filter.UserID, filter.Limit, filter.Offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get vote history: %w", err)
	}

	count, err := s.voteRepo.CountUserVotes(filter.UserID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count user votes: %w", err)
	}

	return votes, count, nil
}

// GetPollStatistics calculates comprehensive statistics for a poll
func (s *VoteService) GetPollStatistics(pollID uint) (*PollStatistics, error) {
	// Validate poll and get with options
	poll, err := s.pollRepo.GetByIDWithOptions(pollID)
	if err != nil {
		return nil, fmt.Errorf("poll not found: %w", err)
	}

	// Get total votes for the poll
	totalVotes, err := s.voteRepo.GetVoteCountByPoll(pollID)
	if err != nil {
		return nil, fmt.Errorf("failed to get total votes: %w", err)
	}

	// Get vote statistics by option
	voteStats, err := s.voteRepo.GetVoteStatsByPoll(pollID)
	if err != nil {
		return nil, fmt.Errorf("failed to get vote statistics: %w", err)
	}

	// Get total active users for participation rate calculation
	activeStatus := models.UserStatusActive
	totalUsers, err := s.userRepo.CountWithFilters(nil, &activeStatus)
	if err != nil {
		return nil, fmt.Errorf("failed to count active users: %w", err)
	}

	// Calculate participation rate
	participationRate := float64(0)
	if totalUsers > 0 {
		participationRate = float64(totalVotes) / float64(totalUsers) * 100
	}

	// Build option statistics
	optionStats := make([]OptionStatistic, 0, len(poll.Options))
	voteDistribution := make(map[uint]VoteDistribution)

	for _, option := range poll.Options {
		voteCount := voteStats[option.ID]
		percentage := float64(0)
		if totalVotes > 0 {
			percentage = float64(voteCount) / float64(totalVotes) * 100
		}

		optionStats = append(optionStats, OptionStatistic{
			OptionID:   option.ID,
			OptionText: option.OptionText,
			VoteCount:  voteCount,
			Percentage: percentage,
		})

		voteDistribution[option.ID] = VoteDistribution{
			Count:      voteCount,
			Percentage: percentage,
		}
	}

	return &PollStatistics{
		PollID:            pollID,
		TotalVotes:        totalVotes,
		TotalUsers:        totalUsers,
		ParticipationRate: participationRate,
		OptionStats:       optionStats,
		VoteDistribution:  voteDistribution,
	}, nil
}

// GetParticipationStatistics calculates participation statistics for monitoring
func (s *VoteService) GetParticipationStatistics(pollID uint) (map[string]interface{}, error) {
	stats, err := s.GetPollStatistics(pollID)
	if err != nil {
		return nil, err
	}

	// Calculate additional participation metrics
	usersVoted := stats.TotalVotes
	usersNotVoted := stats.TotalUsers - usersVoted

	return map[string]interface{}{
		"poll_id":            pollID,
		"total_users":        stats.TotalUsers,
		"users_voted":        usersVoted,
		"users_not_voted":    usersNotVoted,
		"participation_rate": stats.ParticipationRate,
		"total_votes":        stats.TotalVotes,
		"vote_distribution":  stats.VoteDistribution,
	}, nil
}

// GetRealTimeResults returns real-time voting results for WebSocket broadcasting
func (s *VoteService) GetRealTimeResults(pollID uint) (map[string]interface{}, error) {
	stats, err := s.GetPollStatistics(pollID)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"poll_id":       pollID,
		"total_votes":   stats.TotalVotes,
		"updated_at":    time.Now(),
		"option_stats":  stats.OptionStats,
		"participation": stats.ParticipationRate,
	}, nil
}

// CanUserVoteOnPoll checks if a user can vote on a specific poll
func (s *VoteService) CanUserVoteOnPoll(userID, pollID uint) (bool, string, error) {
	// Check user
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return false, "User not found", err
	}

	if !user.CanVote() {
		return false, "User is not authorized to vote", nil
	}

	// Check poll
	poll, err := s.pollRepo.GetByID(pollID)
	if err != nil {
		return false, "Poll not found", err
	}

	if !poll.CanAcceptVotes() {
		return false, "Poll is not accepting votes", nil
	}

	// Check if already voted
	hasVoted, err := s.voteRepo.HasUserVoted(userID, pollID)
	if err != nil {
		return false, "Failed to check voting status", err
	}

	if hasVoted {
		return false, "User has already voted on this poll", nil
	}

	return true, "User can vote", nil
}

// GetVotesByPoll retrieves all votes for a poll (admin only)
func (s *VoteService) GetVotesByPoll(adminID, pollID uint) ([]*models.Vote, error) {
	// Verify admin permissions
	admin, err := s.userRepo.GetByID(adminID)
	if err != nil {
		return nil, fmt.Errorf("admin not found: %w", err)
	}

	if !admin.IsAdmin() {
		return nil, errors.New("only admins can view all votes")
	}

	// Validate poll
	_, err = s.pollRepo.GetByID(pollID)
	if err != nil {
		return nil, fmt.Errorf("poll not found: %w", err)
	}

	return s.voteRepo.GetVotesWithDetails(pollID)
}

// DeleteVote removes a vote (admin only, for corrections)
func (s *VoteService) DeleteVote(adminID, voteID uint) error {
	// Verify admin permissions
	admin, err := s.userRepo.GetByID(adminID)
	if err != nil {
		return fmt.Errorf("admin not found: %w", err)
	}

	if !admin.IsAdmin() {
		return errors.New("only admins can delete votes")
	}

	// Validate vote exists
	_, err = s.voteRepo.GetByID(voteID)
	if err != nil {
		return fmt.Errorf("vote not found: %w", err)
	}

	return s.voteRepo.Delete(voteID)
}