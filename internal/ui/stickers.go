package ui

import (
	"bytes"
	"context"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"strings"
	"time"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"ircgram/internal/chat"
	"ircgram/internal/nekos"
	"ircgram/internal/tenor"
)

type stickerEntry struct {
	name  string
	local string
	url   string
	img   image.Image
	op    paint.ImageOp
	click widget.Clickable
}

type stickerPicker struct {
	open    bool
	loading bool
	errMsg  string
	entries []*stickerEntry
	tab     int
	loadID  uint64

	search widget.Editor
	list   widget.List

	nekos     *nekos.Client
	tenor     *tenor.Client
	target    *chat.Conversation
	localBtn  widget.Clickable
	nekosBtn  widget.Clickable
	tenorBtn  widget.Clickable
	searchBtn widget.Clickable
	moreBtn   widget.Clickable
	closeBtn  widget.Clickable
	backdrop  widget.Clickable
	moreIndex int
}

func newStickerPicker() *stickerPicker {
	p := &stickerPicker{}
	p.list.Axis = layout.Vertical
	p.search.SingleLine = true
	p.search.Submit = true
	p.nekos = nekos.NewClient()
	p.tenor = tenor.NewClient(os.Getenv("TENOR_API_KEY"))
	return p
}

func (p *stickerPicker) Open(a *App) {
	p.open = true
	p.loading = false
	p.errMsg = ""
	p.target = a.store.Active
	p.moreIndex = 1
	p.startNekos(a, "hug")
}

func (p *stickerPicker) Close() {
	p.open = false
	p.loading = false
	p.entries = nil
	p.target = nil
	p.loadID++
}

func (p *stickerPicker) loadLocal() {
	list := chat.StickerList()
	p.entries = make([]*stickerEntry, 0, len(list))
	for _, s := range list {
		p.entries = append(p.entries, &stickerEntry{name: s.Name, local: s.Path, img: s.Image})
	}
	p.tab = 0
	p.loading = false
	p.errMsg = ""
}

func (p *stickerPicker) startNekosLoad(a *App, category string, appendResults bool) {
	p.tab = 1
	p.loading = true
	p.errMsg = ""
	p.loadID++
	id := p.loadID
	go p.loadNekos(a, category, id, appendResults)
}

func (p *stickerPicker) loadNekos(a *App, category string, id uint64, appendResults bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	res, err := p.nekos.Random(ctx, category, 20)
	entries := make([]*stickerEntry, 0, len(res))
	if err == nil {
		for i := range res {
			r := res[i]
			data, derr := downloadGIF(ctx, r.URL)
			var img image.Image
			if derr == nil {
				img, _, _ = image.Decode(bytes.NewReader(data))
			}
			name := r.AnimeName
			if name == "" {
				name = category
			}
			entries = append(entries, &stickerEntry{name: name, url: r.URL, img: img})
		}
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if !p.open || p.tab != 1 || p.loadID != id {
		return
	}
	if appendResults {
		const maxStickerResults = 160
		remaining := maxStickerResults - len(p.entries)
		if remaining <= 0 {
			p.loading = false
			p.errMsg = "Alle Sticker-Kategorien sind geladen."
			a.window.Invalidate()
			return
		}
		if len(entries) > remaining {
			entries = entries[:remaining]
		}
		p.entries = append(p.entries, entries...)
	} else {
		p.entries = entries
	}
	p.loading = false
	if err != nil {
		p.errMsg = "Nekos: " + err.Error()
	} else if len(entries) == 0 {
		p.errMsg = "Nekos hat keine Sticker geliefert."
	}
	a.window.Invalidate()
}

func (p *stickerPicker) startTenorLoad(a *App, query string) {
	p.tab = 2
	p.loading = true
	p.errMsg = ""
	p.loadID++
	id := p.loadID
	go p.loadTenor(a, query, id)
}

func (p *stickerPicker) loadTenor(a *App, query string, id uint64) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	res, err := p.tenor.Search(ctx, query, 40)
	entries := make([]*stickerEntry, 0, len(res))
	if err == nil {
		for _, r := range res {
			data, derr := downloadGIF(ctx, r.PreviewURL)
			var img image.Image
			if derr == nil {
				img, _, _ = image.Decode(bytes.NewReader(data))
			}
			name := r.Description
			if name == "" {
				name = "Tenor sticker"
			}
			entries = append(entries, &stickerEntry{name: name, url: r.URL, img: img})
		}
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if !p.open || p.tab != 2 || p.loadID != id {
		return
	}
	p.entries = entries
	p.loading = false
	if err != nil {
		p.errMsg = "Tenor: " + err.Error()
	} else if len(entries) == 0 {
		p.errMsg = "Tenor hat keine Sticker auf der Suchseite gefunden."
	}
	a.window.Invalidate()
}

// handle processes picker keyboard and search events.
func (p *stickerPicker) handle(gtx layout.Context, a *App) {
	if !p.open {
		return
	}
	if ev, ok := gtx.Event(key.Filter{Name: key.NameEscape}); ok {
		if _, ok := ev.(key.Event); ok {
			p.Close()
			return
		}
	}
	if ev, ok := p.search.Update(gtx); ok {
		if _, ok := ev.(widget.SubmitEvent); ok {
			query := strings.TrimSpace(p.search.Text())
			switch p.tab {
			case 1:
				p.startNekos(a, query)
			case 2:
				p.startTenorLoad(a, query)
			}
		}
	}
}

func (a *App) pickerScreen(gtx layout.Context) layout.Dimensions {
	p := a.picker
	panelW := min(gtx.Constraints.Max.X, gtx.Dp(360))
	panelH := min(gtx.Constraints.Max.Y, gtx.Dp(760))
	panelW = max(1, panelW)
	panelH = max(1, panelH)
	gtx.Constraints.Min = image.Pt(panelW, panelH)
	gtx.Constraints.Max = image.Pt(panelW, panelH)

	return FillBG(gtx, colorSidebar, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return p.header(gtx, a, a.th)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return HLine(gtx, colorLine)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return p.grid(gtx, a, a.th)
			}),
		)
	})
}

