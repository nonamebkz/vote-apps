# Export Service

This package provides export functionality for poll results in PDF and Excel formats.

## Features

- **PDF Export**: Generate comprehensive PDF reports with poll information, statistics, and results
- **Excel Export**: Create multi-sheet Excel workbooks with detailed voting data
- **File Management**: Save exports to disk with automatic cleanup of old files
- **Secure Downloads**: Generate secure download links with permission checks

## Usage

### Initialize the Service

```go
import "polling-system/pkg/export"

exportService := export.NewService("./storage/exports")
```

### Prepare Export Data

```go
// Assuming you have a poll with all relations loaded
data := exportService.PrepareExportData(poll, totalUsers)
```

### Export to PDF

```go
pdfData, err := exportService.ExportToPDF(data)
if err != nil {
    // Handle error
}

// Save to file
filename := exportService.GenerateFilename(pollID, export.FormatPDF)
filepath, err := exportService.SaveToFile(pdfData, filename)
```

### Export to Excel

```go
excelData, err := exportService.ExportToExcel(data)
if err != nil {
    // Handle error
}

// Save to file
filename := exportService.GenerateFilename(pollID, export.FormatExcel)
filepath, err := exportService.SaveToFile(excelData, filename)
```

### Cleanup Old Files

```go
// Remove files older than 24 hours
maxAge := 24 * time.Hour
err := exportService.CleanupOldFiles(maxAge)
```

## API Endpoints

The export functionality is exposed through the following endpoints:

### Export Poll Results (with file save)
```
GET /api/admin/polls/{id}/export?format=pdf|excel
```

Returns export metadata including download URL.

### Direct Export (no file save)
```
GET /api/admin/polls/{id}/export/direct?format=pdf|excel
```

Returns the file directly for immediate download.

### Download Exported File
```
GET /api/admin/polls/{id}/export/download/{filename}
```

Downloads a previously exported file.

### Get Supported Formats
```
GET /api/admin/export/formats
```

Returns list of supported export formats.

### Cleanup Old Exports
```
POST /api/admin/export/cleanup?max_age_hours=24
```

Removes export files older than specified hours.

## Export Data Structure

### PDF Export Includes:
- Poll title and description
- Creator information
- Poll status and dates
- Statistics (total votes, participation rate)
- Results table with vote counts and percentages
- Vote details (limited to prevent overflow)

### Excel Export Includes:
Three sheets:
1. **Summary**: Poll information and statistics
2. **Results**: Vote counts and percentages per option
3. **Votes**: Detailed vote records with user information

## Requirements Validation

This implementation satisfies the following requirements:

- **Requirement 4.1**: Exports results in PDF and Excel formats
- **Requirement 4.2**: Includes all voting data, statistics, and timestamps
- **Requirement 4.3**: Supports real-time data for active polls
- **Requirement 4.4**: Provides secure download links

## Testing

Run tests with:
```bash
go test ./pkg/export/...
```

Tests cover:
- Service initialization
- Data preparation
- PDF generation
- Excel generation
- File operations
- Cleanup functionality
