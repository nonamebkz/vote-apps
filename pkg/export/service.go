package export

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/jung-kurt/gofpdf"
	"github.com/xuri/excelize/v2"
	"polling-system/internal/models"
)

// ExportFormat represents the export format type
type ExportFormat string

const (
	FormatPDF   ExportFormat = "pdf"
	FormatExcel ExportFormat = "excel"
)

// ExportData represents the data structure for export
type ExportData struct {
	Poll         *models.Poll
	Options      []OptionData
	Votes        []VoteData
	Statistics   Statistics
	GeneratedAt  time.Time
	TotalUsers   int
}

// OptionData represents option data for export
type OptionData struct {
	ID         uint
	Text       string
	VoteCount  int
	Percentage float64
}

// VoteData represents vote data for export
type VoteData struct {
	ID         uint
	UserID     uint
	Username   string
	VoterID    string
	OptionText string
	VoteTime   time.Time
}

// Statistics represents poll statistics for export
type Statistics struct {
	TotalVotes        int
	TotalUsers        int
	ParticipationRate float64
	StartTime         time.Time
	EndTime           time.Time
	Duration          time.Duration
	Status            string
}

// Service handles export operations
type Service struct {
	exportDir string
}

// NewService creates a new export service
func NewService(exportDir string) *Service {
	// Create export directory if it doesn't exist
	if err := os.MkdirAll(exportDir, 0755); err != nil {
		fmt.Printf("Warning: failed to create export directory: %v\n", err)
	}
	
	return &Service{
		exportDir: exportDir,
	}
}

// PrepareExportData prepares data for export from poll with all relations
func (s *Service) PrepareExportData(poll *models.Poll, totalUsers int) *ExportData {
	data := &ExportData{
		Poll:        poll,
		GeneratedAt: time.Now(),
		TotalUsers:  totalUsers,
	}

	// Calculate total votes
	totalVotes := 0
	for _, option := range poll.Options {
		totalVotes += option.VoteCount
	}

	// Prepare options data
	data.Options = make([]OptionData, len(poll.Options))
	for i, option := range poll.Options {
		percentage := 0.0
		if totalVotes > 0 {
			percentage = float64(option.VoteCount) / float64(totalVotes) * 100
		}
		
		data.Options[i] = OptionData{
			ID:         option.ID,
			Text:       option.OptionText,
			VoteCount:  option.VoteCount,
			Percentage: percentage,
		}
	}

	// Prepare votes data
	data.Votes = make([]VoteData, len(poll.Votes))
	for i, vote := range poll.Votes {
		username := ""
		voterID := ""
		if vote.User.ID != 0 {
			username = vote.User.Username
			voterID = vote.User.VoterID
		}
		
		optionText := ""
		if vote.Option.ID != 0 {
			optionText = vote.Option.OptionText
		}
		
		data.Votes[i] = VoteData{
			ID:         vote.ID,
			UserID:     vote.UserID,
			Username:   username,
			VoterID:    voterID,
			OptionText: optionText,
			VoteTime:   vote.VoteTime,
		}
	}

	// Calculate statistics
	participationRate := 0.0
	if totalUsers > 0 {
		participationRate = float64(len(poll.Votes)) / float64(totalUsers) * 100
	}

	duration := poll.EndDate.Sub(poll.StartDate)
	if poll.Status == models.PollStatusActive || poll.Status == models.PollStatusPaused {
		duration = time.Since(poll.StartDate)
	}

	data.Statistics = Statistics{
		TotalVotes:        totalVotes,
		TotalUsers:        totalUsers,
		ParticipationRate: participationRate,
		StartTime:         poll.StartDate,
		EndTime:           poll.EndDate,
		Duration:          duration,
		Status:            string(poll.Status),
	}

	return data
}

