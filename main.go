package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"image/color"
	_ "image/png"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

var (
	filePath string
	fileName string
	//go:embed fonts/dist/fonts.store
	fontStoreRaw []byte
	//go:embed fonts/dist/fonts.json
	fontMapRaw []byte // unicode hex: [offset, size]
	fontImages map[rune]*ebiten.Image
	xUnit      int
	yUnit      int
)

type Line struct {
	prev   *Line
	next   *Line
	values []rune
}

type Cursor struct {
	line *Line
	x    int
}

type Editor struct {
	start    *Line
	cursor   *Cursor
	scroll   int
	modified bool
}

func init() {
	fontImages = make(map[rune]*ebiten.Image)
	var fontMap map[string][]int
	json.Unmarshal(fontMapRaw, &fontMap)
	for hex, info := range fontMap {
		offset := info[0]
		size := info[1]
		pngBytes := fontStoreRaw[offset : offset+size]
		imgRef, _, err := ebitenutil.NewImageFromReader(bytes.NewReader(pngBytes))
		if err != nil {
			log.Fatalln(err)
		}
		code, err := strconv.ParseUint(hex[2:], 16, 32)
		if err != nil {
			log.Fatalln(err)
		}
		fontImages[rune(code)] = imgRef
	}

	zeroBounds := fontImages[rune('0')].Bounds()
	xUnit = zeroBounds.Dx()
	yUnit = zeroBounds.Dy()
}

func (e *Editor) Load() error {
	f, err := os.Open(filePath)
	if err != nil {
		log.Fatalln(err)
	}
	defer f.Close()

	fileName = filepath.Base(filePath)
	b, err := os.ReadFile(filePath)
	if err != nil {
		log.Fatalln(err)
	}

	source := string(b)

	e.modified = false
	e.scroll = 0
	e.start = &Line{values: make([]rune, 0)}
	e.cursor = &Cursor{line: e.start, x: 0}
	currentLine := e.start

	if len(source) == 0 {
		currentLine.values = append(currentLine.values, '\n')
	} else {
		for _, char := range source {
			currentLine.values = append(currentLine.values, char)
			if char == '\n' {
				nextLine := &Line{values: make([]rune, 0)}
				currentLine.next = nextLine
				nextLine.prev = currentLine
				currentLine = nextLine
			}
		}
	}

	// Ensure the final line ends with `\n`
	if len(currentLine.values) > 0 && currentLine.values[len(currentLine.values)-1] != '\n' {
		currentLine.values = append(currentLine.values, '\n')
	}

	// Remove dangling line
	if currentLine.prev != nil {
		currentLine.prev.next = nil
	}

	return nil
}

func (e *Editor) HandleRune(r rune) {
	if r == '\n' {
		before := e.cursor.line
		after := e.cursor.line.next

		shiftedValues := make([]rune, 0)
		leftBehindValues := make([]rune, 0)
		shiftedValues = append(shiftedValues, e.cursor.line.values[e.cursor.x:]...)
		leftBehindValues = append(leftBehindValues, e.cursor.line.values[:e.cursor.x]...)
		leftBehindValues = append(leftBehindValues, '\n')
		e.cursor.line.values = leftBehindValues

		e.cursor.line = &Line{
			values: shiftedValues,
			prev:   before,
			next:   after,
		}
		e.cursor.x = 0

		if before != nil {
			before.next = e.cursor.line
		}
		if after != nil {
			after.prev = e.cursor.line
		}
	} else {
		modifiedLine := make([]rune, 0)
		modifiedLine = append(modifiedLine, e.cursor.line.values[:e.cursor.x]...)
		modifiedLine = append(modifiedLine, r)
		modifiedLine = append(modifiedLine, e.cursor.line.values[e.cursor.x:]...)
		e.cursor.line.values = modifiedLine
		e.cursor.x++
	}

	e.modified = true
}

