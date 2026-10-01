package chat

import (
	"image"
	"path"
	"regexp"
	"strings"
)

type AttachState int

const (
	AttIdle      AttachState = iota // eingehend, noch nicht abgeholt
	AttLoading                      // eingehend, wird abgeholt
	AttReady                        // eingehend, Bild liegt vor
	AttWaiting                      // ausgehend, wartet auf Abholung
	AttDelivered                    // ausgehend, Empfänger hat es abgeholt
	AttFailed
)

// Attachment ist ein per Wormhole übertragenes Bild.
// Wird immer als Zeiger weitergereicht, damit Statusänderungen in der UI sichtbar werden.
type Attachment struct {
	Code     string
	Name     string
	Outgoing bool
	State    AttachState
	Err      string
	Image    image.Image
}

// Wire-Format in der IRC-Nachricht (für andere Clients lesbar):
//
//	[wormhole] 7-crossover-clockwork foto.png
var attachRe = regexp.MustCompile(`^\[wormhole\] (\d+(?:-[a-z]+)+) (.{1,200})$`)

func FormatAttachment(code, name string) string {
	return "[wormhole] " + code + " " + SafeName(name)
}

// ParseAttachment erkennt eine Wormhole-Nachricht, sonst nil.
func ParseAttachment(text string) *Attachment {
	m := attachRe.FindStringSubmatch(strings.TrimSpace(text))
	if m == nil {
		return nil
	}
	return &Attachment{Code: m[1], Name: SafeName(m[2]), State: AttIdle}
}

// SafeName entfernt Pfade und Steuerzeichen; Dateinamen kommen von Fremden.
func SafeName(n string) string {
	n = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, n)
	n = path.Base(strings.ReplaceAll(n, "\\", "/"))
	if n == "." || n == "/" || n == "" {
		return "bild"
	}
	if r := []rune(n); len(r) > 100 {
		n = string(r[:100])
	}
	return n
}
