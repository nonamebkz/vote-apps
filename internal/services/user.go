package services

import (
	"errors"
	"fmt"

	"polling-system/internal/models"
	"polling-system/internal/repositories"
)

// UserService handles user management business logic
type UserService struct {
	userRepo repositories.UserRepositoryInterface
}

// NewUserService creates a new user service
func NewUserService(userRepo repositories.UserRepositoryInterface) *UserService {
	return &UserService{
		userRepo: userRepo,
	}
}

// CreateUserRequest represents a user creation request
type CreateUserRequest struct {
	Username string          `json:"username" validate:"required,min=3,max=50"`
	Password string          `json:"password" validate:"required,min=6"`
	Role     models.UserRole `json:"role" validate:"required,oneof=admin voter"`
}

// UpdateUserRequest represents a user update request
type UpdateUserRequest struct {
	Username *string          `json:"username,omitempty" validate:"omitempty,min=3,max=50"`
	Password *string          `json:"password,omitempty" validate:"omitempty,min=6"`
	Role     *models.UserRole `json:"role,omitempty" validate:"omitempty,oneof=admin voter"`
}

// UserListFilter represents filters for listing users
type UserListFilter struct {
	Role   *models.UserRole   `json:"role,omitempty"`
	Status *models.UserStatus `json:"status,omitempty"`
	Limit  int                `json:"limit" validate:"min=1,max=100"`
	Offset int                `json:"offset" validate:"min=0"`
}

// CreateUser creates a new user with unique Voter_ID generation
func (s *UserService) CreateUser(adminID uint, req CreateUserRequest) (*models.User, error) {
	// Verify admin permissions
	admin, err := s.userRepo.GetByID(adminID)
	if err != nil {
		return nil, fmt.Errorf("admin not found: %w", err)
	}

	if !admin.IsAdmin() {
		return nil, errors.New("only admins can create users")
	}

	// Validate role
	if req.Role != models.RoleAdmin && req.Role != models.RoleVoter {
		return nil, errors.New("invalid role: must be 'admin' or 'voter'")
	}

	// Check if username already exists
	exists, err := s.userRepo.ExistsByUsername(req.Username)
	if err != nil {
		return nil, fmt.Errorf("failed to check username existence: %w", err)
	}
	if exists {
		return nil, errors.New("username already exists")
	}

	// Create user
	user := &models.User{
		Username: req.Username,
		Role:     req.Role,
		Status:   models.UserStatusActive,
	}

	// Hash password
	if err := user.HashPassword(req.Password); err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Generate unique Voter_ID (will be done by BeforeCreate hook)
	// Ensure uniqueness by checking if generated ID already exists
	maxRetries := 10
	for i := 0; i < maxRetries; i++ {
		if err := user.GenerateVoterID(); err != nil {
			return nil, fmt.Errorf("failed to generate voter ID: %w", err)
		}

		exists, err := s.userRepo.ExistsByVoterID(user.VoterID)
		if err != nil {
			return nil, fmt.Errorf("failed to check voter ID existence: %w", err)
		}

		if !exists {
			break // Unique ID found
		}

		if i == maxRetries-1 {
			return nil, errors.New("failed to generate unique voter ID after multiple attempts")
		}
	}

	// Save user
	if err := s.userRepo.Create(user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

// GetUser retrieves a user by ID
func (s *UserService) GetUser(userID uint) (*models.User, error) {
	return s.userRepo.GetByID(userID)
}

// GetUserByUsername retrieves a user by username
func (s *UserService) GetUserByUsername(username string) (*models.User, error) {
	return s.userRepo.FindByUsername(username)
}

// GetUserByVoterID retrieves a user by voter ID
func (s *UserService) GetUserByVoterID(voterID string) (*models.User, error) {
	return s.userRepo.FindByVoterID(voterID)
}

// UpdateUser updates an existing user
func (s *UserService) UpdateUser(adminID, userID uint, req UpdateUserRequest) (*models.User, error) {
	// Verify admin permissions
	admin, err := s.userRepo.GetByID(adminID)
	if err != nil {
		return nil, fmt.Errorf("admin not found: %w", err)
	}

	if !admin.IsAdmin() {
		return nil, errors.New("only admins can update users")
	}

	// Get existing user
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	// Update fields
	if req.Username != nil {
		// Check if new username already exists (excluding current user)
		exists, err := s.userRepo.ExistsByUsername(*req.Username)
		if err != nil {
			return nil, fmt.Errorf("failed to check username existence: %w", err)
		}
		if exists && user.Username != *req.Username {
			return nil, errors.New("username already exists")
		}
		user.Username = *req.Username
	}

	if req.Password != nil {
		if err := user.HashPassword(*req.Password); err != nil {
			return nil, fmt.Errorf("failed to hash password: %w", err)
		}
	}

	if req.Role != nil {
		if *req.Role != models.RoleAdmin && *req.Role != models.RoleVoter {
			return nil, errors.New("invalid role: must be 'admin' or 'voter'")
		}
		user.Role = *req.Role
	}

	// Update user
	if err := s.userRepo.Update(user); err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	return user, nil
}

// ActivateUser activates a user account
func (s *UserService) ActivateUser(adminID, userID uint) (*models.User, error) {
	return s.changeUserStatus(adminID, userID, models.UserStatusActive)
}

// DeactivateUser deactivates a user account
func (s *UserService) DeactivateUser(adminID, userID uint) (*models.User, error) {
	return s.changeUserStatus(adminID, userID, models.UserStatusInactive)
}

// changeUserStatus changes the status of a user
func (s *UserService) changeUserStatus(adminID, userID uint, newStatus models.UserStatus) (*models.User, error) {
	// Verify admin permissions
	admin, err := s.userRepo.GetByID(adminID)
	if err != nil {
		return nil, fmt.Errorf("admin not found: %w", err)
	}

	if !admin.IsAdmin() {
		return nil, errors.New("only admins can change user status")
	}

	// Prevent admin from deactivating themselves
	if adminID == userID && newStatus == models.UserStatusInactive {
		return nil, errors.New("admin cannot deactivate their own account")
	}

	// Update status
	if err := s.userRepo.UpdateStatus(userID, newStatus); err != nil {
		return nil, fmt.Errorf("failed to update user status: %w", err)
	}

	// Return updated user
	return s.userRepo.GetByID(userID)
}

// ListUsers lists users with optional filters
func (s *UserService) ListUsers(adminID uint, filter UserListFilter) ([]*models.User, int64, error) {
	// Verify admin permissions
	admin, err := s.userRepo.GetByID(adminID)
	if err != nil {
		return nil, 0, fmt.Errorf("admin not found: %w", err)
	}

	if !admin.IsAdmin() {
		return nil, 0, errors.New("only admins can list users")
	}

	// Set default limit if not provided
	if filter.Limit == 0 {
		filter.Limit = 20
	}

	users, err := s.userRepo.ListWithFilters(filter.Limit, filter.Offset, filter.Role, filter.Status)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list users: %w", err)
	}

	count, err := s.userRepo.CountWithFilters(filter.Role, filter.Status)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count users: %w", err)
	}

	return users, count, nil
}

