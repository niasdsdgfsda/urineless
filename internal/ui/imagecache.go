package ui

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	_ "image/jpeg"
	_ "image/png"
	"sync"
	"time"

	"gioui.org/layout"
	"gioui.org/op/paint"
	"gioui.org/widget"
)

type cachedImage struct {
	img      image.Image
	animated *gif.GIF
	frames   []image.Image
	ops      []paint.ImageOp
}

var (
	imageCacheMu sync.RWMutex
	imageCache   = make(map[string]cachedImage)
)

func getCachedImage(key string) (image.Image, *gif.GIF, []image.Image, []paint.ImageOp, bool) {
	if key == "" {
		return nil, nil, nil, nil, false
	}
	imageCacheMu.RLock()
	defer imageCacheMu.RUnlock()
	ci, ok := imageCache[key]
	return ci.img, ci.animated, ci.frames, ci.ops, ok
}

func setCachedImage(key string, img image.Image, animated *gif.GIF, frames []image.Image, ops []paint.ImageOp) {
	if key == "" || img == nil {
		return
	}
	imageCacheMu.Lock()
	imageCache[key] = cachedImage{img: img, animated: animated, frames: frames, ops: ops}
	imageCacheMu.Unlock()
}

func decodeImage(data []byte) (image.Image, *gif.GIF, []image.Image) {
	if g, err := gif.DecodeAll(bytes.NewReader(data)); err == nil && len(g.Image) > 0 {
		if len(g.Image) == 1 {
			return g.Image[0], nil, nil
		}
		frames := composeGIF(g)
		return frames[0], g, frames
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err == nil {
		return img, nil, nil
	}
	return nil, nil, nil
}

func composeGIF(g *gif.GIF) []image.Image {
	if len(g.Image) == 0 {
		return nil
	}
	bounds := g.Image[0].Bounds()
	if bounds.Empty() {
		bounds = image.Rect(0, 0, g.Config.Width, g.Config.Height)
	}
	if bounds.Empty() {
		bounds = image.Rect(0, 0, 100, 100)
	}

	out := make([]image.Image, len(g.Image))
	canvas := image.NewRGBA(bounds)
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{color.Transparent}, image.Point{}, draw.Src)

	var prevCanvas *image.RGBA

	for i, srcImg := range g.Image {
		if i > 0 && len(g.Disposal) > i-1 && g.Disposal[i-1] == gif.DisposalPrevious {
			if prevCanvas != nil {
				draw.Draw(canvas, canvas.Bounds(), prevCanvas, image.Point{}, draw.Src)
			}
		} else {
			prevCanvas = image.NewRGBA(bounds)
			draw.Draw(prevCanvas, canvas.Bounds(), canvas, image.Point{}, draw.Src)
		}

		draw.Draw(canvas, srcImg.Bounds(), srcImg, srcImg.Bounds().Min, draw.Over)

		frame := image.NewRGBA(bounds)
		draw.Draw(frame, bounds, canvas, image.Point{}, draw.Src)
		out[i] = frame

		if len(g.Disposal) > i {
			switch g.Disposal[i] {
			case gif.DisposalBackground:
				draw.Draw(canvas, srcImg.Bounds(), &image.Uniform{color.Transparent}, image.Point{}, draw.Src)
			case gif.DisposalPrevious:
				// handled in next iteration
			}
		}
	}
	return out
}

func renderImage(gtx layout.Context, invalidate func(), op *paint.ImageOp, img image.Image, animated *gif.GIF, frames []image.Image, ops *[]paint.ImageOp, frameIdx *int, lastUpdate *time.Time) layout.Dimensions {
	if animated != nil && len(frames) > 1 {
		if len(*ops) != len(frames) {
			newOps := make([]paint.ImageOp, len(frames))
			for i, f := range frames {
				newOps[i] = paint.NewImageOp(f)
			}
			*ops = newOps
		}
		now := gtx.Now
		if now.IsZero() {
			now = time.Now()
		}
		d := animated.Delay[*frameIdx]
		if d <= 0 {
			d = 6
		}
		delay := time.Duration(d) * 10 * time.Millisecond
		if delay < 20*time.Millisecond {
			delay = 20*time.Millisecond
		}
		if lastUpdate.IsZero() {
			*lastUpdate = now
		}
		if now.Sub(*lastUpdate) >= delay {
			*frameIdx = (*frameIdx + 1) % len(frames)
			*lastUpdate = now
		}
		if invalidate != nil {
			invalidate()
		}
		return widget.Image{Src: (*ops)[*frameIdx], Fit: widget.Contain, Scale: gtx.Metric.PxPerDp}.Layout(gtx)
	}
	if *op == (paint.ImageOp{}) && img != nil {
		*op = paint.NewImageOp(img)
	}
	return widget.Image{Src: *op, Fit: widget.Contain, Scale: gtx.Metric.PxPerDp}.Layout(gtx)
}

func renderImageCover(gtx layout.Context, invalidate func(), op *paint.ImageOp, img image.Image, animated *gif.GIF, frames []image.Image, ops *[]paint.ImageOp, frameIdx *int, lastUpdate *time.Time) layout.Dimensions {
	if animated != nil && len(frames) > 1 {
		if len(*ops) != len(frames) {
			newOps := make([]paint.ImageOp, len(frames))
			for i, f := range frames {
				newOps[i] = paint.NewImageOp(f)
			}
			*ops = newOps
		}
		now := gtx.Now
		if now.IsZero() {
			now = time.Now()
		}
		d := animated.Delay[*frameIdx]
		if d <= 0 {
			d = 6
		}
		delay := time.Duration(d) * 10 * time.Millisecond
		if delay < 20*time.Millisecond {
			delay = 20*time.Millisecond
		}
		if lastUpdate.IsZero() {
			*lastUpdate = now
		}
		if now.Sub(*lastUpdate) >= delay {
			*frameIdx = (*frameIdx + 1) % len(frames)
			*lastUpdate = now
		}
		if invalidate != nil {
			invalidate()
		}
		return widget.Image{Src: (*ops)[*frameIdx], Fit: widget.Cover, Scale: gtx.Metric.PxPerDp}.Layout(gtx)
	}
	if *op == (paint.ImageOp{}) && img != nil {
		*op = paint.NewImageOp(img)
	}
	return widget.Image{Src: *op, Fit: widget.Cover, Scale: gtx.Metric.PxPerDp}.Layout(gtx)
}