func (e *Editor) Update() error {
	// Log key number
	// for i := 0; i < int(ebiten.KeyMax); i++ {
	// 	if inpututil.IsKeyJustPressed(ebiten.Key(i)) {
	// 		println(i)
	// 		return nil
	// 	}
	// }

	// Quit
	if inpututil.IsKeyJustPressed(ebiten.KeyQ) && ebiten.IsKeyPressed(ebiten.KeyMetaLeft) {
		os.Exit(0)
		return nil
	}

	// Save
	if inpututil.IsKeyJustPressed(ebiten.KeyS) && ebiten.IsKeyPressed(ebiten.KeyMetaLeft) {
		allRunes := e.GetAllRunes()
		saveFile, err := os.Create(filePath)
		if err != nil {
			log.Fatalln(err)
		}
		_, err = saveFile.Write([]byte(string(allRunes)))
		if err != nil {
			log.Fatalln(err)
		}
		e.modified = false
		return nil
	}

	// Paste
	if inpututil.IsKeyJustPressed(ebiten.KeyV) && ebiten.IsKeyPressed(ebiten.KeyMetaLeft) {
		pasteBytes, err := macOSpaste()
		if err != nil {
			log.Fatalln(err)
		}
		for _, r := range string(pasteBytes) {
			e.HandleRune(r)
		}
		return nil
	}

	// Cut line
	if inpututil.IsKeyJustPressed(ebiten.KeyX) && ebiten.IsKeyPressed(ebiten.KeyMetaLeft) {
		copyBytes := []byte(string(e.cursor.line.values))
		err := macOScopy(copyBytes)
		if err != nil {
			log.Fatalln(err)
		}

		// Remove the line
		if e.cursor.line.prev == nil && e.cursor.line.next == nil {
			e.cursor.line.values = []rune{'\n'}
		} else {
			if e.cursor.line.next == nil {
				e.cursor.line.prev.next = nil
				e.cursor.line = e.cursor.line.prev
			} else if e.cursor.line.prev == nil {
				// Make sure that e.start isn't pointing to a 'dead line'
				e.start = e.cursor.line.next
				e.cursor.line.next.prev = nil
				e.cursor.line = e.cursor.line.next
			} else {
				e.cursor.line.prev.next = e.cursor.line.next
				e.cursor.line.next.prev = e.cursor.line.prev
				e.cursor.line = e.cursor.line.next
			}

			e.cursor.x = int(math.Min(float64(e.cursor.x), float64(len(e.cursor.line.values)-1)))
		}
		return nil
	}

	// Copy all
	if inpututil.IsKeyJustPressed(ebiten.KeyC) && ebiten.IsKeyPressed(ebiten.KeyMetaLeft) {
		allRunes := e.GetAllRunes()
		copyBytes := []byte(string(allRunes))
		err := macOScopy(copyBytes)
		if err != nil {
			log.Fatalln(err)
		}
		return nil
	}

	// Enter
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		e.HandleRune('\n')
		return nil
	}

	// Tab (just insert four spaces)
	if inpututil.IsKeyJustPressed(ebiten.KeyTab) {
		for i := 0; i < 4; i++ {
			e.HandleRune(' ')
		}
		return nil
	}

	// Backspace
	if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) {
		if e.cursor.x == 0 {
			if e.cursor.line.prev != nil {
				e.cursor.x = len(e.cursor.line.prev.values) - 1
				e.cursor.line.prev.values = e.cursor.line.prev.values[:len(e.cursor.line.prev.values)-1]
				e.cursor.line.prev.values = append(e.cursor.line.prev.values, e.cursor.line.values...)
				e.cursor.line.prev.next = e.cursor.line.next
				if e.cursor.line.next != nil {
					e.cursor.line.next.prev = e.cursor.line.prev
				}
				e.cursor.line = e.cursor.line.prev
			}
		} else {
			e.cursor.x--
			e.cursor.line.values = append(e.cursor.line.values[:e.cursor.x], e.cursor.line.values[e.cursor.x+1:]...)
		}
		e.modified = true
		return nil
	}

	// Scroll
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) && ebiten.IsKeyPressed(ebiten.KeyShift) {
		if e.scroll > 0 {
			e.scroll--
			if e.cursor.line.prev != nil {
				e.cursor.line = e.cursor.line.prev
			}
			e.cursor.x = int(math.Min(float64(e.cursor.x), float64(len(e.cursor.line.values)-1)))
		}
		return nil
	} else if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) && ebiten.IsKeyPressed(ebiten.KeyShift) {
		e.scroll++
		if e.cursor.line.next != nil {
			e.cursor.line = e.cursor.line.next
		}
		e.cursor.x = int(math.Min(float64(e.cursor.x), float64(len(e.cursor.line.values)-1)))
		return nil
	}

	// Movement
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
		if ebiten.IsKeyPressed(ebiten.KeyMetaLeft) {
			for e.cursor.x < len(e.cursor.line.values)-1 {
				e.cursor.x++
			}
			return nil
		}
		if e.cursor.x < len(e.cursor.line.values)-1 {
			e.cursor.x++
		} else if e.cursor.line.next != nil {
			e.cursor.line = e.cursor.line.next
			e.cursor.x = 0
		}
	} else if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
		if ebiten.IsKeyPressed(ebiten.KeyMetaLeft) {
			e.cursor.x = 0
			return nil
		}
		if e.cursor.x > 0 {
			e.cursor.x--
		} else if e.cursor.line.prev != nil {
			e.cursor.line = e.cursor.line.prev
			e.cursor.x = len(e.cursor.line.values) - 1
		}
	} else if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) {
		if ebiten.IsKeyPressed(ebiten.KeyMetaLeft) {
			for e.cursor.line.prev != nil {
				e.cursor.line = e.cursor.line.prev
			}
			return nil
		}
		if e.cursor.line.prev != nil {
			e.cursor.x = int(math.Min(float64(e.cursor.x), float64(len(e.cursor.line.prev.values)-1)))
			e.cursor.line = e.cursor.line.prev
		}
	} else if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) {
		if ebiten.IsKeyPressed(ebiten.KeyMetaLeft) {
			for e.cursor.line.next != nil {
				e.cursor.line = e.cursor.line.next
			}
			return nil
		}
		if e.cursor.line.next != nil {
			e.cursor.x = int(math.Min(float64(e.cursor.x), float64(len(e.cursor.line.next.values)-1)))
			e.cursor.line = e.cursor.line.next
		}
	}

	// Keys which are valid input
	for i := 0; i < int(ebiten.KeyMax); i++ {
		key := ebiten.Key(i)
		if inpututil.IsKeyJustPressed(key) {
			shift := ebiten.IsKeyPressed(ebiten.KeyShift)
			keyRune, printable := KeyToRune(key, shift)

			// Skip unprintable keys (like Enter/Esc)
			if !printable {
				continue
			}

			// Skip runes that we don't have images for
			if _, ok := fontImages[keyRune]; !ok {
				continue
			}

			e.HandleRune(keyRune)
		}
	}
	return nil
}

