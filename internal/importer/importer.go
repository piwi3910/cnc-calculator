// Package importer provides CSV and Excel import functionality for part lists.
// It supports automatic delimiter detection, flexible column mapping, and
// case-insensitive header recognition.
package importer

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/piwi3910/cnc-calculator/internal/model"
	"github.com/xuri/excelize/v2"
)

// ImportResult holds the results of an import operation.
type ImportResult struct {
	Parts    []model.Part
	Errors   []string
	Warnings []string
}

// ColumnMapping maps semantic column roles to their indices in the data.
type ColumnMapping struct {
	Label    int
	Width    int
	Height   int
	Quantity int
	Grain    int
}

// headerAliases maps canonical column names to their accepted aliases (all lowercase).
var headerAliases = map[string][]string{
	"label":    {"label", "name", "part", "part name", "description", "desc", "piece", "item"},
	"width":    {"width", "w", "length", "len", "x"},
	"height":   {"height", "h", "depth", "d", "y"},
	"quantity": {"quantity", "qty", "count", "num", "amount", "pcs", "pieces"},
	"grain":    {"grain", "grain direction", "direction", "grain dir", "orientation"},
}

// DetectCSVDelimiter reads the file content and determines the most likely CSV delimiter.
// It tries comma, semicolon, tab, and pipe. The delimiter that produces the most
// consistent (non-one) column count across lines wins.
func DetectCSVDelimiter(data []byte) rune {
	candidates := []rune{',', ';', '\t', '|'}
	bestDelimiter := ','
	bestScore := 0

	for _, delim := range candidates {
		reader := csv.NewReader(bytes.NewReader(data))
		reader.Comma = delim
		reader.LazyQuotes = true
		reader.FieldsPerRecord = -1 // Allow variable field counts

		records, err := reader.ReadAll()
		if err != nil || len(records) < 1 {
			continue
		}

		// Score: count how many rows have the same column count as the first row
		// Only consider delimiters that produce more than 1 column
		firstCols := len(records[0])
		if firstCols < 2 {
			continue
		}

		score := 0
		for _, row := range records {
			if len(row) == firstCols {
				score++
			}
		}

		// Prefer delimiters with higher consistency and more columns
		weighted := score*10 + firstCols
		if weighted > bestScore {
			bestScore = weighted
			bestDelimiter = delim
		}
	}

	return bestDelimiter
}

// DetectColumns examines a header row and returns a ColumnMapping.
// It performs case-insensitive matching against known aliases for each column role.
// Returns the mapping and true if a header was detected, or a default positional
// mapping and false if no header was found.
func DetectColumns(row []string) (ColumnMapping, bool) {
	mapping := ColumnMapping{
		Label:    -1,
		Width:    -1,
		Height:   -1,
		Quantity: -1,
		Grain:    -1,
	}

	isHeader := false
	for i, cell := range row {
		normalized := strings.ToLower(strings.TrimSpace(cell))
		for role, aliases := range headerAliases {
			for _, alias := range aliases {
				if normalized == alias {
					isHeader = true
					switch role {
					case "label":
						if mapping.Label == -1 {
							mapping.Label = i
						}
					case "width":
						if mapping.Width == -1 {
							mapping.Width = i
						}
					case "height":
						if mapping.Height == -1 {
							mapping.Height = i
						}
					case "quantity":
						if mapping.Quantity == -1 {
							mapping.Quantity = i
						}
					case "grain":
						if mapping.Grain == -1 {
							mapping.Grain = i
						}
					}
				}
			}
		}
	}

	if !isHeader {
		// Fall back to positional mapping: Label, Width, Height, Quantity, Grain
		return ColumnMapping{
			Label:    0,
			Width:    1,
			Height:   2,
			Quantity: 3,
			Grain:    4,
		}, false
	}

	return mapping, true
}

// parseGrain converts a grain direction string to a model.Grain value.
// It returns the grain value and a boolean indicating whether the string was recognized.
func parseGrain(s string) (model.Grain, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "horizontal", "h":
		return model.GrainHorizontal, true
	case "vertical", "v":
		return model.GrainVertical, true
	case "", "none", "n", "-":
		return model.GrainNone, true
	default:
		return model.GrainNone, false
	}
}

// getCell safely retrieves a cell value from a row by column index.
// Returns empty string if the index is out of range or negative.
func getCell(row []string, idx int) string {
	if idx < 0 || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
}

// parseRow extracts a Part from a row using the given column mapping.
// Returns the part, any error message, and any warning message.
func parseRow(row []string, mapping ColumnMapping, rowLabel string, partCount int) (model.Part, string, string) {
	label := getCell(row, mapping.Label)
	if label == "" {
		label = fmt.Sprintf("Part %d", partCount+1)
	}

	widthStr := getCell(row, mapping.Width)
	if widthStr == "" {
		return model.Part{}, fmt.Sprintf("%s: Missing width value", rowLabel), ""
	}
	width, err := strconv.ParseFloat(widthStr, 64)
	if err != nil {
		return model.Part{}, fmt.Sprintf("%s: Invalid width '%s'", rowLabel, widthStr), ""
	}

	heightStr := getCell(row, mapping.Height)
	if heightStr == "" {
		return model.Part{}, fmt.Sprintf("%s: Missing height value", rowLabel), ""
	}
	height, err := strconv.ParseFloat(heightStr, 64)
	if err != nil {
		return model.Part{}, fmt.Sprintf("%s: Invalid height '%s'", rowLabel, heightStr), ""
	}

	qtyStr := getCell(row, mapping.Quantity)
	if qtyStr == "" {
		return model.Part{}, fmt.Sprintf("%s: Missing quantity value", rowLabel), ""
	}
	qty, err := strconv.Atoi(qtyStr)
	if err != nil {
		return model.Part{}, fmt.Sprintf("%s: Invalid quantity '%s'", rowLabel, qtyStr), ""
	}

	if width <= 0 || height <= 0 || qty <= 0 {
		return model.Part{}, fmt.Sprintf("%s: Width, height, and quantity must be positive", rowLabel), ""
	}

	part := model.NewPart(label, width, height, qty)

	// Optional grain direction
	var warning string
	grainStr := getCell(row, mapping.Grain)
	if grainStr != "" {
		grain, ok := parseGrain(grainStr)
		if ok {
			part.Grain = grain
		} else {
			warning = fmt.Sprintf("%s: Unknown grain direction '%s', defaulting to None", rowLabel, grainStr)
		}
	}

	return part, "", warning
}

