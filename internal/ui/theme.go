package ui

import (
	"image/color"

	"gioui.org/font/gofont"
	"gioui.org/text"
	"gioui.org/widget/material"
)

// Pastell-Palette: Bubblegum-Pink, Lavendel, Himmelblau.
var (
	colorAccent   = color.NRGBA{R: 0x2E, G: 0xA8, B: 0xFF, A: 0xFF}
	colorLavender = color.NRGBA{R: 0x4C, G: 0x8D, B: 0xFF, A: 0xFF}
	colorSidebar  = color.NRGBA{R: 0x11, G: 0x1C, B: 0x27, A: 0xFF}
	colorHover    = color.NRGBA{R: 0x1F, G: 0x2B, B: 0x39, A: 0xFF}
	colorChatBG   = color.NRGBA{R: 0x0E, G: 0x16, B: 0x21, A: 0xFF}
	colorIncoming = color.NRGBA{R: 0x1D, G: 0x2A, B: 0x36, A: 0xFF}
	colorOutgoing = color.NRGBA{R: 0x2B, G: 0x52, B: 0x78, A: 0xFF}
	colorField    = color.NRGBA{R: 0x1D, G: 0x2A, B: 0x36, A: 0xFF}
	colorMuted    = color.NRGBA{R: 0x8A, G: 0x9A, B: 0xB1, A: 0xFF}
	colorLine     = color.NRGBA{R: 0x21, G: 0x2E, B: 0x3C, A: 0xFF}
	colorSystemBG = color.NRGBA{R: 0x52, G: 0x5D, B: 0x6D, A: 0xFF}
	colorWhite    = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	colorDanger   = color.NRGBA{R: 0xFF, G: 0x63, B: 0x63, A: 0xFF}
	colorOK       = color.NRGBA{R: 0x4A, G: 0xD0, B: 0x97, A: 0xFF}
	colorStickerCaption = color.NRGBA{R: 0xAF, G: 0xBB, B: 0xD1, A: 0xD8}
	colorLoginBG  = color.NRGBA{R: 0x0B, G: 0x12, B: 0x1A, A: 0xFF}
	colorTextMain = color.NRGBA{R: 0xF3, G: 0xF7, B: 0xFC, A: 0xFF}

	// Logo
	colorLogo  = color.NRGBA{R: 0x60, G: 0xB5, B: 0xFF, A: 0xFF}
	colorShine = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xB0}
	colorFace  = color.NRGBA{R: 0xF3, G: 0xF7, B: 0xFC, A: 0xFF}
	colorCheek = color.NRGBA{R: 0x7D, G: 0xD7, B: 0xFF, A: 0xE0}

	avatarPalette = []color.NRGBA{
		{R: 0x2E, G: 0xA8, B: 0xFF, A: 0xFF},
		{R: 0x6C, G: 0x7B, B: 0xFF, A: 0xFF},
		{R: 0x52, G: 0xC7, B: 0xA7, A: 0xFF},
		{R: 0xF6, G: 0xB1, B: 0x4E, A: 0xFF},
		{R: 0xD9, G: 0x6C, B: 0xD0, A: 0xFF},
		{R: 0x5D, G: 0xD2, B: 0xFF, A: 0xFF},
		{R: 0xF0, G: 0x6A, B: 0xA8, A: 0xFF},
	}
)

func newTheme() *material.Theme {
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	th.Palette.Fg = colorTextMain
	th.Palette.Bg = colorChatBG
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

var colorBackdrop = color.NRGBA{R: 0x2A, G: 0x1B, B: 0x30, A: 0xB0}

var colorShadow = color.NRGBA{R: 0x4A, G: 0x35, B: 0x50, A: 0x40}
