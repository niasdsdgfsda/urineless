// Package chat enthält das UI-unabhängige Datenmodell (Chats, Nachrichten, Ungelesen-Zähler).
// Der Store ist nicht thread-safe; der Aufrufer sperrt von außen.
package chat

import (
	"strings"
	"time"
)

type Kind int

const (
	Server Kind = iota
	Channel
	Private
)

const maxMessages = 500

type Message struct {
	Sender string
	Text   string
	Time   time.Time
	Mine   bool
	System bool

	Attachment *Attachment // nil bei normalem Text
}

type Conversation struct {
	Name     string
	Kind     Kind
	Messages []Message
	Unread   int
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
