// Copyright (c) 2024 Andrew Healey
//
// Example of using Editor in an ebiten application.

package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"

	"github.com/flopp/go-findfont"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/healeycodes/noter"
	"golang.design/x/clipboard"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

type clipBoard struct {
}

func (cb *clipBoard) ReadText() []byte {
	return clipboard.Read(clipboard.FmtText)
}

func (cb *clipBoard) WriteText(content []byte) {
	clipboard.Write(clipboard.FmtText, content)
}

type fileContent struct {
	FilePath string
}

func (fc *fileContent) FileName() (name string) {
	return path.Base(fc.FilePath)
}

func (fc *fileContent) ReadText() (content []byte) {
	file, err := os.Open(fc.FilePath)
	if err != nil {
		// It's ok if the file does not (yet) exist.
		return
	}
	defer file.Close()

	content, err = io.ReadAll(file)
	if err != nil {
		panic(err)
	}

	return
}

func (fc *fileContent) WriteText(content []byte) {
	file, err := os.Create(fc.FilePath)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	_, err = file.Write(content)
	if err != nil {
		panic(err)
	}
}

type options struct {
	font_name string
	font_size float64
	font_dpi  float64
}

func init() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of noter:\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "noter [flags] <filename>\n")
		flag.PrintDefaults()
	}
}

func execute(file_path string, opts *options) (err error) {
	var font_face font.Face

	if len(opts.font_name) > 0 {
		var font_path string
		font_path, err = findfont.Find(opts.font_name)
		if err != nil {
			return
		}

		var font_data []byte
		font_data, err = ioutil.ReadFile(font_path)
		if err != nil {
			return
		}

		var font_sfnt *opentype.Font
		font_sfnt, err = opentype.Parse(font_data)
		if err != nil {
			return
		}

		font_opts := opentype.FaceOptions{
			Size: opts.font_size,
			DPI:  opts.font_dpi,
		}
		font_face, err = opentype.NewFace(font_sfnt, &font_opts)
		if err != nil {
			return
		}
		defer font_face.Close()
	}

	content := &fileContent{FilePath: file_path}

	editor := noter.NewEditor(
		noter.WithClipboard(&clipBoard{}),
		noter.WithContent(content),
		noter.WithContentName(content.FileName()),
		noter.WithTopBar(true),
		noter.WithBottomBar(true),
		noter.WithFontFace(font_face),
	)

	width, height := editor.Size()
	ebiten.SetWindowSize(width, height)
	ebiten.SetWindowTitle("noter")
	if err = ebiten.RunGame(editor); err != nil {
		return
	}

	return
}

func main() {
	var opts options

	flag.StringVar(&opts.font_name, "font", "", "TrueType font name")
	flag.Float64Var(&opts.font_size, "fontsize", 12.0, "Font size")
	flag.Float64Var(&opts.font_dpi, "fontdpi", 96.0, "Font DPI")

	flag.Parse()

	var filePath string
	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	} else {
		// This is the way
		filePath = flag.Arg(0)
	}

	err := execute(filePath, &opts)

	if err != nil {
		panic(err)
	}
}
