package noter

import (
	"reflect"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestKeyToRune(t *testing.T) {
	r, printable := KeyToRune(ebiten.KeyA, true)
	if r != 'A' && printable {
		t.Fatalf(`Expected 'A' and printable: true, got: %v %v`, r, printable)
	}

	r, printable = KeyToRune(ebiten.KeyB, false)
	if r != 'b' && printable {
		t.Fatalf(`Expected 'b' and printable: true, got: %v %v`, r, printable)
	}

	r, printable = KeyToRune(ebiten.KeyEnter, false)
	if r != 0 && !printable {
		t.Fatalf(`Expected '0' and printable: false, got: %v %v`, r, printable)
	}
}

func TestGetLineNumber(t *testing.T) {
	line1 := &Line{}
	line2 := &Line{}
	line1.next = line2
	line2.prev = line1
	editor := &Editor{
		start: line1,
		cursor: &Cursor{
			line2,
			0,
		},
	}

	lineNum := editor.GetLineNumber()
	want := 1
	if lineNum != want {
		t.Fatalf(`Expected current line number to be %v, got: %v`, want, lineNum)
	}
}

func TestGetAllRunes(t *testing.T) {
	line1 := &Line{values: []rune{'a', '\n'}}
	line2 := &Line{values: []rune{'b', '\n'}}
	line1.next = line2
	line2.prev = line1
	editor := &Editor{
		start: line1,
		cursor: &Cursor{
			line2,
			0,
		},
	}

	allRunes := editor.GetAllRunes()
	if reflect.DeepEqual(allRunes, []rune{'a', '\n', 'b', '\n'}) != true {
		t.Fatalf(`Expected allRunes to return document runes, got: %v`, allRunes)
	}
}

func TestDeleteRune(t *testing.T) {
	line1 := &Line{values: []rune{'a', '\n'}}
	editor := &Editor{
		start: line1,
		cursor: &Cursor{
			line1,
			1,
		},
	}

	editor.DeleteSinglePrevious()
	if len(line1.values) != 0 && line1.values[0] != '\n' {
		t.Fatalf("Delete operation did not work correctly, got: %v", line1.values)
	}
}

func TestDeleteLine(t *testing.T) {
	line1 := &Line{values: []rune{'a', '\n'}}
	line2 := &Line{values: []rune{'b', '\n'}}
	line1.next = line2
	line2.prev = line1
	editor := &Editor{
		start: line1,
		cursor: &Cursor{
			line2,
			1,
		},
	}

	editor.DeleteSinglePrevious()
	editor.DeleteSinglePrevious()
	if len(line1.values) != 0 && line1.next != nil {
		t.Fatalf("Delete operation did not work correctly, got: %v, %v", line1.values, line1.next)
	}
}

func TestHighlightLineAndGetHighlightedRunes(t *testing.T) {
	line1 := &Line{values: []rune{'a', '\n'}}
	line2 := &Line{values: []rune{'b', '\n'}}
	line1.next = line2
	line2.prev = line1
	editor := &Editor{
		start: line1,
		cursor: &Cursor{
			line2,
			1,
		},
		// This would normally happen in editor.Load()
		highlighted: make(map[*Line]map[int]bool),
	}

	editor.HighlightLine()
	if reflect.DeepEqual(editor.GetHighlightedRunes(), []rune{'b', '\n'}) != true {
		t.Fatalf(`Expected GetHighlightedRunes to return line2's runes, got: %v`, editor.GetHighlightedRunes())
	}
}

func TestSearch(t *testing.T) {
	line1 := &Line{values: []rune{'a', '\n'}}
	line2 := &Line{values: []rune{'b', '\n'}}
	line1.next = line2
	line2.prev = line1
	editor := &Editor{
		start: line1,
		cursor: &Cursor{
			line2,
			1,
		},
	}

	editor.mode = SEARCH_MODE
	// This would normally happen in editor.Load()
	editor.searchHighlights = map[*Line]map[int]bool{}
	editor.searchTerm = []rune{'b'}
	editor.Search()

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
