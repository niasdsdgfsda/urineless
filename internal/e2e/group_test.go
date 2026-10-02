package e2e

import (
	"strings"
	"testing"
)

func recvAll(g *GroupManager, ch, nick string, lines []string) GroupResult {
	var r GroupResult
	for _, l := range lines {
		r = g.Receive(ch, nick, l)
	}
	return r
}

func TestGroupRoundTrip(t *testing.T) {
	alice, bob := NewGroupManager(), NewGroupManager()

	// Schlüssel läuft über SKEY1 (im echten Betrieb durch die Signal-Sitzung)
	ch, key, ok := ParseSKey(FormatSKey("#Test", alice.GetOrGenMyKey("#Test")))
	if !ok {
		t.Fatal("SKEY1 nicht lesbar")
	}
	bob.StorePeerKey(ch, "Alice", key)

	lines, err := alice.Encrypt("#test", "hallo gruppe")
	if err != nil {
		t.Fatal(err)
	}
	if r := recvAll(bob, "#test", "alice", lines); !r.HasText || r.Text != "hallo gruppe" {
		t.Fatalf("Bob hat %+v", r)
	}

	// lange Nachricht -> mehrere Chunks
	long := strings.Repeat("uwu ", 500)
	lines, err = alice.Encrypt("#test", long)
	if err != nil || len(lines) < 2 {
		t.Fatalf("Chunking: %d Zeilen, %v", len(lines), err)
	}
	for _, l := range lines {
		if len(l) > 400 {
			t.Fatalf("Zeile zu lang: %d", len(l))
		}
	}
	if r := recvAll(bob, "#test", "alice", lines); !r.HasText || r.Text != long {
		t.Fatal("lange Nachricht defekt")
	}

	// Carol hat noch keinen Schlüssel -> Anfrage, Nachricht wird nachgeliefert
	carol := NewGroupManager()
	lines, _ = alice.Encrypt("#test", "zu spaet")
	r := recvAll(carol, "#test", "alice", lines)
	if r.HasText || !r.NeedKey {
		t.Fatalf("Carol: %+v", r)
	}
	got := carol.StorePeerKey("#test", "alice", alice.GetOrGenMyKey("#test"))
	if len(got) != 1 || got[0] != "zu spaet" {
		t.Fatalf("Nachlieferung: %v", got)
	}

	// Rotation: alter Schlüssel passt nicht mehr, Bob fragt neu an
	alice.Rotate("#test")
	lines, _ = alice.Encrypt("#test", "neuer schluessel")
	r = recvAll(bob, "#test", "alice", lines)
	if r.HasText || !r.NeedKey {
		t.Fatalf("nach Rotation: %+v", r)
	}
	got = bob.StorePeerKey("#test", "alice", alice.GetOrGenMyKey("#test"))
	if len(got) != 1 || got[0] != "neuer schluessel" {
		t.Fatalf("Nachlieferung nach Rotation: %v", got)
	}

	// anderer Kanal -> AAD verhindert Replay
	lines, _ = alice.Encrypt("#test", "nur hier")
	if r := recvAll(bob, "#anderer", "alice", lines); r.HasText {
		t.Fatal("Replay in anderem Kanal wurde akzeptiert")
	}
}
