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
	if a.picker.open && gtx.Constraints.Max.X < gtx.Dp(560) {
		return a.chatPanel(gtx)
	}
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Rigid(a.sidebar),
		layout.Flexed(1, a.chatPanel),
	)
}

func (a *App) sidebar(gtx layout.Context) layout.Dimensions {
	w := min(gtx.Dp(300), max(gtx.Dp(180), gtx.Constraints.Max.X*32/100))
	gtx.Constraints.Min = image.Pt(w, gtx.Constraints.Max.Y)
	gtx.Constraints.Max.X = w

	return FillBG(gtx, colorSidebar, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(a.brandBar),
			layout.Rigid(a.joinBar),
			layout.Flexed(1, a.convListView),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return HLine(gtx, colorLine)
			}),
			layout.Rigid(a.accountBar),
		)
	})
}

// brandBar: Logo + Name oben in der Sidebar.
func (a *App) brandBar(gtx layout.Context) layout.Dimensions {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Inset{Top: 18, Bottom: 12, Left: 16, Right: 16}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return Logo(gtx, 36) }),
			layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						l := material.H6(a.th, "urineless")
						l.Font.Weight = font.Bold
						l.Color = colorTextMain
						return l.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						l := material.Caption(a.th, "private IRC")
						l.Color = colorMuted
						return l.Layout(gtx)
					}),
				)
			}),
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
				return Pill(gtx, colorField, 20, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: 9, Bottom: 9, Left: 16, Right: 16}.Layout(gtx,
						material.Editor(a.th, &a.joinEd, "#kanal / nick ...").Layout)
				})
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				b := material.Button(a.th, &a.joinBtn, "+")
				b.Background = colorAccent
				b.CornerRadius = unit.Dp(16)
				b.Inset = layout.Inset{Top: 8, Bottom: 8, Left: 12, Right: 12}
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

	return layout.Inset{Top: 2, Bottom: 2, Left: 8, Right: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			bg := colorSidebar
			if active {
				bg = colorHover
			} else if click.Hovered() {
				bg = colorHover
			}
			nameCol, subCol := colorTextMain, colorMuted
			if active {
				nameCol, subCol = colorWhite, colorWhite
			}

			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return Pill(gtx, bg, 18, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: 8, Bottom: 8, Left: 10, Right: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return Avatar(gtx, a.th, c.Name, 44)
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
				l.Color = colorTextMain
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
