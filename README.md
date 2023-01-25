# 📝 noter

A text editor for macOS. Built using the [Ebitengine](https://github.com/hajimehoshi/ebiten) game engine.

![A screenshot of the editor running. It looks like nano. It has a text file called "A Bird, came down the Walk" opened.](https://github.com/healeycodes/noter/blob/main/preview.png)

It's a little bit like `nano`.

## Shortcuts

Command+
- c: copy line
- x: cut line
- v: paste
- x: save
- q: quit without saving
- left/right: skips to start/end of line
- up/down: is the same as page up/page down

## Development

Run the fonts build script `bash fonts.sh`

Run the editor `go run . -- "A Bird, came down the Walk.txt"`

## Build

`go build .`

## Tests

`go test ./...`

## Roadmap

- More tests
- Search?
