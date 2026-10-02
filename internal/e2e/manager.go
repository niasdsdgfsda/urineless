// Package e2e verschlüsselt Privatchats Ende-zu-Ende mit dem Signal-Protokoll
// (github.com/GoCodeAlone/libsignal-go: PQXDH + Double Ratchet).
// Alles läuft als normale PRIVMSG-Zeilen über IRC; der Server sieht nur Chiffretext.
package e2e

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/GoCodeAlone/libsignal-go/address"
	"github.com/GoCodeAlone/libsignal-go/protocol"
	"github.com/GoCodeAlone/libsignal-go/session"
	"github.com/GoCodeAlone/libsignal-go/stores"
	"github.com/GoCodeAlone/libsignal-go/stores/inmem"
)

const (
	maxPlain    = 4000
	maxPending  = 20
	requestWait = 20 * time.Second
)

// Result ist das Ergebnis beim Verarbeiten einer eingehenden E2E-Zeile.
type Result struct {
	Text    string   // entschlüsselter Klartext
	HasText bool     // true, wenn Text gültig ist
	Reply   []string // Zeilen, die an den Peer zu senden sind (Handshake, gepufferte Nachrichten)
	Note    string   // Hinweis für den Nutzer (Systemmeldung)
}

type assembly struct {
	total   int
	parts   map[int]string
	started time.Time
}

type Manager struct {
	mu      sync.Mutex
	st      *store
	pending map[string][]string // Peer -> Klartexte, die auf das Schlüsselpaket warten
	asked   map[string]time.Time
	asm     map[string]*assembly
	Group   *GroupManager
}

// Open öffnet (oder erzeugt) den verschlüsselten Schlüsselspeicher.
func Open(path, passphrase string) (*Manager, error) {
	if passphrase == "" {
		return nil, errors.New("leeres Passwort")
	}
	st, err := openStore(path, passphrase)
	if err != nil {
		return nil, err
	}
	return &Manager{
		st:      st,
		pending: map[string][]string{},
		asked:   map[string]time.Time{},
		asm:     map[string]*assembly{},
		Group:   NewGroupManager(),
	}, nil
}

func addr(peer string) address.ProtocolAddress {
	d, _ := address.NewDeviceID(deviceNum)
	return address.NewProtocolAddress(strings.ToLower(peer), d)
}

// ---- Senden ----

