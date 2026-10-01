package ui

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"gioui.org/app"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/explorer"

	"ircgram/internal/chat"
	"ircgram/internal/irc"
)

// App hält den gesamten UI- und Verbindungszustand.
// Alles wird von a.mu geschützt: der Frame-Handler sperrt pro Frame,
// Hintergrund-Goroutinen sperren bei jeder Änderung.
type App struct {
	window *app.Window
	th     *material.Theme

	mu         sync.Mutex
	store      *chat.Store
	client     *irc.Client
	autojoin   []string
	connecting bool
	loginErr   string
	focusInput bool

	// Login
	serverEd, nickEd, chansEd widget.Editor
	tlsBox                    widget.Bool
	connectBtn                widget.Clickable

	// Chat
	joinEd        widget.Editor
	joinBtn       widget.Clickable
	msgEd         widget.Editor
	sendBtn       widget.Clickable
	disconnectBtn widget.Clickable
	convList      widget.List
	rowClicks     map[string]*widget.Clickable
	msgLists      map[string]*widget.List

	// Bild-Anhänge (Wormhole)
	expl      *explorer.Explorer
	imgBtn    widget.Clickable
	attClicks map[*chat.Attachment]*widget.Clickable
	imgOps    map[*chat.Attachment]paint.ImageOp
}

func newApp(w *app.Window) *App {
	a := &App{
		window:    w,
		th:        newTheme(),
		store:     chat.NewStore(),
		rowClicks: map[string]*widget.Clickable{},
		msgLists:  map[string]*widget.List{},
		expl:      explorer.NewExplorer(w),
		attClicks: map[*chat.Attachment]*widget.Clickable{},
		imgOps:    map[*chat.Attachment]paint.ImageOp{},
	}
	for _, ed := range []*widget.Editor{&a.serverEd, &a.nickEd, &a.chansEd, &a.joinEd, &a.msgEd} {
		ed.SingleLine = true
	}
	a.joinEd.Submit = true
	a.msgEd.Submit = true
	a.serverEd.SetText("irc.libera.chat:6697")
	a.nickEd.SetText(fmt.Sprintf("GioUser%d", rand.Intn(1000)))
	a.chansEd.SetText("#libera")
	a.tlsBox.Value = true
	a.convList.Axis = layout.Vertical
	return a
}

// Run ist die Hauptschleife des Fensters.
func Run(window *app.Window) error {
	a := newApp(window)
	var ops op.Ops
	for {
		evt := window.Event()
		a.expl.ListenEvents(evt) // nötig, damit der Dateidialog funktioniert
		switch e := evt.(type) {
		case app.DestroyEvent:
			a.mu.Lock()
			if a.client != nil {
				a.client.Quit()
			}
			a.mu.Unlock()
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			a.mu.Lock()
			a.layout(gtx)
			a.mu.Unlock()
			e.Frame(gtx.Ops)
		}
	}
}

func (a *App) layout(gtx layout.Context) layout.Dimensions {
	if a.client == nil {
		return a.loginScreen(gtx)
	}
	return a.mainScreen(gtx)
}

func (a *App) click(key string) *widget.Clickable {
	c, ok := a.rowClicks[key]
	if !ok {
		c = new(widget.Clickable)
		a.rowClicks[key] = c
	}
	return c
}

func (a *App) listFor(c *chat.Conversation) *widget.List {
	l, ok := a.msgLists[c.Key()]
	if !ok {
		l = &widget.List{}
		l.Axis = layout.Vertical
		l.ScrollToEnd = true
		a.msgLists[c.Key()] = l
	}
	return l
}

func (a *App) requestFocus(gtx layout.Context) {
	if a.focusInput {
		a.focusInput = false
		gtx.Execute(key.FocusCmd{Tag: &a.msgEd})
	}
}

// submitted prüft, ob im Editor Enter gedrückt wurde (Submit = true).
func submitted(gtx layout.Context, ed *widget.Editor) bool {
	done := false
	for {
		ev, ok := ed.Update(gtx)
		if !ok {
			break
		}
		if _, ok := ev.(widget.SubmitEvent); ok {
			done = true
		}
	}
	return done
}

// ---- Verbinden ----

// startConnect wird mit gehaltenem Lock aus dem Frame aufgerufen.
func (a *App) startConnect() {
	server := strings.TrimSpace(a.serverEd.Text())
	nick := strings.TrimSpace(a.nickEd.Text())
	var chans []string
	for _, c := range strings.FieldsFunc(a.chansEd.Text(), func(r rune) bool { return r == ',' || r == ' ' }) {
		if !strings.HasPrefix(c, "#") && !strings.HasPrefix(c, "&") {
			c = "#" + c
		}
		chans = append(chans, c)
	}
	useTLS := a.tlsBox.Value
	if server == "" || nick == "" {
		a.loginErr = "Server und Nickname werden benötigt."
		return
	}
	a.connecting = true
	a.loginErr = ""

	go func() {
		c, err := irc.Dial(server, nick, useTLS)
		a.mu.Lock()
		defer a.mu.Unlock()
		defer a.window.Invalidate()
		a.connecting = false
		if err != nil {
			a.loginErr = "Verbindung fehlgeschlagen: " + err.Error()
			return
		}
		a.client = c
		a.autojoin = chans
		a.store = chat.NewStore()
		a.msgLists = map[string]*widget.List{}
		a.attClicks = map[*chat.Attachment]*widget.Clickable{}
		a.imgOps = map[*chat.Attachment]paint.ImageOp{}
		a.store.Sys(a.store.Server(), "Verbinde mit "+server+" …")
		a.focusInput = true
		go a.pump(c)
	}()
}

