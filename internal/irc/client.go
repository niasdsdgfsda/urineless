// Package irc enthält einen minimalen IRC-Client, der Events über einen Channel liefert.
package irc

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

type Kind int

const (
	EvConnected    Kind = iota // Registrierung abgeschlossen (001)
	EvDisconnected             // Verbindung weg
	EvServer                   // Server-Text (Numerics, Notices)
	EvMessage                  // PRIVMSG an Kanal oder privat
	EvJoin                     // jemand anderes betritt Kanal
	EvPart                     // jemand anderes verlässt Kanal
	EvJoinSelf                 // wir betreten Kanal
	EvPartSelf                 // wir verlassen Kanal (auch nach KICK)
	EvNames                    // Teil der Mitgliederliste (Text = Nicks mit Leerzeichen)
	EvQuit                     // jemand verlässt den Server
	EvNick                     // Nickwechsel (Nick = alt, Text = neu)
	EvKick                     // jemand anderes wurde gekickt (Nick = Opfer)
)

type Event struct {
	Kind   Kind
	Target string // Kanal bzw. Nick des Gesprächspartners
	Nick   string // Absender
	Text   string
	Time   time.Time
}

type Client struct {
	conn   net.Conn
	wmu    sync.Mutex
	nmu    sync.RWMutex
	nick   string
	Events chan Event
}

var lineSanitizer = strings.NewReplacer("\r", "", "\n", " ")

func Dial(addr, nick string, useTLS bool) (*Client, error) {
	d := &net.Dialer{Timeout: 10 * time.Second}
	var conn net.Conn
	var err error
	if useTLS {
		host, _, _ := net.SplitHostPort(addr)
		conn, err = tls.DialWithDialer(d, "tcp", addr, &tls.Config{ServerName: host})
	} else {
		conn, err = d.Dial("tcp", addr)
	}
	if err != nil {
		return nil, err
	}
	c := &Client{conn: conn, nick: nick, Events: make(chan Event, 256)}
	c.Raw("NICK %s", nick)
	c.Raw("USER %s 0 * :urineless", nick)
	go c.readLoop()
	return c, nil
}

func (c *Client) Nick() string {
	c.nmu.RLock()
	defer c.nmu.RUnlock()
	return c.nick
}

func (c *Client) setNick(n string) {
	c.nmu.Lock()
	c.nick = n
	c.nmu.Unlock()
}

func (c *Client) Raw(format string, args ...any) {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	fmt.Fprint(c.conn, lineSanitizer.Replace(fmt.Sprintf(format, args...))+"\r\n")
}

func (c *Client) Join(ch string)          { c.Raw("JOIN %s", ch) }
func (c *Client) Part(ch string)          { c.Raw("PART %s", ch) }
func (c *Client) Privmsg(to, text string) { c.Raw("PRIVMSG %s :%s", to, text) }
func (c *Client) SetNick(n string)        { c.Raw("NICK %s", n) }
func (c *Client) Quit() {
	c.Raw("QUIT :bye bye uwu")
	c.conn.Close()
}

func (c *Client) emit(e Event) {
	e.Time = time.Now()
	c.Events <- e
}

func (c *Client) readLoop() {
	r := bufio.NewReader(c.conn)
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			c.emit(Event{Kind: EvDisconnected, Text: err.Error()})
			close(c.Events)
			return
		}
		c.handle(strings.TrimRight(line, "\r\n"))
	}
}

func isChannel(s string) bool { return strings.HasPrefix(s, "#") || strings.HasPrefix(s, "&") }

func isNumeric(s string) bool {
	if len(s) != 3 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func (c *Client) handle(line string) {
	prefix, cmd, params := parse(line)
	if cmd == "" {
		return
	}
	nick := prefix
	if i := strings.IndexByte(prefix, '!'); i >= 0 {
		nick = prefix[:i]
	}
	self := strings.EqualFold(nick, c.Nick())

	switch cmd {
	case "PING":
		if len(params) > 0 {
			c.Raw("PONG :%s", params[len(params)-1])
		}
	case "001":
		if len(params) > 0 {
			c.setNick(params[0])
		}
		c.emit(Event{Kind: EvConnected})
	case "PRIVMSG", "NOTICE":
		if len(params) < 2 {
			return
		}
		target, text := params[0], params[1]
		if cmd == "NOTICE" && !strings.Contains(prefix, "!") {
			c.emit(Event{Kind: EvServer, Text: text})
			return
		}
		if !isChannel(target) {
			target = nick // private Nachricht: Gespräch ist der Absender
		}
		if strings.HasPrefix(text, "\x01ACTION ") {
			text = "* " + nick + " " + strings.TrimSuffix(strings.TrimPrefix(text, "\x01ACTION "), "\x01")
		} else if strings.HasPrefix(text, "\x01") {
			return // andere CTCP ignorieren
		}
		c.emit(Event{Kind: EvMessage, Target: target, Nick: nick, Text: text})
	case "JOIN":
		if len(params) == 0 {
			return
		}
		k := EvJoin
		if self {
			k = EvJoinSelf
		}
		c.emit(Event{Kind: k, Target: params[0], Nick: nick})
	case "PART":
		if len(params) == 0 {
			return
		}
		k := EvPart
		if self {
			k = EvPartSelf
		}
		c.emit(Event{Kind: k, Target: params[0], Nick: nick})
	case "KICK":
		if len(params) < 2 {
			return
		}
		reason := ""
		if len(params) > 2 {
			reason = params[2]
		}
		if strings.EqualFold(params[1], c.Nick()) {
			c.emit(Event{Kind: EvPartSelf, Target: params[0], Nick: nick, Text: reason})
			return
		}
		c.emit(Event{Kind: EvKick, Target: params[0], Nick: params[1], Text: reason})
	case "QUIT":
		c.emit(Event{Kind: EvQuit, Nick: nick})
	case "NICK":
		if len(params) > 0 {
			if self {
				c.setNick(params[0])
				c.emit(Event{Kind: EvServer, Text: "Du heißt jetzt " + params[0]})
			}
			c.emit(Event{Kind: EvNick, Nick: nick, Text: params[0]})
		}
	case "353": // RPL_NAMREPLY: <me> <=|*|@> <#kanal> :<nicks>
		if len(params) >= 4 {
			c.emit(Event{Kind: EvNames, Target: params[2], Text: params[3]})
		}
	case "366": // Ende der NAMES-Liste
	default:
		if isNumeric(cmd) && len(params) > 1 {
			c.emit(Event{Kind: EvServer, Text: strings.Join(params[1:], " ")})
		}
	}
}

func parse(line string) (prefix, cmd string, params []string) {
	if strings.HasPrefix(line, ":") {
		i := strings.IndexByte(line, ' ')
		if i < 0 {
			return line[1:], "", nil
		}
		prefix = line[1:i]
		line = strings.TrimLeft(line[i+1:], " ")
	}
	trailing, hasTrailing := "", false
	if i := strings.Index(line, " :"); i >= 0 {
		trailing, hasTrailing = line[i+2:], true
		line = line[:i]
	}
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return
	}
	cmd = strings.ToUpper(fields[0])
	params = fields[1:]
	if hasTrailing {
		params = append(params, trailing)
	}
	return
}
