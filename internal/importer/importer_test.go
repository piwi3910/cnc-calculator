package importer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/piwi3910/cnc-calculator/internal/model"
	"github.com/xuri/excelize/v2"
)

// ─── DetectCSVDelimiter Tests ──────────────────────────────

func TestDetectCSVDelimiter_Comma(t *testing.T) {
	data := []byte("Label,Width,Height,Qty\nShelf,600,300,2\nDoor,400,800,1\n")
	got := DetectCSVDelimiter(data)
	if got != ',' {
		t.Errorf("expected comma delimiter, got %q", got)
	}
}

func TestDetectCSVDelimiter_Semicolon(t *testing.T) {
	data := []byte("Label;Width;Height;Qty\nShelf;600;300;2\nDoor;400;800;1\n")
	got := DetectCSVDelimiter(data)
	if got != ';' {
		t.Errorf("expected semicolon delimiter, got %q", got)
	}
}

func TestDetectCSVDelimiter_Tab(t *testing.T) {
	data := []byte("Label\tWidth\tHeight\tQty\nShelf\t600\t300\t2\nDoor\t400\t800\t1\n")
	got := DetectCSVDelimiter(data)
	if got != '\t' {
		t.Errorf("expected tab delimiter, got %q", got)
	}
}

func TestDetectCSVDelimiter_Pipe(t *testing.T) {
	data := []byte("Label|Width|Height|Qty\nShelf|600|300|2\nDoor|400|800|1\n")
	got := DetectCSVDelimiter(data)
	if got != '|' {
		t.Errorf("expected pipe delimiter, got %q", got)
	}
}

// ─── DetectColumns Tests ───────────────────────────────────

func TestDetectColumns_StandardHeaders(t *testing.T) {
	row := []string{"Label", "Width", "Height", "Quantity", "Grain"}
	mapping, isHeader := DetectColumns(row)

	if !isHeader {
		t.Error("expected header to be detected")
	}
	if mapping.Label != 0 {
		t.Errorf("expected Label at 0, got %d", mapping.Label)
	}
	if mapping.Width != 1 {
		t.Errorf("expected Width at 1, got %d", mapping.Width)
	}
	if mapping.Height != 2 {
		t.Errorf("expected Height at 2, got %d", mapping.Height)
	}
	if mapping.Quantity != 3 {
		t.Errorf("expected Quantity at 3, got %d", mapping.Quantity)
	}
	if mapping.Grain != 4 {
		t.Errorf("expected Grain at 4, got %d", mapping.Grain)
	}
}

func TestDetectColumns_CaseInsensitive(t *testing.T) {
	row := []string{"NAME", "WIDTH", "HEIGHT", "QTY", "GRAIN"}
	mapping, isHeader := DetectColumns(row)

	if !isHeader {
		t.Error("expected header to be detected")
	}
	if mapping.Label != 0 {
		t.Errorf("expected Label at 0, got %d", mapping.Label)
	}
	if mapping.Width != 1 {
		t.Errorf("expected Width at 1, got %d", mapping.Width)
	}
}

func TestDetectColumns_AlternativeNames(t *testing.T) {
	row := []string{"Part Name", "W", "H", "Pcs", "Direction"}
	mapping, isHeader := DetectColumns(row)

	if !isHeader {
		t.Error("expected header to be detected")
	}
	if mapping.Label != 0 {
		t.Errorf("expected Label at 0, got %d", mapping.Label)
	}
	if mapping.Width != 1 {
		t.Errorf("expected Width at 1, got %d", mapping.Width)
	}
	if mapping.Height != 2 {
		t.Errorf("expected Height at 2, got %d", mapping.Height)
	}
	if mapping.Quantity != 3 {
		t.Errorf("expected Quantity at 3, got %d", mapping.Quantity)
	}
	if mapping.Grain != 4 {
		t.Errorf("expected Grain at 4, got %d", mapping.Grain)
	}
}

func TestDetectColumns_ReorderedColumns(t *testing.T) {
	row := []string{"Qty", "Height", "Width", "Label"}
	mapping, isHeader := DetectColumns(row)

	if !isHeader {
		t.Error("expected header to be detected")
	}
	if mapping.Quantity != 0 {
		t.Errorf("expected Quantity at 0, got %d", mapping.Quantity)
	}
	if mapping.Height != 1 {
		t.Errorf("expected Height at 1, got %d", mapping.Height)
	}
	if mapping.Width != 2 {
		t.Errorf("expected Width at 2, got %d", mapping.Width)
	}
	if mapping.Label != 3 {
		t.Errorf("expected Label at 3, got %d", mapping.Label)
	}
}