// isEmptyRow returns true if the row has no meaningful content.
func isEmptyRow(row []string) bool {
	for _, cell := range row {
		if strings.TrimSpace(cell) != "" {
			return false
		}
	}
	return true
}

// ImportCSV imports parts from a CSV file.
// It automatically detects the delimiter and maps columns by header names.
// Supports comma, semicolon, tab, and pipe delimiters.
func ImportCSV(path string) ImportResult {
	result := ImportResult{}

	data, err := os.ReadFile(path)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Cannot open file: %v", err))
		return result
	}

	if len(bytes.TrimSpace(data)) == 0 {
		result.Errors = append(result.Errors, "File is empty")
		return result
	}

	delimiter := DetectCSVDelimiter(data)
	if delimiter != ',' {
		delimName := map[rune]string{';': "semicolon", '\t': "tab", '|': "pipe"}[delimiter]
		result.Warnings = append(result.Warnings, fmt.Sprintf("Detected %s delimiter", delimName))
	}

	reader := csv.NewReader(bytes.NewReader(data))
	reader.Comma = delimiter
	reader.LazyQuotes = true
	reader.FieldsPerRecord = -1

	records, err := reader.ReadAll()
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Cannot read CSV: %v", err))
		return result
	}

	if len(records) == 0 {
		result.Errors = append(result.Errors, "File is empty")
		return result
	}

	result = importFromRows(records, "Line", result.Warnings)
	return result
}

// ImportCSVFromReader imports parts from a CSV reader with a specific delimiter.
// This is useful for testing or when the delimiter is already known.
func ImportCSVFromReader(reader io.Reader, delimiter rune) ImportResult {
	result := ImportResult{}

	csvReader := csv.NewReader(reader)
	csvReader.Comma = delimiter
	csvReader.LazyQuotes = true
	csvReader.FieldsPerRecord = -1

	records, err := csvReader.ReadAll()
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Cannot read CSV: %v", err))
		return result
	}

	if len(records) == 0 {
		result.Errors = append(result.Errors, "File is empty")
		return result
	}

	return importFromRows(records, "Line", nil)
}

// ImportExcel imports parts from an Excel (.xlsx, .xls) file.
// Reads the first sheet and auto-detects column mapping from headers.
func ImportExcel(path string) ImportResult {
	result := ImportResult{}

	f, err := excelize.OpenFile(path)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Cannot open Excel file: %v", err))
		return result
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		result.Errors = append(result.Errors, "Excel file has no sheets")
		return result
	}

	rows, err := f.GetRows(sheets[0])
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Cannot read Excel data: %v", err))
		return result
	}

	if len(rows) == 0 {
		result.Errors = append(result.Errors, "Sheet is empty")
		return result
	}

	return importFromRows(rows, "Row", nil)
}

// importFromRows is the shared import logic for both CSV and Excel data.
// It detects headers, maps columns, and parses each row into parts.
func importFromRows(rows [][]string, rowPrefix string, initialWarnings []string) ImportResult {
	result := ImportResult{
		Warnings: initialWarnings,
	}

	if len(rows) == 0 {
		result.Errors = append(result.Errors, "No data rows found")
		return result
	}

	// Detect columns from first row
	mapping, hasHeader := DetectColumns(rows[0])
	startRow := 0
	if hasHeader {
		startRow = 1
		result.Warnings = append(result.Warnings, "Detected header row, skipping")

		// Validate that required columns were found
		missing := []string{}
		if mapping.Width == -1 {
			missing = append(missing, "Width")
		}
		if mapping.Height == -1 {
			missing = append(missing, "Height")
		}
		if mapping.Quantity == -1 {
			missing = append(missing, "Quantity")
		}
		if len(missing) > 0 {
			result.Errors = append(result.Errors, fmt.Sprintf("Required columns not found in header: %s", strings.Join(missing, ", ")))
			return result
		}
	} else {
		// No header: check if first row is numeric (positional mapping)
		if len(rows[0]) >= 3 {
			if _, err := strconv.ParseFloat(strings.TrimSpace(rows[0][1]), 64); err != nil {
				// First column after label is not numeric - might be an unrecognized header
				// Skip it as a header but use positional mapping
				startRow = 1
				result.Warnings = append(result.Warnings, "Detected header row, skipping")
			}
		}
	}

	for i := startRow; i < len(rows); i++ {
		row := rows[i]
		lineNum := i + 1

		if isEmptyRow(row) {
			continue
		}

		rowLabel := fmt.Sprintf("%s %d", rowPrefix, lineNum)
		part, errMsg, warning := parseRow(row, mapping, rowLabel, len(result.Parts))

		if errMsg != "" {
			result.Errors = append(result.Errors, errMsg)
			continue
		}
		if warning != "" {
			result.Warnings = append(result.Warnings, warning)
		}

		result.Parts = append(result.Parts, part)
	}

	return result
}
