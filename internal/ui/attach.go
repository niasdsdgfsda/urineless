package ui

import (
	"context"
	"errors"
	"image"
	"io"
	"os"
	"path/filepath"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op/paint"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/explorer"

	"ircgram/internal/chat"
	"ircgram/internal/irc"
	"ircgram/internal/transfer"
)

// Wenn true, werden Bilder in Privatchats sofort abgeholt. Standard ist false:
// Ein Fremder soll keinen Download auf deinem Rechner auslösen können.
// (In Kanälen gilt immer "Klick zum Laden", weil der Code nur einmal einlösbar ist.)
const autoLoadPrivate = false

// ---- Anzeige ----

func (a *App) attClick(att *chat.Attachment) *widget.Clickable {
	c, ok := a.attClicks[att]
	if !ok {
		c = new(widget.Clickable)
		a.attClicks[att] = c
	}
	return c
}

func (a *App) imageOp(att *chat.Attachment) paint.ImageOp {
	op, ok := a.imgOps[att]
	if !ok {
		op = paint.NewImageOp(att.Image)
		a.imgOps[att] = op
	}
	return op
}

func statusText(att *chat.Attachment) string {
	switch att.State {
	case chat.AttWaiting:
		return "Wartet auf Abholung · " + att.Code
	case chat.AttDelivered:
		return "Zugestellt"
	case chat.AttLoading:
		return "Lade über Wormhole …"
	case chat.AttFailed:
		return "Fehler: " + att.Err
	}
	return ""
}

// attachmentBody liefert den Inhalt einer Bild-Blase (Bild oder Platzhalter + Status).
func (a *App) attachmentBody(att *chat.Attachment) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		click := a.attClick(att)
		if click.Clicked(gtx) && att.Image == nil && att.State != chat.AttLoading {
			a.receive(att)
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if att.Image != nil {
					return a.imageView(gtx, att)
				}
				return a.placeholder(gtx, att, click)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				txt := statusText(att)
				if txt == "" {
					return layout.Dimensions{}
				}
				l := material.Caption(a.th, txt)
				l.Color = colorMuted
				return l.Layout(gtx)
			}),
		)
	}
}

func (a *App) imageView(gtx layout.Context, att *chat.Attachment) layout.Dimensions {
	maxW := min(gtx.Constraints.Max.X, gtx.Dp(320))
	gtx.Constraints = layout.Constraints{Max: image.Pt(maxW, gtx.Dp(320))}
	return widget.Image{
		Src:   a.imageOp(att),
		Fit:   widget.ScaleDown,
		Scale: gtx.Metric.PxPerDp, // 1 Bildpixel = 1 Bildschirmpixel
	}.Layout(gtx)
}

func (a *App) placeholder(gtx layout.Context, att *chat.Attachment, click *widget.Clickable) layout.Dimensions {
	gtx.Constraints.Min.X = min(gtx.Dp(200), gtx.Constraints.Max.X)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			l := material.Body2(a.th, "Bild: "+att.Name)
			l.Font.Weight = font.Bold
			return l.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if att.State == chat.AttLoading {
				return layout.Dimensions{}
			}
			return layout.Inset{Top: 6, Bottom: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				b := material.Button(a.th, click, "Bild laden")
				b.CornerRadius = 12
				b.Inset = layout.Inset{Top: 6, Bottom: 6, Left: 14, Right: 14}
				return b.Layout(gtx)
			})
		}),
	)
}

// ---- Empfangen ----

// receive holt das Bild im Hintergrund ab. Wird mit gehaltenem Lock aufgerufen.
func (a *App) receive(att *chat.Attachment) {
	if att.State == chat.AttLoading {
		return
	}
	att.State, att.Err = chat.AttLoading, ""
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		img, err := transfer.Receive(ctx, att.Code)

		a.mu.Lock()
		defer a.mu.Unlock()
		defer a.window.Invalidate()
		if err != nil {
			att.State, att.Err = chat.AttFailed, err.Error()
			return
		}
		att.Image, att.State = img, chat.AttReady
	}()
}

// ---- Senden ----

func (a *App) note(conv *chat.Conversation, text string) {
	a.mu.Lock()
	a.store.Sys(conv, text)
	a.mu.Unlock()
	a.window.Invalidate()
}

// pickImage öffnet den Dateidialog. Wird mit gehaltenem Lock aus dem Frame aufgerufen.
func (a *App) pickImage() {
	conv, client := a.store.Active, a.client
	if conv.Kind == chat.Server {
		a.store.Sys(conv, "Öffne zuerst einen Kanal oder Privatchat.")
		return
	}
	if a.expl == nil {
		return
	}
	go func() {
		rc, err := a.expl.ChooseFile(".png", ".jpg", ".jpeg", ".gif")
		if err != nil {
			if !errors.Is(err, explorer.ErrUserDecline) {
				a.note(conv, "Dateidialog nicht verfügbar ("+err.Error()+"). Nutze /img /pfad/zum/bild.png")
			}
			return
		}
		defer rc.Close()
		name := "bild"
		if n, ok := rc.(interface{ Name() string }); ok {
			name = filepath.Base(n.Name())
		}
		data, err := io.ReadAll(io.LimitReader(rc, transfer.MaxBytes+1))
		if err != nil {
			a.note(conv, "Datei nicht lesbar: "+err.Error())
			return
		}
		a.sendImage(conv, client, name, data)
	}()
}

// sendImageFile ist die Variante für /img <pfad>. Wird mit gehaltenem Lock aufgerufen.
func (a *App) sendImageFile(conv *chat.Conversation, path string) {
	client := a.client
	go func() {
		st, err := os.Stat(path)
		if err != nil {
			a.note(conv, "Datei nicht gefunden: "+path)
			return
		}
		if st.Size() > transfer.MaxBytes {
			a.note(conv, "Datei zu groß (max. 10 MB)")
			return
		}
		data, err := os.ReadFile(path)
		if err != nil {
			a.note(conv, "Datei nicht lesbar: "+err.Error())
			return
		}
		a.sendImage(conv, client, filepath.Base(path), data)
	}()
}

// sendImage läuft in einer Goroutine (ohne Lock) und blockiert, bis die Übertragung endet.
func (a *App) sendImage(conv *chat.Conversation, client *irc.Client, name string, data []byte) {
	name = chat.SafeName(name)
	img, err := transfer.DecodeImage(data)
	if err != nil {
		a.note(conv, "Bild nicht gesendet: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	code, done, err := transfer.Send(ctx, name, data)
	if err != nil {
		a.note(conv, "Wormhole-Fehler: "+err.Error())
		return
	}

	att := &chat.Attachment{Code: code, Name: name, Image: img, Outgoing: true, State: chat.AttWaiting}
	a.mu.Lock()
	if a.client != client {
		a.mu.Unlock()
		return
	}
	client.Privmsg(conv.Name, chat.FormatAttachment(code, name))
	a.store.Post(conv, chat.Message{
		Sender: client.Nick(), Text: "Bild: " + name, Mine: true, Attachment: att,
	})
	if conv.Kind == chat.Channel {
		a.store.Sys(conv, "Hinweis: Ein Wormhole-Code gilt nur einmal – nur der erste Empfänger bekommt das Bild.")
	}
	a.mu.Unlock()
	a.window.Invalidate()

	err = <-done

	a.mu.Lock()
	if err != nil {
		att.State, att.Err = chat.AttFailed, err.Error()
	} else {
		att.State = chat.AttDelivered
	}
	a.mu.Unlock()
	a.window.Invalidate()
}
