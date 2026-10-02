package ui

import (
	"ircgram/internal/nekos"
)

// newNekosClient baut einen Client (kein Key nötig).
func newNekosClient() *nekos.Client {
	return nekos.NewClient()
}
