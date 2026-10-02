package ui

import (
	"image"
	"image/color"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// viewer ist der Vollbild-Bildbetrachter mit Zoom & Pan.
type viewer struct {
	open   bool
	img    paint.ImageOp
	imgSz  image.Point
	scale  float32 // 1.0 = eingepasst
	offset f32.Point
	prev   f32.Point
	tag    struct{} // pointer.Tag
}

func (v *viewer) Show(op paint.ImageOp, sz image.Point) {
	v.open = true
	v.img = op
	v.imgSz = sz
	v.scale = 1
	v.offset = f32.Point{}
}

func (v *viewer) Hide() { v.open = false }

// handle verarbeitet Zoom/Pan/Close. Wird im Frame aufgerufen.
func (v *viewer) handle(gtx layout.Context) {
	if !v.open {
		return
	}
	if ev, ok := gtx.Event(key.Filter{Name: key.NameEscape}); ok {
		if _, ok := ev.(key.Event); ok {
			v.Hide()
			return
		}
	}
	if ev, ok := gtx.Event(pointer.Filter{
		Target: &v.tag,
		Kinds:  pointer.Scroll | pointer.Press | pointer.Drag | pointer.Release,
	}); ok {
		if pe, ok := ev.(pointer.Event); ok {
			switch pe.Kind {
			case pointer.Scroll:
				v.scale *= 1 + pe.Scroll.Y*0.1
				if v.scale < 0.2 {
					v.scale = 0.2
				}
				if v.scale > 8 {
					v.scale = 8
				}
			case pointer.Press:
				v.prev = pe.Position
			case pointer.Drag:
				v.offset = v.offset.Add(pe.Position.Sub(v.prev))
				v.prev = pe.Position
			}
		}
	}
}

func (v *viewer) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if !v.open {
		return layout.Dimensions{}
	}

	sz := gtx.Constraints.Max
	panelW := sz.X * 62 / 100
	panelH := sz.Y * 78 / 100
	if panelW > 900 {
		panelW = 900
	}
	if panelH > 760 {
		panelH = 760
	}
	if panelW < 240 {
		panelW = 240
	}
	if panelH < 220 {
		panelH = 220
	}
	panelX := sz.X - panelW - 28
	panelY := (sz.Y - panelH) / 2

	paint.FillShape(gtx.Ops, color.NRGBA{0, 0, 0, 0xD8}, clip.Rect{Max: sz}.Op())

	panel := image.Rect(panelX, panelY, panelX+panelW, panelY+panelH)
	paint.FillShape(gtx.Ops, color.NRGBA{R: 0x14, G: 0x18, B: 0x1F, A: 0xE6}, clip.UniformRRect(panel, 18).Op(gtx.Ops))

	center := f32.Pt(float32(panelX)+float32(panelW)/2, float32(panelY)+float32(panelH)/2)
	fitW := float32(panelW) * 0.92 / float32(v.imgSz.X)
	fitH := float32(panelH) * 0.9 / float32(v.imgSz.Y)
	fit := fitW
	if fitH < fit {
		fit = fitH
	}
	total := fit * v.scale
	w := int(float32(v.imgSz.X) * total)
	h := int(float32(v.imgSz.Y) * total)
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}

	pos := image.Point{
		X: int(center.X+v.offset.X) - w/2,
		Y: int(center.Y+v.offset.Y) - h/2,
	}
	stack := op.Offset(pos).Push(gtx.Ops)
	gtx.Constraints = layout.Exact(image.Pt(w, h))
	widget.Image{Src: v.img, Fit: widget.ScaleDown, Scale: 1}.Layout(gtx)
	stack.Pop()

	area := clip.Rect{Max: sz}.Push(gtx.Ops)
	event.Op(gtx.Ops, &v.tag)
	area.Pop()

	hint := material.Caption(th, "Mausrad = Zoom · Ziehen = verschieben · ESC = schließen")
	hint.Color = colorWhite
	layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Right: unit.Dp(36), Bottom: unit.Dp(20)}.Layout(gtx, hint.Layout)
	})

	return layout.Dimensions{Size: sz}
}
