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
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/explorer"

	"ircgram/internal/chat"
	"ircgram/internal/e2e"
	"ircgram/internal/irc"
	"ircgram/internal/nekos"
)

type App struct {
	window *app.Window
	th     *material.Theme

	mu         sync.Mutex
	store      *chat.Store
	client     *irc.Client
	autojoin   []string
	identPass  string
	connecting bool
	loginErr   string
	focusInput bool

	e2e   *e2e.Manager
	jobs  chan func()
	nekos *nekos.Client

	// Login
	serverEd, nickEd, passEd, chansEd, keyEd widget.Editor
	tlsBox                                   widget.Bool
	connectBtn                               widget.Clickable
	loginList                                widget.List

	// Chat
	joinEd        widget.Editor
	joinBtn       widget.Clickable
	msgEd         widget.Editor
	sendBtn       widget.Clickable
	disconnectBtn widget.Clickable
	convList      widget.List
	rowClicks     map[string]*widget.Clickable
	msgLists      map[string]*widget.List

	// Anhänge
	expl      *explorer.Explorer
	imgBtn    widget.Clickable
	attClicks map[*chat.Attachment]*widget.Clickable

	// Sticker / Viewer / Paste
	viewer     viewer
	pasteBtn   widget.Clickable
	stickerBtn widget.Clickable
	picker     *stickerPicker
}

func newApp(w *app.Window) *App {
	a := &App{
		window:    w,
		th:        newTheme(),
		store:     chat.NewStore(),
		jobs:      make(chan func(), 256),
		nekos:     newNekosClient(),
		rowClicks: map[string]*widget.Clickable{},
		msgLists:  map[string]*widget.List{},
		expl:      explorer.NewExplorer(w),
		attClicks: map[*chat.Attachment]*widget.Clickable{},
		picker:    newStickerPicker(),
	}
	_ = InitClipboard()
	go func() {
		for job := range a.jobs {
			job()
		}
	}()
	for _, ed := range []*widget.Editor{&a.serverEd, &a.nickEd, &a.passEd, &a.chansEd, &a.keyEd, &a.joinEd, &a.msgEd} {
		ed.SingleLine = true
		ed.Submit = true
	}
	a.passEd.Mask = '•'
	a.keyEd.Mask = '•'
	a.serverEd.SetText("irc.libera.chat:6697")
	a.nickEd.SetText(fmt.Sprintf("uwu%d", rand.Intn(1000)))
	a.chansEd.SetText("#libera")
	a.tlsBox.Value = true
	a.convList.Axis = layout.Vertical
	a.loginList.Axis = layout.Vertical
	return a
}

func Run(window *app.Window) error {
	a := newApp(window)
	var ops op.Ops
	for {
		evt := window.Event()
		a.expl.ListenEvents(evt)
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
			a.handleGlobalKeys(gtx)
			a.mu.Lock()
			a.viewer.handle(gtx)
			a.layout(gtx)
			if a.picker.open {
				println("[APP] picker.handle")
				a.picker.handle(gtx, a)
			}
			a.mu.Unlock()
			e.Frame(gtx.Ops)
		}
	}
}

func (a *App) layout(gtx layout.Context) layout.Dimensions {
	if a.client == nil {
		return a.loginScreen(gtx)
	}
	if a.picker.open {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Flexed(1, a.mainScreen),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				w := min(gtx.Constraints.Max.X, max(gtx.Dp(180), min(gtx.Dp(360), gtx.Constraints.Max.X*36/100)))
				gtx.Constraints.Min.X = w
				gtx.Constraints.Max.X = w
				return a.pickerScreen(gtx)
			}),
		)
	}
	d := a.mainScreen(gtx)
	if a.viewer.open {
		a.viewer.Layout(gtx, a.th)
	}
	return d
}

