package exchange

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"farsiforge/pkg/project"

	"github.com/xuri/excelize/v2"
)

// ExportXLSX exports project entries to an Excel file for translation.
// Columns: ID | Source | Translation | Status | Context | File | Path | Notes
func ExportXLSX(proj *project.Project, outputPath string) error {
	f := excelize.NewFile()
	defer f.Close()

	sheet := "Translations"
	f.SetSheetName(f.GetSheetName(0), sheet)

	// Set headers
	headers := []string{"ID", "Source (متن اصلی)", "Translation (ترجمه)", "Status", "Context", "File", "Path", "Notes"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}

	// Style header row
	style, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 12},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#4472C4"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	f.SetCellStyle(sheet, "A1", "H1", style)

	// Set column widths
	f.SetColWidth(sheet, "A", "A", 10)   // ID
	f.SetColWidth(sheet, "B", "B", 40)   // Source
	f.SetColWidth(sheet, "C", "C", 40)   // Translation
	f.SetColWidth(sheet, "D", "D", 15)   // Status
	f.SetColWidth(sheet, "E", "E", 15)   // Context
	f.SetColWidth(sheet, "F", "F", 30)   // File
	f.SetColWidth(sheet, "G", "G", 30)   // Path
	f.SetColWidth(sheet, "H", "H", 20)   // Notes

	// RTL alignment for Source and Translation columns
	rtlStyle, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Horizontal: "right", Vertical: "center", ReadingOrder: 2},
	})

	// Data rows
	for i, entry := range proj.Entries {
		row := i + 2
		f.SetCellValue(sheet, cell(1, row), entry.ID)
		f.SetCellValue(sheet, cell(2, row), entry.Source)
		f.SetCellValue(sheet, cell(3, row), entry.Translation)
		f.SetCellValue(sheet, cell(4, row), string(entry.Status))
		f.SetCellValue(sheet, cell(5, row), entry.Context)
		f.SetCellValue(sheet, cell(6, row), entry.File)
		f.SetCellValue(sheet, cell(7, row), entry.Path)
		f.SetCellValue(sheet, cell(8, row), entry.Notes)

		// Apply RTL style to source and translation columns
		f.SetCellStyle(sheet, cell(2, row), cell(3, row), rtlStyle)
	}

	// Freeze header row
	f.SetPanes(sheet, &excelize.Panes{
		Freeze:      true,
		XSplit:      0,
		YSplit:      1,
		TopLeftCell: "A2",
		ActivePane:  "bottomLeft",
	})

	// Enable autofilter
	f.AutoFilter(sheet, "A1:H1", []excelize.AutoFilterOptions{})

	return f.SaveAs(outputPath)
}

// ImportXLSX imports translations from an Excel file.
func ImportXLSX(proj *project.Project, inputPath string) (int, error) {
	f, err := excelize.OpenFile(inputPath)
	if err != nil {
		return 0, fmt.Errorf("open xlsx: %w", err)
	}
	defer f.Close()

	sheet := "Translations"
	rows, err := f.GetRows(sheet)
	if err != nil {
		return 0, fmt.Errorf("read sheet: %w", err)
	}

	if len(rows) < 2 {
		return 0, fmt.Errorf("no data rows in xlsx")
	}

	count := 0
	// Skip header row
	for _, row := range rows[1:] {
		if len(row) < 3 {
			continue
		}
		id := strings.TrimSpace(row[0])
		translation := strings.TrimSpace(row[2])

		if id == "" || translation == "" {
			continue
		}

		status := project.StatusTranslated
		if len(row) > 3 && row[3] != "" {
			s := project.Status(strings.TrimSpace(row[3]))
			if s != "" {
				status = s
			}
		}

		if err := proj.SetTranslation(id, translation, status); err == nil {
			count++
		}
	}

	return count, nil
}

// ExportCSV exports project entries to a CSV file.
func ExportCSV(proj *project.Project, outputPath string) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write BOM for Excel UTF-8 detection
	file.Write([]byte{0xEF, 0xBB, 0xBF})

	w := csv.NewWriter(file)
	defer w.Flush()

	// Header
	w.Write([]string{"ID", "Source", "Translation", "Status", "Context", "File", "Path", "Notes"})

	// Data
	for _, entry := range proj.Entries {
		w.Write([]string{
			entry.ID,
			entry.Source,
			entry.Translation,
			string(entry.Status),
			entry.Context,
			entry.File,
			entry.Path,
			entry.Notes,
		})
	}

	return w.Error()
}

// ImportCSV imports translations from a CSV file.
func ImportCSV(proj *project.Project, inputPath string) (int, error) {
	file, err := os.Open(inputPath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	// Skip BOM if present
	bom := make([]byte, 3)
	n, _ := file.Read(bom)
	if n < 3 || bom[0] != 0xEF || bom[1] != 0xBB || bom[2] != 0xBF {
		file.Seek(0, 0)
	}

	r := csv.NewReader(file)
	rows, err := r.ReadAll()
	if err != nil {
		return 0, err
	}

	if len(rows) < 2 {
		return 0, fmt.Errorf("no data rows")
	}

	count := 0
	for _, row := range rows[1:] {
		if len(row) < 3 {
			continue
		}
		id := strings.TrimSpace(row[0])
		translation := strings.TrimSpace(row[2])
		if id == "" || translation == "" {
			continue
		}
		status := project.StatusTranslated
		if len(row) > 3 && row[3] != "" {
			status = project.Status(row[3])
		}
		if err := proj.SetTranslation(id, translation, status); err == nil {
			count++
		}
	}

	return count, nil
}

// Export exports to XLSX or CSV based on file extension.
func Export(proj *project.Project, outputPath string) error {
	ext := strings.ToLower(filepath.Ext(outputPath))
	switch ext {
	case ".xlsx":
		return ExportXLSX(proj, outputPath)
	case ".csv":
		return ExportCSV(proj, outputPath)
	default:
		return fmt.Errorf("unsupported format: %s (use .xlsx or .csv)", ext)
	}
}

// Import imports from XLSX or CSV based on file extension.
func Import(proj *project.Project, inputPath string) (int, error) {
	ext := strings.ToLower(filepath.Ext(inputPath))
	switch ext {
	case ".xlsx":
		return ImportXLSX(proj, inputPath)
	case ".csv":
		return ImportCSV(proj, inputPath)
	default:
		return 0, fmt.Errorf("unsupported format: %s (use .xlsx or .csv)", ext)
	}
}

func cell(col, row int) string {
	name, _ := excelize.CoordinatesToCellName(col, row)
	return name
}
