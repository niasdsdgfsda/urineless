package ui

import (
	"bytes"
	"context"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"strings"
	"sync"
	"time"

	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"ircgram/internal/chat"
	"ircgram/internal/gifcities"
	"ircgram/internal/nekos"
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
	page    int
	loadID  uint64

	search widget.Editor
	list   widget.List

	nekos        *nekos.Client
	gifcities    *gifcities.Client
	target       *chat.Conversation
	localBtn     widget.Clickable
	nekosBtn     widget.Clickable
	gifcitiesBtn widget.Clickable
	searchBtn widget.Clickable
	moreBtn   widget.Clickable
	closeBtn  widget.Clickable
	backdrop  widget.Clickable
	prevBtn   widget.Clickable
	nextBtn   widget.Clickable
	moreIndex int
}

func newStickerPicker() *stickerPicker {
	p := &stickerPicker{}
	p.list.Axis = layout.Vertical
	p.search.SingleLine = true
	p.search.Submit = true
	p.nekos = nekos.NewClient()
	p.gifcities = gifcities.NewClient()
	return p
}

func (p *stickerPicker) Open(a *App) {
	p.open = true
	p.loading = false
	p.errMsg = ""
	target := a.store.Active
	if target == nil || target.Kind == chat.Server {
		for _, c := range a.store.Convs {
			if c.Kind == chat.Channel || c.Kind == chat.Private {
				target = c
				break
			}
		}
	}
	p.target = target
	p.moreIndex = 1
	p.page = 0
	p.loadLocal(a)
}

func (p *stickerPicker) Close() {
	p.open = false
	p.loading = false
	p.entries = nil
	p.target = nil
	p.loadID++
}

func (p *stickerPicker) loadLocal(a *App) {
	p.tab = 0
	p.page = 0
	p.loading = true
	p.errMsg = ""
	p.loadID++
	id := p.loadID

	go func() {
		list := chat.StickerList()
		entries := make([]*stickerEntry, 0, len(list))
		for _, s := range list {
			entries = append(entries, &stickerEntry{name: s.Name, local: s.Path, img: s.Image})
		}
		a.mu.Lock()
		defer a.mu.Unlock()
		if !p.open || p.tab != 0 || p.loadID != id {
			return
		}
		p.entries = entries
		p.loading = false
		a.window.Invalidate()
	}()
}

func (p *stickerPicker) startNekosLoad(a *App, category string, appendResults bool) {
	p.tab = 1
	p.loading = true
	p.errMsg = ""
	if !appendResults {
		p.page = 0
	}
	p.loadID++
	id := p.loadID
	go p.loadNekos(a, category, id, appendResults)
}

func (p *stickerPicker) loadNekos(a *App, category string, id uint64, appendResults bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	res, err := p.nekos.Random(ctx, category, 20)
	if err != nil || len(res) == 0 {
		a.mu.Lock()
		defer a.mu.Unlock()
		if p.open && p.tab == 1 && p.loadID == id {
			p.loading = false
			if err != nil {
				p.errMsg = "Nekos: " + err.Error()
			} else {
				p.errMsg = "Nekos hat keine Sticker geliefert."
			}
			a.window.Invalidate()
		}
		return
	}

	newEntries := make([]*stickerEntry, len(res))
	for i, r := range res {
		name := r.AnimeName
		if name == "" {
			name = category
		}
		newEntries[i] = &stickerEntry{name: name, url: r.URL}
	}

	a.mu.Lock()
	if !p.open || p.tab != 1 || p.loadID != id {
		a.mu.Unlock()
		return
	}

	var targetEntries []*stickerEntry
	if appendResults {
		const maxStickerResults = 160
		remaining := maxStickerResults - len(p.entries)
		if remaining <= 0 {
			p.loading = false
			p.errMsg = "Alle Sticker-Kategorien sind geladen."
			a.window.Invalidate()
			a.mu.Unlock()
			return
		}
		if len(newEntries) > remaining {
			newEntries = newEntries[:remaining]
		}
		p.entries = append(p.entries, newEntries...)
		targetEntries = newEntries
	} else {
		p.entries = newEntries
		targetEntries = newEntries
		p.page = 0
	}
	p.loading = false
	a.window.Invalidate()
	a.mu.Unlock()

	// Concurrent image downloader (8 workers) with memory cache
	type downloadJob struct {
		entry *stickerEntry
		url   string
	}
	jobs := make(chan downloadJob, len(targetEntries))
	for _, e := range targetEntries {
		if cached, ok := getCachedImage(e.url); ok {
			a.mu.Lock()
			e.img = cached
			a.mu.Unlock()
			continue
		}
		jobs <- downloadJob{entry: e, url: e.url}
	}
	close(jobs)

	numWorkers := 8
	if len(targetEntries) < numWorkers {
		numWorkers = len(targetEntries)
	}
	var wg sync.WaitGroup
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				if cached, ok := getCachedImage(job.url); ok {
					a.mu.Lock()
					job.entry.img = cached
					if p.open && p.loadID == id {
						a.window.Invalidate()
					}
					a.mu.Unlock()
					continue
				}
				data, derr := downloadGIF(ctx, job.url)
				if derr == nil {
					if img, _, err := image.Decode(bytes.NewReader(data)); err == nil {
						setCachedImage(job.url, img)
						a.mu.Lock()
						job.entry.img = img
						if p.open && p.loadID == id {
							a.window.Invalidate()
						}
						a.mu.Unlock()
					}
				}
			}
		}()
	}
	wg.Wait()
}

