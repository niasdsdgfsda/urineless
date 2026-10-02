package ui

import (
	"bytes"
	"context"
	"errors"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"golang.design/x/clipboard"
)

var clipboardReady bool

// InitClipboard sollte einmal beim Start aufgerufen werden.
func InitClipboard() error {
	if err := clipboard.Init(); err != nil {
		return err
	}
	clipboardReady = true
	return nil
}

// ClipboardImage liest ein Bild aus dem Clipboard. ok=false wenn keins drin ist.
func ClipboardImage() (image.Image, []byte, bool) {
	if !clipboardReady {
		return nil, nil, false
	}
	data, err := clipboard.Read(context.Background(), clipboard.FmtImage)
	if err != nil {
		if errors.Is(err, clipboard.ErrNoData) {
			return nil, nil, false
		}
		return nil, nil, false
	}
	if len(data) == 0 {
		return nil, nil, false
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, nil, false
	}
	return img, data, true
}
