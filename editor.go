// MIT License
//
// Copyright (c) 2024 Andrew Healey
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package noter

import (
	"fmt"
	"image/color"
	"log"
	"os"
	"sort"
	"strings"
	"unicode"

	"github.com/hajimehoshi/bitmapfont/v3"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font"
)

const (
	EDITOR_DEFAULT_ROWS = 25
	EDITOR_DEFAULT_COLS = 80
)

type editorLine struct {
	prev   *editorLine
	next   *editorLine
	values []rune
}

type editorCursor struct {
	line *editorLine
	x    int
}

func (c *editorCursor) FixPosition() {
	limit := len(c.line.values) - 1
	if c.x > limit {
		c.x = limit
	}
}

// Content is an interface to a clipboard or file to read/write data.
// We use this instead of io.ReadWriter as we do not want to handle
// errors or buffered reads in the Editor; we force that to the caller
// of the editor.
type Content interface {
	ReadText() []byte // Read the entire content of the text clipboard.
	WriteText([]byte) // Write replaces the entire content of the text clipboard.
}

// dummyContent provides a trivial text storage implementation.
type dummyContent struct {
	content string
}

func (cb *dummyContent) ReadText() []byte {
	return []byte(cb.content)
}

func (cb *dummyContent) WriteText(content []byte) {
	// 'string' cast will make a duplicate of the content.
	cb.content = string(content)
}

type fontInfo struct {
	face   font.Face // Font itself.
	ascent int       // ascent of the font above the baseline's origin.
	xUnit  int       // xUnit is the text advance of the '0' glyph.
	yUnit  int       // yUnit is the line height of the font.
}

// Create a new fontInfo
func newfontInfo(font_face font.Face) (fi *fontInfo) {
	metrics := font_face.Metrics()
	advance, _ := font_face.GlyphAdvance('0')

	fi = &fontInfo{
		face:   font_face,
		ascent: metrics.Ascent.Ceil(),
		xUnit:  advance.Ceil(),
		yUnit:  metrics.Height.Ceil(),
	}

	return fi
}

const (
	EDIT_MODE = iota
	SEARCH_MODE
)

var noop = func() bool { return false }

// Editor is an ebiten.Game compliant interface, which presents a
// simple text editor.
type Editor struct {
	// Settable options
	font_info        *fontInfo
	font_color       color.Color
	select_color     color.Color
	search_color     color.Color
	cursor_color     color.Color
	background_image *ebiten.Image
	clipboard        Content
	content          Content
	content_name     string
	rows             int
	cols             int
	width            int
	height           int
	width_padding    int
	bot_bar          bool
	top_bar          bool

	// Internal state
	screen           *ebiten.Image
	top_padding      int
	bot_padding      int
	mode             uint
	searchIndex      int
	searchTerm       []rune
	start            *editorLine
	cursor           *editorCursor
	modified         bool
	highlighted      map[*editorLine]map[int]bool
	searchHighlights map[*editorLine]map[int]bool
	undoStack        []func() bool
}

// EditorOption is an option that can be sent to NewEditor()
type EditorOption func(e *Editor)

// WithContent sets the content accessor, and permits saving and loading.
// If set to nil, an in-memory content manager is used.
func WithContent(opt Content) EditorOption {
	return func(e *Editor) {
		if opt == nil {
			opt = &dummyContent{}
		}
		e.content = opt
	}
}

// WithContentName sets the name of the content
func WithContentName(opt string) EditorOption {
	return func(e *Editor) {
		e.content_name = opt
	}
}

// WithTopBar enables the display of the first row as a top bar.
func WithTopBar(enabled bool) EditorOption {
	return func(e *Editor) {
		e.top_bar = enabled
	}
}

// WithBottomBar enables the display of the last row as a help display.
func WithBottomBar(enabled bool) EditorOption {
	return func(e *Editor) {
		e.bot_bar = enabled
	}
}

// WithClipboard sets the clipboard accessor.
// If set to nil, an in-memory content manager is used.
func WithClipboard(opt Content) EditorOption {
	return func(e *Editor) {
		if opt == nil {
			opt = &dummyContent{}
		}
		e.clipboard = opt
	}
}

// WithFontFace set the default font.
// If set to nil, the monospace font `github.com/hajimehoshi/bitmapfont/v3`
// is used.
func WithFontFace(opt font.Face) EditorOption {
	return func(e *Editor) {
		if opt == nil {
			opt = bitmapfont.Face
		}
		e.font_info = newfontInfo(opt)
	}
}

// WithFontColor sets the color of the text.
func WithFontColor(opt color.Color) EditorOption {
	return func(e *Editor) {
		e.font_color = opt
	}
}

// WithHighlightColor sets the color of the select highlight over the text.
func WithHighlightColor(opt color.Color) EditorOption {
	return func(e *Editor) {
		e.select_color = opt
	}
}

