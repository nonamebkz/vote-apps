package qr

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/skip2/go-qrcode"
)

// Service handles QR code generation and management
type Service struct {
	baseURL    string
	storageDir string
}

// NewService creates a new QR code service
func NewService(baseURL, storageDir string) *Service {
	return &Service{
		baseURL:    baseURL,
		storageDir: storageDir,
	}
}

// GenerateVotingURL creates a unique voting URL for a poll
func (s *Service) GenerateVotingURL(pollID uint) string {
	return fmt.Sprintf("%s/vote/%d", s.baseURL, pollID)
}

// GenerateQRCode generates a QR code for a poll's voting URL
func (s *Service) GenerateQRCode(pollID uint) ([]byte, error) {
	votingURL := s.GenerateVotingURL(pollID)
	
	// Generate QR code with medium error correction level
	qrCode, err := qrcode.Encode(votingURL, qrcode.Medium, 256)
	if err != nil {
		return nil, fmt.Errorf("failed to generate QR code: %w", err)
	}
	
	return qrCode, nil
}

// SaveQRCode generates and saves a QR code to file system
func (s *Service) SaveQRCode(pollID uint) (string, error) {
	// Generate QR code
	qrData, err := s.GenerateQRCode(pollID)
	if err != nil {
		return "", err
	}
	
	// Ensure storage directory exists
	if err := os.MkdirAll(s.storageDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create storage directory: %w", err)
	}
	
	// Create file path
	filename := fmt.Sprintf("poll_%d_qr.png", pollID)
	filePath := filepath.Join(s.storageDir, filename)
	
	// Save to file
	if err := os.WriteFile(filePath, qrData, 0644); err != nil {
		return "", fmt.Errorf("failed to save QR code: %w", err)
	}
	
	return filePath, nil
}

// GetQRCodePath returns the file path for a poll's QR code
func (s *Service) GetQRCodePath(pollID uint) string {
	filename := fmt.Sprintf("poll_%d_qr.png", pollID)
	return filepath.Join(s.storageDir, filename)
}

// QRCodeExists checks if a QR code file exists for a poll
func (s *Service) QRCodeExists(pollID uint) bool {
	filePath := s.GetQRCodePath(pollID)
	_, err := os.Stat(filePath)
	return err == nil
}

// DeleteQRCode removes a QR code file for a poll
func (s *Service) DeleteQRCode(pollID uint) error {
	filePath := s.GetQRCodePath(pollID)
	if !s.QRCodeExists(pollID) {
		return nil // Already deleted or never existed
	}
	
	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("failed to delete QR code: %w", err)
	}
	
	return nil
}

// GetQRCodeURL returns the public URL for accessing a QR code
func (s *Service) GetQRCodeURL(pollID uint) string {
	return fmt.Sprintf("%s/api/qr/%d", s.baseURL, pollID)
}