// ExportToPDF exports poll data to PDF format
func (s *Service) ExportToPDF(data *ExportData) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()

	// Set font
	pdf.SetFont("Arial", "B", 16)
	
	// Title
	pdf.Cell(0, 10, fmt.Sprintf("Poll Results: %s", data.Poll.Title))
	pdf.Ln(15)

	// Poll information
	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(0, 8, "Poll Information")
	pdf.Ln(10)
	
	pdf.SetFont("Arial", "", 10)
	pdf.Cell(0, 6, fmt.Sprintf("Description: %s", data.Poll.Description))
	pdf.Ln(6)
	pdf.Cell(0, 6, fmt.Sprintf("Created By: %s", data.Poll.Creator.Username))
	pdf.Ln(6)
	pdf.Cell(0, 6, fmt.Sprintf("Status: %s", data.Statistics.Status))
	pdf.Ln(6)
	pdf.Cell(0, 6, fmt.Sprintf("Start Date: %s", data.Statistics.StartTime.Format("2006-01-02 15:04:05")))
	pdf.Ln(6)
	pdf.Cell(0, 6, fmt.Sprintf("End Date: %s", data.Statistics.EndTime.Format("2006-01-02 15:04:05")))
	pdf.Ln(6)
	pdf.Cell(0, 6, fmt.Sprintf("Generated At: %s", data.GeneratedAt.Format("2006-01-02 15:04:05")))
	pdf.Ln(15)

	// Statistics
	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(0, 8, "Statistics")
	pdf.Ln(10)
	
	pdf.SetFont("Arial", "", 10)
	pdf.Cell(0, 6, fmt.Sprintf("Total Votes: %d", data.Statistics.TotalVotes))
	pdf.Ln(6)
	pdf.Cell(0, 6, fmt.Sprintf("Total Users: %d", data.Statistics.TotalUsers))
	pdf.Ln(6)
	pdf.Cell(0, 6, fmt.Sprintf("Participation Rate: %.2f%%", data.Statistics.ParticipationRate))
	pdf.Ln(15)

	// Results
	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(0, 8, "Results")
	pdf.Ln(10)

	// Table header
	pdf.SetFont("Arial", "B", 10)
	pdf.Cell(80, 8, "Option")
	pdf.Cell(30, 8, "Votes")
	pdf.Cell(30, 8, "Percentage")
	pdf.Ln(8)

	// Table content
	pdf.SetFont("Arial", "", 10)
	for _, option := range data.Options {
		pdf.Cell(80, 6, option.Text)
		pdf.Cell(30, 6, strconv.Itoa(option.VoteCount))
		pdf.Cell(30, 6, fmt.Sprintf("%.2f%%", option.Percentage))
		pdf.Ln(6)
	}

	// Vote details (if space allows)
	if len(data.Votes) > 0 && pdf.GetY() < 200 {
		pdf.Ln(10)
		pdf.SetFont("Arial", "B", 12)
		pdf.Cell(0, 8, "Vote Details")
		pdf.Ln(10)

		// Vote table header
		pdf.SetFont("Arial", "B", 9)
		pdf.Cell(40, 6, "Username")
		pdf.Cell(30, 6, "Voter ID")
		pdf.Cell(60, 6, "Option")
		pdf.Cell(40, 6, "Vote Time")
		pdf.Ln(6)

		// Vote table content (limit to available space)
		pdf.SetFont("Arial", "", 8)
		maxVotes := 20 // Limit to prevent overflow
		for i, vote := range data.Votes {
			if i >= maxVotes || pdf.GetY() > 270 {
				pdf.Cell(0, 6, fmt.Sprintf("... and %d more votes", len(data.Votes)-i))
				break
			}
			
			pdf.Cell(40, 5, vote.Username)
			pdf.Cell(30, 5, vote.VoterID)
			pdf.Cell(60, 5, vote.OptionText)
			pdf.Cell(40, 5, vote.VoteTime.Format("2006-01-02 15:04"))
			pdf.Ln(5)
		}
	}

	// Generate PDF bytes
	var buf bytes.Buffer
	err := pdf.Output(&buf)
	if err != nil {
		return nil, fmt.Errorf("failed to generate PDF: %w", err)
	}

	return buf.Bytes(), nil
}

