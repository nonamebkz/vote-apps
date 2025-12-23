package repositories

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
	"polling-system/internal/models"
)

// UserRepositoryInterface defines user-specific repository operations
type UserRepositoryInterface interface {
	BaseRepository[models.User]
	FindByUsername(username string) (*models.User, error)
	FindByVoterID(voterID string) (*models.User, error)
	UpdateStatus(userID uint, status models.UserStatus) error
	ListWithFilters(limit, offset int, role *models.UserRole, status *models.UserStatus) ([]*models.User, error)
	CountWithFilters(role *models.UserRole, status *models.UserStatus) (int64, error)
	ExistsByUsername(username string) (bool, error)
	ExistsByVoterID(voterID string) (bool, error)
}

// UserRepository handles user data access
type UserRepository struct {
	*BaseRepositoryImpl[models.User]
}

// NewUserRepository creates a new user repository
func NewUserRepository(db *gorm.DB) UserRepositoryInterface {
	return &UserRepository{
		BaseRepositoryImpl: NewBaseRepository[models.User](db),
	}
}

// FindByUsername finds a user by username
func (r *UserRepository) FindByUsername(username string) (*models.User, error) {
	if username == "" {
		return nil, errors.New("username cannot be empty")
	}
	
	var user models.User
	result := r.GetDB().Where("username = ?", username).First(&user)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("user with username '%s' not found", username)
		}
		return nil, fmt.Errorf("failed to find user by username '%s': %w", username, result.Error)
	}
	
	return &user, nil
}

// FindByVoterID finds a user by voter ID
func (r *UserRepository) FindByVoterID(voterID string) (*models.User, error) {
	if voterID == "" {
		return nil, errors.New("voter ID cannot be empty")
	}
	
	var user models.User
	result := r.GetDB().Where("voter_id = ?", voterID).First(&user)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("user with voter ID '%s' not found", voterID)
		}
		return nil, fmt.Errorf("failed to find user by voter ID '%s': %w", voterID, result.Error)
	}
	
	return &user, nil
}

// UpdateStatus updates a user's status
func (r *UserRepository) UpdateStatus(userID uint, status models.UserStatus) error {
	if userID == 0 {
		return errors.New("invalid user ID: cannot be zero")
	}
	
	if status != models.UserStatusActive && status != models.UserStatusInactive {
		return fmt.Errorf("invalid status: %s", status)
	}
	
	result := r.GetDB().Model(&models.User{}).Where("id = ?", userID).Update("status", status)
	if result.Error != nil {
		return fmt.Errorf("failed to update user status: %w", result.Error)
	}
	
	if result.RowsAffected == 0 {
		return fmt.Errorf("user with ID %d not found", userID)
	}
	
	return nil
}

// ListWithFilters retrieves users with optional role and status filters
func (r *UserRepository) ListWithFilters(limit, offset int, role *models.UserRole, status *models.UserStatus) ([]*models.User, error) {
	if limit < 0 {
		return nil, errors.New("limit cannot be negative")
	}
	if offset < 0 {
		return nil, errors.New("offset cannot be negative")
	}
	
	query := r.GetDB().Limit(limit).Offset(offset)
	
	if role != nil {
		if *role != models.RoleAdmin && *role != models.RoleVoter {
			return nil, fmt.Errorf("invalid role filter: %s", *role)
		}
		query = query.Where("role = ?", *role)
	}
	
	if status != nil {
		if *status != models.UserStatusActive && *status != models.UserStatusInactive {
			return nil, fmt.Errorf("invalid status filter: %s", *status)
		}
		query = query.Where("status = ?", *status)
	}
	
	var users []*models.User
	result := query.Find(&users)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to list users with filters: %w", result.Error)
	}
	
	return users, nil
}

// CountWithFilters counts users with optional role and status filters
func (r *UserRepository) CountWithFilters(role *models.UserRole, status *models.UserStatus) (int64, error) {
	query := r.GetDB().Model(&models.User{})
	
	if role != nil {
		if *role != models.RoleAdmin && *role != models.RoleVoter {
			return 0, fmt.Errorf("invalid role filter: %s", *role)
		}
		query = query.Where("role = ?", *role)
	}
	
	if status != nil {
		if *status != models.UserStatusActive && *status != models.UserStatusInactive {
			return 0, fmt.Errorf("invalid status filter: %s", *status)
		}
		query = query.Where("status = ?", *status)
	}
	
	var count int64
	result := query.Count(&count)
	if result.Error != nil {
		return 0, fmt.Errorf("failed to count users with filters: %w", result.Error)
	}
	
	return count, nil
}

// ExistsByUsername checks if a user exists with the given username
func (r *UserRepository) ExistsByUsername(username string) (bool, error) {
	if username == "" {
		return false, errors.New("username cannot be empty")
	}
	
	var count int64
	result := r.GetDB().Model(&models.User{}).Where("username = ?", username).Count(&count)
	if result.Error != nil {
		return false, fmt.Errorf("failed to check username existence: %w", result.Error)
	}
	
	return count > 0, nil
}

// ExistsByVoterID checks if a user exists with the given voter ID
func (r *UserRepository) ExistsByVoterID(voterID string) (bool, error) {
	if voterID == "" {
		return false, errors.New("voter ID cannot be empty")
	}
	
	var count int64
	result := r.GetDB().Model(&models.User{}).Where("voter_id = ?", voterID).Count(&count)
	if result.Error != nil {
		return false, fmt.Errorf("failed to check voter ID existence: %w", result.Error)
	}
	
	return count > 0, nil
}