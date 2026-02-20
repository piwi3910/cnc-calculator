package ui

import "github.com/piwi3910/cnc-calculator/internal/model"

const defaultMaxDepth = 50

// Snapshot captures the parts and stocks state at a point in time.
type Snapshot struct {
	Parts  []model.Part
	Stocks []model.StockSheet
	Label  string // Human-readable description (e.g. "Add Part")
}

// History manages undo/redo stacks of project snapshots.
type History struct {
	undoStack []Snapshot
	redoStack []Snapshot
	maxDepth  int
}

// NewHistory creates a History with the default max depth of 50.
func NewHistory() *History {
	return &History{
		maxDepth: defaultMaxDepth,
	}
}

// Push saves a snapshot onto the undo stack and clears the redo stack.
// This should be called before the modification is applied.
func (h *History) Push(s Snapshot) {
	h.undoStack = append(h.undoStack, s)
	if len(h.undoStack) > h.maxDepth {
		h.undoStack = h.undoStack[len(h.undoStack)-h.maxDepth:]
	}
	h.redoStack = nil
}

// Undo pops the most recent snapshot from the undo stack and pushes
// the current state onto the redo stack. Returns the snapshot to restore
// and true, or an empty snapshot and false if nothing to undo.
func (h *History) Undo(current Snapshot) (Snapshot, bool) {
	if len(h.undoStack) == 0 {
		return Snapshot{}, false
	}
	// Pop from undo
	last := h.undoStack[len(h.undoStack)-1]
	h.undoStack = h.undoStack[:len(h.undoStack)-1]
	// Push current state onto redo
	h.redoStack = append(h.redoStack, current)
	return last, true
}

// Redo pops the most recent snapshot from the redo stack and pushes
// the current state onto the undo stack. Returns the snapshot to restore
// and true, or an empty snapshot and false if nothing to redo.
func (h *History) Redo(current Snapshot) (Snapshot, bool) {
	if len(h.redoStack) == 0 {
		return Snapshot{}, false
	}
	// Pop from redo
	last := h.redoStack[len(h.redoStack)-1]
	h.redoStack = h.redoStack[:len(h.redoStack)-1]
	// Push current state onto undo
	h.undoStack = append(h.undoStack, current)
	return last, true
}

// CanUndo returns true if there is at least one snapshot to undo.
func (h *History) CanUndo() bool {
	return len(h.undoStack) > 0
}

// CanRedo returns true if there is at least one snapshot to redo.
func (h *History) CanRedo() bool {
	return len(h.redoStack) > 0
}

// Clear removes all undo and redo history.
func (h *History) Clear() {
	h.undoStack = nil
	h.redoStack = nil
}

// copyParts returns a deep copy of a parts slice.
func copyParts(parts []model.Part) []model.Part {
	if parts == nil {
		return nil
	}
	cp := make([]model.Part, len(parts))
	copy(cp, parts)
	return cp
}

// copyStocks returns a deep copy of a stocks slice.
func copyStocks(stocks []model.StockSheet) []model.StockSheet {
	if stocks == nil {
		return nil
	}
	cp := make([]model.StockSheet, len(stocks))
	for i, s := range stocks {
		cp[i] = s
		// Deep copy the Tabs.CustomZones slice
		if s.Tabs.CustomZones != nil {
			cp[i].Tabs.CustomZones = make([]model.TabZone, len(s.Tabs.CustomZones))
			copy(cp[i].Tabs.CustomZones, s.Tabs.CustomZones)
		}
	}
	return cp
}

// MakeSnapshot creates a snapshot from the current project state with a label.
func MakeSnapshot(parts []model.Part, stocks []model.StockSheet, label string) Snapshot {
	return Snapshot{
		Parts:  copyParts(parts),
		Stocks: copyStocks(stocks),
		Label:  label,
	}
}
