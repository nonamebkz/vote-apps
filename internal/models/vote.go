package models

import (
	"time"

	"gorm.io/gorm"
)

// Vote represents a user's vote on a poll option
type Vote struct {
	ID       uint      `gorm:"primaryKey" json:"id"`
	PollID   uint      `gorm:"not null;uniqueIndex:idx_user_poll" json:"poll_id" validate:"required"`
	OptionID uint      `gorm:"not null" json:"option_id" validate:"required"`
	UserID   uint      `gorm:"not null;uniqueIndex:idx_user_poll" json:"user_id" validate:"required"`
	VoteTime time.Time `gorm:"not null" json:"vote_time"`

	// Relations
	Poll   Poll   `gorm:"foreignKey:PollID" json:"poll,omitempty"`
	Option Option `gorm:"foreignKey:OptionID" json:"option,omitempty"`
	User   User   `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// TableName returns the table name for Vote model
func (Vote) TableName() string {
	return "votes"
}

// BeforeCreate is a GORM hook that sets the vote time before creating
func (v *Vote) BeforeCreate(tx *gorm.DB) error {
	v.VoteTime = time.Now()
	return nil
}

// AfterCreate is a GORM hook that updates the option vote count after creating a vote
func (v *Vote) AfterCreate(tx *gorm.DB) error {
	return tx.Model(&Option{}).Where("id = ?", v.OptionID).UpdateColumn("vote_count", gorm.Expr("vote_count + ?", 1)).Error
}

// AfterDelete is a GORM hook that updates the option vote count after deleting a vote
func (v *Vote) AfterDelete(tx *gorm.DB) error {
	return tx.Model(&Option{}).Where("id = ?", v.OptionID).UpdateColumn("vote_count", gorm.Expr("vote_count - ?", 1)).Error
}