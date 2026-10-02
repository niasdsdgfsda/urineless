package chat

import (
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
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
	Name  string
	Path  string
	Image image.Image
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
		img, _, err := image.Decode(f)
		f.Close()
		if err != nil {
			continue
		}
		out = append(out, Sticker{Name: name, Path: p, Image: img})
	}
	return out
}
