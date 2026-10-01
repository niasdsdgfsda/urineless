package ui

import (
	"image/color"

	"gioui.org/font/gofont"
	"gioui.org/text"
	"gioui.org/widget/material"
)

var (
	colorAccent    = color.NRGBA{R: 0x33, G: 0x90, B: 0xEC, A: 0xFF}
	colorSidebar   = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	colorHover     = color.NRGBA{R: 0xF1, G: 0xF3, B: 0xF4, A: 0xFF}
	colorChatBG    = color.NRGBA{R: 0xD7, G: 0xE3, B: 0xD0, A: 0xFF}
	colorIncoming  = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	colorOutgoing  = color.NRGBA{R: 0xEF, G: 0xFD, B: 0xDE, A: 0xFF}
	colorField     = color.NRGBA{R: 0xF1, G: 0xF3, B: 0xF4, A: 0xFF}
	colorMuted     = color.NRGBA{R: 0x7F, G: 0x8B, B: 0x95, A: 0xFF}
	colorLine      = color.NRGBA{R: 0xE3, G: 0xE6, B: 0xE8, A: 0xFF}
	colorSystemBG  = color.NRGBA{R: 0x6E, G: 0x87, B: 0x69, A: 0xFF}
	colorWhite     = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	colorDanger    = color.NRGBA{R: 0xE5, G: 0x39, B: 0x35, A: 0xFF}
	colorLoginBG   = color.NRGBA{R: 0xF4, G: 0xF6, B: 0xF8, A: 0xFF}
	colorTextMain  = color.NRGBA{R: 0x11, G: 0x11, B: 0x11, A: 0xFF}
	avatarPalette  = []color.NRGBA{
		{R: 0xE1, G: 0x70, B: 0x76, A: 0xFF},
		{R: 0xE5, G: 0x94, B: 0x4B, A: 0xFF},
		{R: 0x9B, G: 0x7C, B: 0xD6, A: 0xFF},
		{R: 0x4C, G: 0xAF, B: 0x6E, A: 0xFF},
		{R: 0x41, G: 0xB1, B: 0xC4, A: 0xFF},
		{R: 0x4F, G: 0x9B, B: 0xE0, A: 0xFF},
		{R: 0xD9, G: 0x6C, B: 0xA8, A: 0xFF},
	}
)

func newTheme() *material.Theme {
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	th.Palette.Fg = colorTextMain
	th.Palette.Bg = colorWhite
	th.Palette.ContrastBg = colorAccent
	th.Palette.ContrastFg = colorWhite
	return th
}

func hashColor(s string) color.NRGBA {
	h := 0
	for _, r := range s {
		h = h*31 + int(r)
	}
	if h < 0 {
		h = -h
	}
	return avatarPalette[h%len(avatarPalette)]
}
