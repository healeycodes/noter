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
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

var (
	filepath string
	//go:embed fonts/dist/fonts.store
	fontStoreRaw []byte
	//go:embed fonts/dist/fonts.json
	fontMapRaw []byte // hex: [offset, size]
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
	start  *Line
	cursor *Cursor
	scroll int
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
	e.scroll = 0
	e.start = &Line{values: make([]rune, 0)}

	f, err := os.Open(filepath)
	if err != nil {
		log.Fatalln(err)
	}
	defer f.Close()

	b, err := os.ReadFile(filepath)
	if err != nil {
		log.Fatalln(err)
	}

	source := []rune(string(b))

	// Turn empty files into `\n`
	// Also, all files must end with `\n`
	if len(source) == 0 || source[len(source)-1] != rune('\n') {
		source = append(source, rune('\n'))
	}

	currentLine := e.start
	e.cursor = &Cursor{line: e.start, x: 0}

	for _, char := range source {
		currentLine.values = append(currentLine.values, char)
		if char == '\n' {
			nextLine := &Line{values: make([]rune, 0)}
			currentLine.next = nextLine
			nextLine.prev = currentLine
			currentLine = nextLine
		}
	}

	// TODO: make next line nil?

	return nil
}

func (e *Editor) Update() error {
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		before := e.cursor.line
		after := e.cursor.line.next

		shiftedValues := make([]rune, 0)
		leftBehindValues := make([]rune, 0)
		shiftedValues = append(shiftedValues, e.cursor.line.values[e.cursor.x:]...)
		leftBehindValues = append(leftBehindValues, e.cursor.line.values[:e.cursor.x]...)
		leftBehindValues = append(leftBehindValues, rune('\n'))
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
	}

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
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
		if e.cursor.x < len(e.cursor.line.values)-1 {
			e.cursor.x++
		} else if e.cursor.line.next != nil {
			e.cursor.line = e.cursor.line.next
			e.cursor.x = 0
		}
	} else if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
		if e.cursor.x > 0 {
			e.cursor.x--
		} else if e.cursor.line.prev != nil {
			e.cursor.line = e.cursor.line.prev
			e.cursor.x = len(e.cursor.line.values) - 1
		}
	} else if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) {
		if e.cursor.line.prev != nil {
			e.cursor.x = int(math.Min(float64(e.cursor.x), float64(len(e.cursor.line.prev.values)-1)))
			e.cursor.line = e.cursor.line.prev
		}
	} else if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) {
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
			keyRune, printable := keyToRune(key, shift)

			// Skip unprintable keys (like Enter/Esc)
			if !printable {
				continue
			}

			// Skip runes that we don't have images for
			if _, ok := fontImages[keyRune]; !ok {
				continue
			}

			modifiedLine := make([]rune, 0)
			modifiedLine = append(modifiedLine, e.cursor.line.values[:e.cursor.x]...)
			modifiedLine = append(modifiedLine, keyRune)
			modifiedLine = append(modifiedLine, e.cursor.line.values[e.cursor.x:]...)
			e.cursor.line.values = modifiedLine
			e.cursor.x++
		}
	}
	return nil
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
	slots := (yLayout - int(yPadding*2)) / yUnit

	// Handle top bar
	topBar := []rune("file.txt (modified)")
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
	botBar := []rune("^+x (save&quit) ^+k (cut line) ^+v (paste) ^+q (quit)")
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
		if y == slots {
			break
		}

		// Handle one line
		for x, char := range curLine.values {
			opts := &ebiten.DrawImageOptions{}
			opts.GeoM.Translate(float64(x*xUnit)+xPadding, float64(y*yUnit)+yPadding)

			// Render cursor
			if e.cursor.line == curLine && x == e.cursor.x {
				ebitenutil.DrawRect(screen, float64(x*xUnit)+xPadding, float64(y*yUnit)+yPadding, float64(xUnit), float64(yUnit), color.RGBA{
					0, 0, 0, 100,
				})
			}

			if char != '\n' {
				fontImage, ok := fontImages[char]
				if !ok {
					// Render a red square for unknown characters (like `\t`)
					ebitenutil.DrawRect(screen, float64(x*xUnit)+xPadding, float64(y*yUnit)+yPadding, float64(xUnit), float64(yUnit), color.RGBA{
						90, 0, 0, 60,
					})
					screen.DrawImage(fontImages[rune('?')], opts)
				} else {
					screen.DrawImage(fontImage, opts)
				}
			}
		}
		curLine = curLine.next
		y++
	}
}

func (e *Editor) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return 320, 320
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: noter <filepath>")
		os.Exit(1)
	} else if len(os.Args) == 3 {
		filepath = os.Args[2]
	} else {
		filepath = os.Args[1]
	}

	editor := &Editor{}
	err := editor.Load()
	if err != nil {
		log.Fatalln(err)
	}

	ebiten.SetWindowSize(640, 640)
	ebiten.SetWindowTitle("noter")
	if err = ebiten.RunGame(editor); err != nil {
		log.Fatalln(err)
	}
}

func keyToRune(k ebiten.Key, shift bool) (rune, bool) {
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
	case ebiten.KeyTab:
		ret = "\t"
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
