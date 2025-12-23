package services

import (
	"fmt"
	"time"

	"polling-system/internal/models"
	"polling-system/internal/repositories"
	"polling-system/pkg/export"
)

// ExportService handles export business logic
type ExportService struct {
	pollRepo repositories.PollRepositoryInterface
	userRepo repositories.UserRepositoryInterface
	voteRepo repositories.VoteRepositoryInterface
	exporter *export.Service
}

// NewExportService creates a new export service
func NewExportService(
	pollRepo repositories.PollRepositoryInterface,
	userRepo repositories.UserRepositoryInterface,
	voteRepo repositories.VoteRepositoryInterface,
	exporter *export.Service,
) *ExportService {
	return &ExportService{
		pollRepo: pollRepo,
		userRepo: userRepo,
		voteRepo: voteRepo,
		exporter: exporter,
	}
}

// ExportRequest represents an export request
type ExportRequest struct {
	PollID uint                `json:"poll_id" validate:"required"`
	Format export.ExportFormat `json:"format" validate:"required,oneof=pdf excel"`
}

// ExportResponse represents an export response
type ExportResponse struct {
	Filename    string    `json:"filename"`
	FilePath    string    `json:"file_path"`
	Format      string    `json:"format"`
	Size        int       `json:"size"`
	GeneratedAt time.Time `json:"generated_at"`
	DownloadURL string    `json:"download_url"`
}

// ExportPollResults exports poll results in the specified format
func (s *ExportService) ExportPollResults(pollID uint, format export.ExportFormat, userID uint) (*ExportResponse, error) {
	// Verify user has permission to export
	if err := s.verifyExportPermission(pollID, userID); err != nil {
		return nil, fmt.Errorf("permission denied: %w", err)
	}

	// Get poll with all related data
	poll, err := s.pollRepo.GetByIDWithAll(pollID)
	if err != nil {
		return nil, fmt.Errorf("failed to get poll: %w", err)
	}

	// Get total user count for participation calculation
	totalUsers, err := s.getTotalActiveUsers()
	if err != nil {
		return nil, fmt.Errorf("failed to get total users: %w", err)
	}

	// Prepare export data
	exportData := s.exporter.PrepareExportData(poll, int(totalUsers))

	// Generate export based on format
	var data []byte
	switch format {
	case export.FormatPDF:
		data, err = s.exporter.ExportToPDF(exportData)
		if err != nil {
			return nil, fmt.Errorf("failed to generate PDF: %w", err)
		}
	case export.FormatExcel:
		data, err = s.exporter.ExportToExcel(exportData)
		if err != nil {
			return nil, fmt.Errorf("failed to generate Excel: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported export format: %s", format)
	}

	// Generate filename and save file
	filename := s.exporter.GenerateFilename(pollID, format)
	filePath, err := s.exporter.SaveToFile(data, filename)
	if err != nil {
		return nil, fmt.Errorf("failed to save export file: %w", err)
	}

	// Create response
	response := &ExportResponse{
		Filename:    filename,
		FilePath:    filePath,
		Format:      string(format),
		Size:        len(data),
		GeneratedAt: time.Now(),
		DownloadURL: fmt.Sprintf("/api/admin/polls/%d/export/download/%s", pollID, filename),
	}

	return response, nil
}

// ExportPollResultsToBytes exports poll results and returns the raw bytes
func (s *ExportService) ExportPollResultsToBytes(pollID uint, format export.ExportFormat, userID uint) ([]byte, string, error) {
	// Verify user has permission to export
	if err := s.verifyExportPermission(pollID, userID); err != nil {
		return nil, "", fmt.Errorf("permission denied: %w", err)
	}

	// Get poll with all related data
	poll, err := s.pollRepo.GetByIDWithAll(pollID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get poll: %w", err)
	}

	// Get total user count for participation calculation
	totalUsers, err := s.getTotalActiveUsers()
	if err != nil {
		return nil, "", fmt.Errorf("failed to get total users: %w", err)
	}

	// Prepare export data
	exportData := s.exporter.PrepareExportData(poll, int(totalUsers))

	// Generate export based on format
	var data []byte
	switch format {
	case export.FormatPDF:
		data, err = s.exporter.ExportToPDF(exportData)
		if err != nil {
			return nil, "", fmt.Errorf("failed to generate PDF: %w", err)
		}
	case export.FormatExcel:
		data, err = s.exporter.ExportToExcel(exportData)
		if err != nil {
			return nil, "", fmt.Errorf("failed to generate Excel: %w", err)
		}
	default:
		return nil, "", fmt.Errorf("unsupported export format: %s", format)
	}

	filename := s.exporter.GenerateFilename(pollID, format)
	return data, filename, nil
}

// GetExportHistory returns the export history for a poll (if files exist)
func (s *ExportService) GetExportHistory(pollID uint, userID uint) ([]string, error) {
	// Verify user has permission
	if err := s.verifyExportPermission(pollID, userID); err != nil {
		return nil, fmt.Errorf("permission denied: %w", err)
	}

	// This is a simplified implementation - in a real system you might want to
	// track export history in the database
	return []string{}, nil
}

// CleanupOldExports removes old export files
func (s *ExportService) CleanupOldExports(maxAge time.Duration) error {
	return s.exporter.CleanupOldFiles(maxAge)
}

// DeleteExportFile deletes a specific export file
func (s *ExportService) DeleteExportFile(filename string, userID uint) error {
	// In a real implementation, you might want to verify the user has permission
	// to delete this specific file
	return s.exporter.DeleteFile(filename)
}

// verifyExportPermission checks if user has permission to export poll results
func (s *ExportService) verifyExportPermission(pollID, userID uint) error {
	// Get user
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	// Admins can export any poll
	if user.IsAdmin() {
		return nil
	}

	// Get poll to check if user is the creator
	poll, err := s.pollRepo.GetByID(pollID)
	if err != nil {
		return fmt.Errorf("poll not found: %w", err)
	}

	// Poll creators can export their own polls
	if poll.CreatedBy == userID {
		return nil
	}

	return fmt.Errorf("only admins and poll creators can export poll results")
}

// getTotalActiveUsers returns the count of active users
func (s *ExportService) getTotalActiveUsers() (int64, error) {
	// Use CountWithFilters to count active users
	activeStatus := models.UserStatusActive
	return s.userRepo.CountWithFilters(nil, &activeStatus)
}

// GetSupportedFormats returns the list of supported export formats
func (s *ExportService) GetSupportedFormats() []export.ExportFormat {
	return []export.ExportFormat{
		export.FormatPDF,
		export.FormatExcel,
	}
}

// ValidateExportRequest validates an export request
func (s *ExportService) ValidateExportRequest(req ExportRequest) error {
	if req.PollID == 0 {
		return fmt.Errorf("poll ID is required")
	}

	supportedFormats := s.GetSupportedFormats()
	for _, format := range supportedFormats {
		if req.Format == format {
			return nil
		}
	}

	return fmt.Errorf("unsupported export format: %s", req.Format)
}

// GetExportStats returns statistics about exports
func (s *ExportService) GetExportStats() map[string]interface{} {
	return map[string]interface{}{
		"supported_formats": s.GetSupportedFormats(),
		"export_directory":  s.exporter.GetExportDir(),
	}
}