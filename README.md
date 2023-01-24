# 📝 noter

A text editor for macOS. Built using the [Ebiten](https://github.com/hajimehoshi/ebiten) game engine.

![A screenshot of the editor running. It looks like nano. It has a text file called "A Bird, came down the Walk" opened.](https://github.com/healeycodes/noter/blob/main/preview.png)

It's a bit like `nano`.

## Development

Run the fonts build script `bash fonts.sh`

Run the editor `go run . -- some_file.txt`

## Tests

`go test ./...`

## Roadmap

- Scrolling improvements? Maybe add some "intelligence" here.
  - Rather than manual scrolling, let's scroll the page when the cursor goes under, or above, the view
  - Scroll into view when user types off-screen
