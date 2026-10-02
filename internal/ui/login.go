package ui

import (
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

func (a *App) loginScreen(gtx layout.Context) layout.Dimensions {
	if a.connectBtn.Clicked(gtx) {
		a.startConnect()
	}
	enter := false
	for _, ed := range []*widget.Editor{&a.serverEd, &a.nickEd, &a.passEd, &a.chansEd, &a.keyEd} {
		if submitted(gtx, ed) {
			enter = true
		}
	}
	if enter {
		a.startConnect()
	}

	gtx.Constraints.Min = gtx.Constraints.Max
	return FillBG(gtx, colorLoginBG, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = gtx.Constraints.Max
		return material.List(a.th, &a.loginList).Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: 28, Bottom: 28, Left: 16, Right: 16}.Layout(gtx, a.loginCard)
			})
		})
	})
}

func (a *App) loginCard(gtx layout.Context) layout.Dimensions {
	th := a.th
	gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(420))
	gtx.Constraints.Min.X = gtx.Constraints.Max.X

	return Pill(gtx, colorWhite, 32, func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(28).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return Logo(gtx, 96) }),
				layout.Rigid(layout.Spacer{Height: 10}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					t := material.H4(th, "urineless")
					t.Font.Weight = font.Bold
					t.Color = colorAccent
					return t.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					s := material.Body2(th, "privates IRC, nix läuft aus uwu")
					s.Color = colorMuted
					return s.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: 22}.Layout),

				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return Field(gtx, material.Editor(th, &a.serverEd, "Server (irc.libera.chat:6697)").Layout)
				}),
				layout.Rigid(layout.Spacer{Height: 10}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return Field(gtx, material.Editor(th, &a.nickEd, "Nickname").Layout)
				}),
				layout.Rigid(layout.Spacer{Height: 10}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return Field(gtx, material.Editor(th, &a.passEd, "NickServ-Passwort (optional)").Layout)
				}),
				layout.Rigid(layout.Spacer{Height: 10}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return Field(gtx, material.Editor(th, &a.chansEd, "Kanäle (#libera, #foo)").Layout)
				}),
				layout.Rigid(layout.Spacer{Height: 10}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return Field(gtx, material.Editor(th, &a.keyEd, "Schlüssel-Passwort (E2E, lokal)").Layout)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: 4, Left: 6, Right: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						c := material.Caption(th, "Schützt deine Verschlüsselungs-Schlüssel auf der Festplatte. Merk es dir gut!")
						c.Color = colorMuted
						return c.Layout(gtx)
					})
				}),
				layout.Rigid(layout.Spacer{Height: 10}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return material.CheckBox(th, &a.tlsBox, "TLS verwenden").Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: 14}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					label := "Verbinden"
					if a.connecting {
						label = "verbinde ..."
					}
					b := material.Button(th, &a.connectBtn, label)
					b.CornerRadius = unit.Dp(22)
					b.Inset = layout.Inset{Top: 13, Bottom: 13, Left: 18, Right: 18}
					return b.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if a.loginErr == "" {
						return layout.Dimensions{}
					}
					return layout.Inset{Top: 12}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						e := material.Body2(th, a.loginErr)
						e.Color = colorDanger
						return e.Layout(gtx)
					})
				}),
			)
		})
	})
}
