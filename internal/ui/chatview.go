package ui

import (
	"strconv"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"ircgram/internal/chat"
)

func (a *App) chatPanel(gtx layout.Context) layout.Dimensions {
	gtx.Constraints.Min = gtx.Constraints.Max
	conv := a.store.Active

	return FillBG(gtx, colorChatBG, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.chatHeader(gtx, conv) }),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return a.messageList(gtx, conv) }),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.inputBar(gtx, conv) }),
		)
	})
}

func (a *App) chatHeader(gtx layout.Context, conv *chat.Conversation) layout.Dimensions {
	subtitle := map[chat.Kind]string{
		chat.Server:  "Serverprotokoll",
		chat.Channel: "Kanal",
		chat.Private: "Privatchat",
	}[conv.Kind]
	if conv.Kind == chat.Channel {
		subtitle += " · " + strconv.Itoa(conv.MemberCount()) + " Mitglieder"
	}

	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return FillBG(gtx, colorWhite, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return layout.Inset{Top: 8, Bottom: 8, Left: 14, Right: 14}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return Avatar(gtx, a.th, conv.Name, 40)
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									l := material.Body1(a.th, conv.Name)
									l.Font.Weight = font.Bold
									l.MaxLines = 1
									return l.Layout(gtx)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									l := material.Caption(a.th, subtitle)
									l.Color = colorMuted
									return l.Layout(gtx)
								}),
							)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.e2eBadge(gtx, conv)
						}),
					)
				})
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return HLine(gtx, colorLine) }),
	)
}

// e2eBadge zeigt grün "e2e an" oder rot "ohne E2E".
func (a *App) e2eBadge(gtx layout.Context, conv *chat.Conversation) layout.Dimensions {
	if conv.Kind == chat.Server {
		return layout.Dimensions{}
	}
	txt, col := "e2e an", colorOK
	if !a.secure(conv) {
		txt, col = "ohne E2E", colorDanger
	}
	return Pill(gtx, col, 100, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: 3, Bottom: 3, Left: 10, Right: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			l := material.Caption(a.th, txt)
			l.Color = colorWhite
			l.Font.Weight = font.Bold
			return l.Layout(gtx)
		})
	})
}

func (a *App) messageList(gtx layout.Context, conv *chat.Conversation) layout.Dimensions {
	list := a.listFor(conv)
	msgs := conv.Messages

	if len(msgs) == 0 && conv.Kind != chat.Server {
		gtx.Constraints.Min = gtx.Constraints.Max
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			l := material.Body1(a.th, "noch nichts los hier ... schreib was nettes >w<")
			l.Color = colorMuted
			return l.Layout(gtx)
		})
	}

	return layout.Inset{Top: 4, Bottom: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return material.List(a.th, list).Layout(gtx, len(msgs), func(gtx layout.Context, i int) layout.Dimensions {
			m := msgs[i]
			return layout.Inset{Top: 3, Bottom: 3, Left: 16, Right: 16}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				if m.System {
					return SystemNote(gtx, a.th, m.Text)
				}
				showName := !m.Mine && conv.Kind == chat.Channel &&
					(i == 0 || msgs[i-1].System || msgs[i-1].Sender != m.Sender)

				tag := ""
				switch {
				case m.Enc:
					tag = "e2e"
				case !m.Mine && a.secure(conv):
					tag = "ohne E2E!"
				}

				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				dir := layout.W
				if m.Mine {
					dir = layout.E
				}
				var body layout.Widget
				if m.Attachment != nil {
					body = a.attachmentBody(m.Attachment)
				}
				return dir.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return Bubble(gtx, a.th, m, showName, tag, body)
				})
			})
		})
	})
}

func (a *App) inputBar(gtx layout.Context, conv *chat.Conversation) layout.Dimensions {
	a.requestFocus(gtx)

	if a.imgBtn.Clicked(gtx) {
		a.pickImage()
	}
	send := a.sendBtn.Clicked(gtx)
	if submitted(gtx, &a.msgEd) {
		send = true
	}
	if send {
		text := strings.TrimSpace(a.msgEd.Text())
		a.msgEd.SetText("")
		if text != "" {
			a.submit(text)
		}
	}

	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return FillBG(gtx, colorWhite, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.UniformInset(10).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					b := material.Button(a.th, &a.imgBtn, "Bild")
					b.Background = colorLavender
					b.CornerRadius = unit.Dp(20)
					b.Inset = layout.Inset{Top: 10, Bottom: 10, Left: 16, Right: 16}
					return b.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return Pill(gtx, colorField, 100, func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Top: 10, Bottom: 10, Left: 18, Right: 18}.Layout(gtx,
							material.Editor(a.th, &a.msgEd, "Nachricht ...").Layout)
					})
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					b := material.Button(a.th, &a.sendBtn, "Senden")
					b.CornerRadius = unit.Dp(20)
					b.Inset = layout.Inset{Top: 10, Bottom: 10, Left: 18, Right: 18}
					return b.Layout(gtx)
				}),
			)
		})
	})
}