func (e *Editor) GetAllRunes() []rune {
	all := make([]rune, 0)
	cur := e.start
	for cur != nil {
		all = append(all, cur.values...)
		cur = cur.next
	}
	return all
}

func (e *Editor) GetLineNumber() int {
	cur := e.start
	count := 1
	for cur != e.cursor.line {
		count++
		cur = cur.next
	}
	return count
}

func (e *Editor) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{255, 255, 255, 0xff})
	xPadding := float64(xUnit) / 2
	yPadding := float64(yUnit) * 1.25

	// The screen is larger than the layout!
	_xScreen, _yScreen := ebiten.WindowSize()
	xLayout := _xScreen / 2
	yLayout := _yScreen / 2

	// How many lines of text can be displayed
	// (there's yPadding for top and bottom bar)
	xSlots := (yLayout - int(yPadding*2)) / yUnit

	// How many characters can be displayed width-wise
	// ySlots := (xLayout - int(xPadding*2)) / xUnit

	// Handle top bar
	modifiedText := ""
	if e.modified {
		modifiedText = "(modified)"
	}
	topBar := []rune(fmt.Sprintf("%s %s", fileName, modifiedText))
	for x, char := range topBar {
		opts := &ebiten.DrawImageOptions{}
		opts.GeoM.Translate(float64(x*xUnit)+xPadding, 0)
		fontImage, ok := fontImages[char]
		if !ok {
			// Filler character for an unknown character (missing image)
			screen.DrawImage(fontImages[rune('?')], opts)
		} else {
			screen.DrawImage(fontImage, opts)
		}
	}
	ebitenutil.DrawLine(screen, 0, float64(yUnit+1), float64(xLayout), float64(yUnit+1), color.RGBA{
		0, 0, 0, 100,
	})

	// Handle bottom bar
	botBar := []rune(fmt.Sprintf("(x cut, v paste, s save, q quit, c copyall) [%v:%v:%v] ", e.GetLineNumber(), e.cursor.x+1, e.cursor.line.values[e.cursor.x]))
	for x, char := range botBar {
		opts := &ebiten.DrawImageOptions{}
		opts.GeoM.Translate(float64(x*xUnit)+xPadding, float64(yLayout-yUnit))
		fontImage, ok := fontImages[char]
		if !ok {
			// Filler character for an unknown character (missing image)
			screen.DrawImage(fontImages[rune('?')], opts)
		} else {
			screen.DrawImage(fontImage, opts)
		}
	}
	ebitenutil.DrawLine(screen, 0, float64(yLayout-yUnit-2), float64(xLayout), float64(yLayout-yUnit-2), color.RGBA{
		0, 0, 0, 100,
	})

	// Handle all lines
	curLine := e.start
	y := 0

	scrollCounter := e.scroll
	for scrollCounter > 0 && curLine != nil {
		scrollCounter--
		// Skip all the lines above the scroll position
		curLine = curLine.next
	}

	for curLine != nil {
		// Don't render outside the line area
		if y == xSlots {
			break
		}

		// Handle each line
		x := 0
		xOverSpilled := false
		for _, char := range curLine.values {
			opts := &ebiten.DrawImageOptions{}

			// Render cursor
			if e.cursor.line == curLine && x == e.cursor.x {
				// If we're about to render off the edge of the screen
				// it means we're trying to view a very long line.
				// Clear the current line and start from zero (hiding everything to the left)
				if float64(x*xUnit)+xPadding >= float64(xLayout) {
					if xOverSpilled {
						// Only clear the current line once, otherwise stop rendering
						// otherwise we'll get visual bugs
						break
					}
					x = 0
					xOverSpilled = true
					ebitenutil.DrawRect(screen, 0, float64(y*yUnit)+yPadding, float64(xLayout), float64(yUnit), color.RGBA{
						255, 255, 255, 255,
					})
				}

				// Draw grey cursor background
				ebitenutil.DrawRect(screen, float64(x*xUnit)+xPadding, float64(y*yUnit)+yPadding, float64(xUnit), float64(yUnit), color.RGBA{
					0, 0, 0, 100,
				})
			}

			opts.GeoM.Translate(float64(x*xUnit)+xPadding, float64(y*yUnit)+yPadding)
			if char != '\n' {
				fontImage, ok := fontImages[char]
				if !ok {
					// Render a red square [?] for unknown characters
					ebitenutil.DrawRect(screen, float64(x*xUnit)+xPadding, float64(y*yUnit)+yPadding, float64(xUnit), float64(yUnit), color.RGBA{
						90, 0, 0, 60,
					})
					screen.DrawImage(fontImages[rune('?')], opts)
				} else {
					screen.DrawImage(fontImage, opts)
				}
			}
			x++
		}
		curLine = curLine.next
		y++
	}
}

