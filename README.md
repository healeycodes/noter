# 📝 noter

A text editor for macOS. Built using the [Ebiten](https://github.com/hajimehoshi/ebiten) game engine.

`<image here>`

It's a bit like `nano`.

## Development

Grab the fonts from: https://github.com/TakWolf/ark-pixel-font. Move the directory at `assets/glyphs/12/monospaced` to be inside `./fonts` e.g. the following file should now exist: `./fonts/monospaced/0000-007F Basic Latin/0021.png`.

Run the font build script `python3 build_fonts.py`

Run the editor `go run . -- some_file.txt`

## Tests

`go test ./...`

## Roadmap

- Scrolling improvements? Maybe add some "intelligence" here.
  - Rather than manual scrolling, let's scroll the page when the cursor goes under, or above, the view
  - Scroll into view when user types off-screen
