package ui

import (
	"image"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"ircgram/internal/chat"
)

func (a *App) mainScreen(gtx layout.Context) layout.Dimensions {
	if a.disconnectBtn.Clicked(gtx) {
		a.disconnect()
		return a.loginScreen(gtx)
	}
	gtx.Constraints.Min = gtx.Constraints.Max
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Rigid(a.sidebar),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return VLine(gtx, colorLine)
		}),
		layout.Flexed(1, a.chatPanel),
	)
}

func (a *App) sidebar(gtx layout.Context) layout.Dimensions {
	w := gtx.Dp(320)
	gtx.Constraints.Min = image.Pt(w, gtx.Constraints.Max.Y)
	gtx.Constraints.Max.X = w

	return FillBG(gtx, colorSidebar, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(a.joinBar),
			layout.Flexed(1, a.convListView),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return HLine(gtx, colorLine)
			}),
			layout.Rigid(a.accountBar),
		)
	})
}

// joinBar: Suchfeld-ähnliche Zeile zum Betreten von Kanälen / Öffnen von Privatchats.
func (a *App) joinBar(gtx layout.Context) layout.Dimensions {
	if a.joinBtn.Clicked(gtx) || submitted(gtx, &a.joinEd) {
		a.openOrJoin(a.joinEd.Text())
		a.joinEd.SetText("")
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.UniformInset(10).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return Pill(gtx, colorField, 100, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: 9, Bottom: 9, Left: 16, Right: 16}.Layout(gtx,
						material.Editor(a.th, &a.joinEd, "#kanal betreten / Nick öffnen").Layout)
				})
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				b := material.Button(a.th, &a.joinBtn, "+")
				b.CornerRadius = unit.Dp(20)
				b.Inset = layout.Inset{Top: 8, Bottom: 8, Left: 14, Right: 14}
				return b.Layout(gtx)
			}),
		)
	})
}

func (a *App) convListView(gtx layout.Context) layout.Dimensions {
	convs := a.store.Convs
	return material.List(a.th, &a.convList).Layout(gtx, len(convs), func(gtx layout.Context, i int) layout.Dimensions {
		return a.convRow(gtx, convs[i])
	})
}

func (a *App) convRow(gtx layout.Context, c *chat.Conversation) layout.Dimensions {
	click := a.click(c.Key())
	if click.Clicked(gtx) {
		a.store.Select(c)
		a.focusInput = true
		a.window.Invalidate()
	}
	active := c == a.store.Active

	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		bg := colorSidebar
		if active {
			bg = colorAccent
		} else if click.Hovered() {
			bg = colorHover
		}
		nameCol, subCol := colorTextMain, colorMuted
		if active {
			nameCol, subCol = colorWhite, colorWhite
		}

		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return FillBG(gtx, bg, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.Inset{Top: 8, Bottom: 8, Left: 12, Right: 12}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return Avatar(gtx, a.th, c.Name, 50)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						last, hasLast := c.Last()
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
									layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
										l := material.Body1(a.th, c.Name)
										l.Font.Weight = font.Bold
										l.Color = nameCol
										l.MaxLines = 1
										return l.Layout(gtx)
									}),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										if !hasLast {
											return layout.Dimensions{}
										}
										l := material.Caption(a.th, last.Time.Format("15:04"))
										l.Color = subCol
										return l.Layout(gtx)
									}),
								)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
									layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
										l := material.Body2(a.th, preview(last, hasLast))
										l.Color = subCol
										l.MaxLines = 1
										return l.Layout(gtx)
									}),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										if c.Unread == 0 {
											return layout.Dimensions{}
										}
										return layout.Inset{Left: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
											return Badge(gtx, a.th, c.Unread, colorAccent)
										})
									}),
								)
							}),
						)
					}),
				)
			})
		})
	})
}

func preview(m chat.Message, ok bool) string {
	if !ok {
		return "Noch keine Nachrichten"
	}
	t := strings.ReplaceAll(m.Text, "\n", " ")
	if m.Attachment != nil {
		t = "Bild: " + m.Attachment.Name
	}
	if m.Sender != "" && !m.System {
		if m.Mine {
			return "Du: " + t
		}
		return m.Sender + ": " + t
	}
	return t
}

func (a *App) accountBar(gtx layout.Context) layout.Dimensions {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.UniformInset(10).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return Avatar(gtx, a.th, a.client.Nick(), 36)
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				l := material.Body1(a.th, a.client.Nick())
				l.MaxLines = 1
				return l.Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				b := material.Button(a.th, &a.disconnectBtn, "Trennen")
				b.Background = colorDanger
				b.CornerRadius = unit.Dp(16)
				b.Inset = layout.Inset{Top: 6, Bottom: 6, Left: 12, Right: 12}
				return b.Layout(gtx)
			}),
		)
	})
}