func TestDetectColumns_NoHeader(t *testing.T) {
	row := []string{"Shelf", "600", "300", "2"}
	mapping, isHeader := DetectColumns(row)

	if isHeader {
		t.Error("expected no header detection for numeric data")
	}
	// Should fall back to positional
	if mapping.Label != 0 || mapping.Width != 1 || mapping.Height != 2 || mapping.Quantity != 3 {
		t.Errorf("expected positional mapping, got %+v", mapping)
	}
}

// ─── CSV Import Tests ──────────────────────────────────────

func TestImportCSVFromReader_WithHeaders(t *testing.T) {
	data := "Label,Width,Height,Quantity,Grain\nShelf,600,300,2,Horizontal\nDoor,400,800,1,Vertical\n"
	result := ImportCSVFromReader(strings.NewReader(data), ',')

	if len(result.Errors) > 0 {
		t.Errorf("unexpected errors: %v", result.Errors)
	}
	if len(result.Parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(result.Parts))
	}

	if result.Parts[0].Label != "Shelf" {
		t.Errorf("expected label 'Shelf', got '%s'", result.Parts[0].Label)
	}
	if result.Parts[0].Width != 600 {
		t.Errorf("expected width 600, got %f", result.Parts[0].Width)
	}
	if result.Parts[0].Height != 300 {
		t.Errorf("expected height 300, got %f", result.Parts[0].Height)
	}
	if result.Parts[0].Quantity != 2 {
		t.Errorf("expected quantity 2, got %d", result.Parts[0].Quantity)
	}
	if result.Parts[0].Grain != model.GrainHorizontal {
		t.Errorf("expected GrainHorizontal, got %v", result.Parts[0].Grain)
	}

	if result.Parts[1].Grain != model.GrainVertical {
		t.Errorf("expected GrainVertical, got %v", result.Parts[1].Grain)
	}
}

func TestImportCSVFromReader_WithoutHeaders(t *testing.T) {
	data := "Shelf,600,300,2\nDoor,400,800,1\n"
	result := ImportCSVFromReader(strings.NewReader(data), ',')

	if len(result.Parts) != 2 {
		t.Fatalf("expected 2 parts, got %d (errors: %v)", len(result.Parts), result.Errors)
	}
	if result.Parts[0].Label != "Shelf" {
		t.Errorf("expected label 'Shelf', got '%s'", result.Parts[0].Label)
	}
	if result.Parts[0].Width != 600 {
		t.Errorf("expected width 600, got %f", result.Parts[0].Width)
	}
}

func TestImportCSVFromReader_SemicolonDelimiter(t *testing.T) {
	data := "Label;Width;Height;Quantity\nShelf;600;300;2\n"
	result := ImportCSVFromReader(strings.NewReader(data), ';')

	if len(result.Errors) > 0 {
		t.Errorf("unexpected errors: %v", result.Errors)
	}
	if len(result.Parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(result.Parts))
	}
	if result.Parts[0].Label != "Shelf" {
		t.Errorf("expected label 'Shelf', got '%s'", result.Parts[0].Label)
	}
}

func TestImportCSVFromReader_TabDelimiter(t *testing.T) {
	data := "Label\tWidth\tHeight\tQuantity\nShelf\t600\t300\t2\n"
	result := ImportCSVFromReader(strings.NewReader(data), '\t')

	if len(result.Errors) > 0 {
		t.Errorf("unexpected errors: %v", result.Errors)
	}
	if len(result.Parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(result.Parts))
	}
}

func TestImportCSVFromReader_ReorderedColumns(t *testing.T) {
	data := "Qty,Height,Width,Name\n2,300,600,Shelf\n"
	result := ImportCSVFromReader(strings.NewReader(data), ',')

	if len(result.Errors) > 0 {
		t.Errorf("unexpected errors: %v", result.Errors)
	}
	if len(result.Parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(result.Parts))
	}
	if result.Parts[0].Label != "Shelf" {
		t.Errorf("expected label 'Shelf', got '%s'", result.Parts[0].Label)
	}
	if result.Parts[0].Width != 600 {
		t.Errorf("expected width 600, got %f", result.Parts[0].Width)
	}
	if result.Parts[0].Height != 300 {
		t.Errorf("expected height 300, got %f", result.Parts[0].Height)
	}
	if result.Parts[0].Quantity != 2 {
		t.Errorf("expected quantity 2, got %d", result.Parts[0].Quantity)
	}
}

