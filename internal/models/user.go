package models

import (
	"time"
	"crypto/rand"
	"encoding/hex"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// UserRole represents the role of a user
type UserRole string

const (
	RoleAdmin UserRole = "admin"
	RoleVoter UserRole = "voter"
)

// UserStatus represents the status of a user
type UserStatus string

const (
	UserStatusActive   UserStatus = "active"
	UserStatusInactive UserStatus = "inactive"
)

// User represents a user in the system
type User struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	Username  string     `gorm:"unique;not null" json:"username" validate:"required,min=3,max=50"`
	Password  string     `gorm:"not null" json:"-" validate:"required,min=6"`
	Role      UserRole   `gorm:"not null" json:"role" validate:"required,oneof=admin voter"`
	Status    UserStatus `gorm:"not null;default:active" json:"status" validate:"required,oneof=active inactive"`
	VoterID   string     `gorm:"unique;not null" json:"voter_id"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Relations
	CreatedPolls []Poll `gorm:"foreignKey:CreatedBy" json:"created_polls,omitempty"`
	Votes        []Vote `gorm:"foreignKey:UserID" json:"votes,omitempty"`
}

// TableName returns the table name for User model
func (User) TableName() string {
	return "users"
}

// IsAdmin checks if the user is an admin
func (u *User) IsAdmin() bool {
	return u.Role == RoleAdmin
}

// IsActive checks if the user is active
func (u *User) IsActive() bool {
	return u.Status == UserStatusActive
}

// CanVote checks if the user can vote (active voter or admin)
func (u *User) CanVote() bool {
	return u.IsActive() && (u.Role == RoleVoter || u.Role == RoleAdmin)
}

// HashPassword hashes the user's password using bcrypt
func (u *User) HashPassword(password string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.Password = string(hashedPassword)
	return nil
}

// CheckPassword verifies if the provided password matches the hashed password
func (u *User) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password))
	return err == nil
}

// GenerateVoterID generates a unique voter ID using crypto/rand
func (u *User) GenerateVoterID() error {
	bytes := make([]byte, 16) // 128 bits
	if _, err := rand.Read(bytes); err != nil {
		return err
	}
	u.VoterID = hex.EncodeToString(bytes)
	return nil
}

// BeforeCreate is a GORM hook that generates VoterID before creating user
func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.VoterID == "" {
		if err := u.GenerateVoterID(); err != nil {
			return err
		}
	}
	return nil
}