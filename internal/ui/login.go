package ui

import (
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

func (a *App) loginScreen(gtx layout.Context) layout.Dimensions {
	if a.connectBtn.Clicked(gtx) && !a.connecting {
		a.startConnect()
	}

	gtx.Constraints.Min = gtx.Constraints.Max
	paint.FillShape(gtx.Ops, colorLoginBG, clip.Rect{Max: gtx.Constraints.Max}.Op())
	return layout.Center.Layout(gtx, a.loginCard)
}

func (a *App) loginCard(gtx layout.Context) layout.Dimensions {
	w := min(gtx.Dp(380), gtx.Constraints.Max.X-gtx.Dp(32))
	gtx.Constraints.Min.X = w
	gtx.Constraints.Max.X = w

	gap := layout.Spacer{Height: unit.Dp(10)}.Layout

	return Pill(gtx, colorWhite, 16, func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(24).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return Avatar(gtx, a.th, "IRC", 88)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(14)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					l := material.H5(a.th, "IRCgram")
					l.Font.Weight = font.Bold
					return l.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					l := material.Body2(a.th, "Melde dich bei einem IRC-Server an")
					l.Color = colorMuted
					return l.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(20)}.Layout),

				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return Field(gtx, material.Editor(a.th, &a.serverEd, "Server:Port").Layout)
				}),
				layout.Rigid(gap),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return Field(gtx, material.Editor(a.th, &a.nickEd, "Nickname").Layout)
				}),
				layout.Rigid(gap),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return Field(gtx, material.Editor(a.th, &a.chansEd, "Kanäle (#a, #b)").Layout)
				}),
				layout.Rigid(gap),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return material.CheckBox(a.th, &a.tlsBox, "TLS verwenden").Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if a.loginErr == "" {
						return layout.Dimensions{}
					}
					return layout.Inset{Top: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						l := material.Body2(a.th, a.loginErr)
						l.Color = colorDanger
						return l.Layout(gtx)
					})
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(16)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					label := "Verbinden"
					if a.connecting {
						label = "Verbinde …"
						gtx = gtx.Disabled()
					}
					b := material.Button(a.th, &a.connectBtn, label)
					b.CornerRadius = unit.Dp(10)
					b.Inset = layout.UniformInset(12)
					return b.Layout(gtx)
				}),
			)
		})
	})
}
