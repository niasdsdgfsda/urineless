package ui

import (
	"image"
	"image/color"
	"strings"

	"gioui.org/f32"
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

// Logo zeichnet den urineless-Tropfen mit uwu-Gesicht (reine Vektorgrafik, keine Assets).
func Logo(gtx layout.Context, size unit.Dp) layout.Dimensions {
	s := float32(gtx.Dp(size))
	pt := func(x, y float32) f32.Point { return f32.Pt(x*s, y*s) }
	dot := func(cx, cy, rx, ry float32, col color.NRGBA) {
		rect := image.Rect(int((cx-rx)*s), int((cy-ry)*s), int((cx+rx)*s), int((cy+ry)*s))
		paint.FillShape(gtx.Ops, col, clip.Ellipse(rect).Op(gtx.Ops))
	}

	// Tropfen
	var p clip.Path
	p.Begin(gtx.Ops)
	p.MoveTo(pt(.50, .02))
	p.CubeTo(pt(.60, .30), pt(.92, .48), pt(.92, .66))
	p.CubeTo(pt(.92, .86), pt(.74, .98), pt(.50, .98))
	p.CubeTo(pt(.26, .98), pt(.08, .86), pt(.08, .66))
	p.CubeTo(pt(.08, .48), pt(.40, .30), pt(.50, .02))
	p.Close()
	paint.FillShape(gtx.Ops, colorLogo, clip.Outline{Path: p.End()}.Op())

	// Glanzpunkt, Bäckchen, Augen
	dot(.30, .55, .045, .075, colorShine)
	dot(.25, .74, .07, .045, colorCheek)
	dot(.75, .74, .07, .045, colorCheek)
	dot(.37, .64, .045, .06, colorFace)
	dot(.63, .64, .045, .06, colorFace)

	// "w"-Mund
	var m clip.Path
	m.Begin(gtx.Ops)
	m.MoveTo(pt(.43, .72))
	m.QuadTo(pt(.465, .80), pt(.50, .72))
	m.QuadTo(pt(.535, .80), pt(.57, .72))
	paint.FillShape(gtx.Ops, colorFace, clip.Stroke{Path: m.End(), Width: s * 0.025}.Op())

	return layout.Dimensions{Size: image.Pt(int(s), int(s))}
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

// Badge ist der Ungelesen-Zähler.
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

// Bubble zeichnet eine Chat-Blase. tag steht vor der Uhrzeit (z. B. "e2e").
func Bubble(gtx layout.Context, th *material.Theme, m chat.Message, showName bool, tag string, body layout.Widget) layout.Dimensions {
	if body == nil {
		l := material.Body1(th, m.Text)
		l.Color = colorTextMain
		if m.Mine {
			l.Color = colorWhite
		}
		body = l.Layout
	}
	bg := colorIncoming
	if m.Mine {
		bg = colorOutgoing
	}
	foot := m.Time.Format("15:04")
	if tag != "" {
		foot = tag + " · " + foot
	}
	maxW := max(gtx.Constraints.Max.X*72/100, min(gtx.Constraints.Max.X, gtx.Dp(160)))
	gtx.Constraints.Min = image.Point{}
	gtx.Constraints.Max.X = maxW

	return Pill(gtx, bg, 18, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: 7, Bottom: 5, Left: 12, Right: 12}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
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
							l := material.Caption(th, foot)
							l.Color = colorMuted
							if m.Mine {
								l.Color = colorWhite
							}
							return l.Layout(gtx)
						}),
					)
				}),
			)
		})
	})
}

func StickerPost(gtx layout.Context, th *material.Theme, m chat.Message, showName bool, tag string, body layout.Widget) layout.Dimensions {
	if body == nil {
		l := material.Body1(th, m.Text)
		l.Color = colorTextMain
		if m.Mine {
			l.Color = colorWhite
		}
		body = l.Layout
	}
	maxW := min(gtx.Constraints.Max.X, gtx.Dp(320))
	gtx.Constraints.Max.X = maxW
	gtx.Constraints.Min = image.Point{}
	alignment := layout.Start
	if m.Mine {
		alignment = layout.End
	}
	return layout.Inset{Top: 1, Bottom: 1}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical, Alignment: alignment}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if !showName {
					return layout.Dimensions{}
				}
				l := material.Caption(th, m.Sender)
				l.Color = hashColor(m.Sender)
				l.Font.Weight = font.Bold
				return l.Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: 0, Bottom: 0, Left: 0, Right: 0}.Layout(gtx, body)
			}),
		)
	})
}

// SystemNote ist die zentrierte Pille für Join/Part/Server-Meldungen.
func SystemNote(gtx layout.Context, th *material.Theme, text string) layout.Dimensions {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Max.X = gtx.Constraints.Max.X * 9 / 10
		return Pill(gtx, colorSystemBG, 100, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: 3, Bottom: 3, Left: 12, Right: 12}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				l := material.Caption(th, text)
				l.Color = colorWhite
				return l.Layout(gtx)
			})
		})
	})
}

// Field ist ein Eingabefeld im Pillen-Look über die volle Breite.
func Field(gtx layout.Context, w layout.Widget) layout.Dimensions {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return Pill(gtx, colorField, 22, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: 11, Bottom: 11, Left: 18, Right: 18}.Layout(gtx, w)
	})
}