func (p *stickerPicker) startGifCitiesLoad(a *App, query string) {
	p.tab = 2
	p.loading = true
	p.errMsg = ""
	p.page = 0
	p.loadID++
	id := p.loadID
	go p.loadGifCities(a, query, id)
}

func (p *stickerPicker) loadGifCities(a *App, query string, id uint64) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	res, err := p.gifcities.Search(ctx, query, 40)
	if err != nil || len(res) == 0 {
		a.mu.Lock()
		defer a.mu.Unlock()
		if p.open && p.tab == 2 && p.loadID == id {
			p.loading = false
			if err != nil {
				p.errMsg = "GifCities: " + err.Error()
			} else {
				p.errMsg = "GifCities hat keine passenden GIFs gefunden."
			}
			a.window.Invalidate()
		}
		return
	}

	entries := make([]*stickerEntry, len(res))
	for i, r := range res {
		name := r.Description
		if name == "" {
			name = "GifCities GIF"
		}
		entries[i] = &stickerEntry{name: name, url: r.URL}
	}

	a.mu.Lock()
	if !p.open || p.tab != 2 || p.loadID != id {
		a.mu.Unlock()
		return
	}
	p.entries = entries
	p.page = 0
	p.loading = false
	a.window.Invalidate()
	a.mu.Unlock()

	// Concurrent image downloader (8 workers) with memory cache
	type downloadJob struct {
		entry      *stickerEntry
		previewURL string
	}
	jobs := make(chan downloadJob, len(res))
	for i, r := range res {
		e := entries[i]
		if cached, ok := getCachedImage(r.PreviewURL); ok {
			a.mu.Lock()
			e.img = cached
			a.mu.Unlock()
			continue
		}
		jobs <- downloadJob{entry: e, previewURL: r.PreviewURL}
	}
	close(jobs)

	numWorkers := 8
	if len(entries) < numWorkers {
		numWorkers = len(entries)
	}
	var wg sync.WaitGroup
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				if cached, ok := getCachedImage(job.previewURL); ok {
					a.mu.Lock()
					job.entry.img = cached
					if p.open && p.loadID == id {
						a.window.Invalidate()
					}
					a.mu.Unlock()
					continue
				}
				data, derr := downloadGIF(ctx, job.previewURL)
				if derr == nil {
					if img, _, err := image.Decode(bytes.NewReader(data)); err == nil {
						setCachedImage(job.previewURL, img)
						a.mu.Lock()
						job.entry.img = img
						if p.open && p.loadID == id {
							a.window.Invalidate()
						}
						a.mu.Unlock()
					}
				}
			}
		}()
	}
	wg.Wait()
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
			p.page = 0
			switch p.tab {
			case 1:
				p.startNekos(a, query)
			case 2:
				p.startGifCitiesLoad(a, query)
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
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return p.paginationFooter(gtx, a.th)
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
	inset := unit.Dp(14)
	if gtx.Constraints.Max.X < gtx.Dp(280) {
		inset = unit.Dp(7)
	}
	b.Inset = layout.Inset{Top: 7, Bottom: 7, Left: inset, Right: inset}
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
							p.loadLocal(a)
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
						return p.button(gtx, th, &p.gifcitiesBtn, "GifCities", bg, fg, func() {
							p.startGifCitiesLoad(a, "cat")
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
				hint := "Kategorie"
				if p.tab == 2 {
					hint = "GifCities-Suche"
				}
				search := func() {
					query := strings.TrimSpace(p.search.Text())
					p.page = 0
					if p.tab == 1 {
						p.startNekos(a, query)
					} else {
						if query == "" {
							query = "cat"
						}
						p.startGifCitiesLoad(a, query)
					}
				}
				more := func(gtx layout.Context) layout.Dimensions {
					if p.tab != 1 {
						return layout.Dimensions{}
					}
					label := "Mehr Sticker"
					if p.loading {
						label = "Lade …"
					}
					return p.button(gtx, th, &p.moreBtn, label, colorLavender, colorWhite, func() {
						if !p.loading {
							p.loadMoreNekos(a)
						}
					})
				}
				compact := gtx.Constraints.Max.X < gtx.Dp(280)
				return layout.Inset{Top: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					searchRow := func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return material.Editor(th, &p.search, hint).Layout(gtx)
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								label := "Suchen"
								if compact {
									label = "Go"
								}
								return p.button(gtx, th, &p.searchBtn, label, colorAccent, colorWhite, search)
							}),
						)
					}
					if compact {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(searchRow),
							layout.Rigid(more),
						)
					}
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, searchRow),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Left: 6}.Layout(gtx, more)
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
			hint = "Keine GifCities-Ergebnisse."
		}
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			l := material.Body2(th, hint)
			l.Color = colorMuted
			return l.Layout(gtx)
		})
	}

	const pageSize = 30
	total := len(p.entries)
	totalPages := (total + pageSize - 1) / pageSize
	if totalPages < 1 {
		totalPages = 1
	}
	if p.page >= totalPages {
		p.page = totalPages - 1
	}
	if p.page < 0 {
		p.page = 0
	}
	start := p.page * pageSize
	end := min(total, start+pageSize)
	pageEntries := p.entries[start:end]

	cols := max(2, min(8, gtx.Constraints.Max.X/gtx.Dp(124)))
	rows := (len(pageEntries) + cols - 1) / cols

	return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return material.List(th, &p.list).Layout(gtx, rows, func(gtx layout.Context, row int) layout.Dimensions {
			children := make([]layout.FlexChild, 0, cols)
			for c := 0; c < cols; c++ {
				idx := row*cols + c
				if idx >= len(pageEntries) {
					children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Dimensions{}
					}))
					continue
				}
				e := pageEntries[idx]
				children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return p.cell(gtx, a, th, e)
				}))
			}
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, children...)
		})
	})
}