// Prepare liefert die IRC-Zeilen für eine ausgehende Nachricht.
// Gibt es noch keine Sitzung, wird der Text gepuffert und stattdessen
// eine Schlüsselanfrage erzeugt (nil, wenn bereits eine läuft).
func (m *Manager) Prepare(peer, text string) ([]string, error) {
	if len(text) > maxPlain {
		return nil, errors.New("Nachricht zu lang für E2E")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	ctx := context.Background()
	a := addr(peer)
	rec, err := m.st.LoadSession(ctx, a)
	if err != nil {
		return nil, err
	}
	if rec != nil && rec.HasCurrentState() {
		return m.encryptLocked(ctx, a, text)
	}
	key := strings.ToLower(peer)
	if len(m.pending[key]) < maxPending {
		m.pending[key] = append(m.pending[key], text)
	}
	if time.Since(m.asked[key]) < requestWait {
		return nil, nil
	}
	m.asked[key] = time.Now()
	return frame(kindRequest, nil), nil
}

func (m *Manager) encryptLocked(ctx context.Context, a address.ProtocolAddress, text string) ([]string, error) {
	sm, pm, err := session.Encrypt(ctx, []byte(text), a, m.st, m.st, nil, rand.Reader)
	if err != nil {
		return nil, err
	}
	if pm != nil {
		return frame(kindPreKey, pm.Serialize()), nil
	}
	return frame(kindSignal, sm.Serialize()), nil
}

// ---- Empfangen ----

// Receive verarbeitet eine eingehende E2E-Zeile (Teil einer Nachricht).
func (m *Manager) Receive(peer, line string) Result {
	m.mu.Lock()
	defer m.mu.Unlock()
	payload, ok := m.assemble(peer, line)
	if !ok {
		return Result{}
	}
	if len(payload) == 0 {
		return Result{}
	}
	ctx := context.Background()
	a := addr(peer)
	kind, body := payload[0], payload[1:]
	switch kind {
	case kindRequest:
		b, err := m.st.newBundle()
		if err != nil {
			return Result{Note: "E2E: Schlüsselpaket konnte nicht erzeugt werden: " + err.Error()}
		}
		return Result{Reply: frame(kindBundle, b)}
	case kindBundle:
		return m.onBundle(ctx, peer, a, body)
	case kindPreKey:
		return m.onPreKey(ctx, peer, a, body)
	case kindSignal:
		return m.onSignal(ctx, peer, a, body)
	}
	return Result{}
}

func (m *Manager) assemble(peer, line string) ([]byte, bool) {
	rest := strings.TrimPrefix(line, Prefix)
	f := strings.SplitN(rest, " ", 3)
	if len(f) != 3 {
		return nil, false
	}
	var idx, total int
	if _, err := fmt.Sscanf(f[1], "%d/%d", &idx, &total); err != nil ||
		total < 1 || total > maxChunks || idx < 1 || idx > total {
		return nil, false
	}
	now := time.Now()
	for k, as := range m.asm {
		if now.Sub(as.started) > 2*time.Minute {
			delete(m.asm, k)
		}
	}
	key := strings.ToLower(peer) + "/" + f[0]
	as := m.asm[key]
	if as == nil {
		as = &assembly{total: total, parts: map[int]string{}, started: now}
		m.asm[key] = as
	}
	if as.total != total {
		delete(m.asm, key)
		return nil, false
	}
	as.parts[idx] = f[2]
	if len(as.parts) < total {
		return nil, false
	}
	delete(m.asm, key)
	var sb strings.Builder
	for i := 1; i <= total; i++ {
		sb.WriteString(as.parts[i])
	}
	payload, err := base64.RawStdEncoding.DecodeString(sb.String())
	if err != nil {
		return nil, false
	}
	return payload, true
}

func (m *Manager) untrustedNote(peer string) string {
	return "WARNUNG: Der Schlüssel von " + peer + " hat sich geändert (neues Gerät, Neuinstallation – oder ein Angriff). " +
		"Prüfe die Sicherheitsnummer über einen anderen Kanal (/e2e verify " + peer + ") und bestätige dann mit /e2e trust " + peer + "."
}

func (m *Manager) onBundle(ctx context.Context, peer string, a address.ProtocolAddress, body []byte) Result {
	key := strings.ToLower(peer)
	if _, asked := m.asked[key]; !asked {
		return Result{Note: "E2E: unaufgefordertes Schlüsselpaket von " + peer + " ignoriert"}
	}
	b, err := parseBundle(body)
	if err != nil {
		return Result{Note: "E2E: ungültiges Schlüsselpaket von " + peer}
	}
	if err := session.ProcessPreKeyBundle(ctx, rand.Reader, a, b, m.st, m.st); err != nil {
		if errors.Is(err, session.ErrUntrustedIdentity) {
			return Result{Note: m.untrustedNote(peer)}
		}
		return Result{Note: "E2E: Handshake mit " + peer + " fehlgeschlagen: " + err.Error()}
	}
	delete(m.asked, key)

	var reply []string
	var note []string
	for _, t := range m.pending[key] {
		ls, err := m.encryptLocked(ctx, a, t)
		if err != nil {
			note = append(note, "E2E: Nachricht an "+peer+" nicht gesendet: "+err.Error())
			continue
		}
		reply = append(reply, ls...)
	}
	delete(m.pending, key)

	fp := ""
	if id, ok := m.st.d.Identities[key]; ok {
		fp = fingerprint(m.st.id.PublicKey.Serialize(), id)
	}
	note = append(note, "Verschlüsselte Sitzung mit "+peer+" aufgebaut. Sicherheitsnummer: "+fp+
		" – vergleiche sie über einen anderen Kanal, sonst könnte der Server mitlesen.")
	return Result{Reply: reply, Note: strings.Join(note, "\n")}
}

func (m *Manager) onPreKey(ctx context.Context, peer string, a address.ProtocolAddress, body []byte) Result {
	pk, err := protocol.DeserializePreKeySignalMessage(body)
	if err != nil {
		return Result{Note: "E2E: defekte Nachricht von " + peer}
	}
	trusted, err := m.st.IsTrustedIdentity(ctx, a, pk.IdentityKey(), stores.Receiving)
	if err != nil || !trusted {
		return Result{Note: m.untrustedNote(peer)}
	}
	rec, err := m.st.LoadSession(ctx, a)
	if err != nil {
		return Result{Note: "E2E: Sitzung von " + peer + " nicht lesbar: " + err.Error()}
	}
	base := pk.BaseKey().Serialize()
	var plain []byte
	if rec != nil && rec.HasCurrentState() && bytes.Equal(rec.CurrentState().AliceBaseKey(), base) {
		plain, err = session.Decrypt(ctx, pk.Message(), a, m.st, rand.Reader)
		if err != nil {
			return Result{Note: m.decryptNote(peer, err)}
		}
	} else {
		state, err := m.st.bobState(pk)
		if err != nil {
			return Result{Note: "E2E: keine passenden Schlüssel für " + peer + " (" + err.Error() + "). Sitzung zurücksetzen: /e2e reset " + peer}
		}
		state.SetAliceBaseKey(base)
		state.SetRemoteRegistrationID(pk.RegistrationID())
		tmp := inmem.NewSessionStore()
		if err := tmp.StoreSession(ctx, a, session.NewSessionRecord(state)); err != nil {
			return Result{Note: "E2E: " + err.Error()}
		}
		plain, err = session.Decrypt(ctx, pk.Message(), a, tmp, rand.Reader)
		if err != nil {
			return Result{Note: m.decryptNote(peer, err)}
		}
		got, err := tmp.LoadSession(ctx, a)
		if err != nil || got == nil {
			return Result{Note: "E2E: Sitzung konnte nicht übernommen werden"}
		}
		if rec == nil {
			rec = got
		} else if err := rec.PromoteState(got.CurrentState()); err != nil {
			return Result{Note: "E2E: " + err.Error()}
		}
		if err := m.st.StoreSession(ctx, a, rec); err != nil {
			return Result{Note: "E2E: " + err.Error()}
		}
		_ = m.st.consume(pk)
	}
	_, _ = m.st.SaveIdentity(ctx, a, pk.IdentityKey())

	res := Result{Text: string(plain), HasText: true}
	if rec == nil || rec.PreviousSessionCount() == 0 {
		if id, ok := m.st.d.Identities[strings.ToLower(peer)]; ok {
			res.Note = "Verschlüsselte Sitzung mit " + peer + ". Sicherheitsnummer: " +
				fingerprint(m.st.id.PublicKey.Serialize(), id)
		}
	}
	return res
}

func (m *Manager) onSignal(ctx context.Context, peer string, a address.ProtocolAddress, body []byte) Result {
	sm, err := protocol.DeserializeSignalMessage(body)
	if err != nil {
		return Result{Note: "E2E: defekte Nachricht von " + peer}
	}
	plain, err := session.Decrypt(ctx, sm, a, m.st, rand.Reader)
	if err != nil {
		return Result{Note: m.decryptNote(peer, err)}
	}
	return Result{Text: string(plain), HasText: true}
}

func (m *Manager) decryptNote(peer string, err error) string {
	switch {
	case errors.Is(err, session.ErrSessionNotFound):
		return "E2E: Nachricht von " + peer + " nicht entschlüsselbar (keine Sitzung). Zurücksetzen: /e2e reset " + peer
	case errors.Is(err, session.ErrDuplicateMessage):
		return ""
	default:
		return "E2E: Nachricht von " + peer + " nicht entschlüsselbar (" + err.Error() + "). Zurücksetzen: /e2e reset " + peer
	}
}

// ---- Verwaltung ----

// Fingerprint liefert die Sicherheitsnummer für den Peer.
func (m *Manager) Fingerprint(peer string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.st.d.Identities[strings.ToLower(peer)]
	if !ok {
		return "", errors.New("noch keine verschlüsselte Sitzung mit " + peer)
	}
	return fingerprint(m.st.id.PublicKey.Serialize(), id), nil
}

// Reset verwirft die Sitzung und fordert ein neues Schlüsselpaket an (Zeilen für den Peer).
func (m *Manager) Reset(peer string) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := strings.ToLower(peer)
	delete(m.st.d.Sessions, key)
	delete(m.pending, key)
	_ = m.st.save()
	m.asked[key] = time.Now()
	return frame(kindRequest, nil)
}

// Trust akzeptiert einen geänderten Schlüssel: Identität und Sitzung werden verworfen,
// der nächste Handshake übernimmt den neuen Schlüssel.
func (m *Manager) Trust(peer string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := strings.ToLower(peer)
	delete(m.st.d.Identities, key)
	delete(m.st.d.Sessions, key)
	_ = m.st.save()
}
