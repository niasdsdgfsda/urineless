package chat

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// stickerDir liefert den lokalen Sticker-Ordner (wird beim ersten Zugriff erstellt).
func stickerDir() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	p := filepath.Join(dir, "urineless", "stickers")
	_ = os.MkdirAll(p, 0o755)
	return p
}

// StickerDir ist der primäre Ordner (für /sticker help).
func StickerDir() string { return stickerDir() }

// Sticker ist ein lokaler Sticker mit geladenem Vorschaubild.
type Sticker struct {
	Name     string
	Path     string
	Image    image.Image
	Animated *gif.GIF
	Frames   []image.Image
}

// ResolveSticker findet eine Sticker-Datei anhand eines Namens.
// Sucht: <config>/urineless/stickers/<name>.{png,jpg,jpeg,gif}
// sowie ./stickers/<name>.{...} im Arbeitsverzeichnis.
func ResolveSticker(name string) (string, bool) {
	name = strings.Trim(strings.TrimSpace(name), ":/ ")
	if name == "" || strings.ContainsAny(name, `/\`) {
		return "", false
	}
	exts := []string{".png", ".jpg", ".jpeg", ".gif"}
	dirs := []string{stickerDir(), "stickers"}
	for _, d := range dirs {
		for _, e := range exts {
			p := filepath.Join(d, name+e)
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				return p, true
			}
		}
	}
	return "", false
}

// StickerNames listet alle verfügbaren Sticker-Namen (ohne Endung).
func StickerNames() []string {
	seen := map[string]bool{}
	var out []string
	for _, d := range []string{stickerDir(), "stickers"} {
		ents, err := os.ReadDir(d)
		if err != nil {
			continue
		}
		for _, e := range ents {
			if e.IsDir() {
				continue
			}
			n := e.Name()
			ext := strings.ToLower(filepath.Ext(n))
			if ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".gif" {
				continue
			}
			base := strings.TrimSuffix(n, ext)
			if !seen[base] {
				seen[base] = true
				out = append(out, base)
			}
		}
	}
	sort.Strings(out)
	return out
}

// StickerList lädt alle lokalen Sticker inkl. Vorschaubild.
// Fehlerhafte Dateien werden übersprungen.
func StickerList() []Sticker {
	var out []Sticker
	for _, name := range StickerNames() {
		p, ok := ResolveSticker(name)
		if !ok {
			continue
		}
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		data, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			continue
		}
		img, animated, frames := decodeImage(data)
		if img == nil {
			continue
		}
		out = append(out, Sticker{Name: name, Path: p, Image: img, Animated: animated, Frames: frames})
	}
	return out
}

func decodeImage(data []byte) (image.Image, *gif.GIF, []image.Image) {
	if g, err := gif.DecodeAll(bytes.NewReader(data)); err == nil && len(g.Image) > 0 {
		if len(g.Image) == 1 {
			return g.Image[0], nil, nil
		}
		frames := composeGIF(g)
		return frames[0], g, frames
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err == nil {
		return img, nil, nil
	}
	return nil, nil, nil
}

func composeGIF(g *gif.GIF) []image.Image {
	if len(g.Image) == 0 {
		return nil
	}
	bounds := g.Image[0].Bounds()
	if bounds.Empty() {
		bounds = image.Rect(0, 0, g.Config.Width, g.Config.Height)
	}
	if bounds.Empty() {
		bounds = image.Rect(0, 0, 100, 100)
	}

	out := make([]image.Image, len(g.Image))
	canvas := image.NewRGBA(bounds)
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{color.Transparent}, image.Point{}, draw.Src)

	var prevCanvas *image.RGBA

	for i, srcImg := range g.Image {
		if i > 0 && len(g.Disposal) > i-1 && g.Disposal[i-1] == gif.DisposalPrevious {
			if prevCanvas != nil {
				draw.Draw(canvas, canvas.Bounds(), prevCanvas, image.Point{}, draw.Src)
			}
		} else {
			prevCanvas = image.NewRGBA(bounds)
			draw.Draw(prevCanvas, canvas.Bounds(), canvas, image.Point{}, draw.Src)
		}

		draw.Draw(canvas, srcImg.Bounds(), srcImg, srcImg.Bounds().Min, draw.Over)

		frame := image.NewRGBA(bounds)
		draw.Draw(frame, bounds, canvas, image.Point{}, draw.Src)
		out[i] = frame

		if len(g.Disposal) > i {
			switch g.Disposal[i] {
			case gif.DisposalBackground:
				draw.Draw(canvas, srcImg.Bounds(), &image.Uniform{color.Transparent}, image.Point{}, draw.Src)
			case gif.DisposalPrevious:
				// handled in next iteration
			}
		}
	}
	return out
}
