package ui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"net/http"
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
	if att.Sticker {
		switch att.State {
		case chat.AttLoading:
			return "lädt …"
		case chat.AttFailed:
			return "Fehler: " + att.Err
		}
		return ""
	}
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
			if att.URL != "" {
				a.receiveURL(att)
			} else {
				a.receive(att)
			}
		}
		if att.URL != "" && att.Image == nil && att.State == chat.AttIdle && att.AutoLoad() {
			a.receiveURL(att)
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
	maxSize := gtx.Dp(320)
	if att.Sticker {
		maxSize = gtx.Dp(150)
	}
	maxW := min(gtx.Constraints.Max.X, maxSize)
	gtx.Constraints = layout.Constraints{Max: image.Pt(maxW, maxSize)}
	click := a.attClick(att)
	if click.Clicked(gtx) {
		a.viewer.Show(a.imageOp(att), att.Image.Bounds().Size())
	}
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return widget.Image{
			Src:   a.imageOp(att),
			Fit:   widget.ScaleDown,
			Scale: gtx.Metric.PxPerDp,
		}.Layout(gtx)
	})
}

func (a *App) placeholder(gtx layout.Context, att *chat.Attachment, click *widget.Clickable) layout.Dimensions {
	gtx.Constraints.Min.X = min(gtx.Dp(200), gtx.Constraints.Max.X)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			label := "Bild: " + att.Name
			if att.Sticker {
				label = "Sticker"
			}
			l := material.Body2(a.th, label)
			l.Font.Weight = font.Bold
			return l.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if att.State == chat.AttLoading {
				return layout.Dimensions{}
			}
			return layout.Inset{Top: 6, Bottom: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				label := "Bild laden"
				if att.Sticker {
					label = "Sticker laden"
				}
				b := material.Button(a.th, click, label)
				b.Background = colorLavender
				b.CornerRadius = 14
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

// pasteImageFromClipboard holt ein Bild aus dem Clipboard und sendet es.
// Wird mit gehaltenem Lock aus dem Frame aufgerufen.
func (a *App) pasteImageFromClipboard() {
	conv, client := a.store.Active, a.client
	if conv == nil || client == nil || conv.Kind == chat.Server {
		return
	}
	_, data, ok := ClipboardImage()
	if !ok {
		a.store.Sys(conv, "Kein Bild im Clipboard gefunden.")
		return
	}
	go a.sendImage(conv, client, "clipboard.png", data)
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
	a.sendImageFileKind(conv, path, false)
}

func (a *App) sendStickerFile(conv *chat.Conversation, path string) {
	a.sendImageFileKind(conv, path, true)
}

func (a *App) sendImageFileKind(conv *chat.Conversation, path string, sticker bool) {
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
		a.sendImageKind(conv, client, filepath.Base(path), data, sticker)
	}()
}

// sendImage läuft in einer Goroutine (ohne Lock) und blockiert, bis die Übertragung endet.
func (a *App) sendImage(conv *chat.Conversation, client *irc.Client, name string, data []byte) {
	a.sendImageKind(conv, client, name, data, false)
}

func (a *App) sendImageKind(conv *chat.Conversation, client *irc.Client, name string, data []byte, sticker bool) {
	name = chat.SafeName(name)
	img, err := transfer.DecodeImage(data)
	if err != nil {
		a.note(conv, "Bild nicht gesendet: "+err.Error())
		return
	}

	att := &chat.Attachment{
		Name:     name,
		Image:    img,
		Outgoing: true,
		Sticker:  sticker,
		State:    chat.AttLoading,
	}

	a.mu.Lock()
	if a.client != client {
		a.mu.Unlock()
		return
	}
	enc := a.secure(conv)
	a.store.Post(conv, chat.Message{
		Sender: client.Nick(), Text: "Bild: " + name, Mine: true, Enc: enc, Attachment: att,
	})
	a.window.Invalidate()
	a.mu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
		defer cancel()
		code, done, err := transfer.Send(ctx, name, data)

		a.mu.Lock()
		if err != nil {
			att.State, att.Err = chat.AttFailed, "Wormhole: "+err.Error()
			a.mu.Unlock()
			a.window.Invalidate()
			return
		}

		att.Code = code
		att.State = chat.AttWaiting
		wireText := chat.FormatAttachment(code, name)
		if sticker {
			wireText = chat.FormatStickerAttachment(code, name)
		}
		a.deliver(conv, client, wireText)
		if conv.Kind == chat.Channel {
			a.store.Sys(conv, "Hinweis: Ein Wormhole-Code gilt nur einmal – nur der erste Empfänger bekommt das Bild.")
		}
		a.mu.Unlock()
		a.window.Invalidate()

		select {
		case err := <-done:
			a.mu.Lock()
			if err != nil {
				att.State, att.Err = chat.AttFailed, err.Error()
			} else {
				att.State = chat.AttDelivered
			}
			a.mu.Unlock()
			a.window.Invalidate()
		case <-time.After(10 * time.Minute):
			a.mu.Lock()
			if att.State == chat.AttWaiting {
				att.State, att.Err = chat.AttFailed, "Zeitüberschreitung beim Warten auf Abholung"
			}
			a.mu.Unlock()
			a.window.Invalidate()
		}
	}()
}

// sendImageURL sends a direct media link and immediately renders it locally.
// The caller holds a.mu, matching deliver and sendImageFile.
func (a *App) sendImageURL(conv *chat.Conversation, client *irc.Client, u string, preview image.Image) error {
	att := chat.ParseAttachment(u)
	if att == nil {
		return fmt.Errorf("ununterstützter Bild-Link")
	}
	if preview != nil {
		setCachedImage(u, preview)
		att.Image = preview
		att.State = chat.AttReady
	} else if cached, ok := getCachedImage(u); ok {
		att.Image = cached
		att.State = chat.AttReady
	}
	att.Sticker = true
	enc := a.secure(conv)
	a.deliver(conv, client, u)
	a.store.Post(conv, chat.Message{
		Sender: client.Nick(), Text: "Bild: " + att.Name, Mine: true, Enc: enc, Attachment: att,
	})
	a.window.Invalidate()
	return nil
}

func (a *App) receiveURL(att *chat.Attachment) {
	if att.State == chat.AttLoading {
		return
	}
	if cached, ok := getCachedImage(att.URL); ok {
		att.Image = cached
		att.State = chat.AttReady
		return
	}
	att.State, att.Err = chat.AttLoading, ""
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		img, err := downloadImageURL(ctx, att.URL)

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

func downloadImageURL(ctx context.Context, u string) (image.Image, error) {
	if cached, ok := getCachedImage(u); ok {
		return cached, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "urineless/0.1")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	setCachedImage(u, img)
	return img, nil
}
