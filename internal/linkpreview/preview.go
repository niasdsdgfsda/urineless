// Package linkpreview fetches OpenGraph metadata for rich link previews in chat.
package linkpreview

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

type Preview struct {
	URL         string
	Title       string
	Description string
	ImageURL    string
	SiteName    string
}

var (
	ogTitleRe  = regexp.MustCompile(`(?i)<meta[^>]+property=["']og:title["'][^>]+content=["']([^"']+)["']`)
	ogTitleRe2 = regexp.MustCompile(`(?i)<meta[^>]+content=["']([^"']+)["'][^>]+property=["']og:title["']`)

	ogDescRe  = regexp.MustCompile(`(?i)<meta[^>]+property=["']og:description["'][^>]+content=["']([^"']+)["']`)
	ogDescRe2 = regexp.MustCompile(`(?i)<meta[^>]+content=["']([^"']+)["'][^>]+property=["']og:description["']`)

	ogImageRe  = regexp.MustCompile(`(?i)<meta[^>]+property=["']og:image["'][^>]+content=["']([^"']+)["']`)
	ogImageRe2 = regexp.MustCompile(`(?i)<meta[^>]+content=["']([^"']+)["'][^>]+property=["']og:image["']`)

	ogSiteRe  = regexp.MustCompile(`(?i)<meta[^>]+property=["']og:site_name["'][^>]+content=["']([^"']+)["']`)
	titleTagRe = regexp.MustCompile(`(?i)<title[^>]*>([^<]+)</title>`)
)

func Fetch(ctx context.Context, rawURL string) (*Preview, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	s := string(body)
	p := &Preview{URL: rawURL}

	if m := ogTitleRe.FindStringSubmatch(s); len(m) > 1 {
		p.Title = m[1]
	} else if m := ogTitleRe2.FindStringSubmatch(s); len(m) > 1 {
		p.Title = m[1]
	} else if m := titleTagRe.FindStringSubmatch(s); len(m) > 1 {
		p.Title = strings.TrimSpace(m[1])
	}

	if m := ogDescRe.FindStringSubmatch(s); len(m) > 1 {
		p.Description = m[1]
	} else if m := ogDescRe2.FindStringSubmatch(s); len(m) > 1 {
		p.Description = m[1]
	}

	if m := ogImageRe.FindStringSubmatch(s); len(m) > 1 {
		p.ImageURL = m[1]
	} else if m := ogImageRe2.FindStringSubmatch(s); len(m) > 1 {
		p.ImageURL = m[1]
	}

	if m := ogSiteRe.FindStringSubmatch(s); len(m) > 1 {
		p.SiteName = m[1]
	} else if u, err := url.Parse(rawURL); err == nil {
		p.SiteName = u.Hostname()
	}

	if p.Title == "" && p.ImageURL == "" {
		return nil, fmt.Errorf("no preview metadata found")
	}

	return p, nil
}
