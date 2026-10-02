package e2e

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Gruppenverschlüsselung per "Sender Keys":
//
//   - Jeder Teilnehmer hat pro Kanal einen eigenen zufälligen AES-256-Schlüssel.
//   - Nachrichten gehen als "GRP1 <id> <i>/<n> <chunk>" in den Kanal (AES-GCM).
//   - Den Schlüssel bekommt man NIE im Kanal, sondern nur über die paarweise
//     Signal-Sitzung (PQXDH + Double Ratchet) als "SKEY1 <kanal> <key>".
//   - Verteilung per Pull: Wer eine GRP1-Nachricht nicht lesen kann, schickt dem
//     Absender privat "SKREQ1 <kanal>" und bekommt den Schlüssel zurück.
//     So wird kein Unbeteiligter im Kanal mit Handshakes zugespammt.
//   - Verlässt jemand den Kanal, wird der eigene Schlüssel erneuert (Rotate).

const (
	GroupPrefix = "GRP1 "
	SKeyPrefix  = "SKEY1 "
	SKReqPrefix = "SKREQ1 "

	groupKeyLen = 32
	maxWaiting  = 20
	keyAskWait  = 30 * time.Second
)

// GroupResult ist das Ergebnis beim Verarbeiten einer eingehenden GRP1-Zeile.
type GroupResult struct {
	Text    string // entschlüsselter Klartext
	HasText bool
	Note    string // Hinweis für den Nutzer
	NeedKey bool   // true: dem Absender SKREQ1 schicken
}

type groupAsm struct {
	total   int
	parts   map[int]string
	started time.Time
}

type GroupManager struct {
	mu       sync.Mutex
	myKeys   map[string][]byte            // kanal(klein) -> mein Sender-Key
	peerKeys map[string]map[string][]byte // kanal(klein) -> nick(klein) -> Sender-Key
	asm      map[string]*groupAsm
	waiting  map[string][][]byte  // kanal/nick -> Chiffretexte, die auf den Schlüssel warten
	asked    map[string]time.Time // kanal/nick -> letzte Schlüsselanfrage
}

func NewGroupManager() *GroupManager {
	return &GroupManager{
		myKeys:   map[string][]byte{},
		peerKeys: map[string]map[string][]byte{},
		asm:      map[string]*groupAsm{},
		waiting:  map[string][][]byte{},
		asked:    map[string]time.Time{},
	}
}

func lc(s string) string { return strings.ToLower(s) }

func groupAAD(channel string) []byte { return []byte("urineless-grp1|" + lc(channel)) }

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// GetOrGenMyKey liefert meinen Sender-Key für den Kanal (wird bei Bedarf erzeugt).
func (g *GroupManager) GetOrGenMyKey(channel string) []byte {
	g.mu.Lock()
	defer g.mu.Unlock()
	ch := lc(channel)
	key, ok := g.myKeys[ch]
	if !ok {
		key = make([]byte, groupKeyLen)
		_, _ = rand.Read(key)
		g.myKeys[ch] = key
	}
	return append([]byte(nil), key...)
}

// Rotate verwirft meinen Schlüssel; der nächste Versand erzeugt einen neuen.
func (g *GroupManager) Rotate(channel string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.myKeys, lc(channel))
}

// ForgetPeer vergisst den Schlüssel eines Teilnehmers (z. B. nach PART/QUIT).
func (g *GroupManager) ForgetPeer(channel, nick string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	ch, n := lc(channel), lc(nick)
	delete(g.peerKeys[ch], n)
	delete(g.waiting, ch+"/"+n)
	delete(g.asked, ch+"/"+n)
}

// RenamePeer überträgt gespeicherte Schlüssel bei einem Nickwechsel.
func (g *GroupManager) RenamePeer(oldNick, newNick string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	o, n := lc(oldNick), lc(newNick)
	for _, m := range g.peerKeys {
		if k, ok := m[o]; ok {
			m[n] = k
			delete(m, o)
		}
	}
}