func (p *stickerPicker) startNekos(a *App, category string) {
	if category == "" {
		category = "hug"
	}
	p.startNekosLoad(a, category, false)
}

func (p *stickerPicker) loadMoreNekos(a *App) {
	categories := nekos.GIFCategories
	if len(categories) == 0 {
		return
	}
	category := categories[p.moreIndex%len(categories)]
	p.moreIndex++
	p.startNekosLoad(a, category, true)
}

func (p *stickerPicker) button(gtx layout.Context, th *material.Theme, click *widget.Clickable, label string, backgroundColor, foregroundColor color.NRGBA, onClick func()) layout.Dimensions {
	if click.Clicked(gtx) {
		onClick()
	}
	b := material.Button(th, click, label)
	b.Background = backgroundColor
	b.Color = foregroundColor
	b.CornerRadius = unit.Dp(16)
	b.Inset = layout.Inset{Top: 8, Bottom: 8, Left: 14, Right: 14}
	return b.Layout(gtx)
}

func (p *stickerPicker) header(gtx layout.Context, a *App, th *material.Theme) layout.Dimensions {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	closeLabel := "Zurück"
	if gtx.Constraints.Max.X < gtx.Dp(520) {
		closeLabel = "×"
	}
	return layout.Inset{Top: 12, Bottom: 12, Left: 12, Right: 12}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						bg, fg := colorField, colorTextMain
						if p.tab == 0 {
							bg, fg = colorAccent, colorWhite
						}
						return p.button(gtx, th, &p.localBtn, "Lokal", bg, fg, func() {
							p.loadID++
							p.loadLocal()
						})
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						bg, fg := colorField, colorTextMain
						if p.tab == 1 {
							bg, fg = colorAccent, colorWhite
						}
						return p.button(gtx, th, &p.nekosBtn, "Nekos", bg, fg, func() {
							p.startNekos(a, "hug")
						})
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						bg, fg := colorField, colorTextMain
						if p.tab == 2 {
							bg, fg = colorAccent, colorWhite
						}
						return p.button(gtx, th, &p.tenorBtn, "Tenor", bg, fg, func() {
							p.startTenorLoad(a, "sticker")
						})
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Dimensions{}
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return p.button(gtx, th, &p.closeBtn, closeLabel, colorField, colorTextMain, func() {
							p.Close()
						})
					}),
				)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if p.tab != 1 && p.tab != 2 {
					return layout.Dimensions{}
				}
				return layout.Inset{Top: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							hint := "Kategorie (hug, pat, kiss ...)"
							if p.tab == 2 {
								hint = "Search Tenor"
							}
							return material.Editor(th, &p.search, hint).Layout(gtx)
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return p.button(gtx, th, &p.searchBtn, "Suchen", colorAccent, colorWhite, func() {
								query := strings.TrimSpace(p.search.Text())
								if p.tab == 1 {
									p.startNekos(a, query)
								} else {
									if query == "" {
										query = "sticker"
									}
									p.startTenorLoad(a, query)
								}
							})
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if p.tab != 1 {
								return layout.Dimensions{}
							}
							return layout.Inset{Left: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								label := "Mehr Sticker"
								if p.loading {
									label = "Lade …"
								}
								return p.button(gtx, th, &p.moreBtn, label, colorLavender, colorWhite, func() {
									if !p.loading {
										p.loadMoreNekos(a)
									}
								})
							})
						}),
					)
				})
			}),
		)
	})
}

