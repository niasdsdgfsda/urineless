package ui

import (
	"image"
	"sync"
)

var (
	imageCacheMu sync.RWMutex
	imageCache   = make(map[string]image.Image)
)

func getCachedImage(key string) (image.Image, bool) {
	if key == "" {
		return nil, false
	}
	imageCacheMu.RLock()
	defer imageCacheMu.RUnlock()
	img, ok := imageCache[key]
	return img, ok
}

func setCachedImage(key string, img image.Image) {
	if key == "" || img == nil {
		return
	}
	imageCacheMu.Lock()
	imageCache[key] = img
	imageCacheMu.Unlock()
}
