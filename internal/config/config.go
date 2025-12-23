package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds all configuration for the application
type Config struct {
	DatabaseURL    string
	JWTSecret      string
	JWTExpiration  time.Duration
	ServerPort     string
	Environment    string
	BaseURL        string
	CORSOrigins    []string
	RateLimitRPS   int
	UploadPath     string
	QRCodePath     string
}

// Load loads configuration from environment variables
func Load() *Config {
	return &Config{
		DatabaseURL:    getEnv("DATABASE_URL", "postgres://user:password@localhost/polling_db?sslmode=disable"),
		JWTSecret:      getEnv("JWT_SECRET", "your-secret-key-change-in-production"),
		JWTExpiration:  getDurationEnv("JWT_EXPIRATION", 24*time.Hour),
		ServerPort:     getEnv("PORT", "8080"),
		Environment:    getEnv("ENVIRONMENT", "development"),
		BaseURL:        getEnv("BASE_URL", "http://localhost:8080"),
		CORSOrigins:    []string{getEnv("CORS_ORIGINS", "*")},
		RateLimitRPS:   getIntEnv("RATE_LIMIT_RPS", 100),
		UploadPath:     getEnv("UPLOAD_PATH", "./uploads"),
		QRCodePath:     getEnv("QR_CODE_PATH", "./qrcodes"),
	}
}

// getEnv gets an environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getIntEnv gets an integer environment variable with a default value
func getIntEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getDurationEnv gets a duration environment variable with a default value
func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}