func (a *App) handleGlobalKeys(gtx layout.Context) {
	ev, ok := gtx.Event(key.Filter{Name: "V", Required: key.ModShortcut})
	if !ok {
		return
	}
	if _, ok := ev.(key.Event); !ok {
		return
	}
	a.mu.Lock()
	conv, client := a.store.Active, a.client
	if client == nil || conv == nil || conv.Kind == chat.Server {
		a.mu.Unlock()
		return
	}
	_, data, ok := ClipboardImage()
	if !ok {
		a.mu.Unlock()
		return
	}
	a.mu.Unlock()
	go a.sendImage(conv, client, "clipboard.png", data)
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

func (a *App) startConnect() {
	if a.connecting {
		return
	}
	server := strings.TrimSpace(a.serverEd.Text())
	nick := strings.TrimSpace(a.nickEd.Text())
	password := strings.TrimSpace(a.passEd.Text())
	keyPass := a.keyEd.Text()
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
	if keyPass == "" {
		a.loginErr = "Wähle ein Schlüssel-Passwort."
		return
	}
	mgr, err := e2e.Open(e2ePath(), keyPass)
	if err != nil {
		a.loginErr = "Schlüsselspeicher: " + err.Error()
		return
	}
	a.e2e = mgr
	a.connecting = true
	a.loginErr = ""
	a.identPass = password

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
		return
	}
	s := a.store
	switch ev.Kind {
	case irc.EvConnected:
		s.Sys(s.Server(), "Verbunden als "+c.Nick()+" uwu")
		if a.identPass != "" {
			c.Privmsg("NickServ", "IDENTIFY "+c.Nick()+" "+a.identPass)
			a.identPass = ""
		}
		for _, ch := range a.autojoin {
			conv := s.Ensure(ch)
			s.Select(conv)
			c.Join(ch)
		}
	case irc.EvDisconnected:
		a.client = nil
		a.loginErr = "Verbindung getrennt: " + ev.Text
	case irc.EvServer:
		s.Sys(s.Server(), ev.Text)
	case irc.EvMessage:
		a.incoming(c, s.Ensure(ev.Target), ev)
	case irc.EvJoinSelf:
		conv := s.Ensure(ev.Target)
		conv.ResetMembers()
		s.Select(conv)
		a.focusInput = true
		s.Sys(conv, "Du bist "+ev.Target+" beigetreten")
		if conv.Kind == chat.Channel && a.e2e != nil {
			s.Sys(conv, "Gruppen-E2E ist an – nur Urineless-Nutzer können mitlesen. (/e2e off zum Abschalten)")
		}
	case irc.EvPartSelf:
		if conv := s.Find(ev.Target); conv != nil {
			s.Remove(conv)
		}
	case irc.EvNames:
		if conv := s.Find(ev.Target); conv != nil {
			for _, n := range strings.Fields(ev.Text) {
				conv.AddMember(n)
			}
		}
	case irc.EvJoin:
		if conv := s.Find(ev.Target); conv != nil {
			conv.AddMember(ev.Nick)
			s.Sys(conv, ev.Nick+" ist beigetreten")
		}
	case irc.EvPart:
		if conv := s.Find(ev.Target); conv != nil {
			conv.DelMember(ev.Nick)
			a.rekey(conv, ev.Nick)
			s.Sys(conv, ev.Nick+" hat den Kanal verlassen")
		}
	case irc.EvKick:
		if conv := s.Find(ev.Target); conv != nil {
			conv.DelMember(ev.Nick)
			a.rekey(conv, ev.Nick)
			s.Sys(conv, ev.Nick+" wurde aus dem Kanal geworfen")
		}
	case irc.EvQuit:
		for _, conv := range s.Convs {
			if conv.Kind == chat.Channel && conv.DelMember(ev.Nick) {
				a.rekey(conv, ev.Nick)
				s.Sys(conv, ev.Nick+" hat den Server verlassen")
			}
		}
	case irc.EvNick:
		for _, conv := range s.Convs {
			if conv.Kind == chat.Channel && conv.RenameMember(ev.Nick, ev.Text) {
				s.Sys(conv, ev.Nick+" heißt jetzt "+ev.Text)
			}
		}
		if a.e2e != nil {
			a.e2e.Group.RenamePeer(ev.Nick, ev.Text)
		}
	}
}