func TestImportCSVFromReader_EmptyFile(t *testing.T) {
	data := ""
	result := ImportCSVFromReader(strings.NewReader(data), ',')

	if len(result.Errors) == 0 {
		t.Error("expected error for empty file")
	}
}

func TestImportCSVFromReader_InvalidWidth(t *testing.T) {
	data := "Label,Width,Height,Quantity\nShelf,abc,300,2\n"
	result := ImportCSVFromReader(strings.NewReader(data), ',')

	if len(result.Errors) == 0 {
		t.Error("expected error for invalid width")
	}
	if len(result.Parts) != 0 {
		t.Errorf("expected 0 parts, got %d", len(result.Parts))
	}
}

func TestImportCSVFromReader_InvalidQuantity(t *testing.T) {
	data := "Label,Width,Height,Quantity\nShelf,600,300,abc\n"
	result := ImportCSVFromReader(strings.NewReader(data), ',')

	if len(result.Errors) == 0 {
		t.Error("expected error for invalid quantity")
	}
}

func TestImportCSVFromReader_NegativeValues(t *testing.T) {
	data := "Label,Width,Height,Quantity\nShelf,-600,300,2\n"
	result := ImportCSVFromReader(strings.NewReader(data), ',')

	if len(result.Errors) == 0 {
		t.Error("expected error for negative width")
	}
}

func TestImportCSVFromReader_ZeroQuantity(t *testing.T) {
	data := "Label,Width,Height,Quantity\nShelf,600,300,0\n"
	result := ImportCSVFromReader(strings.NewReader(data), ',')

	if len(result.Errors) == 0 {
		t.Error("expected error for zero quantity")
	}
}

func TestImportCSVFromReader_MixedValidAndInvalid(t *testing.T) {
	data := "Label,Width,Height,Quantity\nGood,600,300,2\nBad,abc,300,2\nAlsoGood,400,200,1\n"
	result := ImportCSVFromReader(strings.NewReader(data), ',')

	if len(result.Parts) != 2 {
		t.Errorf("expected 2 valid parts, got %d", len(result.Parts))
	}
	if len(result.Errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(result.Errors))
	}
}

func TestImportCSVFromReader_EmptyRows(t *testing.T) {
	data := "Label,Width,Height,Quantity\nShelf,600,300,2\n\n\nDoor,400,800,1\n"
	result := ImportCSVFromReader(strings.NewReader(data), ',')

	if len(result.Parts) != 2 {
		t.Errorf("expected 2 parts (skipping empty rows), got %d (errors: %v)", len(result.Parts), result.Errors)
	}
}

func TestImportCSVFromReader_EmptyLabel(t *testing.T) {
	data := "Label,Width,Height,Quantity\n,600,300,2\n"
	result := ImportCSVFromReader(strings.NewReader(data), ',')

	if len(result.Parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(result.Parts))
	}
	if result.Parts[0].Label != "Part 1" {
		t.Errorf("expected auto-generated label 'Part 1', got '%s'", result.Parts[0].Label)
	}
}

func TestImportCSVFromReader_GrainParsing(t *testing.T) {
	tests := []struct {
		input    string
		expected model.Grain
		warning  bool
	}{
		{"Horizontal", model.GrainHorizontal, false},
		{"horizontal", model.GrainHorizontal, false},
		{"H", model.GrainHorizontal, false},
		{"h", model.GrainHorizontal, false},
		{"Vertical", model.GrainVertical, false},
		{"vertical", model.GrainVertical, false},
		{"V", model.GrainVertical, false},
		{"v", model.GrainVertical, false},
		{"None", model.GrainNone, false},
		{"none", model.GrainNone, false},
		{"N", model.GrainNone, false},
		{"n", model.GrainNone, false},
		{"-", model.GrainNone, false},
		{"", model.GrainNone, false},
		{"diagonal", model.GrainNone, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			data := "Label,Width,Height,Quantity,Grain\nPart,600,300,1," + tt.input + "\n"
			result := ImportCSVFromReader(strings.NewReader(data), ',')

			if len(result.Parts) != 1 {
				t.Fatalf("expected 1 part, got %d (errors: %v)", len(result.Parts), result.Errors)
			}
			if result.Parts[0].Grain != tt.expected {
				t.Errorf("grain %q: expected %v, got %v", tt.input, tt.expected, result.Parts[0].Grain)
			}
			hasWarning := false
			for _, w := range result.Warnings {
				if strings.Contains(w, "Unknown grain direction") {
					hasWarning = true
				}
			}
			if tt.warning && !hasWarning {
				t.Errorf("grain %q: expected warning but got none", tt.input)
			}
			if !tt.warning && hasWarning {
				t.Errorf("grain %q: unexpected warning", tt.input)
			}
		})
	}
}

