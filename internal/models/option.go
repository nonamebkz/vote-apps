package models

// Option represents a voting option for a poll
type Option struct {
	ID         uint   `gorm:"primaryKey" json:"id"`
	PollID     uint   `gorm:"not null" json:"poll_id" validate:"required"`
	OptionText string `gorm:"not null" json:"option_text" validate:"required,min=1,max=500"`
	VoteCount  int    `gorm:"default:0" json:"vote_count"`

	// Relations
	Poll  Poll   `gorm:"foreignKey:PollID" json:"poll,omitempty"`
	Votes []Vote `gorm:"foreignKey:OptionID;constraint:OnDelete:CASCADE" json:"votes,omitempty"`
}

// TableName returns the table name for Option model
func (Option) TableName() string {
	return "options"
}

// GetPercentage calculates the percentage of votes for this option
func (o *Option) GetPercentage(totalVotes int) float64 {
	if totalVotes == 0 {
		return 0
	}
	return float64(o.VoteCount) / float64(totalVotes) * 100
}