func (e *Editor) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	_xScreen, _yScreen := ebiten.WindowSize()
	return _xScreen / 2, _yScreen / 2
}

// Supports macOS UK keyboard
func KeyToRune(k ebiten.Key, shift bool) (rune, bool) {
	ret := ""

	switch k {

	// Alphas
	case ebiten.KeyA:
		ret = "A"
	case ebiten.KeyB:
		ret = "B"
	case ebiten.KeyC:
		ret = "C"
	case ebiten.KeyD:
		ret = "D"
	case ebiten.KeyE:
		ret = "E"
	case ebiten.KeyF:
		ret = "F"
	case ebiten.KeyG:
		ret = "G"
	case ebiten.KeyH:
		ret = "H"
	case ebiten.KeyI:
		ret = "I"
	case ebiten.KeyJ:
		ret = "J"
	case ebiten.KeyK:
		ret = "K"
	case ebiten.KeyL:
		ret = "L"
	case ebiten.KeyM:
		ret = "M"
	case ebiten.KeyN:
		ret = "N"
	case ebiten.KeyO:
		ret = "O"
	case ebiten.KeyP:
		ret = "P"
	case ebiten.KeyQ:
		ret = "Q"
	case ebiten.KeyR:
		ret = "R"
	case ebiten.KeyS:
		ret = "S"
	case ebiten.KeyT:
		ret = "T"
	case ebiten.KeyU:
		ret = "U"
	case ebiten.KeyV:
		ret = "V"
	case ebiten.KeyW:
		ret = "W"
	case ebiten.KeyX:
		ret = "X"
	case ebiten.KeyY:
		ret = "Y"
	case ebiten.KeyZ:
		ret = "Z"

	// Specials
	case ebiten.KeyBackquote:
		if shift {
			ret = "~"
		} else {
			ret = "`"
		}
	case ebiten.KeyBackslash:
		if shift {
			ret = "|"
		} else {
			ret = "\\"
		}
	case ebiten.KeyBracketLeft:
		if shift {
			ret = "{"
		} else {
			ret = "["
		}
	case ebiten.KeyBracketRight:
		if shift {
			ret = "}"
		} else {
			ret = "]"
		}
	case ebiten.KeyComma:
		if shift {
			ret = "<"
		} else {
			ret = ","
		}
	case ebiten.KeyDigit0:
		if shift {
			ret = ")"
		} else {
			ret = "0"
		}
	case ebiten.KeyDigit1:
		if shift {
			ret = "!"
		} else {
			ret = "1"
		}
	case ebiten.KeyDigit2:
		if shift {
			ret = "@"
		} else {
			ret = "2"
		}
	case ebiten.KeyDigit3:
		if shift {
			ret = "£"
		} else {
			ret = "3"
		}
	case ebiten.KeyDigit4:
		if shift {
			ret = "$"
		} else {
			ret = "4"
		}
	case ebiten.KeyDigit5:
		if shift {
			ret = "%"
		} else {
			ret = "5"
		}
	case ebiten.KeyDigit6:
		if shift {
			ret = "^"
		} else {
			ret = "6"
		}
	case ebiten.KeyDigit7:
		if shift {
			ret = "&"
		} else {
			ret = "7"
		}
	case ebiten.KeyDigit8:
		if shift {
			ret = "*"
		} else {
			ret = "8"
		}
	case ebiten.KeyDigit9:
		if shift {
			ret = "("
		} else {
			ret = "9"
		}
	case ebiten.KeyMinus:
		if shift {
			ret = "_"
		} else {
			ret = "-"
		}
	case ebiten.KeyEqual:
		if shift {
			ret = "+"
		} else {
			ret = "="
		}
	case ebiten.KeyPeriod:
		if shift {
			ret = ">"
		} else {
			ret = "."
		}
	case ebiten.KeyQuote:
		if shift {
			ret = "\""
		} else {
			ret = "'"
		}
	case ebiten.KeySemicolon:
		if shift {
			ret = ":"
		} else {
			ret = ";"
		}
	case ebiten.KeySlash:
		if shift {
			ret = "?"
		} else {
			ret = "/"
		}

	// Spacing
	case ebiten.KeySpace:
		ret = " "
	}

	// Handle case (only affects alphas)
	if shift {
		ret = strings.ToUpper(ret)
	} else {
		ret = strings.ToLower(ret)
	}

	if ret == "" {
		return rune(0), false
	}

	return rune(ret[0]), true
}

func macOScopy(copyBytes []byte) error {
	cmd := exec.Command("pbcopy")
	in, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	if _, err := in.Write(copyBytes); err != nil {
		return err
	}
	if err := in.Close(); err != nil {
		return err
	}
	if err := cmd.Wait(); err != nil {
		return err
	}
	return nil
}

func macOSpaste() ([]byte, error) {
	cmd := exec.Command("pbpaste")
	pasteBytes, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return pasteBytes, nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: noter <filepath>")
		os.Exit(1)
	} else if len(os.Args) == 3 {
		// Allow `go run . -- a.txt` for now..
		filePath = os.Args[2]
	} else {
		// This is the way
		filePath = os.Args[1]
	}

	editor := &Editor{}
	err := editor.Load()
	if err != nil {
		log.Fatalln(err)
	}

	ebiten.SetWindowSize(800, 500)
	ebiten.SetWindowTitle("noter")
	if err = ebiten.RunGame(editor); err != nil {
		log.Fatalln(err)
	}
}