// WithSearchColor sets the color of the search highlight over the text.
func WithSearchColor(opt color.Color) EditorOption {
	return func(e *Editor) {
		e.search_color = opt
	}
}

// WithCursorColor sets the color of the cursor over the text.
func WithCursorColor(opt color.Color) EditorOption {
	return func(e *Editor) {
		e.cursor_color = opt
	}
}

// WithBackgroundColor sets the color of the background.
func WithBackgroundColor(opt color.Color) EditorOption {
	return func(e *Editor) {
		// Make a single pixel image with the background color.
		// We will scale it to fit.
		img := ebiten.NewImage(1, 1)
		img.Fill(opt)
		WithBackgroundImage(img)(e)
	}
}

// WithBackgroundImage sets the ebiten.Image in the background.
// It will be scaled to fit the entire background of the editor.
func WithBackgroundImage(opt *ebiten.Image) EditorOption {
	return func(e *Editor) {
		e.background_image = opt
	}
}

// WithRows sets the total number of rows in the editor, including
// the top bar and bottom bar, if enabled. If set to < 0, then:
//   - if WithHeight is set, then the maximum number of rows that would
//     fit, based on font height, is used.
//   - if WithHeight is not set, then the number of rows defaults to 25.
func WithRows(opt int) EditorOption {
	return func(e *Editor) {
		e.rows = opt
	}
}

// WidthHeight sets the image height of the editor.
// If WithRows is set, the font is scaled appropriately to the height.
// If WithRows is not set, the maximum number of rows that would fit
// are used, with any additional padding to the bottom of the editor.
// If not set, see the 'WithRows()' option for the calculation.
func WithHeight(opt int) EditorOption {
	return func(e *Editor) {
		e.height = opt
	}
}

// WithColumns sets the total number of columns in the editor, including
// the line-number area, if enabled. If set to < 0, then:
//   - if WithWidth is set, then the maximum number of columns that would
//     fit, based on font advance of the glyph '0', is used.
//   - if WithWidth is not set, then the number of columns defaults to 80.
func WithColumns(opt int) EditorOption {
	return func(e *Editor) {
		e.cols = opt
	}
}

// WidthWidth sets the image width of the editor.
// If WithColumns is set, the font is scaled appropriately to the width.
// If WithColumns is not set, the maximum number of columns that would fit
// are used, with any additional padding to the bottom of the editor.
// If not set, see the 'WithColumns()' option for the calculation.
func WithWidth(opt int) EditorOption {
	return func(e *Editor) {
		e.height = opt
	}
}

// WithWidthPadding sets the left and right side padding, in pixels.
// If not set, the default is 1/2 of the width of the text advance
// of the font's rune '0'.
func WithWithPadding(opt int) EditorOption {
	return func(e *Editor) {
		e.width_padding = opt
	}
}

// NewEditor creates a new editor. See the EditorOption type for
// available options that can be passed to change its defaults.
//
// If neither the WithHeight nor WithRows options are set, the editor
// defaults to 25 rows.
// The resulting image width is `rows * font.Face.Metrics().Height`
//
// If neither the WithWidth nor the WithCols options are set, the
// editor defaults to 80 columns. The resulting image width
// is `cols * font.Face.GlyphAdvance('0')`
func NewEditor(options ...EditorOption) (e *Editor) {
	e = &Editor{
		rows:          -1,
		cols:          -1,
		width:         -1,
		height:        -1,
		width_padding: -1,
	}

	WithContent(nil)(e)
	WithClipboard(nil)(e)
	WithFontFace(nil)(e)
	WithFontColor(color.Black)(e)
	WithBackgroundColor(color.White)(e)
	WithCursorColor(color.RGBA{0, 0, 0, 90})(e)
	WithHighlightColor(color.RGBA{0, 0, 200, 70})(e)
	WithSearchColor(color.RGBA{0, 200, 0, 70})(e)

	for _, opt := range options {
		opt(e)
	}

	// Determine padding.
	if e.width_padding < 0 {
		e.width_padding = e.font_info.xUnit / 2
	}

	if e.top_bar {
		e.top_padding = int(float64(e.font_info.yUnit) * 1.25)
	}

	if e.bot_bar {
		e.bot_padding = int(float64(e.font_info.yUnit) * 1.25)
	}

	// Set geometry defaults.
	if e.rows < 0 {
		if e.height < 0 {
			e.rows = EDITOR_DEFAULT_ROWS
		} else {
			e.rows = (e.height - (e.top_padding + e.bot_padding)) / e.font_info.yUnit
		}
	}

	if e.cols < 0 {
		if e.width < 0 {
			e.cols = EDITOR_DEFAULT_COLS
		} else {
			e.cols = (e.width - e.width_padding*2) / e.font_info.xUnit
		}
	}

	if e.width < 0 {
		e.width = e.font_info.xUnit*e.cols + e.width_padding*2
	}

	if e.height < 0 {
		e.height = e.font_info.yUnit*e.rows + e.top_padding + e.bot_padding
	}

	text_height := e.height - (e.top_padding + e.bot_padding)
	text_width := e.width - (e.width_padding * 2)

	// Clamp rows and cols to fit.
	if e.rows > text_height/e.font_info.yUnit {
		e.rows = text_height / e.font_info.yUnit
	}

	if e.cols > text_width/e.font_info.xUnit {
		e.cols = text_width / e.font_info.xUnit
	}

	// Load content.
	e.Load()

	return e
}

