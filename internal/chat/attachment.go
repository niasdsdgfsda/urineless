package chat

import (
	"image"
	"image/gif"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"

	"gioui.org/op/paint"
)

type AttachState int

const (
	AttIdle AttachState = iota
	AttLoading
	AttReady
	AttWaiting
	AttDelivered
	AttFailed
)

type Attachment struct {
	Code       string
	Name       string
	Outgoing   bool
	Sticker    bool
	State      AttachState
	Err        string
	Image      image.Image
	Animated   *gif.GIF
	Frames     []image.Image
	Op         paint.ImageOp
	Ops        []paint.ImageOp
	FrameIdx   int
	LastUpdate time.Time
	URL        string
}

var wormholeRe = regexp.MustCompile(`^\[wormhole\] (\d+(?:-[a-z]+)+) (.{1,200})$`)
var stickerWormholeRe = regexp.MustCompile(`^\[sticker\] \[wormhole\] (\d+(?:-[a-z]+)+) (.{1,200})$`)

func FormatAttachment(code, name string) string {
	return "[wormhole] " + code + " " + SafeName(name)
}

func FormatStickerAttachment(code, name string) string {
	return "[sticker] " + FormatAttachment(code, name)
}

var imgURLRe = regexp.MustCompile(`^\[img\]\s+(https?://\S+)$`)
var plainURLRe = regexp.MustCompile(`^https?://\S+$`)

var autoLoadHosts = map[string]bool{
	"nekos.best":         true,
	"cataas.com":         true,
	"media.tenor.com":    true,
	"c.tenor.com":        true,
	"media.giphy.com":    true,
	"i.giphy.com":        true,
	"gifcities.org":      true,
	"blob.gifcities.org":  true,
	"web.archive.org":    true,
	"cdn.bsky.app":       true,
	"bsky.app":           true,
	"fixupx.com":         true,
	"vxtwitter.com":      true,
	"fxtwitter.com":      true,
	"x.com":              true,
}
var imageHosts = map[string]bool{
	"nekos.best":         true,
	"cataas.com":         true,
	"media.tenor.com":    true,
	"c.tenor.com":        true,
	"media.giphy.com":    true,
	"i.giphy.com":        true,
	"gifcities.org":      true,
	"blob.gifcities.org":  true,
	"web.archive.org":    true,
	"cdn.bsky.app":       true,
	"bsky.app":           true,
	"fixupx.com":         true,
	"vxtwitter.com":      true,
	"fxtwitter.com":      true,
	"x.com":              true,
}

func FormatImageURL(u string) string {
	return "[img] " + u
}

func ParseAttachment(text string) *Attachment {
	t := strings.TrimSpace(text)
	if m := stickerWormholeRe.FindStringSubmatch(t); m != nil {
		return &Attachment{Code: m[1], Name: SafeName(m[2]), Sticker: true, State: AttIdle}
	}
	if m := wormholeRe.FindStringSubmatch(t); m != nil {
		return &Attachment{Code: m[1], Name: SafeName(m[2]), State: AttIdle}
	}
	if m := imgURLRe.FindStringSubmatch(t); m != nil {
		u := m[1]
		return &Attachment{URL: u, Name: nameFromURL(u), Sticker: isStickerURL(u), State: AttIdle}
	}
	if plainURLRe.MatchString(t) {
		pu, err := url.Parse(t)
		host := ""
		if err == nil {
			host = strings.ToLower(pu.Hostname())
		}
		tenorHost := isTenorMediaHost(host)
		if err == nil && (pu.Scheme == "https" || pu.Scheme == "http") {
			if tenorHost && pu.Path != "" && pu.Path != "/" {
				return &Attachment{URL: t, Name: nameFromURL(t), Sticker: true, State: AttIdle}
			}
			lowerT := strings.ToLower(t)
			if strings.Contains(lowerT, "gif") ||
				strings.Contains(lowerT, "png") ||
				strings.Contains(lowerT, "jpg") ||
				strings.Contains(lowerT, "jpeg") ||
				strings.Contains(lowerT, "webp") ||
				strings.Contains(lowerT, "img") ||
				strings.Contains(lowerT, "image") ||
				host == "cdn.bsky.app" || host == "bsky.app" {
				return &Attachment{URL: t, Name: nameFromURL(t), Sticker: true, State: AttIdle}
			}
		}
	}
	return nil
}

func isStickerURL(rawURL string) bool {
	return true
}

func (a *Attachment) AutoLoad() bool {
	return a.URL != ""
}

func isTenorMediaHost(host string) bool {
	if host == "media.tenor.com" || host == "c.tenor.com" {
		return true
	}
	if !strings.HasPrefix(host, "media") || !strings.HasSuffix(host, ".tenor.com") {
		return false
	}
	n := strings.TrimSuffix(strings.TrimPrefix(host, "media"), ".tenor.com")
	if n == "" {
		return false
	}
	for _, r := range n {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func nameFromURL(u string) string {
	if pu, err := url.Parse(u); err == nil {
		if b := path.Base(pu.Path); b != "" && b != "/" && b != "." {
			return SafeName(b)
		}
	}
	return "bild"
}

func SafeName(n string) string {
	n = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, n)
	n = path.Base(strings.ReplaceAll(n, "\\", "/"))
	if n == "." || n == "/" || n == "" {
		return "bild"
	}
	if r := []rune(n); len(r) > 100 {
		n = string(r[:100])
	}
	return n
}
