// Package chat enthält das UI-unabhängige Datenmodell (Chats, Nachrichten, Ungelesen-Zähler).
// Der Store ist nicht thread-safe; der Aufrufer sperrt von außen.
package chat

import (
	"crypto/rand"
	"encoding/hex"
	"regexp"
	"sort"
	"strings"
	"time"
)

var msgIDRe = regexp.MustCompile(`^\[id:([a-f0-9]{16})\]\s*(.*)$`)

func NewMessageID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func FormatMessageWithID(id, text string) string {
	if id == "" {
		id = NewMessageID()
	}
	return "[id:" + id + "] " + text
}

func ExtractMessageID(text string) (string, string) {
	if m := msgIDRe.FindStringSubmatch(text); m != nil {
		return m[1], m[2]
	}
	return "", text
}

type Kind int

const (
	Server Kind = iota
	Channel
	Private
)

const maxMessages = 500

type Message struct {
	ID         string
	Sender     string
	Text       string
	Time       time.Time
	Mine       bool
	System     bool
	Enc        bool // Ende-zu-Ende verschlüsselt übertragen
	Attachment *Attachment
}

type Conversation struct {
	Name     string
	Kind     Kind
	Messages []Message
	Unread   int
	NoE2E    bool // E2E für diesen Chat abgeschaltet (/e2e off)

	members map[string]string // nick(klein) -> Anzeigename
}

func (c *Conversation) Key() string {
	if c.Kind == Server {
		return "\x00server"
	}
	return strings.ToLower(c.Name)
}

func (c *Conversation) Last() (Message, bool) {
	if len(c.Messages) == 0 {
		return Message{}, false
	}
	return c.Messages[len(c.Messages)-1], true
}

// ---- Mitglieder (nur für Kanäle sinnvoll) ----

func cleanNick(n string) string { return strings.TrimLeft(n, "@+%&~") }

func (c *Conversation) ResetMembers() { c.members = map[string]string{} }

func (c *Conversation) AddMember(n string) {
	n = cleanNick(n)
	if n == "" {
		return
	}
	if c.members == nil {
		c.members = map[string]string{}
	}
	c.members[strings.ToLower(n)] = n
}

// DelMember entfernt n und meldet, ob n Mitglied war.
func (c *Conversation) DelMember(n string) bool {
	k := strings.ToLower(cleanNick(n))
	_, ok := c.members[k]
	delete(c.members, k)
	return ok
}

func (c *Conversation) HasMember(n string) bool {
	_, ok := c.members[strings.ToLower(cleanNick(n))]
	return ok
}

func (c *Conversation) RenameMember(oldNick, newNick string) bool {
	if !c.DelMember(oldNick) {
		return false
	}
	c.AddMember(newNick)
	return true
}

func (c *Conversation) MemberCount() int { return len(c.members) }

func (c *Conversation) MemberList() []string {
	out := make([]string, 0, len(c.members))
	for _, n := range c.members {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// ---- Store ----

type Store struct {
	Convs  []*Conversation // Index 0 ist immer der Server-Chat
	Active *Conversation
}

func NewStore() *Store {
	srv := &Conversation{Name: "Server", Kind: Server}
	return &Store{Convs: []*Conversation{srv}, Active: srv}
}

func (s *Store) Server() *Conversation { return s.Convs[0] }

func (s *Store) Find(name string) *Conversation {
	for _, c := range s.Convs {
		if c.Kind != Server && strings.EqualFold(c.Name, name) {
			return c
		}
	}
	return nil
}

func (s *Store) Ensure(name string) *Conversation {
	if c := s.Find(name); c != nil {
		return c
	}
	kind := Private
	if strings.HasPrefix(name, "#") || strings.HasPrefix(name, "&") {
		kind = Channel
	}
	c := &Conversation{Name: name, Kind: kind}
	rest := append([]*Conversation{c}, s.Convs[1:]...)
	s.Convs = append(s.Convs[:1], rest...)
	return c
}

func (s *Store) Remove(c *Conversation) {
	if c.Kind == Server {
		return
	}
	for i, x := range s.Convs {
		if x == c {
			s.Convs = append(s.Convs[:i], s.Convs[i+1:]...)
			break
		}
	}
	if s.Active == c {
		s.Active = s.Server()
	}
}

func (s *Store) Select(c *Conversation) {
	s.Active = c
	c.Unread = 0
}

func (s *Store) DeleteMessage(c *Conversation, id string) bool {
	for i, m := range c.Messages {
		if m.ID == id {
			c.Messages = append(c.Messages[:i], c.Messages[i+1:]...)
			return true
		}
	}
	return false
}

func (s *Store) DeleteMessages(c *Conversation, ids map[string]bool) int {
	if len(ids) == 0 {
		return 0
	}
	var kept []Message
	deletedCount := 0
	for _, m := range c.Messages {
		if ids[m.ID] {
			deletedCount++
		} else {
			kept = append(kept, m)
		}
	}
	c.Messages = kept
	return deletedCount
}

func (s *Store) DeleteAllSentMessages(c *Conversation) []string {
	var kept []Message
	var deletedIDs []string
	for _, m := range c.Messages {
		if m.Mine && !m.System && m.ID != "" {
			deletedIDs = append(deletedIDs, m.ID)
		} else {
			kept = append(kept, m)
		}
	}
	c.Messages = kept
	return deletedIDs
}

// Post hängt eine Nachricht an und sortiert den Chat wie Telegram nach oben.
func (s *Store) Post(c *Conversation, m Message) {
	if m.Time.IsZero() {
		m.Time = time.Now()
	}
	c.Messages = append(c.Messages, m)
	if len(c.Messages) > maxMessages {
		c.Messages = c.Messages[len(c.Messages)-maxMessages:]
	}
	if c != s.Active && !m.Mine && !m.System {
		c.Unread++
	}
	if !m.System {
		s.bump(c)
	}
}

func (s *Store) bump(c *Conversation) {
	if c.Kind == Server {
		return
	}
	idx := -1
	for i, x := range s.Convs {
		if x == c {
			idx = i
			break
		}
	}
	if idx <= 1 {
		return
	}
	copy(s.Convs[2:idx+1], s.Convs[1:idx])
	s.Convs[1] = c
}

// Sys ist eine Kurzform für Systemmeldungen.
func (s *Store) Sys(c *Conversation, text string) {
	s.Post(c, Message{Text: text, System: true})
}
