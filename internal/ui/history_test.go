package ui

import (
	"testing"

	"github.com/piwi3910/cnc-calculator/internal/model"
)

func TestNewHistory(t *testing.T) {
	h := NewHistory()
	if h.maxDepth != defaultMaxDepth {
		t.Errorf("expected maxDepth %d, got %d", defaultMaxDepth, h.maxDepth)
	}
	if h.CanUndo() {
		t.Error("new history should not be undoable")
	}
	if h.CanRedo() {
		t.Error("new history should not be redoable")
	}
}

func TestPushAndUndo(t *testing.T) {
	h := NewHistory()

	// Push initial state (before adding a part)
	snap0 := MakeSnapshot(nil, nil, "initial")
	h.Push(snap0)

	if !h.CanUndo() {
		t.Fatal("should be able to undo after push")
	}

	// Current state has one part
	currentParts := []model.Part{{ID: "p1", Label: "Part 1", Width: 100, Height: 50, Quantity: 1}}
	current := MakeSnapshot(currentParts, nil, "current")

	restored, ok := h.Undo(current)
	if !ok {
		t.Fatal("undo should succeed")
	}
	if len(restored.Parts) != 0 {
		t.Errorf("expected 0 parts after undo, got %d", len(restored.Parts))
	}
	if restored.Label != "initial" {
		t.Errorf("expected label 'initial', got %q", restored.Label)
	}
}

func TestUndoRedo(t *testing.T) {
	h := NewHistory()

	// State 0: empty
	snap0 := MakeSnapshot(nil, nil, "empty")
	h.Push(snap0)

	// State 1: one part
	parts1 := []model.Part{{ID: "p1", Label: "Part 1", Width: 100, Height: 50, Quantity: 1}}
	snap1 := MakeSnapshot(parts1, nil, "one part")
	h.Push(snap1)

	// Current state: two parts
	parts2 := []model.Part{
		{ID: "p1", Label: "Part 1", Width: 100, Height: 50, Quantity: 1},
		{ID: "p2", Label: "Part 2", Width: 200, Height: 100, Quantity: 2},
	}
	current := MakeSnapshot(parts2, nil, "two parts")

	// Undo to one part
	restored, ok := h.Undo(current)
	if !ok {
		t.Fatal("first undo should succeed")
	}
	if len(restored.Parts) != 1 {
		t.Errorf("expected 1 part, got %d", len(restored.Parts))
	}

	// Redo back to two parts
	if !h.CanRedo() {
		t.Fatal("should be able to redo")
	}
	redone, ok := h.Redo(restored)
	if !ok {
		t.Fatal("redo should succeed")
	}
	if len(redone.Parts) != 2 {
		t.Errorf("expected 2 parts after redo, got %d", len(redone.Parts))
	}
}

func TestPushClearsRedo(t *testing.T) {
	h := NewHistory()

	snap0 := MakeSnapshot(nil, nil, "empty")
	h.Push(snap0)

	parts1 := []model.Part{{ID: "p1", Label: "Part 1", Width: 100, Height: 50, Quantity: 1}}
	current := MakeSnapshot(parts1, nil, "one part")

	// Undo
	_, ok := h.Undo(current)
	if !ok {
		t.Fatal("undo should succeed")
	}
	if !h.CanRedo() {
		t.Fatal("should be able to redo after undo")
	}

	// Push new state - should clear redo
	snap2 := MakeSnapshot(nil, nil, "new action")
	h.Push(snap2)
	if h.CanRedo() {
		t.Error("redo stack should be cleared after push")
	}
}

func TestMaxDepth(t *testing.T) {
	h := &History{maxDepth: 3}

	for i := 0; i < 5; i++ {
		h.Push(MakeSnapshot(nil, nil, ""))
	}

	if len(h.undoStack) != 3 {
		t.Errorf("expected undo stack length 3, got %d", len(h.undoStack))
	}
}

func TestUndoEmpty(t *testing.T) {
	h := NewHistory()
	current := MakeSnapshot(nil, nil, "current")
	_, ok := h.Undo(current)
	if ok {
		t.Error("undo on empty history should return false")
	}
}

func TestRedoEmpty(t *testing.T) {
	h := NewHistory()
	current := MakeSnapshot(nil, nil, "current")
	_, ok := h.Redo(current)
	if ok {
		t.Error("redo on empty history should return false")
	}
}

