// Copyright (c) 2024 Andrew Healey
//
// Example of using Editor in an ebiten application.

package main

import (
	"fmt"
	"log"
	"io"
	"os"
	"path"

    "github.com/healeycodes/noter"
	"github.com/hajimehoshi/ebiten/v2"
	"golang.design/x/clipboard"
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

func main() {
	var filePath string
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

	content := &fileContent{FilePath: filePath}
	editor := &noter.Editor{
		Clipboard: &clipBoard{}, // Use system clipboard.
		Content:   content,
		FileName:  content.FileName(),
	}

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