// DeleteUser deletes a user (soft delete)
func (s *UserService) DeleteUser(adminID, userID uint) error {
	// Verify admin permissions
	admin, err := s.userRepo.GetByID(adminID)
	if err != nil {
		return fmt.Errorf("admin not found: %w", err)
	}

	if !admin.IsAdmin() {
		return errors.New("only admins can delete users")
	}

	// Prevent admin from deleting themselves
	if adminID == userID {
		return errors.New("admin cannot delete their own account")
	}

	// Delete user
	return s.userRepo.Delete(userID)
}

// GetUserStats returns statistics about users
func (s *UserService) GetUserStats(adminID uint) (map[string]interface{}, error) {
	// Verify admin permissions
	admin, err := s.userRepo.GetByID(adminID)
	if err != nil {
		return nil, fmt.Errorf("admin not found: %w", err)
	}

	if !admin.IsAdmin() {
		return nil, errors.New("only admins can view user statistics")
	}

	// Get total count
	totalUsers, err := s.userRepo.Count()
	if err != nil {
		return nil, fmt.Errorf("failed to count total users: %w", err)
	}

	// Get active users count
	activeStatus := models.UserStatusActive
	activeUsers, err := s.userRepo.CountWithFilters(nil, &activeStatus)
	if err != nil {
		return nil, fmt.Errorf("failed to count active users: %w", err)
	}

	// Get inactive users count
	inactiveStatus := models.UserStatusInactive
	inactiveUsers, err := s.userRepo.CountWithFilters(nil, &inactiveStatus)
	if err != nil {
		return nil, fmt.Errorf("failed to count inactive users: %w", err)
	}

	// Get admin count
	adminRole := models.RoleAdmin
	adminCount, err := s.userRepo.CountWithFilters(&adminRole, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to count admins: %w", err)
	}

	// Get voter count
	voterRole := models.RoleVoter
	voterCount, err := s.userRepo.CountWithFilters(&voterRole, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to count voters: %w", err)
	}

	return map[string]interface{}{
		"total_users":    totalUsers,
		"active_users":   activeUsers,
		"inactive_users": inactiveUsers,
		"admin_count":    adminCount,
		"voter_count":    voterCount,
	}, nil
}

// ValidateUserAccess checks if a user has access to perform an action
func (s *UserService) ValidateUserAccess(userID uint, requiredRole models.UserRole) error {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	if !user.IsActive() {
		return errors.New("user account is inactive")
	}

	if requiredRole == models.RoleAdmin && !user.IsAdmin() {
		return errors.New("admin privileges required")
	}

	return nil
}