func (a *App) incoming(c *irc.Client, conv *chat.Conversation, ev irc.Event) {
	s := a.store
	switch {
	case a.e2e != nil && conv.Kind == chat.Private && strings.HasPrefix(ev.Text, e2e.Prefix):
		res := a.e2e.Receive(ev.Nick, ev.Text)
		if len(res.Reply) > 0 {
			lines, nick := res.Reply, ev.Nick
			a.enqueue(func() { a.sendLines(c, nick, lines) })
		}
		if res.Note != "" {
			s.Sys(conv, res.Note)
		}
		if res.HasText && !a.handleControl(c, ev.Nick, res.Text) {
			a.post(c, conv, ev.Nick, res.Text, ev.Time, true)
		}
	case a.e2e != nil && conv.Kind == chat.Channel && strings.HasPrefix(ev.Text, e2e.GroupPrefix):
		res := a.e2e.Group.Receive(conv.Name, ev.Nick, ev.Text)
		if res.Note != "" {
			s.Sys(conv, res.Note)
		}
		if res.NeedKey {
			a.sendEncrypted(c, ev.Nick, e2e.FormatSKReq(conv.Name))
		}
		if res.HasText {
			a.post(c, conv, ev.Nick, res.Text, ev.Time, true)
		}
	default:
		a.post(c, conv, ev.Nick, ev.Text, ev.Time, false)
	}
}

func (a *App) post(c *irc.Client, conv *chat.Conversation, nick, text string, t time.Time, enc bool) {
	if strings.HasPrefix(text, "\x01ACTION ") {
		text = "* " + nick + " " + strings.TrimSuffix(strings.TrimPrefix(text, "\x01ACTION "), "\x01")
	}
	msg := chat.Message{
		Sender: nick, Text: text, Time: t, Enc: enc,
		Mine: strings.EqualFold(nick, c.Nick()),
	}
	if att := chat.ParseAttachment(text); att != nil {
		if msg.Mine && hasOutgoingAttachment(conv, att) {
			return
		}
		msg.Attachment = att
		msg.Text = "Bild: " + att.Name
		if !msg.Mine && autoLoadPrivate && conv.Kind == chat.Private {
			a.receive(att)
		}
	}
	a.store.Post(conv, msg)
}

func hasOutgoingAttachment(conv *chat.Conversation, incoming *chat.Attachment) bool {
	for i := len(conv.Messages) - 1; i >= 0; i-- {
		msg := conv.Messages[i]
		if !msg.Mine || msg.Attachment == nil {
			continue
		}
		existing := msg.Attachment
		if incoming.Code != "" && existing.Code == incoming.Code {
			return true
		}
		if incoming.URL != "" && existing.URL == incoming.URL {
			return true
		}
	}
	return false
}

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
	if len(text) > 2 && strings.HasPrefix(text, ":") && strings.HasSuffix(text, ":") {
		name := strings.Trim(text, ":")
		if path, ok := chat.ResolveSticker(name); ok {
			a.sendImageFile(conv, path)
			return
		}
	}
	enc := a.secure(conv)
	a.deliver(conv, a.client, text)
	a.store.Post(conv, chat.Message{Sender: a.client.Nick(), Text: text, Time: time.Now(), Mine: true, Enc: enc})
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
			a.client.Part(conv.Name)
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
			enc := a.secure(target)
			a.deliver(target, a.client, text)
			a.store.Post(target, chat.Message{Sender: a.client.Nick(), Text: text, Time: time.Now(), Mine: true, Enc: enc})
		}
	case "me":
		if conv.Kind == chat.Server || rest == "" {
			return
		}
		enc := a.secure(conv)
		a.deliver(conv, a.client, "\x01ACTION "+rest+"\x01")
		a.store.Post(conv, chat.Message{Sender: a.client.Nick(), Text: "* " + a.client.Nick() + " " + rest, Time: time.Now(), Mine: true, Enc: enc})
	case "img", "image", "bild":
		path := strings.Trim(rest, "\"' ")
		if conv.Kind == chat.Server || path == "" {
			a.store.Sys(conv, "Benutzung (in einem Chat): /img /pfad/zum/bild.png")
			return
		}
		a.sendImageFile(conv, path)
	case "sticker", "s":
		if conv.Kind == chat.Server {
			a.store.Sys(conv, "Sticker gehen nur in Chats.")
			return
		}
		if len(args) == 0 {
			a.picker.Open(a)
			return
		}
		path, ok := chat.ResolveSticker(args[0])
		if !ok {
			a.store.Sys(conv, "Sticker '"+args[0]+"' nicht gefunden.")
			return
		}
		a.sendImageFile(conv, path)
	case "nick":
		if len(args) > 0 {
			a.client.SetNick(args[0])
		}
	case "raw", "quote":
		a.client.Raw("%s", rest)
	case "e2e":
		a.e2eCommand(conv, args)
	default:
		a.store.Sys(conv, "Befehle: /join #kanal, /part, /msg nick text, /me text, /img pfad, /sticker [name], /nick name, /e2e ...")
	}
}
