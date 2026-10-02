package ui

import (
	"image/color"

	"gioui.org/font/gofont"
	"gioui.org/text"
	"gioui.org/widget/material"
)

// Pastell-Palette: Bubblegum-Pink, Lavendel, Himmelblau.
var (
	colorAccent   = color.NRGBA{R: 0xF0, G: 0x6A, B: 0xA8, A: 0xFF} // Bubblegum-Pink
	colorLavender = color.NRGBA{R: 0xA7, G: 0x8B, B: 0xE8, A: 0xFF}
	colorSidebar  = color.NRGBA{R: 0xFF, G: 0xF8, B: 0xFB, A: 0xFF}
	colorHover    = color.NRGBA{R: 0xFF, G: 0xE4, B: 0xF0, A: 0xFF}
	colorChatBG   = color.NRGBA{R: 0xF5, G: 0xED, B: 0xFF, A: 0xFF}
	colorIncoming = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	colorOutgoing = color.NRGBA{R: 0xFF, G: 0xD9, B: 0xEA, A: 0xFF}
	colorField    = color.NRGBA{R: 0xFF, G: 0xEC, B: 0xF4, A: 0xFF}
	colorMuted    = color.NRGBA{R: 0x9A, G: 0x84, B: 0xA6, A: 0xFF}
	colorLine     = color.NRGBA{R: 0xF4, G: 0xD8, B: 0xE6, A: 0xFF}
	colorSystemBG = color.NRGBA{R: 0xB3, G: 0x9C, B: 0xD9, A: 0xFF}
	colorWhite    = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	colorDanger   = color.NRGBA{R: 0xE8, G: 0x5A, B: 0x80, A: 0xFF}
	colorOK       = color.NRGBA{R: 0x3D, G: 0xB5, B: 0x89, A: 0xFF}
	colorLoginBG  = color.NRGBA{R: 0xFF, G: 0xE3, B: 0xF0, A: 0xFF}
	colorTextMain = color.NRGBA{R: 0x4A, G: 0x35, B: 0x50, A: 0xFF}

	// Logo
	colorLogo  = color.NRGBA{R: 0x8F, G: 0xCB, B: 0xFF, A: 0xFF} // Wassertropfen
	colorShine = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xB0}
	colorFace  = color.NRGBA{R: 0x4A, G: 0x35, B: 0x50, A: 0xFF}
	colorCheek = color.NRGBA{R: 0xFF, G: 0x9E, B: 0xC4, A: 0xE0}

	avatarPalette = []color.NRGBA{
		{R: 0xF0, G: 0x6A, B: 0xA8, A: 0xFF},
		{R: 0xA7, G: 0x8B, B: 0xE8, A: 0xFF},
		{R: 0x5B, G: 0xA8, B: 0xF0, A: 0xFF},
		{R: 0x3D, G: 0xB5, B: 0x89, A: 0xFF},
		{R: 0xF0, G: 0x9A, B: 0x4B, A: 0xFF},
		{R: 0xD9, G: 0x6C, B: 0xD0, A: 0xFF},
		{R: 0x41, G: 0xB1, B: 0xC4, A: 0xFF},
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
