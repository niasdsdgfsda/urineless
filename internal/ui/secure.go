package ui

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"ircgram/internal/chat"
	"ircgram/internal/e2e"
	"ircgram/internal/irc"
)

func e2ePath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return filepath.Join(dir, "urineless", "e2e.keys")
}

// secure meldet, ob Nachrichten in conv Ende-zu-Ende verschlüsselt werden (Privat oder Kanal).
func (a *App) secure(conv *chat.Conversation) bool {
	return a.e2e != nil && !conv.NoE2E && (conv.Kind == chat.Private || conv.Kind == chat.Channel)
}

// enqueue reiht Netzwerkarbeit in einen einzigen Worker ein. Das hält die
// Reihenfolge der Nachrichten stabil und blockiert nie (Aufruf mit gehaltenem Lock ist ok).
func (a *App) enqueue(job func()) {
	select {
	case a.jobs <- job:
	default:
		go job()
	}
}

// deliver sendet text an den Chat; verschlüsselt für Privatchats und Kanäle.
// Wird mit gehaltenem Lock aufgerufen.
func (a *App) deliver(conv *chat.Conversation, client *irc.Client, text string) {
	target := conv.Name

	if !a.secure(conv) {
		a.enqueue(func() { client.Privmsg(target, text) })
		return
	}

	// 1-zu-1: Signal-Sitzung (PQXDH + Double Ratchet)
	if conv.Kind == chat.Private {
		mgr := a.e2e
		a.enqueue(func() {
			lines, err := mgr.Prepare(target, text)
			if err != nil {
				a.note(conv, "Verschlüsselung fehlgeschlagen: "+err.Error())
				return
			}
			a.sendLines(client, target, lines)
		})
		return
	}

	// Kanal: Sender Key (AES-GCM). Den Schlüssel holen sich die Mitglieder
	// bei Bedarf per SKREQ1 über ihre Signal-Sitzung zu mir.
	g := a.e2e.Group
	a.enqueue(func() {
		lines, err := g.Encrypt(target, text)
		if err != nil {
			a.note(conv, "Gruppen-Verschlüsselung fehlgeschlagen: "+err.Error())
			return
		}
		a.sendLines(client, target, lines)
	})
}

// sendEncrypted schickt text privat an peer durch die Signal-Sitzung (Handshake automatisch).
func (a *App) sendEncrypted(c *irc.Client, peer, text string) {
	mgr := a.e2e
	if mgr == nil {
		return
	}
	a.enqueue(func() {
		lines, err := mgr.Prepare(peer, text)
		if err != nil {
			return
		}
		a.sendLines(c, peer, lines)
	})
}

// sendLines schickt IRC-Zeilen mit kleiner Pause, damit Server kein Flood-Limit auslösen.
func (a *App) sendLines(client *irc.Client, peer string, lines []string) {
	for i, l := range lines {
		if i > 0 {
			time.Sleep(300 * time.Millisecond)
		}
		client.Privmsg(peer, l)
	}
}

// handleControl verarbeitet Steuernachrichten, die durch die Signal-Sitzung kamen.
// Gibt true zurück, wenn text eine Steuernachricht war (dann nicht anzeigen).
func (a *App) handleControl(c *irc.Client, nick, text string) bool {
	s := a.store
	if ch, key, ok := e2e.ParseSKey(text); ok {
		if conv := s.Find(ch); conv != nil && conv.Kind == chat.Channel {
			for _, t := range a.e2e.Group.StorePeerKey(ch, nick, key) {
				a.post(c, conv, nick, t, time.Time{}, true)
			}
		}
		return true
	}
	if ch, ok := e2e.ParseSKReq(text); ok {
		// Schlüssel nur an echte Kanalmitglieder herausgeben.
		if conv := s.Find(ch); conv != nil && conv.Kind == chat.Channel && conv.HasMember(nick) && a.secure(conv) {
			a.sendEncrypted(c, nick, e2e.FormatSKey(conv.Name, a.e2e.Group.GetOrGenMyKey(conv.Name)))
		}
		return true
	}
	return false
}

// rekey: Wenn jemand den Kanal verlässt, bekommt er keine neuen Schlüssel mehr.
func (a *App) rekey(conv *chat.Conversation, nick string) {
	if a.e2e == nil || conv.Kind != chat.Channel {
		return
	}
	a.e2e.Group.ForgetPeer(conv.Name, nick)
	a.e2e.Group.Rotate(conv.Name)
}

func peerArg(conv *chat.Conversation, args []string) string {
	if len(args) > 1 {
		return args[1]
	}
	if conv.Kind == chat.Private {
		return conv.Name
	}
	return ""
}

// e2eCommand verarbeitet /e2e <status|on|off|verify|reset|trust|rotate> [nick].
func (a *App) e2eCommand(conv *chat.Conversation, args []string) {
	s := a.store
	if len(args) == 0 {
		args = []string{"status"}
	}
	sub := strings.ToLower(args[0])
	if a.e2e == nil {
		s.Sys(conv, "E2E ist nicht aktiv.")
		return
	}
	switch sub {
	case "status":
		state := "an"
		if !a.secure(conv) {
			state = "AUS"
		}
		s.Sys(conv, "E2E ist hier "+state+". Befehle: /e2e on|off, verify [nick], reset [nick], trust [nick], rotate")
	case "on":
		conv.NoE2E = false
		s.Sys(conv, "E2E ist hier an.")
	case "off":
		conv.NoE2E = true
		s.Sys(conv, "ACHTUNG: Nachrichten in diesem Chat werden unverschlüsselt gesendet.")
	case "rotate":
		if conv.Kind != chat.Channel {
			s.Sys(conv, "Nur in Kanälen: /e2e rotate")
			return
		}
		a.e2e.Group.Rotate(conv.Name)
		s.Sys(conv, "Gruppenschlüssel erneuert. Mitglieder holen sich den neuen Schlüssel automatisch.")
	case "verify", "reset", "trust":
		peer := peerArg(conv, args)
		if peer == "" {
			s.Sys(conv, "Benutzung: /e2e "+sub+" nick")
			return
		}
		switch sub {
		case "verify":
			fp, err := a.e2e.Fingerprint(peer)
			if err != nil {
				s.Sys(conv, err.Error())
				return
			}
			s.Sys(conv, "Sicherheitsnummer mit "+peer+": "+fp)
		case "reset":
			lines := a.e2e.Reset(peer)
			client := a.client
			a.enqueue(func() { a.sendLines(client, peer, lines) })
			s.Sys(conv, "Sitzung mit "+peer+" zurückgesetzt.")
		case "trust":
			a.e2e.Trust(peer)
			s.Sys(conv, "Schlüssel von "+peer+" als vertrauenswürdig markiert.")
		}
	default:
		s.Sys(conv, "Unbekannt. Befehle: /e2e status|on|off|verify|reset|trust|rotate")
	}
}