// StorePeerKey speichert den Sender-Key von nick und liefert Nachrichten,
// die bisher nicht entschlüsselt werden konnten.
func (g *GroupManager) StorePeerKey(channel, nick string, key []byte) []string {
	if len(key) != groupKeyLen {
		return nil
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	ch, n := lc(channel), lc(nick)
	if g.peerKeys[ch] == nil {
		g.peerKeys[ch] = map[string][]byte{}
	}
	g.peerKeys[ch][n] = append([]byte(nil), key...)
	ck := ch + "/" + n
	var out []string
	for _, ct := range g.waiting[ck] {
		if pt, err := openGroup(key, channel, ct); err == nil {
			out = append(out, pt)
		}
	}
	delete(g.waiting, ck)
	delete(g.asked, ck)
	return out
}

// ---- Senden ----

// Encrypt verschlüsselt text und liefert die fertigen IRC-Zeilen (Chunks).
func (g *GroupManager) Encrypt(channel, plaintext string) ([]string, error) {
	if len(plaintext) > maxPlain {
		return nil, errors.New("Nachricht zu lang für E2E")
	}
	gcm, err := newGCM(g.GetOrGenMyKey(channel))
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	raw := gcm.Seal(nonce, nonce, []byte(plaintext), groupAAD(channel))
	enc := base64.RawStdEncoding.EncodeToString(raw)

	var idb [2]byte
	_, _ = rand.Read(idb[:])
	id := fmt.Sprintf("%x", idb[:])
	total := (len(enc) + chunkSize - 1) / chunkSize
	if total > maxChunks {
		return nil, errors.New("Nachricht zu lang für E2E")
	}
	lines := make([]string, 0, total)
	for i := 0; i < total; i++ {
		lo, hi := i*chunkSize, min((i+1)*chunkSize, len(enc))
		lines = append(lines, fmt.Sprintf("%s%s %d/%d %s", GroupPrefix, id, i+1, total, enc[lo:hi]))
	}
	return lines, nil
}

// ---- Empfangen ----

// Receive verarbeitet eine eingehende GRP1-Zeile aus dem Kanal.
func (g *GroupManager) Receive(channel, nick, line string) GroupResult {
	g.mu.Lock()
	defer g.mu.Unlock()
	ct, ok := g.assemble(channel, nick, line)
	if !ok {
		return GroupResult{}
	}
	ch, n := lc(channel), lc(nick)
	if key := g.peerKeys[ch][n]; key != nil {
		if pt, err := openGroup(key, channel, ct); err == nil {
			return GroupResult{Text: pt, HasText: true}
		}
	}
	// Kein oder veralteter Schlüssel: Chiffretext merken, Schlüssel anfragen.
	ck := ch + "/" + n
	if len(g.waiting[ck]) < maxWaiting {
		g.waiting[ck] = append(g.waiting[ck], ct)
	}
	if time.Since(g.asked[ck]) < keyAskWait {
		return GroupResult{}
	}
	g.asked[ck] = time.Now()
	return GroupResult{NeedKey: true, Note: "Gruppenschlüssel von " + nick + " wird angefragt …"}
}

func (g *GroupManager) assemble(channel, nick, line string) ([]byte, bool) {
	f := strings.SplitN(strings.TrimPrefix(line, GroupPrefix), " ", 3)
	if len(f) != 3 {
		return nil, false
	}
	var idx, total int
	if _, err := fmt.Sscanf(f[1], "%d/%d", &idx, &total); err != nil ||
		total < 1 || total > maxChunks || idx < 1 || idx > total {
		return nil, false
	}
	now := time.Now()
	for k, as := range g.asm {
		if now.Sub(as.started) > 2*time.Minute {
			delete(g.asm, k)
		}
	}
	key := lc(channel) + "/" + lc(nick) + "/" + f[0]
	as := g.asm[key]
	if as == nil {
		as = &groupAsm{total: total, parts: map[int]string{}, started: now}
		g.asm[key] = as
	}
	if as.total != total {
		delete(g.asm, key)
		return nil, false
	}
	as.parts[idx] = f[2]
	if len(as.parts) < total {
		return nil, false
	}
	delete(g.asm, key)
	var sb strings.Builder
	for i := 1; i <= total; i++ {
		sb.WriteString(as.parts[i])
	}
	raw, err := base64.RawStdEncoding.DecodeString(sb.String())
	if err != nil {
		return nil, false
	}
	return raw, true
}

func openGroup(key []byte, channel string, raw []byte) (string, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return "", err
	}
	ns := gcm.NonceSize()
	if len(raw) < ns {
		return "", errors.New("ungültige Nachricht")
	}
	pt, err := gcm.Open(nil, raw[:ns], raw[ns:], groupAAD(channel))
	if err != nil {
		return "", err
	}
	return string(pt), nil
}

// ---- Steuernachrichten (laufen nur durch die paarweise E2E-Sitzung) ----

func FormatSKey(channel string, key []byte) string {
	return SKeyPrefix + channel + " " + base64.RawStdEncoding.EncodeToString(key)
}

func ParseSKey(text string) (channel string, key []byte, ok bool) {
	if !strings.HasPrefix(text, SKeyPrefix) {
		return "", nil, false
	}
	f := strings.Fields(strings.TrimPrefix(text, SKeyPrefix))
	if len(f) != 2 {
		return "", nil, false
	}
	k, err := base64.RawStdEncoding.DecodeString(f[1])
	if err != nil || len(k) != groupKeyLen {
		return "", nil, false
	}
	return f[0], k, true
}

func FormatSKReq(channel string) string { return SKReqPrefix + channel }

func ParseSKReq(text string) (channel string, ok bool) {
	if !strings.HasPrefix(text, SKReqPrefix) {
		return "", false
	}
	f := strings.Fields(strings.TrimPrefix(text, SKReqPrefix))
	if len(f) != 1 {
		return "", false
	}
	return f[0], true
}
