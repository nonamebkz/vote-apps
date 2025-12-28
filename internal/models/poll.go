package models

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

// PollStatus represents the status of a poll
type PollStatus string

const (
	PollStatusDraft  PollStatus = "draft"
	PollStatusActive PollStatus = "active"
	PollStatusPaused PollStatus = "paused"
	PollStatusClosed PollStatus = "closed"
)

// Poll represents a polling session
type Poll struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	Title       string         `gorm:"not null" json:"title" validate:"required,min=3,max=200"`
	Description string         `json:"description" validate:"max=1000"`
	CreatedBy   uint           `gorm:"not null" json:"created_by" validate:"required"`
	StartDate   time.Time      `json:"start_date" validate:"required"`
	EndDate     time.Time      `json:"end_date" validate:"required,gtfield=StartDate"`
	IsActive    bool           `gorm:"default:false" json:"is_active"`
	Status      PollStatus     `gorm:"not null;default:draft" json:"status" validate:"required,oneof=draft active paused closed"`
	QRCodeURL   string         `json:"qr_code_url"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`

	// Relations
	Creator User     `gorm:"foreignKey:CreatedBy" json:"creator,omitempty"`
	Options []Option `gorm:"foreignKey:PollID;constraint:OnDelete:CASCADE" json:"options,omitempty"`
	Votes   []Vote   `gorm:"foreignKey:PollID;constraint:OnDelete:CASCADE" json:"votes,omitempty"`
}

// TableName returns the table name for Poll model
func (Poll) TableName() string {
	return "polls"
}

// CanBeEdited checks if the poll can be edited
func (p *Poll) CanBeEdited() bool {
	return p.Status == PollStatusDraft || p.Status == PollStatusPaused
}

// CanAcceptVotes checks if the poll can accept votes
func (p *Poll) CanAcceptVotes() bool {
	now := time.Now()
	return p.Status == PollStatusActive &&
		p.IsActive &&
		now.After(p.StartDate) &&
		now.Before(p.EndDate)
}

// IsExpired checks if the poll has expired
func (p *Poll) IsExpired() bool {
	return time.Now().After(p.EndDate)
}

// GetTotalVotes returns the total number of votes for this poll
func (p *Poll) GetTotalVotes() int {
	total := 0
	for _, option := range p.Options {
		total += option.VoteCount
	}
	return total
}

// GetParticipationRate calculates participation rate if total users is provided
func (p *Poll) GetParticipationRate(totalUsers int) float64 {
	if totalUsers == 0 {
		return 0
	}
	return float64(len(p.Votes)) / float64(totalUsers) * 100
}

// BeforeUpdate is a GORM hook that prevents editing active polls
func (p *Poll) BeforeUpdate(tx *gorm.DB) error {
	// If ID is 0, we can't fetch the original record.
	// This can happen during some GORM update operations where the ID isn't set on the struct.
	if p.ID == 0 {
		return nil
	}

	// Allow status changes but prevent other field changes for active polls
	var original Poll
	if err := tx.Session(&gorm.Session{}).First(&original, p.ID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil // If not found, nothing to protect
		}
		return err
	}

	// If poll is active and we're trying to change non-status fields
	if original.Status == PollStatusActive && original.IsActive {
		// Check if any field other than Status, IsActive, or UpdatedAt is being changed
		if p.Title != original.Title ||
			p.Description != original.Description ||
			!p.StartDate.Equal(original.StartDate) ||
			!p.EndDate.Equal(original.EndDate) {
			return gorm.ErrInvalidTransaction
		}
	}
	return nil
}

// AfterUpdate is a GORM hook that handles status changes
func (p *Poll) AfterUpdate(tx *gorm.DB) error {
	// Auto-close expired polls
	if p.IsExpired() && p.Status != PollStatusClosed {
		p.Status = PollStatusClosed
		p.IsActive = false
	}
	return nil
}