func TestClear(t *testing.T) {
	h := NewHistory()
	h.Push(MakeSnapshot(nil, nil, "a"))
	h.Push(MakeSnapshot(nil, nil, "b"))

	// Create a redo entry
	current := MakeSnapshot(nil, nil, "current")
	h.Undo(current)

	h.Clear()
	if h.CanUndo() || h.CanRedo() {
		t.Error("after clear, should not be able to undo or redo")
	}
}

func TestDeepCopyParts(t *testing.T) {
	original := []model.Part{{ID: "p1", Label: "Part 1", Width: 100, Height: 50, Quantity: 1}}
	snap := MakeSnapshot(original, nil, "test")

	// Mutate original
	original[0].Label = "Modified"

	if snap.Parts[0].Label != "Part 1" {
		t.Error("snapshot should be independent of original slice")
	}
}

func TestDeepCopyStocks(t *testing.T) {
	original := []model.StockSheet{
		{
			ID: "s1", Label: "Sheet 1", Width: 2440, Height: 1220, Quantity: 1,
			Tabs: model.StockTabConfig{
				Enabled:     true,
				CustomZones: []model.TabZone{{X: 10, Y: 10, Width: 50, Height: 50}},
			},
		},
	}
	snap := MakeSnapshot(nil, original, "test")

	// Mutate original
	original[0].Label = "Modified"
	original[0].Tabs.CustomZones[0].X = 999

	if snap.Stocks[0].Label != "Sheet 1" {
		t.Error("snapshot stocks should be independent of original")
	}
	if snap.Stocks[0].Tabs.CustomZones[0].X != 10 {
		t.Error("snapshot custom zones should be independent of original")
	}
}

func TestCopyNilSlices(t *testing.T) {
	snap := MakeSnapshot(nil, nil, "nil test")
	if snap.Parts != nil {
		t.Error("nil parts should stay nil")
	}
	if snap.Stocks != nil {
		t.Error("nil stocks should stay nil")
	}
}

func TestMultipleUndoRedo(t *testing.T) {
	h := NewHistory()

	// Build up 3 states: empty -> 1 part -> 2 parts -> 3 parts
	h.Push(MakeSnapshot(nil, nil, "empty"))
	h.Push(MakeSnapshot(
		[]model.Part{{ID: "p1", Label: "P1", Width: 10, Height: 10, Quantity: 1}},
		nil, "1 part",
	))
	h.Push(MakeSnapshot(
		[]model.Part{
			{ID: "p1", Label: "P1", Width: 10, Height: 10, Quantity: 1},
			{ID: "p2", Label: "P2", Width: 20, Height: 20, Quantity: 1},
		},
		nil, "2 parts",
	))

	current := MakeSnapshot(
		[]model.Part{
			{ID: "p1", Label: "P1", Width: 10, Height: 10, Quantity: 1},
			{ID: "p2", Label: "P2", Width: 20, Height: 20, Quantity: 1},
			{ID: "p3", Label: "P3", Width: 30, Height: 30, Quantity: 1},
		},
		nil, "3 parts",
	)

	// Undo 3 times to get back to empty
	s, ok := h.Undo(current)
	if !ok || len(s.Parts) != 2 {
		t.Fatalf("first undo: expected 2 parts, got %d", len(s.Parts))
	}

	s, ok = h.Undo(s)
	if !ok || len(s.Parts) != 1 {
		t.Fatalf("second undo: expected 1 part, got %d", len(s.Parts))
	}

	s, ok = h.Undo(s)
	if !ok || len(s.Parts) != 0 {
		t.Fatalf("third undo: expected 0 parts, got %d", len(s.Parts))
	}

	// No more undos
	if h.CanUndo() {
		t.Error("should not be able to undo further")
	}

	// Redo all the way forward
	s, ok = h.Redo(s)
	if !ok || len(s.Parts) != 1 {
		t.Fatalf("first redo: expected 1 part, got %d", len(s.Parts))
	}

	s, ok = h.Redo(s)
	if !ok || len(s.Parts) != 2 {
		t.Fatalf("second redo: expected 2 parts, got %d", len(s.Parts))
	}

	s, ok = h.Redo(s)
	if !ok || len(s.Parts) != 3 {
		t.Fatalf("third redo: expected 3 parts, got %d", len(s.Parts))
	}

	if h.CanRedo() {
		t.Error("should not be able to redo further")
	}
}
