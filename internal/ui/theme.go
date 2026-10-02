package ui

import (
	"image/color"

	"gioui.org/font/gofont"
	"gioui.org/text"
	"gioui.org/widget/material"
)

// Midnight Purple & Indigo Theme (Deep Rich Glow)
var (
	colorAccent   = color.NRGBA{R: 0x9D, G: 0x4E, B: 0xDD, A: 0xFF} // Vibrant Neon Purple
	colorLavender = color.NRGBA{R: 0xB1, G: 0x85, B: 0xDB, A: 0xFF} // Electric Lavender
	colorSidebar  = color.NRGBA{R: 0x0C, G: 0x08, B: 0x13, A: 0xFF} // Deep Rich Violet-Black
	colorHover    = color.NRGBA{R: 0x24, G: 0x1B, B: 0x35, A: 0xFF} // Glowing Hover Violet
	colorChatBG   = color.NRGBA{R: 0x12, G: 0x0D, B: 0x1B, A: 0xFF} // Midnight Purple Chat BG
	colorIncoming = color.NRGBA{R: 0x1E, G: 0x15, B: 0x2F, A: 0xFF} // Rich Bubble
	colorOutgoing = color.NRGBA{R: 0x72, G: 0x09, B: 0xB7, A: 0xFF} // Glowing Indigo Outgoing
	colorField    = color.NRGBA{R: 0x1E, G: 0x15, B: 0x2F, A: 0xFF} // Input Field
	colorMuted    = color.NRGBA{R: 0x9D, G: 0x94, B: 0xB8, A: 0xFF} // Soft Muted Purple-Grey
	colorLine     = color.NRGBA{R: 0x2B, G: 0x1E, B: 0x40, A: 0xFF} // Subtle Border
	colorSystemBG = color.NRGBA{R: 0x3C, G: 0x2A, B: 0x56, A: 0xFF} // System Note BG
	colorWhite    = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	colorDanger   = color.NRGBA{R: 0xFF, G: 0x5C, B: 0x8D, A: 0xFF} // Neon Coral Red
	colorOK       = color.NRGBA{R: 0x06, G: 0xD6, B: 0xA0, A: 0xFF} // Neon Mint Green
	colorStickerCaption = color.NRGBA{R: 0xCF, G: 0xC2, B: 0xEB, A: 0xD8}
	colorLoginBG  = color.NRGBA{R: 0x09, G: 0x06, B: 0x0F, A: 0xFF} // Deepest Login BG
	colorTextMain = color.NRGBA{R: 0xF8, G: 0xF5, B: 0xFF, A: 0xFF} // Bright White-Lavender

	// Logo
	colorLogo  = color.NRGBA{R: 0xC7, G: 0x7D, B: 0xFF, A: 0xFF}
	colorShine = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xC0}
	colorFace  = color.NRGBA{R: 0xF8, G: 0xF5, B: 0xFF, A: 0xFF}
	colorCheek = color.NRGBA{R: 0xE0, G: 0xAA, B: 0xFF, A: 0xE0}

	avatarPalette = []color.NRGBA{
		{R: 0x9D, G: 0x4E, B: 0xDD, A: 0xFF},
		{R: 0x72, G: 0x09, B: 0xB7, A: 0xFF},
		{R: 0x3A, G: 0x0C, B: 0xA3, A: 0xFF},
		{R: 0x43, G: 0x61, B: 0xEE, A: 0xFF},
		{R: 0xF7, G: 0x25, B: 0x85, A: 0xFF},
		{R: 0x48, G: 0xCA, B: 0xE4, A: 0xFF},
		{R: 0x06, G: 0xD6, B: 0xA0, A: 0xFF},
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

var colorBackdrop = color.NRGBA{R: 0x36, G: 0x21, B: 0x48, A: 0xC0}

var colorShadow = color.NRGBA{R: 0x5C, G: 0x32, B: 0x78, A: 0x50}
