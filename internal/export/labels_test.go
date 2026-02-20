package export

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/piwi3910/SlabCut/internal/model"
)

func buildLabelsTestResult() model.OptimizeResult {
	return model.OptimizeResult{
		Sheets: []model.SheetResult{
			{
				Stock: model.StockSheet{
					ID:    "s1",
					Label: "Plywood 2440x1220",
					Width: 2440, Height: 1220, Quantity: 1,
					Tabs: model.StockTabConfig{Enabled: false},
				},
				Placements: []model.Placement{
					{
						Part: model.Part{ID: "p1", Label: "Side Panel", Width: 600, Height: 400, Quantity: 1},
						X:    10, Y: 10, Rotated: false,
					},
					{
						Part: model.Part{ID: "p2", Label: "Top", Width: 500, Height: 300, Quantity: 1},
						X:    620, Y: 10, Rotated: true,
					},
				},
			},
			{
				Stock: model.StockSheet{
					ID:    "s2",
					Label: "MDF 1200x600",
					Width: 1200, Height: 600, Quantity: 1,
					Tabs: model.StockTabConfig{Enabled: false},
				},
				Placements: []model.Placement{
					{
						Part: model.Part{ID: "p3", Label: "Back Panel", Width: 800, Height: 500, Quantity: 1},
						X:    10, Y: 10, Rotated: false,
					},
				},
			},
		},
	}
}

func TestExportLabels_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "labels.pdf")

	result := buildLabelsTestResult()
	err := ExportLabels(path, result)
	if err != nil {
		t.Fatalf("ExportLabels returned error: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("PDF file was not created: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("PDF file is empty")
	}
	if info.Size() < 500 {
		t.Errorf("PDF file seems too small: %d bytes", info.Size())
	}
}

func TestExportLabels_EmptyResult(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.pdf")

	result := model.OptimizeResult{Sheets: nil}
	err := ExportLabels(path, result)
	if err == nil {
		t.Fatal("expected error for empty result, got nil")
	}
}

func TestExportLabels_NoPlacements(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "no_placements.pdf")

	result := model.OptimizeResult{
		Sheets: []model.SheetResult{
			{
				Stock: model.StockSheet{ID: "s1", Label: "Board", Width: 1000, Height: 500, Quantity: 1},
			},
		},
	}
	err := ExportLabels(path, result)
	if err == nil {
		t.Fatal("expected error for result with no placements, got nil")
	}
}

func TestCollectLabelInfos(t *testing.T) {
	result := buildLabelsTestResult()
	labels := CollectLabelInfos(result)

	if len(labels) != 3 {
		t.Fatalf("expected 3 labels, got %d", len(labels))
	}

	// Check first label
	if labels[0].PartLabel != "Side Panel" {
		t.Errorf("expected first label to be 'Side Panel', got %q", labels[0].PartLabel)
	}
	if labels[0].Width != 600 || labels[0].Height != 400 {
		t.Errorf("wrong dimensions: got %.0fx%.0f, want 600x400", labels[0].Width, labels[0].Height)
	}
	if labels[0].SheetIndex != 1 {
		t.Errorf("expected sheet index 1, got %d", labels[0].SheetIndex)
	}
	if labels[0].Rotated {
		t.Error("expected first label not rotated")
	}

	// Check second label (rotated)
	if !labels[1].Rotated {
		t.Error("expected second label to be rotated")
	}

	// Check third label (second sheet)
	if labels[2].SheetIndex != 2 {
		t.Errorf("expected sheet index 2 for third label, got %d", labels[2].SheetIndex)
	}
}

func TestLabelInfo_JSONRoundTrip(t *testing.T) {
	info := LabelInfo{
		PartLabel:  "Test Part",
		Width:      300,
		Height:     200,
		SheetIndex: 1,
		SheetLabel: "Plywood",
		Rotated:    true,
		X:          50,
		Y:          100,
	}

	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded LabelInfo
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.PartLabel != info.PartLabel {
		t.Errorf("label mismatch: got %q, want %q", decoded.PartLabel, info.PartLabel)
	}
	if decoded.Width != info.Width || decoded.Height != info.Height {
		t.Errorf("dimensions mismatch: got %.0fx%.0f, want %.0fx%.0f",
			decoded.Width, decoded.Height, info.Width, info.Height)
	}
	if decoded.Rotated != info.Rotated {
		t.Error("rotated flag mismatch")
	}
}

func TestExportLabels_ManyParts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "many_labels.pdf")

	// Create 35 placements to test multi-page label generation
	placements := make([]model.Placement, 35)
	for i := range placements {
		placements[i] = model.Placement{
			Part: model.Part{
				ID:       "p" + string(rune('A'+i%26)),
				Label:    "Part " + string(rune('A'+i%26)),
				Width:    100 + float64(i*10),
				Height:   50 + float64(i*5),
				Quantity: 1,
			},
			X: float64(i * 110), Y: 10,
		}
	}

	result := model.OptimizeResult{
		Sheets: []model.SheetResult{
			{
				Stock: model.StockSheet{
					ID: "s1", Label: "Large Board", Width: 5000, Height: 3000, Quantity: 1,
					Tabs: model.StockTabConfig{Enabled: false},
				},
				Placements: placements,
			},
		},
	}

	err := ExportLabels(path, result)
	if err != nil {
		t.Fatalf("ExportLabels returned error: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("PDF file was not created: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("PDF file is empty")
	}
}
