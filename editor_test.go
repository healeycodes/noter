package noter

import (
	"reflect"
	"testing"
)

func TestGetLineNumber(t *testing.T) {
	line1 := &editorLine{}
	line2 := &editorLine{}
	line1.next = line2
	line2.prev = line1
	editor := &Editor{
		start: line1,
		cursor: &editorCursor{
			line2,
			0,
		},
	}

	lineNum := editor.getLineNumber()
	want := 1
	if lineNum != want {
		t.Fatalf(`Expected current line number to be %v, got: %v`, want, lineNum)
	}
}

func TestGetAllRunes(t *testing.T) {
	line1 := &editorLine{values: []rune{'a', '\n'}}
	line2 := &editorLine{values: []rune{'b', '\n'}}
	line1.next = line2
	line2.prev = line1
	editor := &Editor{
		start: line1,
		cursor: &editorCursor{
			line2,
			0,
		},
	}

	allRunes := editor.getAllRunes()
	if reflect.DeepEqual(allRunes, []rune{'a', '\n', 'b', '\n'}) != true {
		t.Fatalf(`Expected allRunes to return document runes, got: %v`, allRunes)
	}
}

func TestDeleteRune(t *testing.T) {
	line1 := &editorLine{values: []rune{'a', '\n'}}
	editor := &Editor{
		start: line1,
		cursor: &editorCursor{
			line1,
			1,
		},
	}

	editor.fnDeleteSinglePrevious()
	if len(line1.values) != 0 && line1.values[0] != '\n' {
		t.Fatalf("Delete operation did not work correctly, got: %v", line1.values)
	}
}

func TestDeleteLine(t *testing.T) {
	line1 := &editorLine{values: []rune{'a', '\n'}}
	line2 := &editorLine{values: []rune{'b', '\n'}}
	line1.next = line2
	line2.prev = line1
	editor := &Editor{
		start: line1,
		cursor: &editorCursor{
			line2,
			1,
		},
	}

	editor.fnDeleteSinglePrevious()
	editor.fnDeleteSinglePrevious()
	if len(line1.values) != 0 && line1.next != nil {
		t.Fatalf("Delete operation did not work correctly, got: %v, %v", line1.values, line1.next)
	}
}

func TestHighlightLineAndGetHighlightedRunes(t *testing.T) {
	line1 := &editorLine{values: []rune{'a', '\n'}}
	line2 := &editorLine{values: []rune{'b', '\n'}}
	line1.next = line2
	line2.prev = line1
	editor := &Editor{
		start: line1,
		cursor: &editorCursor{
			line2,
			1,
		},
		// This would normally happen in editor.Load()
		highlighted: make(map[*editorLine]map[int]bool),
	}

	editor.highlightLine()
	if reflect.DeepEqual(editor.getHighlightedRunes(), []rune{'b', '\n'}) != true {
		t.Fatalf(`Expected GetHighlightedRunes to return line2's runes, got: %v`, editor.getHighlightedRunes())
	}
}

func TestSearch(t *testing.T) {
	line1 := &editorLine{values: []rune{'a', '\n'}}
	line2 := &editorLine{values: []rune{'b', '\n'}}
	line1.next = line2
	line2.prev = line1
	editor := &Editor{
		start: line1,
		cursor: &editorCursor{
			line2,
			1,
		},
	}

	editor.mode = SEARCH_MODE
	// This would normally happen in editor.Load()
	editor.searchHighlights = map[*editorLine]map[int]bool{}
	editor.searchTerm = []rune{'b'}
	editor.search()

	if _, ok := editor.searchHighlights[line2]; !ok {
		t.Fatalf("Incorrect search highlights: line2 wasn't highlighted")
	}
	if _, ok := editor.searchHighlights[line2][0]; !ok {
		t.Fatalf("Incorrect search highlights: line index wasn't highlighted")
	}

	if editor.searchHighlights[line2][0] != true {
		t.Fatalf("Incorrect search highlights: boolean was false instead of true")
	}
}

func TestLayout(t *testing.T) {
	editor := NewEditor(
		WithWidth(123),
		WithHeight(456),
	)

	table := [](struct{ screen_w, screen_h, layout_w, layout_h int }){
		{123, 456, 123, 456},
		{0, 0, 123, 456},
		{1024, 768, 123, 456},
	}

	for _, entry := range table {
		w, h := editor.Layout(entry.screen_w, entry.screen_h)
		if w != entry.layout_w || h != entry.layout_h {
			t.Fatalf("Incorrect result (%v,%v) from Editor.Layout(%v,%v); expected (%v,%v)",
				w, h, entry.screen_w, entry.screen_h, entry.layout_w, entry.layout_h)
		}
	}
}

func TestMoveCursorScroll(t *testing.T) {
	editor := NewEditor(
		WithRows(3),
		WithColumns(4),
	)

	editor.WriteText([]byte("1\n2\n3\n4\n5\n6\n7\n8\n"))

	table := [](struct{ row, first_visible int }){
		{0, 0}, // Move to first line.
		{1, 0}, // Move to middle line.
		{2, 0}, // Move to end of first page.
		{3, 1}, // Scrolls down by one.
		{4, 2}, // Scrolls down by one mode.
		{2, 2}, // Back to top.
		{7, 5}, // Move to end.
		{5, 5}, // Move to top view of end.
		{4, 4}, // Move up one line moves one line up.
		{1, 1}, // Move up one line moves one line up.
		{0, 0}, // Move up to first page.
	}

	for _, entry := range table {
		editor.MoveCursor(entry.row, 0)
		if entry.first_visible != editor.firstVisible {
			t.Fatalf("Incorrect move to row %v, expected first visible to %v, was %v", entry.row, entry.first_visible, editor.firstVisible)
		}
	}
}
