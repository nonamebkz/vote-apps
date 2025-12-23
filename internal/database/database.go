package database

import (
	"log"
	"time"
	"polling-system/internal/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Initialize initializes the database connection and runs migrations
func Initialize(databaseURL string) (*gorm.DB, error) {
	// Configure GORM logger
	config := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	}

	// Connect to database
	db, err := gorm.Open(postgres.Open(databaseURL), config)
	if err != nil {
		return nil, err
	}

	// Run auto-migrations
	if err := runMigrations(db); err != nil {
		return nil, err
	}

	// Create indexes
	if err := CreateIndexes(db); err != nil {
		return nil, err
	}

	SeedData(db)

	log.Println("Database initialized successfully")
	return db, nil
}

// runMigrations runs all database migrations
func runMigrations(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.User{},
		&models.Poll{},
		&models.Option{},
		&models.Vote{},
	)
}

// CreateIndexes creates database indexes for performance
func CreateIndexes(db *gorm.DB) error {
	// User indexes
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_users_username ON users(username)").Error; err != nil {
		return err
	}
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_users_voter_id ON users(voter_id)").Error; err != nil {
		return err
	}
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_users_status ON users(status)").Error; err != nil {
		return err
	}

	// Poll indexes
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_polls_created_by ON polls(created_by)").Error; err != nil {
		return err
	}
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_polls_status ON polls(status)").Error; err != nil {
		return err
	}
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_polls_is_active ON polls(is_active)").Error; err != nil {
		return err
	}

	// Vote indexes
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_votes_poll_id ON votes(poll_id)").Error; err != nil {
		return err
	}
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_votes_user_id ON votes(user_id)").Error; err != nil {
		return err
	}
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_votes_option_id ON votes(option_id)").Error; err != nil {
		return err
	}
	if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_votes_user_poll ON votes(user_id, poll_id)").Error; err != nil {
		return err
	}

	// Option indexes
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_options_poll_id ON options(poll_id)").Error; err != nil {
		return err
	}

	log.Println("Database indexes created successfully")
	return nil
}

// SeedData creates initial test data for development and testing
func SeedData(db *gorm.DB) error {
	// Check if data already exists
	var userCount int64
	db.Model(&models.User{}).Count(&userCount)
	if userCount > 0 {
		log.Println("Seed data already exists, skipping...")
		return nil
	}

	// Create admin user
	adminUser := &models.User{
		Username: "admin",
		Role:     models.RoleAdmin,
		Status:   models.UserStatusActive,
	}
	if err := adminUser.HashPassword("admin123"); err != nil {
		return err
	}
	if err := db.Create(adminUser).Error; err != nil {
		return err
	}

	// Create test voters
	testVoters := []struct {
		username string
		password string
	}{
		{"voter1", "password123"},
		{"voter2", "password123"},
		{"voter3", "password123"},
		{"voter4", "password123"},
		{"voter5", "password123"},
	}

	for _, voter := range testVoters {
		user := &models.User{
			Username: voter.username,
			Role:     models.RoleVoter,
			Status:   models.UserStatusActive,
		}
		if err := user.HashPassword(voter.password); err != nil {
			return err
		}
		if err := db.Create(user).Error; err != nil {
			return err
		}
	}

	// Create sample poll
	samplePoll := &models.Poll{
		Title:       "Sample Poll: Favorite Programming Language",
		Description: "Vote for your favorite programming language",
		CreatedBy:   adminUser.ID,
		StartDate:   time.Now(),
		EndDate:     time.Now().Add(24 * time.Hour),
		Status:      models.PollStatusDraft,
		IsActive:    false,
	}
	if err := db.Create(samplePoll).Error; err != nil {
		return err
	}

	// Create options for the sample poll
	options := []string{
		"Go",
		"Python",
		"JavaScript",
		"Java",
		"C++",
	}

	for _, optionText := range options {
		option := &models.Option{
			PollID:     samplePoll.ID,
			OptionText: optionText,
			VoteCount:  0,
		}
		if err := db.Create(option).Error; err != nil {
			return err
		}
	}

	log.Println("Seed data created successfully")
	return nil
}