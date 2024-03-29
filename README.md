# 📝 noter
> My blog posts:
> - [Making a Text Editor with a Game Engine](https://healeycodes.com/making-a-text-editor-with-a-game-engine)
> - [Implementing Highlighting, Search, and Undo](https://healeycodes.com/implementing-highlighting-search-and-undo)

<br>

A text editor for macOS. Built using the [Ebitengine](https://github.com/hajimehoshi/ebiten) game engine.

It's a little bit like `nano`.

<img src="https://github.com/healeycodes/noter/blob/main/preview.png" alt="A screenshot of the editor running. It looks like nano. It has a text file called 'A Bird, came down the Walk' opened." width="535px">


## Shortcuts

Highlight with (shift + arrow key).

Swap lines with option + (up)/(down).

Command +
- (z) undo
- (f) search
- (a) select all
- (c) copy
- (x) cut
- (v) paste
- (x) save
- (q) quit without saving
- (left)/(right) skips to start/end of line
- (up)/(down) skip to start/end of document

## Development

Run the editor `go run github.com/healeycodes/noter/cmd/noter -- "A Bird, came down the Walk.txt"`

## Build

Build `go build ./cmd/noter`

Run the editor `./noter "A Bird, came down the Walk.txt"`

## Tests

`go test .`

## Roadmap

- More tests
- Implement redo?
