package export

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"polling-system/internal/models"
)

func TestNewService(t *testing.T) {
	tempDir := t.TempDir()
	service := NewService(tempDir)
	
	if service == nil {
		t.Fatal("Expected service to be created, got nil")
	}
	
	if service.exportDir != tempDir {
		t.Errorf("Expected export dir to be %s, got %s", tempDir, service.exportDir)
	}
}

func TestPrepareExportData(t *testing.T) {
	service := NewService(t.TempDir())
	
	// Create test poll data
	poll := &models.Poll{
		ID:          1,
		Title:       "Test Poll",
		Description: "Test Description",
		StartDate:   time.Now().Add(-time.Hour),
		EndDate:     time.Now().Add(time.Hour),
		Status:      models.PollStatusActive,
		Creator: models.User{
			ID:       1,
			Username: "admin",
		},
		Options: []models.Option{
			{ID: 1, OptionText: "Option 1", VoteCount: 5},
			{ID: 2, OptionText: "Option 2", VoteCount: 3},
		},
		Votes: []models.Vote{
			{
				ID:       1,
				UserID:   1,
				OptionID: 1,
				VoteTime: time.Now(),
				User:     models.User{ID: 1, Username: "user1", VoterID: "voter1"},
				Option:   models.Option{ID: 1, OptionText: "Option 1"},
			},
		},
	}
	
	data := service.PrepareExportData(poll, 10)
	
	if data == nil {
		t.Fatal("Expected export data to be created, got nil")
	}
	
	if data.Poll.ID != 1 {
		t.Errorf("Expected poll ID to be 1, got %d", data.Poll.ID)
	}
	
	if len(data.Options) != 2 {
		t.Errorf("Expected 2 options, got %d", len(data.Options))
	}
	
	if data.Statistics.TotalVotes != 8 {
		t.Errorf("Expected total votes to be 8, got %d", data.Statistics.TotalVotes)
	}
	
	if data.Statistics.TotalUsers != 10 {
		t.Errorf("Expected total users to be 10, got %d", data.Statistics.TotalUsers)
	}
}

func TestExportToPDF(t *testing.T) {
	service := NewService(t.TempDir())
	
	// Create minimal test data
	data := &ExportData{
		Poll: &models.Poll{
			ID:          1,
			Title:       "Test Poll",
			Description: "Test Description",
			Creator:     models.User{Username: "admin"},
		},
		Options: []OptionData{
			{ID: 1, Text: "Option 1", VoteCount: 5, Percentage: 62.5},
			{ID: 2, Text: "Option 2", VoteCount: 3, Percentage: 37.5},
		},
		Statistics: Statistics{
			TotalVotes:        8,
			TotalUsers:        10,
			ParticipationRate: 80.0,
			StartTime:         time.Now().Add(-time.Hour),
			EndTime:           time.Now().Add(time.Hour),
			Status:            "active",
		},
		GeneratedAt: time.Now(),
	}
	
	pdfData, err := service.ExportToPDF(data)
	if err != nil {
		t.Fatalf("Failed to export to PDF: %v", err)
	}
	
	if len(pdfData) == 0 {
		t.Error("Expected PDF data to be generated, got empty data")
	}
	
	// Check if it starts with PDF header
	if len(pdfData) < 4 || string(pdfData[:4]) != "%PDF" {
		t.Error("Generated data does not appear to be a valid PDF")
	}
}

func TestExportToExcel(t *testing.T) {
	service := NewService(t.TempDir())
	
	// Create minimal test data
	data := &ExportData{
		Poll: &models.Poll{
			ID:          1,
			Title:       "Test Poll",
			Description: "Test Description",
			Creator:     models.User{Username: "admin"},
		},
		Options: []OptionData{
			{ID: 1, Text: "Option 1", VoteCount: 5, Percentage: 62.5},
			{ID: 2, Text: "Option 2", VoteCount: 3, Percentage: 37.5},
		},
		Votes: []VoteData{
			{ID: 1, UserID: 1, Username: "user1", VoterID: "voter1", OptionText: "Option 1", VoteTime: time.Now()},
		},
		Statistics: Statistics{
			TotalVotes:        8,
			TotalUsers:        10,
			ParticipationRate: 80.0,
			StartTime:         time.Now().Add(-time.Hour),
			EndTime:           time.Now().Add(time.Hour),
			Status:            "active",
		},
		GeneratedAt: time.Now(),
	}
	
	excelData, err := service.ExportToExcel(data)
	if err != nil {
		t.Fatalf("Failed to export to Excel: %v", err)
	}
	
	if len(excelData) == 0 {
		t.Error("Expected Excel data to be generated, got empty data")
	}
}

func TestSaveToFile(t *testing.T) {
	tempDir := t.TempDir()
	service := NewService(tempDir)
	
	testData := []byte("test data")
	filename := "test.txt"
	
	filePath, err := service.SaveToFile(testData, filename)
	if err != nil {
		t.Fatalf("Failed to save file: %v", err)
	}
	
	expectedPath := filepath.Join(tempDir, filename)
	if filePath != expectedPath {
		t.Errorf("Expected file path to be %s, got %s", expectedPath, filePath)
	}
	
	// Check if file exists and has correct content
	savedData, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read saved file: %v", err)
	}
	
	if string(savedData) != "test data" {
		t.Errorf("Expected file content to be 'test data', got '%s'", string(savedData))
	}
}

func TestGenerateFilename(t *testing.T) {
	service := NewService(t.TempDir())
	
	filename := service.GenerateFilename(123, FormatPDF)
	if filename == "" {
		t.Error("Expected filename to be generated, got empty string")
	}
	
	// Check if filename contains poll ID and has correct extension
	if !contains(filename, "poll_123") {
		t.Errorf("Expected filename to contain 'poll_123', got %s", filename)
	}
	
	if !contains(filename, ".pdf") {
		t.Errorf("Expected filename to have .pdf extension, got %s", filename)
	}
	
	// Test Excel format
	excelFilename := service.GenerateFilename(456, FormatExcel)
	if !contains(excelFilename, ".xlsx") {
		t.Errorf("Expected Excel filename to have .xlsx extension, got %s", excelFilename)
	}
}

func TestCleanupOldFiles(t *testing.T) {
	tempDir := t.TempDir()
	service := NewService(tempDir)
	
	// Create test files with different ages
	oldFile := filepath.Join(tempDir, "old_file.txt")
	newFile := filepath.Join(tempDir, "new_file.txt")
	
	// Create old file
	if err := os.WriteFile(oldFile, []byte("old"), 0644); err != nil {
		t.Fatalf("Failed to create old file: %v", err)
	}
	
	// Create new file
	if err := os.WriteFile(newFile, []byte("new"), 0644); err != nil {
		t.Fatalf("Failed to create new file: %v", err)
	}
	
	// Change old file's modification time to be older
	oldTime := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(oldFile, oldTime, oldTime); err != nil {
		t.Fatalf("Failed to change old file time: %v", err)
	}
	
	// Cleanup files older than 1 hour
	err := service.CleanupOldFiles(time.Hour)
	if err != nil {
		t.Fatalf("Failed to cleanup old files: %v", err)
	}
	
	// Check that old file is deleted and new file remains
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Error("Expected old file to be deleted")
	}
	
	if _, err := os.Stat(newFile); err != nil {
		t.Error("Expected new file to remain")
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && 
		   (s == substr || 
		    (len(s) > len(substr) && 
		     (s[:len(substr)] == substr || 
		      s[len(s)-len(substr):] == substr || 
		      containsInMiddle(s, substr))))
}

func containsInMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}