func TestImportCSVFromReader_MissingRequiredColumnInHeader(t *testing.T) {
	data := "Label,Width,Grain\nShelf,600,H\n"
	result := ImportCSVFromReader(strings.NewReader(data), ',')

	if len(result.Errors) == 0 {
		t.Error("expected error for missing Height and Quantity columns")
	}
	foundMissing := false
	for _, e := range result.Errors {
		if strings.Contains(e, "Required columns not found") {
			foundMissing = true
		}
	}
	if !foundMissing {
		t.Errorf("expected 'Required columns not found' error, got: %v", result.Errors)
	}
}

// ─── CSV File Import Tests ──────────────────────────────────

func TestImportCSV_File(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "parts.csv")
	content := "Label,Width,Height,Quantity\nShelf,600,300,2\nDoor,400,800,1\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	result := ImportCSV(path)

	if len(result.Errors) > 0 {
		t.Errorf("unexpected errors: %v", result.Errors)
	}
	if len(result.Parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(result.Parts))
	}
}

func TestImportCSV_SemicolonFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "parts.csv")
	content := "Label;Width;Height;Quantity\nShelf;600;300;2\nDoor;400;800;1\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	result := ImportCSV(path)

	if len(result.Parts) != 2 {
		t.Errorf("expected 2 parts, got %d (errors: %v)", len(result.Parts), result.Errors)
	}

	// Should have a warning about semicolon delimiter
	hasSemicolonWarning := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "semicolon") {
			hasSemicolonWarning = true
		}
	}
	if !hasSemicolonWarning {
		t.Error("expected warning about semicolon delimiter detection")
	}
}

func TestImportCSV_FileNotFound(t *testing.T) {
	result := ImportCSV("/nonexistent/path/file.csv")

	if len(result.Errors) == 0 {
		t.Error("expected error for nonexistent file")
	}
}

func TestImportCSV_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.csv")
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	result := ImportCSV(path)

	if len(result.Errors) == 0 {
		t.Error("expected error for empty file")
	}
}

// ─── Excel Import Tests ────────────────────────────────────

func createTestExcel(t *testing.T, rows [][]interface{}) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "parts.xlsx")

	f := excelize.NewFile()
	sheet := f.GetSheetName(0)

	for i, row := range rows {
		for j, cell := range row {
			cellRef, err := excelize.CoordinatesToCellName(j+1, i+1)
			if err != nil {
				t.Fatalf("failed to create cell reference: %v", err)
			}
			if err := f.SetCellValue(sheet, cellRef, cell); err != nil {
				t.Fatalf("failed to set cell value: %v", err)
			}
		}
	}

	if err := f.SaveAs(path); err != nil {
		t.Fatalf("failed to save Excel file: %v", err)
	}
	return path
}

func TestImportExcel_WithHeaders(t *testing.T) {
	path := createTestExcel(t, [][]interface{}{
		{"Label", "Width", "Height", "Quantity", "Grain"},
		{"Shelf", 600, 300, 2, "Horizontal"},
		{"Door", 400, 800, 1, "Vertical"},
	})

	result := ImportExcel(path)

	if len(result.Errors) > 0 {
		t.Errorf("unexpected errors: %v", result.Errors)
	}
	if len(result.Parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(result.Parts))
	}

	if result.Parts[0].Label != "Shelf" {
		t.Errorf("expected 'Shelf', got '%s'", result.Parts[0].Label)
	}
	if result.Parts[0].Width != 600 {
		t.Errorf("expected width 600, got %f", result.Parts[0].Width)
	}
	if result.Parts[0].Grain != model.GrainHorizontal {
		t.Errorf("expected GrainHorizontal, got %v", result.Parts[0].Grain)
	}
}

func TestImportExcel_WithoutHeaders(t *testing.T) {
	path := createTestExcel(t, [][]interface{}{
		{"Shelf", 600, 300, 2},
		{"Door", 400, 800, 1},
	})

	result := ImportExcel(path)

	if len(result.Parts) != 2 {
		t.Fatalf("expected 2 parts, got %d (errors: %v)", len(result.Parts), result.Errors)
	}
}

