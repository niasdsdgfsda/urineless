package e2e

import (
	"path/filepath"
	"strings"
	"testing"
)

// feed gibt alle Zeilen an m weiter und fasst die Ergebnisse zusammen.
func feed(m *Manager, from string, lines []string) Result {
	var out Result
	for _, l := range lines {
		r := m.Receive(from, l)
		out.Reply = append(out.Reply, r.Reply...)
		if r.HasText {
			out.Text, out.HasText = r.Text, true
		}
		if r.Note != "" {
			out.Note += r.Note + "\n"
		}
	}
	return out
}

func TestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	alice, err := Open(filepath.Join(dir, "a.keys"), "pw-a")
	if err != nil {
		t.Fatal(err)
	}
	bob, err := Open(filepath.Join(dir, "b.keys"), "pw-b")
	if err != nil {
		t.Fatal(err)
	}

	// 1. Alice schreibt Bob ohne Sitzung -> Schlüsselanfrage
	req, err := alice.Prepare("bob", "hallo bob")
	if err != nil || len(req) == 0 {
		t.Fatalf("keine Anfrage: %v %v", req, err)
	}
	for _, l := range req {
		if len(l) > 400 {
			t.Fatalf("Zeile zu lang: %d", len(l))
		}
	}
	// 2. Bob antwortet mit Schlüsselpaket
	r := feed(bob, "alice", req)
	if len(r.Reply) == 0 {
		t.Fatal("Bob hat kein Schlüsselpaket gesendet")
	}
	// 3. Alice baut die Sitzung auf und sendet die gepufferte Nachricht
	r = feed(alice, "bob", r.Reply)
	if len(r.Reply) == 0 || !strings.Contains(r.Note, "Sicherheitsnummer") {
		t.Fatalf("Alice: keine Nachricht/Sicherheitsnummer: %+v", r)
	}
	// 4. Bob entschlüsselt
	r = feed(bob, "alice", r.Reply)
	if !r.HasText || r.Text != "hallo bob" {
		t.Fatalf("Bob hat %q erhalten (%+v)", r.Text, r)
	}
	// 5. Bob antwortet, Alice entschlüsselt
	lines, err := bob.Prepare("alice", "hi alice")
	if err != nil {
		t.Fatal(err)
	}
	r = feed(alice, "bob", lines)
	if !r.HasText || r.Text != "hi alice" {
		t.Fatalf("Alice hat %q erhalten (%+v)", r.Text, r)
	}
	// 6. noch eine Runde (Ratchet)
	lines, _ = alice.Prepare("bob", "wormhole 7-test-code")
	r = feed(bob, "alice", lines)
	if !r.HasText || r.Text != "wormhole 7-test-code" {
		t.Fatalf("Bob hat %q erhalten (%+v)", r.Text, r)
	}
	// 7. beide sehen dieselbe Sicherheitsnummer
	fa, _ := alice.Fingerprint("bob")
	fb, _ := bob.Fingerprint("alice")
	if fa == "" || fa != fb {
		t.Fatalf("Sicherheitsnummern verschieden: %q vs %q", fa, fb)
	}
}

func TestPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "k.keys")
	a, err := Open(path, "pw")
	if err != nil {
		t.Fatal(err)
	}
	idA := string(a.st.d.IdPub)
	a2, err := Open(path, "pw")
	if err != nil {
		t.Fatal(err)
	}
	if string(a2.st.d.IdPub) != idA {
		t.Fatal("Identität nach Neuladen verschieden")
	}
	if _, err := Open(path, "falsch"); err == nil {
		t.Fatal("falsches Passwort wurde akzeptiert")
	}
}