func (p *stickerPicker) paginationFooter(gtx layout.Context, th *material.Theme) layout.Dimensions {
	const pageSize = 30
	total := len(p.entries)
	totalPages := (total + pageSize - 1) / pageSize
	if totalPages <= 1 {
		return layout.Dimensions{}
	}

	if p.prevBtn.Clicked(gtx) && p.page > 0 {
		p.page--
	}
	if p.nextBtn.Clicked(gtx) && p.page < totalPages-1 {
		p.page++
	}

	return layout.Inset{Top: 4, Bottom: 8, Left: 12, Right: 12}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return HLine(gtx, colorLine)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Alignment: layout.Middle, Spacing: layout.SpaceBetween}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return p.button(gtx, th, &p.prevBtn, "◄ Zurück", colorField, colorTextMain, func() {})
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							s := "Seite " + itoa(p.page+1) + " / " + itoa(totalPages)
							l := material.Caption(th, s)
							l.Color = colorMuted
							l.Font.Weight = font.Bold
							return l.Layout(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return p.button(gtx, th, &p.nextBtn, "Weiter ►", colorField, colorTextMain, func() {})
						}),
					)
				})
			}),
		)
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
			log.Printf("[STICKER] Cell clicked for entry: name=%q, url=%q, local=%q", e.name, e.url, e.local)
			p.send(a, e)
		}
		return e.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			bg := colorField
			if e.click.Hovered() {
				bg = colorHover
			}
			return Pill(gtx, bg, 12, func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(6)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					if e.img == nil {
						return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							l := material.Caption(th, e.name)
							l.Color = colorMuted
							return l.Layout(gtx)
						})
					}
					return widget.Image{Src: e.op, Fit: widget.Contain, Scale: gtx.Metric.PxPerDp}.Layout(gtx)
				})
			})
		})
	})
}

func (p *stickerPicker) send(a *App, e *stickerEntry) {
	conv := p.target
	if conv == nil || conv.Kind == chat.Server {
		conv = a.store.Active
		if conv == nil || conv.Kind == chat.Server {
			for _, c := range a.store.Convs {
				if c.Kind == chat.Channel || c.Kind == chat.Private {
					conv = c
					break
				}
			}
		}
		if (conv == nil || conv.Kind == chat.Server) && len(a.autojoin) > 0 {
			conv = a.store.Ensure(a.autojoin[0])
			a.store.Select(conv)
		}
	}
	client := a.client
	log.Printf("[STICKER] p.send called: name=%q, url=%q, local=%q, conv=%v, client=%v", e.name, e.url, e.local, conv, client)

	if conv == nil || conv.Kind == chat.Server {
		if conv != nil {
			log.Printf("[STICKER] Cannot send in server tab")
			a.store.Sys(conv, "Bitte tritt zuerst einem Kanal (z. B. #libera) bei, um Sticker zu senden!")
		}
		p.Close()
		return
	}
	if client == nil {
		log.Printf("[STICKER] Cannot send: client is nil")
		a.store.Sys(conv, "Nicht verbunden – Sticker kann nicht gesendet werden.")
		p.Close()
		return
	}
	if e.url != "" {
		log.Printf("[STICKER] Sending URL sticker: %s to %s", e.url, conv.Name)
		if err := a.sendImageURL(conv, client, e.url, e.img); err != nil {
			log.Printf("[STICKER] ERROR sendImageURL: %v", err)
			a.store.Sys(conv, "Sticker-Link konnte nicht gesendet werden: "+err.Error())
		} else {
			log.Printf("[STICKER] sendImageURL succeeded for %s", e.url)
		}
		p.Close()
		return
	}
	if e.local != "" {
		log.Printf("[STICKER] Sending local sticker file: %s to %s", e.local, conv.Name)
		a.sendStickerFile(conv, e.local)
		p.Close()
		return
	}
	log.Printf("[STICKER] Warning: entry has neither url nor local path")
	p.Close()
}