func TestImportExcel_ReorderedColumns(t *testing.T) {
	path := createTestExcel(t, [][]interface{}{
		{"Qty", "Name", "Height", "Width"},
		{2, "Shelf", 300, 600},
	})

	result := ImportExcel(path)

	if len(result.Errors) > 0 {
		t.Errorf("unexpected errors: %v", result.Errors)
	}
	if len(result.Parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(result.Parts))
	}
	if result.Parts[0].Label != "Shelf" {
		t.Errorf("expected 'Shelf', got '%s'", result.Parts[0].Label)
	}
	if result.Parts[0].Width != 600 {
		t.Errorf("expected width 600, got %f", result.Parts[0].Width)
	}
}

func TestImportExcel_FileNotFound(t *testing.T) {
	result := ImportExcel("/nonexistent/file.xlsx")

	if len(result.Errors) == 0 {
		t.Error("expected error for nonexistent file")
	}
}

func TestImportExcel_InvalidData(t *testing.T) {
	path := createTestExcel(t, [][]interface{}{
		{"Label", "Width", "Height", "Quantity"},
		{"Shelf", "abc", 300, 2},
	})

	result := ImportExcel(path)

	if len(result.Errors) == 0 {
		t.Error("expected error for invalid width")
	}
}

// ─── parseGrain Tests ──────────────────────────────────────

func TestParseGrain(t *testing.T) {
	tests := []struct {
		input    string
		expected model.Grain
		ok       bool
	}{
		{"Horizontal", model.GrainHorizontal, true},
		{"horizontal", model.GrainHorizontal, true},
		{"H", model.GrainHorizontal, true},
		{"h", model.GrainHorizontal, true},
		{"Vertical", model.GrainVertical, true},
		{"vertical", model.GrainVertical, true},
		{"V", model.GrainVertical, true},
		{"v", model.GrainVertical, true},
		{"None", model.GrainNone, true},
		{"none", model.GrainNone, true},
		{"N", model.GrainNone, true},
		{"n", model.GrainNone, true},
		{"-", model.GrainNone, true},
		{"", model.GrainNone, true},
		{"  h  ", model.GrainHorizontal, true},
		{"unknown", model.GrainNone, false},
		{"diagonal", model.GrainNone, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			grain, ok := parseGrain(tt.input)
			if grain != tt.expected {
				t.Errorf("parseGrain(%q): expected %v, got %v", tt.input, tt.expected, grain)
			}
			if ok != tt.ok {
				t.Errorf("parseGrain(%q): expected ok=%v, got %v", tt.input, tt.ok, ok)
			}
		})
	}
}

// ─── Edge Cases ────────────────────────────────────────────

func TestImportCSVFromReader_OnlyHeaders(t *testing.T) {
	data := "Label,Width,Height,Quantity\n"
	result := ImportCSVFromReader(strings.NewReader(data), ',')

	if len(result.Parts) != 0 {
		t.Errorf("expected 0 parts for header-only file, got %d", len(result.Parts))
	}
	// Should not have errors (just no data)
}

func TestImportCSVFromReader_WhitespaceInValues(t *testing.T) {
	data := "Label , Width , Height , Quantity\n Shelf , 600 , 300 , 2 \n"
	result := ImportCSVFromReader(strings.NewReader(data), ',')

	if len(result.Parts) != 1 {
		t.Fatalf("expected 1 part, got %d (errors: %v)", len(result.Parts), result.Errors)
	}
	if result.Parts[0].Width != 600 {
		t.Errorf("expected width 600, got %f", result.Parts[0].Width)
	}
}

func TestImportCSVFromReader_DecimalValues(t *testing.T) {
	data := "Label,Width,Height,Quantity\nShelf,600.5,300.25,2\n"
	result := ImportCSVFromReader(strings.NewReader(data), ',')

	if len(result.Parts) != 1 {
		t.Fatalf("expected 1 part, got %d (errors: %v)", len(result.Parts), result.Errors)
	}
	if result.Parts[0].Width != 600.5 {
		t.Errorf("expected width 600.5, got %f", result.Parts[0].Width)
	}
	if result.Parts[0].Height != 300.25 {
		t.Errorf("expected height 300.25, got %f", result.Parts[0].Height)
	}
}
