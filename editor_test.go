package main

import (
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
