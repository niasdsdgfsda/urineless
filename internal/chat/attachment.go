package chat

import (
	"image"
	"net/url"
	"path"
	"regexp"
	"strings"
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
	Code     string
	Name     string
	Outgoing bool
	Sticker  bool
	State    AttachState
	Err      string
	Image    image.Image
	URL      string
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
	"nekos.best":      true,
	"cataas.com":      true,
	"media.tenor.com": true,
	"c.tenor.com":     true,
	"media.giphy.com": true,
	"i.giphy.com":     true,
}
var imageHosts = map[string]bool{
	"nekos.best":      true,
	"cataas.com":      true,
	"media.tenor.com": true,
	"c.tenor.com":     true,
	"media.giphy.com": true,
	"i.giphy.com":     true,
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
		if err == nil && pu.Scheme == "https" && (imageHosts[host] || tenorHost) {
			if tenorHost && pu.Path != "" && pu.Path != "/" {
				return &Attachment{URL: t, Name: nameFromURL(t), Sticker: true, State: AttIdle}
			}
			switch strings.ToLower(path.Ext(pu.Path)) {
			case ".gif", ".jpeg", ".jpg", ".png", ".webp":
				return &Attachment{URL: t, Name: nameFromURL(t), Sticker: isStickerURL(t), State: AttIdle}
			}
		}
	}
	return nil
}

func isStickerURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return isTenorMediaHost(host) || host == "nekos.best"
}

func (a *Attachment) AutoLoad() bool {
	if a.URL == "" {
		return false
	}
	u, err := url.Parse(a.URL)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return autoLoadHosts[host] || isTenorMediaHost(host)
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