// ExportToExcel exports poll data to Excel format
func (s *Service) ExportToExcel(data *ExportData) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()

	// Create sheets
	summarySheet := "Summary"
	resultsSheet := "Results"
	votesSheet := "Votes"

	// Rename default sheet to Summary
	f.SetSheetName("Sheet1", summarySheet)
	
	// Create additional sheets
	_, err := f.NewSheet(resultsSheet)
	if err != nil {
		return nil, fmt.Errorf("failed to create results sheet: %w", err)
	}
	
	_, err = f.NewSheet(votesSheet)
	if err != nil {
		return nil, fmt.Errorf("failed to create votes sheet: %w", err)
	}

	// Summary Sheet
	if err := s.createSummarySheet(f, summarySheet, data); err != nil {
		return nil, fmt.Errorf("failed to create summary sheet: %w", err)
	}

	// Results Sheet
	if err := s.createResultsSheet(f, resultsSheet, data); err != nil {
		return nil, fmt.Errorf("failed to create results sheet: %w", err)
	}

	// Votes Sheet
	if err := s.createVotesSheet(f, votesSheet, data); err != nil {
		return nil, fmt.Errorf("failed to create votes sheet: %w", err)
	}

	// Set active sheet to Summary
	f.SetActiveSheet(0)

	// Generate Excel bytes
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, fmt.Errorf("failed to generate Excel file: %w", err)
	}

	return buf.Bytes(), nil
}

// createSummarySheet creates the summary sheet in Excel
func (s *Service) createSummarySheet(f *excelize.File, sheetName string, data *ExportData) error {
	// Title
	f.SetCellValue(sheetName, "A1", "Poll Results Summary")
	f.SetCellStyle(sheetName, "A1", "A1", s.getTitleStyle(f))

	// Poll Information
	row := 3
	f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), "Poll Information")
	f.SetCellStyle(sheetName, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), s.getHeaderStyle(f))
	row++

	f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), "Title:")
	f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), data.Poll.Title)
	row++

	f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), "Description:")
	f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), data.Poll.Description)
	row++

	f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), "Created By:")
	f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), data.Poll.Creator.Username)
	row++

	f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), "Status:")
	f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), data.Statistics.Status)
	row++

	f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), "Start Date:")
	f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), data.Statistics.StartTime.Format("2006-01-02 15:04:05"))
	row++

	f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), "End Date:")
	f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), data.Statistics.EndTime.Format("2006-01-02 15:04:05"))
	row++

	f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), "Generated At:")
	f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), data.GeneratedAt.Format("2006-01-02 15:04:05"))
	row += 2

	// Statistics
	f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), "Statistics")
	f.SetCellStyle(sheetName, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), s.getHeaderStyle(f))
	row++

	f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), "Total Votes:")
	f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), data.Statistics.TotalVotes)
	row++

	f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), "Total Users:")
	f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), data.Statistics.TotalUsers)
	row++

	f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), "Participation Rate:")
	f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), fmt.Sprintf("%.2f%%", data.Statistics.ParticipationRate))

	// Auto-fit columns
	f.SetColWidth(sheetName, "A", "A", 20)
	f.SetColWidth(sheetName, "B", "B", 30)

	return nil
}

// createResultsSheet creates the results sheet in Excel
func (s *Service) createResultsSheet(f *excelize.File, sheetName string, data *ExportData) error {
	// Headers
	f.SetCellValue(sheetName, "A1", "Option")
	f.SetCellValue(sheetName, "B1", "Vote Count")
	f.SetCellValue(sheetName, "C1", "Percentage")
	
	headerStyle := s.getHeaderStyle(f)
	f.SetCellStyle(sheetName, "A1", "C1", headerStyle)

	// Data
	for i, option := range data.Options {
		row := i + 2
		f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), option.Text)
		f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), option.VoteCount)
		f.SetCellValue(sheetName, fmt.Sprintf("C%d", row), fmt.Sprintf("%.2f%%", option.Percentage))
	}

	// Auto-fit columns
	f.SetColWidth(sheetName, "A", "A", 40)
	f.SetColWidth(sheetName, "B", "B", 15)
	f.SetColWidth(sheetName, "C", "C", 15)

	return nil
}

