package ui

import (
	"image"
	"image/color"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"ircgram/internal/chat"
)

// FillBG füllt den gesamten verfügbaren Bereich mit einer Farbe und legt w darüber.
func FillBG(gtx layout.Context, col color.NRGBA, w layout.Widget) layout.Dimensions {
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			paint.FillShape(gtx.Ops, col, clip.Rect{Max: gtx.Constraints.Min}.Op())
			return layout.Dimensions{Size: gtx.Constraints.Min}
		}),
		layout.Stacked(w),
	)
}

// Pill zeichnet einen abgerundeten Hintergrund exakt um den Inhalt.
func Pill(gtx layout.Context, col color.NRGBA, radius unit.Dp, w layout.Widget) layout.Dimensions {
	m := op.Record(gtx.Ops)
	dims := w(gtx)
	call := m.Stop()
	r := min(gtx.Dp(radius), dims.Size.X/2, dims.Size.Y/2)
	paint.FillShape(gtx.Ops, col,
		clip.UniformRRect(image.Rectangle{Max: dims.Size}, r).Op(gtx.Ops))
	call.Add(gtx.Ops)
	return dims
}

func HLine(gtx layout.Context, col color.NRGBA) layout.Dimensions {
	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(1))
	paint.FillShape(gtx.Ops, col, clip.Rect{Max: size}.Op())
	return layout.Dimensions{Size: size}
}

func VLine(gtx layout.Context, col color.NRGBA) layout.Dimensions {
	size := image.Pt(gtx.Dp(1), gtx.Constraints.Max.Y)
	paint.FillShape(gtx.Ops, col, clip.Rect{Max: size}.Op())
	return layout.Dimensions{Size: size}
}

func initials(name string) string {
	name = strings.TrimLeft(name, "#&@+")
	if name == "" {
		return "?"
	}
	return strings.ToUpper(string([]rune(name)[0]))
}

// Avatar zeichnet einen farbigen Kreis mit Initiale.
func Avatar(gtx layout.Context, th *material.Theme, name string, size unit.Dp) layout.Dimensions {
	sz := gtx.Dp(size)
	rect := image.Rect(0, 0, sz, sz)
	paint.FillShape(gtx.Ops, hashColor(name), clip.Ellipse(rect).Op(gtx.Ops))

	gtx.Constraints = layout.Exact(image.Pt(sz, sz))
	layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		l := material.Label(th, unit.Sp(float32(size)*0.42), initials(name))
		l.Color = colorWhite
		l.Font.Weight = font.Bold
		return l.Layout(gtx)
	})
	return layout.Dimensions{Size: image.Pt(sz, sz)}
}

// Badge ist der blaue Ungelesen-Zähler.
func Badge(gtx layout.Context, th *material.Theme, n int, bg color.NRGBA) layout.Dimensions {
	return Pill(gtx, bg, 100, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: 1, Bottom: 1, Left: 7, Right: 7}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			txt := "99+"
			if n < 100 {
				txt = itoa(n)
			}
			l := material.Caption(th, txt)
			l.Color = colorWhite
			l.Font.Weight = font.Bold
			return l.Layout(gtx)
		})
	})
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [8]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// Bubble zeichnet eine Chat-Blase mit optionalem Absendernamen und Uhrzeit.
func Bubble(gtx layout.Context, th *material.Theme, m chat.Message, showName bool, body layout.Widget) layout.Dimensions {
	if body == nil {
		body = material.Body1(th, m.Text).Layout
	}
	bg := colorIncoming
	if m.Mine {
		bg = colorOutgoing
	}
	maxW := max(gtx.Constraints.Max.X*72/100, min(gtx.Constraints.Max.X, gtx.Dp(160)))
	gtx.Constraints.Min = image.Point{}
	gtx.Constraints.Max.X = maxW

	return Pill(gtx, bg, 14, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: 6, Bottom: 4, Left: 10, Right: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if !showName {
						return layout.Dimensions{}
					}
					l := material.Body2(th, m.Sender)
					l.Font.Weight = font.Bold
					l.Color = hashColor(m.Sender)
					return l.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical, Alignment: layout.End}.Layout(gtx,
						layout.Rigid(body),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							l := material.Caption(th, m.Time.Format("15:04"))
							l.Color = colorMuted
							return l.Layout(gtx)
						}),
					)
				}),
			)
		})
	})
}

// SystemNote ist die zentrierte graue Pille für Join/Part/Server-Meldungen.
func SystemNote(gtx layout.Context, th *material.Theme, text string) layout.Dimensions {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Max.X = gtx.Constraints.Max.X * 9 / 10
		return Pill(gtx, colorSystemBG, 100, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: 3, Bottom: 3, Left: 10, Right: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				l := material.Caption(th, text)
				l.Color = colorWhite
				return l.Layout(gtx)
			})
		})
	})
}

// Field ist ein Eingabefeld im grauen Pillen-Look.
func Field(gtx layout.Context, w layout.Widget) layout.Dimensions {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return Pill(gtx, colorField, 10, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: 10, Bottom: 10, Left: 14, Right: 14}.Layout(gtx, w)
	})
}