func (e *Editor) searchMode() {
	e.resetHighlight()
	e.mode = SEARCH_MODE
	e.searchHighlights = make(map[*editorLine]map[int]bool)
}

func (e *Editor) editMode() {
	e.mode = EDIT_MODE
	e.searchTerm = make([]rune, 0)
	e.searchHighlights = make(map[*editorLine]map[int]bool)
}

func (e *Editor) fnDeleteHighlighted() func() bool {
	highlightCount := 0
	lastHighlightedLine := e.start
	lastHighlightedX := 0
	curLine := e.start
	for curLine != nil {
		if lineWithHighlights, ok := e.highlighted[curLine]; ok {
			lastHighlightedLine = curLine
			lastHighlightedX = 0
			for index := range lineWithHighlights {
				if lastHighlightedX < index {
					lastHighlightedX = index
				}
				highlightCount++
			}
		}
		curLine = curLine.next
	}
	e.cursor.line = lastHighlightedLine
	e.cursor.x = lastHighlightedX + 1

	// When a single new line character is highlighted
	// we need to start deleting from the start of the
	// next line so we can re-use existing deletion logic
	if e.cursor.x == len(e.cursor.line.values) && e.cursor.line.next != nil {
		e.cursor.line = e.cursor.line.next
		e.cursor.x = 0
	}

	highlightedRunes := e.getHighlightedRunes()

	for i := 0; i < highlightCount; i++ {
		e.deletePrevious()
	}

	lineNum := e.getLineNumber()
	curX := e.cursor.x

	return func() bool {
		e.MoveCursor(lineNum, curX)
		for _, r := range highlightedRunes {
			e.handleRune(r)
		}
		return true
	}
}

func (e *Editor) resetHighlight() {
	e.highlighted = make(map[*editorLine]map[int]bool)
}

func (e *Editor) setModified() {
	e.modified = true
}

// Save saves the text to the Content assigned to the editor.
func (e *Editor) Save() {
	if e.content != nil {
		e.content.WriteText(e.ReadText())
		e.modified = false
	}
}

// Load loads the text from the Content assigned to the editor.
func (e *Editor) Load() {
	if e.content != nil {
		e.WriteText(e.content.ReadText())
	}
}

// ReadText returns all of the text in the editor.
// Note that this does not clear the 'modified' state of the editor.
func (e *Editor) ReadText() []byte {
	allRunes := e.getAllRunes()

	return []byte(string(allRunes))
}

