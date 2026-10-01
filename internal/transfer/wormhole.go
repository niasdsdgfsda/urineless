// Package transfer verschickt und empfängt Bilder über Magic-Wormhole.
// Empfangene Daten werden nur im Speicher gehalten (nie auf die Platte geschrieben),
// auf Größe begrenzt und als Bild dekodiert, bevor sie angezeigt werden.
package transfer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"

	"github.com/psanford/wormhole-william/wormhole"
)

const (
	MaxBytes  = 10 << 20   // 10 MB
	maxPixels = 40_000_000 // Schutz vor Dekompressionsbomben
)

// DecodeImage prüft Größe und Abmessungen und dekodiert PNG/JPEG/GIF.
func DecodeImage(data []byte) (image.Image, error) {
	if len(data) > MaxBytes {
		return nil, errors.New("Datei zu groß (max. 10 MB)")
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, errors.New("keine unterstützte Bilddatei (PNG, JPEG, GIF)")
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width*cfg.Height > maxPixels {
		return nil, errors.New("Bildabmessungen zu groß")
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("Bild defekt: %w", err)
	}
	return img, nil
}

// Send bietet die Datei über Wormhole an und liefert den Code sofort zurück.
// Der Kanal done meldet später das Ergebnis der Übertragung (nil = zugestellt).
func Send(ctx context.Context, name string, data []byte) (code string, done <-chan error, err error) {
	var c wormhole.Client
	code, status, err := c.SendFile(ctx, name, bytes.NewReader(data))
	if err != nil {
		return "", nil, err
	}
	ch := make(chan error, 1)
	go func() {
		res := <-status
		switch {
		case res.OK:
			ch <- nil
		case res.Error != nil:
			ch <- res.Error
		default:
			ch <- errors.New("Übertragung abgebrochen")
		}
	}()
	return code, ch, nil
}

// Receive holt die Datei zum Code ab und dekodiert sie als Bild.
func Receive(ctx context.Context, code string) (image.Image, error) {
	var c wormhole.Client
	msg, err := c.Receive(ctx, code)
	if err != nil {
		return nil, err
	}
	if msg.Type != wormhole.TransferFile {
		msg.Reject()
		return nil, errors.New("kein Datei-Transfer")
	}
	if msg.TransferBytes64 > MaxBytes {
		msg.Reject()
		return nil, errors.New("Datei zu groß (max. 10 MB)")
	}
	data, err := io.ReadAll(io.LimitReader(msg, MaxBytes+1))
	if err != nil {
		return nil, err
	}
	return DecodeImage(data)
}