// createVotesSheet creates the votes sheet in Excel
func (s *Service) createVotesSheet(f *excelize.File, sheetName string, data *ExportData) error {
	// Headers
	f.SetCellValue(sheetName, "A1", "Vote ID")
	f.SetCellValue(sheetName, "B1", "Username")
	f.SetCellValue(sheetName, "C1", "Voter ID")
	f.SetCellValue(sheetName, "D1", "Option")
	f.SetCellValue(sheetName, "E1", "Vote Time")
	
	headerStyle := s.getHeaderStyle(f)
	f.SetCellStyle(sheetName, "A1", "E1", headerStyle)

	// Data
	for i, vote := range data.Votes {
		row := i + 2
		f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), vote.ID)
		f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), vote.Username)
		f.SetCellValue(sheetName, fmt.Sprintf("C%d", row), vote.VoterID)
		f.SetCellValue(sheetName, fmt.Sprintf("D%d", row), vote.OptionText)
		f.SetCellValue(sheetName, fmt.Sprintf("E%d", row), vote.VoteTime.Format("2006-01-02 15:04:05"))
	}

	// Auto-fit columns
	f.SetColWidth(sheetName, "A", "A", 10)
	f.SetColWidth(sheetName, "B", "B", 20)
	f.SetColWidth(sheetName, "C", "C", 25)
	f.SetColWidth(sheetName, "D", "D", 40)
	f.SetColWidth(sheetName, "E", "E", 20)

	return nil
}

// getTitleStyle returns the title style for Excel
func (s *Service) getTitleStyle(f *excelize.File) int {
	style, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold: true,
			Size: 16,
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
		},
	})
	return style
}

// getHeaderStyle returns the header style for Excel
func (s *Service) getHeaderStyle(f *excelize.File) int {
	style, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold: true,
			Size: 12,
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#E0E0E0"},
			Pattern: 1,
		},
	})
	return style
}

// SaveToFile saves export data to a file
func (s *Service) SaveToFile(data []byte, filename string) (string, error) {
	filepath := filepath.Join(s.exportDir, filename)
	
	if err := os.WriteFile(filepath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to save file: %w", err)
	}
	
	return filepath, nil
}

// GenerateFilename generates a filename for export
func (s *Service) GenerateFilename(pollID uint, format ExportFormat) string {
	timestamp := time.Now().Format("20060102_150405")
	extension := "pdf"
	if format == FormatExcel {
		extension = "xlsx"
	}
	
	return fmt.Sprintf("poll_%d_results_%s.%s", pollID, timestamp, extension)
}

// CleanupOldFiles removes old export files (older than specified duration)
func (s *Service) CleanupOldFiles(maxAge time.Duration) error {
	entries, err := os.ReadDir(s.exportDir)
	if err != nil {
		return fmt.Errorf("failed to read export directory: %w", err)
	}
	
	cutoff := time.Now().Add(-maxAge)
	
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		
		info, err := entry.Info()
		if err != nil {
			continue
		}
		
		if info.ModTime().Before(cutoff) {
			filepath := filepath.Join(s.exportDir, entry.Name())
			if err := os.Remove(filepath); err != nil {
				fmt.Printf("Warning: failed to remove old export file %s: %v\n", filepath, err)
			}
		}
	}
	
	return nil
}

// DeleteFile removes a specific export file
func (s *Service) DeleteFile(filename string) error {
	filepath := filepath.Join(s.exportDir, filename)
	
	if err := os.Remove(filepath); err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}
	
	return nil
}

// GetExportDir returns the export directory path
func (s *Service) GetExportDir() string {
	return s.exportDir
}