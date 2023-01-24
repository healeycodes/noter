# 📝 noter

A text editor for macOS. Built using the [Ebiten](https://github.com/hajimehoshi/ebiten) game engine.

![A screenshot of the editor running. It looks like nano. It has a text file called "A Bird, came down the Walk" opened.](https://github.com/healeycodes/noter/blob/main/preview.png)

It's a little bit like `nano`.

## Development

Run the fonts build script `bash fonts.sh`

Run the editor `go run . -- "A Bird, came down the Walk.txt"`

## Build

`go build .`

## Tests

`go test ./...`

## Roadmap

- Scrolling improvements
  - If we're about to draw the cursor above/below the screen, then force a scroll (might cause a flicker)
  - Scroll into view when user types off-screen