// WriteText replaces all of the text in the editor.
// Note that this clears the 'modified' state of the editor, and disables
// all selection highlighting.
func (e *Editor) WriteText(text []byte) {
	source := string(text)

	e.editMode()
	e.undoStack = make([]func() bool, 0)
	e.searchTerm = make([]rune, 0)
	e.highlighted = make(map[*editorLine]map[int]bool)
	e.start = &editorLine{values: make([]rune, 0)}
	e.cursor = &editorCursor{line: e.start, x: 0}
	currentLine := e.start

	if len(source) == 0 {
		currentLine.values = append(currentLine.values, '\n')
	} else {
		for _, char := range source {
			currentLine.values = append(currentLine.values, char)
			if char == '\n' {
				nextLine := &editorLine{values: make([]rune, 0)}
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
}

func (e *Editor) search() {
	// Always reset search highlights (for empty searches)
	e.searchHighlights = make(map[*editorLine]map[int]bool)

	if len(e.searchTerm) == 0 {
		return
	}

	curLine := e.start
	searchTermIndex := 0

	// Store the location of all runes that are part of a result
	// this will be used render search highlights
	possibleMatches := make(map[*editorLine]map[int]bool, 0)

	// Store the starting lines and line indexes of every match
	// this will be used to tab between results
	possibleLines := make([]*editorLine, 0)
	possibleXs := make([]int, 0)

	for curLine != nil {
		for index, r := range curLine.values {
			if unicode.ToLower(e.searchTerm[searchTermIndex]) == unicode.ToLower(r) {

				// We've found the possible start of a match
				if searchTermIndex == 0 {
					possibleLines = append(possibleLines, curLine)
					possibleXs = append(possibleXs, index)
				}
				searchTermIndex++

				// We've found part of a possible match
				if _, ok := possibleMatches[curLine]; !ok {
					possibleMatches[curLine] = make(map[int]bool)
				}
				possibleMatches[curLine][index] = true
			} else {
				// Clear up the incorrect possible start
				if searchTermIndex > 0 {
					possibleLines = possibleLines[:len(possibleLines)-1]
					possibleXs = possibleXs[:len(possibleXs)-1]
				}

				searchTermIndex = 0

				// Clear up the incorrect possible match parts
				possibleMatches = make(map[*editorLine]map[int]bool, 0)
			}

			// We found a full match. Save the match parts for highlighting
			// and reset all state to check for more matches
			if searchTermIndex == len(e.searchTerm) {
				for line := range possibleMatches {
					for x := range possibleMatches[line] {
						if _, ok := e.searchHighlights[line]; !ok {
							e.searchHighlights[line] = make(map[int]bool)
						}
						e.searchHighlights[line][x] = true
					}
				}

				searchTermIndex = 0
				possibleMatches = make(map[*editorLine]map[int]bool, 0)
			}
		}
		curLine = curLine.next
	}

	// Were there any full matches?
	if len(possibleLines) > 0 {

		// Have we tabbed before the first full match?
		if e.searchIndex == -1 {
			e.cursor.line = possibleLines[len(possibleLines)-1]
			e.cursor.x = possibleXs[len(possibleXs)-1]
			e.searchIndex = len(possibleLines) - 1
			return
		}

		// Have we tabbed beyond the final full match?
		if e.searchIndex > len(possibleLines)-1 {
			e.searchIndex = 0
		}

		// Move to the desired match
		e.cursor.line = possibleLines[e.searchIndex]
		e.cursor.x = possibleXs[e.searchIndex]
		return
	}

	// There were no matches, reset so that the next search can hit the first match it finds
	e.searchIndex = 0
}

func (e *Editor) fnHandleRuneSingle(r rune) func() bool {
	undoDeleteHighlighted := func() bool { return false }
	if len(e.highlighted) != 0 {
		undoDeleteHighlighted = e.fnDeleteHighlighted()
	}

	e.handleRune(r)

	lineNum := e.getLineNumber()
	curX := e.cursor.x
	return func() bool {
		e.MoveCursor(lineNum, curX)
		e.deletePrevious()
		undoDeleteHighlighted()
		return true
	}
}

func (e *Editor) fnHandleRuneMulti(rs []rune) func() bool {
	undoDeleteHighlighted := func() bool { return false }
	if len(e.highlighted) != 0 {
		undoDeleteHighlighted = e.fnDeleteHighlighted()
	}

	for _, r := range rs {
		e.handleRune(r)
	}

	lineNum := e.getLineNumber()
	curX := e.cursor.x
	return func() bool {
		e.MoveCursor(lineNum, curX)
		for i := 0; i < len(rs); i++ {
			e.deletePrevious()
		}
		undoDeleteHighlighted()
		return true
	}
}

func (e *Editor) handleRune(r rune) {
	if e.mode == SEARCH_MODE {
		e.searchTerm = append(e.searchTerm, r)
		e.search()
		return
	}

	if len(e.highlighted) != 0 {
		e.resetHighlight()
	}

	if r == '\n' {
		before := e.cursor.line
		after := e.cursor.line.next

		shiftedValues := make([]rune, 0)
		leftBehindValues := make([]rune, 0)
		shiftedValues = append(shiftedValues, e.cursor.line.values[e.cursor.x:]...)
		leftBehindValues = append(leftBehindValues, e.cursor.line.values[:e.cursor.x]...)
		leftBehindValues = append(leftBehindValues, '\n')
		e.cursor.line.values = leftBehindValues

		e.cursor.line = &editorLine{
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

	e.setModified()
}

// Update the editor state.
func (e *Editor) Update() error {
	// Update the internal image when complete.
	defer e.updateImage()

	// // Log key number
	// for i := 0; i < int(ebiten.KeyMax); i++ {
	// 	if inpututil.IsKeyJustPressed(ebiten.Key(i)) {
	// 		println(i)
	// 		return nil
	// 	}
	// }

	// Modifiers
	command := ebiten.IsKeyPressed(ebiten.KeyMeta) || ebiten.IsKeyPressed(ebiten.KeyControl)
	shift := ebiten.IsKeyPressed(ebiten.KeyShift)
	option := ebiten.IsKeyPressed(ebiten.KeyAlt)

	// Arrows
	right := inpututil.IsKeyJustPressed(ebiten.KeyArrowRight)
	left := inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft)
	up := inpututil.IsKeyJustPressed(ebiten.KeyArrowUp)
	down := inpututil.IsKeyJustPressed(ebiten.KeyArrowDown)

	// Enter search mode
	if command && inpututil.IsKeyJustPressed(ebiten.KeyF) {
		if e.mode == SEARCH_MODE {
			e.editMode()
		} else {
			e.searchMode()
		}
		return nil
	}

	// Next/previous search match
	if (up || down) && e.mode == SEARCH_MODE {
		if up {
			if e.searchIndex > -1 {
				e.searchIndex--
			}
		} else if down {
			e.searchIndex++
		}
		e.search()
		return nil
	}

	// Exit search mode
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		e.editMode()
		return nil
	}

	// Undo
	if command && inpututil.IsKeyJustPressed(ebiten.KeyZ) {
		e.editMode()
		e.resetHighlight()

		for len(e.undoStack) > 0 {
			notNoop := e.undoStack[len(e.undoStack)-1]()
			e.undoStack = e.undoStack[:len(e.undoStack)-1]
			if notNoop {
				break
			}
		}
		return nil
	}

	// Quit
	if command && inpututil.IsKeyJustPressed(ebiten.KeyQ) {
		os.Exit(0)
		return nil
	}

	// Save
	if command && inpututil.IsKeyJustPressed(ebiten.KeyS) {
		e.Save()
		return nil
	}

	// Highlight all
	if command && inpututil.IsKeyJustPressed(ebiten.KeyA) {
		e.editMode()
		e.fnSelectAll()
		return nil
	}

	// Paste
	if command && inpututil.IsKeyJustPressed(ebiten.KeyV) {
		pasteBytes := e.clipboard.ReadText()
		rs := []rune{}
		for _, r := range string(pasteBytes) {
			rs = append(rs, r)
		}
		e.storeUndoAction(e.fnHandleRuneMulti(rs))
		e.setModified()
		return nil
	}

	// Cut highlight
	if command && inpututil.IsKeyJustPressed(ebiten.KeyX) {
		copyRunes := e.getHighlightedRunes()
		if len(copyRunes) == 0 {
			return nil
		}

		e.clipboard.WriteText([]byte(string(copyRunes)))

		e.storeUndoAction(e.fnDeleteHighlighted())
		e.resetHighlight()

		e.setModified()
		return nil
	}

	// Copy highlight
	if command && inpututil.IsKeyJustPressed(ebiten.KeyC) {
		if len(e.highlighted) == 0 {
			return nil
		}
		copyRunes := e.getHighlightedRunes()
		copyBytes := []byte(string(copyRunes))
		e.clipboard.WriteText(copyBytes)
		return nil
	}

	// Handle movement
	if right || left || up || down {
		e.editMode()

		// Clear up old highlighting
		if !shift {
			e.resetHighlight()
		}

		// Option scanning finds the next emptyType after hitting a non-emptyType
		// TODO: the characters that we filter for needs improving
		emptyTypes := map[rune]bool{' ': true, '.': true, ',': true}

		if right {
			if option {
				// Find the next empty
				for e.cursor.x < len(e.cursor.line.values)-2 {
					if shift {
						e.highlight(e.cursor.line, e.cursor.x)
					}
					e.cursor.x++
					if ok := emptyTypes[e.cursor.line.values[e.cursor.x]]; !ok {
					} else {
						break
					}
					if shift {
						e.highlight(e.cursor.line, e.cursor.x)
					}
				}
			} else if command {
				for e.cursor.x < len(e.cursor.line.values)-1 {
					if shift {
						e.highlight(e.cursor.line, e.cursor.x)
					}
					e.cursor.x++
				}
			} else {
				if e.cursor.x < len(e.cursor.line.values)-1 {
					if shift {
						e.highlight(e.cursor.line, e.cursor.x)
					}
					e.cursor.x++
				} else if e.cursor.line.next != nil {
					if shift {
						e.highlight(e.cursor.line, len(e.cursor.line.values)-1)
					}
					e.cursor.line = e.cursor.line.next
					e.cursor.x = 0
				}
			}
		} else if left {
			if option {
				// Find the next non-empty
				for e.cursor.x > 0 {
					e.cursor.x--
					if shift {
						e.highlight(e.cursor.line, e.cursor.x)
					}
					if ok := emptyTypes[e.cursor.line.values[e.cursor.x]]; !ok {
						break
					}
				}

				// Find the next empty
				for e.cursor.x > 0 {
					if ok := emptyTypes[e.cursor.line.values[e.cursor.x-1]]; !ok {
						if shift {
							e.highlight(e.cursor.line, e.cursor.x)
						}
					} else {
						break
					}
					e.cursor.x--
					if shift {
						e.highlight(e.cursor.line, e.cursor.x)
					}
				}
			} else if command {
				for e.cursor.x > 0 {
					e.cursor.x--
					if shift {
						e.highlight(e.cursor.line, e.cursor.x)
					}
				}
			} else {
				if e.cursor.x > 0 {
					e.cursor.x--
					if shift {
						e.highlight(e.cursor.line, e.cursor.x)
					}
				} else if e.cursor.line.prev != nil {
					e.cursor.line = e.cursor.line.prev
					e.cursor.x = len(e.cursor.line.values) - 1
					if shift {
						e.highlight(e.cursor.line, e.cursor.x)
					}
				}
			}
		} else if up {
			if option {
				e.storeUndoAction(e.fnSwapUp())
			} else if command {
				if shift {
					e.highlightLineToLeft()
				}
				for e.cursor.line.prev != nil {
					if shift {
						e.highlightLine()
					}
					e.cursor.line = e.cursor.line.prev
					e.cursor.x = 0
					if shift {
						e.highlightLineToRight()
					}
				}
			} else {
				for x := e.cursor.x - 1; shift && x >= 0; x-- {
					e.highlight(e.cursor.line, x)
				}
				if e.cursor.line.prev != nil {
					e.cursor.line = e.cursor.line.prev
					for x := e.cursor.x; shift && x < len(e.cursor.line.values); x++ {
						e.highlight(e.cursor.line, x)
					}
				} else {
					e.cursor.x = 0
				}
				e.cursor.FixPosition()
			}
		} else if down {
			if option {
				e.storeUndoAction(e.fnSwapDown())
			} else if command {
				for e.cursor.line.next != nil {
					if shift {
						e.highlightLineToRight()
					}
					e.cursor.line = e.cursor.line.next
					if shift {
						e.highlightLineToLeft()
					}
				}
				// Instead of fixing position, we actually want the document end
				if shift {
					e.highlightLineToRight()
				}
				e.cursor.x = len(e.cursor.line.values) - 1
			} else {
				if e.cursor.line.next != nil {
					if shift {
						e.highlightLineToRight()
					}
					e.cursor.line = e.cursor.line.next
					e.cursor.FixPosition()
					if shift {
						e.highlightLineToLeft()
					}
				} else {
					e.cursor.x = len(e.cursor.line.values) - 1
				}
			}
		}

		return nil
	}

	// Enter
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		if e.mode == SEARCH_MODE {
			e.searchIndex++
			e.search()
		} else {
			e.storeUndoAction(e.fnHandleRuneSingle('\n'))
		}
		return nil
	}

	// Tab
	if inpututil.IsKeyJustPressed(ebiten.KeyTab) {
		if e.mode == SEARCH_MODE {
			e.searchIndex++
			e.search()
			return nil
		}
		// Just insert four spaces
		for i := 0; i < 4; i++ {
			e.storeUndoAction(e.fnHandleRuneSingle(' '))
		}
		return nil
	}

	// Backspace
	if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) {
		if e.mode == SEARCH_MODE {
			if len(e.searchTerm) > 0 {
				e.searchTerm = e.searchTerm[:len(e.searchTerm)-1]
			}
			e.search()
			return nil
		}
		// Delete all highlighted content
		if len(e.highlighted) != 0 {
			e.storeUndoAction(e.fnDeleteHighlighted())
		} else {
			// Or..
			e.storeUndoAction(e.fnDeleteSinglePrevious())
		}

		e.resetHighlight()
		e.setModified()
		return nil
	}

	// Keys which are valid input
	for i := 0; i < int(ebiten.KeyMax); i++ {
		key := ebiten.Key(i)
		if inpututil.IsKeyJustPressed(key) {
			keyRune, printable := KeyToRune(key, shift)

			// Skip unprintable keys (like Enter/Esc)
			if !printable {
				continue
			}

			e.storeUndoAction(e.fnHandleRuneSingle(keyRune))
		}
	}
	return nil
}

func (e *Editor) storeUndoAction(fun func() bool) {
	if e.mode == EDIT_MODE {
		e.undoStack = append(e.undoStack, fun)
	}
}

func (e *Editor) fnReturnToCursor(line *editorLine, startingX int) func() {
	destination := e.getLineNumberFromLine(line)
	return func() {
		i := 1
		e.cursor.line = e.start
		for i != destination {
			i++
			e.cursor.line = e.cursor.line.next
		}
		e.cursor.x = startingX
	}
}

func (e *Editor) fnSwapDown() func() bool {
	if e.cursor.line.next != nil {
		tempValues := e.cursor.line.values
		e.cursor.line.values = e.cursor.line.next.values
		e.cursor.line.next.values = tempValues
		e.cursor.line = e.cursor.line.next
		e.cursor.FixPosition()

		lineNum := e.getLineNumber()
		curX := e.cursor.x
		return func() bool {
			e.MoveCursor(lineNum, curX)
			tempValues := e.cursor.line.values
			e.cursor.line.values = e.cursor.line.prev.values
			e.cursor.line.prev.values = tempValues
			e.cursor.line = e.cursor.line.prev
			e.cursor.FixPosition()
			return true
		}
	}
	return noop
}

func (e *Editor) fnSwapUp() func() bool {
	if e.cursor.line.prev != nil {
		tempValues := e.cursor.line.values
		e.cursor.line.values = e.cursor.line.prev.values
		e.cursor.line.prev.values = tempValues
		e.cursor.line = e.cursor.line.prev
		e.cursor.FixPosition()

		lineNum := e.getLineNumber()
		curX := e.cursor.x
		return func() bool {
			e.MoveCursor(lineNum, curX)
			tempValues := e.cursor.line.values
			e.cursor.line.values = e.cursor.line.next.values
			e.cursor.line.next.values = tempValues
			e.cursor.line = e.cursor.line.next
			e.cursor.FixPosition()
			return true
		}
	}
	return noop
}

func (e *Editor) fnSelectAll() {
	e.cursor.line = e.start
	e.highlightLine()

	for e.cursor.line.next != nil {
		e.cursor.line = e.cursor.line.next
		e.cursor.x = len(e.cursor.line.values) - 1
		e.highlightLine()
	}
}

func (e *Editor) fnDeleteSinglePrevious() func() bool {
	if e.cursor.line == e.start && e.cursor.x == 0 {
		return noop
	}

	if e.cursor.x-1 < 0 {
		e.deletePrevious()
		lineNum := e.getLineNumber()
		curX := e.cursor.x
		return func() bool {
			e.MoveCursor(lineNum, curX)
			e.handleRune('\n')
			return true
		}
	} else {
		curRune := e.cursor.line.values[e.cursor.x-1]
		e.deletePrevious()
		lineNum := e.getLineNumber()
		curX := e.cursor.x
		return func() bool {
			e.MoveCursor(lineNum, curX)
			e.handleRune(curRune)
			return true
		}
	}
}

func (e *Editor) deletePrevious() {
	// Instead of allowing an empty document, "clear it" by writing a new line character
	if e.cursor.line == e.start && len(e.cursor.line.values) == 1 {
		e.cursor.line.values = []rune{'\n'}
		e.cursor.FixPosition()
		return
	}

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

func (e *Editor) getHighlightedRunes() []rune {
	copyRunes := make([]rune, 0)
	curLine := e.start
	for curLine != nil {
		if highlightedLine, ok := e.highlighted[curLine]; ok {
			highlightedIndexes := make([]int, 0)
			for index := range highlightedLine {
				highlightedIndexes = append(highlightedIndexes, index)
			}
			sort.Ints(highlightedIndexes)
			for _, i := range highlightedIndexes {
				copyRunes = append(copyRunes, curLine.values[i])
			}
		}
		curLine = curLine.next
	}
	return copyRunes
}

func (e *Editor) highlightLine() {
	for x := range e.cursor.line.values {
		e.highlight(e.cursor.line, x)
	}
}

func (e *Editor) highlightLineToRight() {
	for x := e.cursor.x; x < len(e.cursor.line.values); x++ {
		e.highlight(e.cursor.line, x)
	}
}

func (e *Editor) highlightLineToLeft() {
	for x := e.cursor.x - 1; x > -1; x-- {
		e.highlight(e.cursor.line, x)
	}
}

func (e *Editor) highlight(line *editorLine, x int) {
	if _, ok := e.highlighted[line]; ok {
		e.highlighted[line][x] = true
	} else {
		e.highlighted[line] = map[int]bool{x: true}
	}
}

func (e *Editor) getAllRunes() []rune {
	all := make([]rune, 0)
	cur := e.start
	for cur != nil {
		all = append(all, cur.values...)
		cur = cur.next
	}
	return all
}

// Cursor returns the current cursor position.
func (e *Editor) Cursor() (row int, col int) {
	return e.getLineNumberFromLine(e.cursor.line) - 1, e.cursor.x
}

// MoveCursor moves the cursor to the specified location.
// If `row` is `-1` then the cursor will be on the final row.
// If `col` is `-1` then the cursor is moved to the final rune in the row.
func (e *Editor) MoveCursor(row int, col int) {
	e.cursor.line = e.start
	i := 0
	for i != row {
		if e.cursor.line.next == nil {
			if row < 0 {
				// We're moving to the last line.
				break
			}
			log.Fatalf("attempted illegal move to %v %v", row, col)
		}
		e.cursor.line = e.cursor.line.next
		i++
	}
	if col == -1 {
		e.cursor.x = len(e.cursor.line.values) - 1
	} else {
		e.cursor.x = col
	}
}

// Get the cursor's current line number
func (e *Editor) getLineNumber() int {
	return e.getLineNumberFromLine(e.cursor.line) - 1
}

func (e *Editor) getLineNumberFromLine(line *editorLine) int {
	cur := e.start
	count := 1
	for cur != line && cur != e.cursor.line {
		count++
		cur = cur.next
	}
	return count
}

// Return the size in pixels of the editor.
func (e *Editor) Size() (width, height int) {
	return e.width, e.height
}

// Draw the editor onto the screen, scaled to full size.
func (e *Editor) Draw(screen *ebiten.Image) {
	// Scale editor to the screen region we want to draw into.
	im_width, im_height := screen.Size()
	sc_width := float64(im_width) / float64(e.width)
	sc_height := float64(im_height) / float64(e.height)
	opts := ebiten.DrawImageOptions{}
	opts.GeoM.Scale(sc_width, sc_height)
	screen.DrawImage(e.screen, &opts)
}

// updateImage updates the internal image.
func (e *Editor) updateImage() {
	// Generate an internal image, if we don't have one.
	if e.screen == nil {
		e.screen = ebiten.NewImage(e.width, e.height)
	}
	screen := e.screen

	// Draw the background
	if e.background_image != nil {
		bg_width, bg_height := e.background_image.Size()
		sc_width := float64(e.width) / float64(bg_width)
		sc_height := float64(e.height) / float64(bg_height)
		opts := ebiten.DrawImageOptions{}
		opts.GeoM.Scale(sc_width, sc_height)
		e.screen.DrawImage(e.background_image, &opts)
	}

	// Collect font metrics.
	xUnit := e.font_info.xUnit
	yUnit := e.font_info.yUnit
	fontAscent := e.font_info.ascent
	textColor := e.font_color

	// Handle top bar
	if e.top_bar {
		modifiedText := ""
		if e.modified {
			modifiedText = "(modified)"
		}

		topBar := ">"
		if e.mode == SEARCH_MODE {
			topBar = string(append([]rune(topBar), e.searchTerm...))
		} else {
			topBar = fmt.Sprintf("%s %s", e.content_name, modifiedText)
		}

		text.Draw(screen, string(topBar), e.font_info.face,
			e.width_padding, fontAscent,
			textColor)
		ebitenutil.DrawLine(e.screen, 0, float64(yUnit+1), float64(e.width), float64(yUnit+1), textColor)
	}

	if e.bot_bar {
		// Handle bottom bar
		botBar := fmt.Sprintf("(x)cut (c)opy (v)paste (s)ave (q)uit (f)search [%v:%v:%v] ", e.getLineNumber()+1, e.cursor.x+1, e.cursor.line.values[e.cursor.x])
		text.Draw(screen, string(botBar), e.font_info.face,
			e.width_padding, e.height-yUnit+fontAscent,
			textColor)

		ebitenutil.DrawLine(screen, 0, float64(e.height-yUnit-2), float64(e.width), float64(e.height-yUnit-2), textColor)
	}

	// Handle all lines
	curLine := e.start
	y := 0

	// Find the screen chunk to render
	lineNum := e.getLineNumber()
	screenChunksToSkip := lineNum / e.rows
	for i := 0; i < screenChunksToSkip*e.rows; i++ {
		// Skip to that screen chunk
		curLine = curLine.next
	}

	for curLine != nil {
		// Don't render outside the line area
		if y == e.rows {
			break
		}

		// Handle each line (only render the visible section)
		xStart := 0
		charactersPerScreen := int(float64(e.width-e.width_padding*2) / float64(xUnit))
		if e.cursor.line == curLine && e.cursor.x > charactersPerScreen {
			xStart = ((e.cursor.x / charactersPerScreen) * charactersPerScreen) + 1
		}

		for x, _ := range curLine.values[xStart:] {
			// `x` is the render location
			// `lineIndex` is the line position
			lineIndex := x + xStart

			// Render highlighting (if any)
			if highlight, ok := e.highlighted[curLine]; ok {
				if _, ok := highlight[lineIndex]; ok {
					// Draw blue-y purple highlight background
					ebitenutil.DrawRect(
						screen,
						float64(x*xUnit+e.width_padding),
						float64(y*yUnit+e.top_padding),
						float64(xUnit),
						float64(yUnit),
						e.select_color,
					)
				}
			}

			// Render search highlighting (if any)
			if searchHighlight, ok := e.searchHighlights[curLine]; ok {
				if _, ok := searchHighlight[lineIndex]; ok {
					// Draw green highlight background
					ebitenutil.DrawRect(screen,
						float64(x*xUnit+e.width_padding),
						float64(y*yUnit+e.top_padding),
						float64(xUnit),
						float64(yUnit),
						e.search_color,
					)
				}
			}

			// Render cursor
			if e.cursor.line == curLine && lineIndex == e.cursor.x {
				// Draw gray cursor background
				ebitenutil.DrawRect(screen,
					float64(x*xUnit+e.width_padding),
					float64(y*yUnit+e.top_padding),
					float64(xUnit),
					float64(yUnit),
					e.cursor_color,
				)
			}
		}
		text.Draw(screen, string(curLine.values[xStart:]), e.font_info.face,
			e.width_padding, e.top_padding+y*yUnit+fontAscent,
			textColor)

		curLine = curLine.next
		y++
	}
}

func (e *Editor) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return outsideWidth, outsideHeight
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