func (p *stickerPicker) grid(gtx layout.Context, a *App, th *material.Theme) layout.Dimensions {
	if p.loading && len(p.entries) == 0 {
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			l := material.Body2(th, "Lade ...")
			l.Color = colorMuted
			return l.Layout(gtx)
		})
	}
	if p.errMsg != "" {
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			l := material.Body2(th, p.errMsg)
			l.Color = colorDanger
			return l.Layout(gtx)
		})
	}
	if len(p.entries) == 0 {
		hint := "Keine lokalen Sticker."
		if p.tab == 1 {
			hint = "Keine Nekos-Ergebnisse."
		} else if p.tab == 2 {
			hint = "Keine Tenor-Ergebnisse."
		}
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			l := material.Body2(th, hint)
			l.Color = colorMuted
			return l.Layout(gtx)
		})
	}

	cols := max(2, min(8, gtx.Constraints.Max.X/gtx.Dp(124)))
	rows := (len(p.entries) + cols - 1) / cols

	return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return material.List(th, &p.list).Layout(gtx, rows, func(gtx layout.Context, row int) layout.Dimensions {
			children := make([]layout.FlexChild, 0, cols)
			for c := 0; c < cols; c++ {
				idx := row*cols + c
				if idx >= len(p.entries) {
					children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Dimensions{}
					}))
					continue
				}
				e := p.entries[idx]
				children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return p.cell(gtx, a, th, e)
				}))
			}
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, children...)
		})
	})
}

func (p *stickerPicker) cell(gtx layout.Context, a *App, th *material.Theme, e *stickerEntry) layout.Dimensions {
	if e.op == (paint.ImageOp{}) && e.img != nil {
		e.op = paint.NewImageOp(e.img)
	}
	gtx.Constraints.Min.Y = gtx.Dp(112)
	gtx.Constraints.Max.Y = gtx.Dp(112)

	return layout.Inset{Top: 4, Bottom: 4, Left: 4, Right: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		if e.click.Clicked(gtx) {
			p.send(a, e)
		}
		return e.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			if e.img == nil {
				return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					l := material.Caption(th, e.name)
					l.Color = colorMuted
					return l.Layout(gtx)
				})
			}
			return layout.UniformInset(unit.Dp(3)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return widget.Image{Src: e.op, Fit: widget.Contain, Scale: gtx.Metric.PxPerDp}.Layout(gtx)
			})
		})
	})
}

func (p *stickerPicker) send(a *App, e *stickerEntry) {
	conv := p.target
	client := a.client
	if conv == nil {
		conv = a.store.Active
	}
	if conv == nil || client == nil || conv.Kind == chat.Server {
		p.Close()
		return
	}
	if e.url != "" {
		if err := a.sendImageURL(conv, client, e.url, e.img); err != nil {
			a.store.Sys(conv, "Sticker-Link konnte nicht gesendet werden: "+err.Error())
		}
		p.Close()
		return
	}
	if e.local != "" {
		a.sendStickerFile(conv, e.local)
		p.Close()
		return
	}
	p.Close()
}
