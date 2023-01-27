# 📝 noter
> My blog post: [Making a Text Editor with a Game Engine](https://healeycodes.com/making-a-text-editor-with-a-game-engine)

<br>

A text editor for macOS. Built using the [Ebitengine](https://github.com/hajimehoshi/ebiten) game engine.

It's a little bit like `nano`.

![A screenshot of the editor running. It looks like nano. It has a text file called "A Bird, came down the Walk" opened.](https://github.com/healeycodes/noter/blob/main/noter.png)

## Shortcuts

Highlight with (shift + arrow key).

Command +
- (a) select all
- (c) copy
- (x) cut
- (v) paste
- (x) save
- (q) quit without saving
- (left)/(right) skips to start/end of line
- (up)/(down) skip to start/end of document

## Development

Run the fonts build script `bash fonts.sh`

Run the editor `go run . -- "A Bird, came down the Walk.txt"`

## Build

Build `go build .`

Run the editor `./noter "A Bird, came down the Walk.txt"`

## Tests

`go test ./...`

## Roadmap

- More tests
- Search?