func (a *App) disconnect() {
	if a.client != nil {
		a.client.Quit()
		a.client = nil
	}
	a.loginErr = ""
}

// ---- IRC-Events ----

func (a *App) pump(c *irc.Client) {
	for ev := range c.Events {
		a.handle(c, ev)
	}
}

func (a *App) handle(c *irc.Client, ev irc.Event) {
	a.mu.Lock()
	defer a.mu.Unlock()
	defer a.window.Invalidate()
	if a.client != c {
		return // veraltete Verbindung
	}
	s := a.store
	switch ev.Kind {
	case irc.EvConnected:
		s.Sys(s.Server(), "Verbunden als "+c.Nick())
		for _, ch := range a.autojoin {
			c.Join(ch)
		}
	case irc.EvDisconnected:
		a.client = nil
		a.loginErr = "Verbindung getrennt: " + ev.Text
	case irc.EvServer:
		s.Sys(s.Server(), ev.Text)
	case irc.EvMessage:
		conv := s.Ensure(ev.Target)
		msg := chat.Message{
			Sender: ev.Nick, Text: ev.Text, Time: ev.Time,
			Mine: strings.EqualFold(ev.Nick, c.Nick()),
		}
		if att := chat.ParseAttachment(ev.Text); att != nil && !msg.Mine {
			msg.Attachment = att
			msg.Text = "Bild: " + att.Name
			if autoLoadPrivate && conv.Kind == chat.Private {
				a.receive(att)
			}
		}
		s.Post(conv, msg)
	case irc.EvJoinSelf:
		conv := s.Ensure(ev.Target)
		s.Select(conv)
		a.focusInput = true
		s.Sys(conv, "Du bist "+ev.Target+" beigetreten")
	case irc.EvPartSelf:
		if conv := s.Find(ev.Target); conv != nil {
			s.Remove(conv)
		}
	case irc.EvJoin:
		if conv := s.Find(ev.Target); conv != nil {
			s.Sys(conv, ev.Nick+" ist beigetreten")
		}
	case irc.EvPart:
		if conv := s.Find(ev.Target); conv != nil {
			s.Sys(conv, ev.Nick+" hat den Kanal verlassen")
		}
	}
}

// ---- Eingaben des Nutzers ----

// openOrJoin: "#kanal" betreten, sonst Privatchat öffnen.
func (a *App) openOrJoin(name string) {
	defer a.window.Invalidate()
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	if strings.HasPrefix(name, "#") || strings.HasPrefix(name, "&") {
		a.client.Join(name)
		return
	}
	a.store.Select(a.store.Ensure(name))
	a.focusInput = true
}

func (a *App) submit(text string) {
	defer a.window.Invalidate()
	conv := a.store.Active
	if strings.HasPrefix(text, "/") {
		a.command(conv, text)
		return
	}
	if conv.Kind == chat.Server {
		a.store.Sys(conv, "Hier kann nicht geschrieben werden. Nutze /join #kanal oder /msg nick text (/help).")
		return
	}
	a.client.Privmsg(conv.Name, text)
	a.store.Post(conv, chat.Message{Sender: a.client.Nick(), Text: text, Time: time.Now(), Mine: true})
}

func (a *App) command(conv *chat.Conversation, line string) {
	fields := strings.Fields(line)
	cmd := strings.ToLower(strings.TrimPrefix(fields[0], "/"))
	args := fields[1:]
	rest := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), fields[0]))

	switch cmd {
	case "join", "j":
		if len(args) == 0 {
			a.store.Sys(conv, "Benutzung: /join #kanal")
			return
		}
		ch := args[0]
		if !strings.HasPrefix(ch, "#") && !strings.HasPrefix(ch, "&") {
			ch = "#" + ch
		}
		a.client.Join(ch)
	case "part", "leave":
		if conv.Kind == chat.Server {
			return
		}
		if conv.Kind == chat.Channel {
			a.client.Part(conv.Name) // Entfernen passiert beim EvPartSelf
		} else {
			a.store.Remove(conv)
		}
	case "msg", "query":
		if len(args) == 0 {
			a.store.Sys(conv, "Benutzung: /msg nick text")
			return
		}
		target := a.store.Ensure(args[0])
		a.store.Select(target)
		a.focusInput = true
		if text := strings.TrimSpace(strings.TrimPrefix(rest, args[0])); text != "" {
			a.client.Privmsg(target.Name, text)
			a.store.Post(target, chat.Message{Sender: a.client.Nick(), Text: text, Mine: true})
		}
	case "me":
		if conv.Kind == chat.Server || rest == "" {
			return
		}
		a.client.Privmsg(conv.Name, "\x01ACTION "+rest+"\x01")
		a.store.Post(conv, chat.Message{Sender: a.client.Nick(), Text: "* " + a.client.Nick() + " " + rest, Mine: true})
	case "img", "image", "bild":
		path := strings.Trim(rest, "\"' ")
		if conv.Kind == chat.Server || path == "" {
			a.store.Sys(conv, "Benutzung (in einem Chat): /img /pfad/zum/bild.png")
			return
		}
		a.sendImageFile(conv, path)
	case "nick":
		if len(args) > 0 {
			a.client.SetNick(args[0])
		}
	case "raw", "quote":
		a.client.Raw("%s", rest)
	default:
		a.store.Sys(conv, "Befehle: /join #kanal, /part, /msg nick text, /me text, /img pfad, /nick name, /raw ...")
	}
}
