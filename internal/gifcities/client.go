// Package gifcities provides a client for the Internet Archive's GifCities search.
package gifcities

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const siteBase = "https://gifcities.org"
const maxSearchPageBytes = 5 << 20

var blobURLRe = regexp.MustCompile(`https://blob\.gifcities\.org/gifcities/[A-Za-z0-9_-]+\.gif`)

type Result struct {
	URL         string
	PreviewURL  string
	Description string
}

type Client struct {
	http *http.Client
	site string
}

func NewClient() *Client {
	return &Client{
		http: &http.Client{Timeout: 15 * time.Second},
		site: siteBase,
	}
}

func (c *Client) Search(ctx context.Context, query string, limit int) ([]Result, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, errors.New("enter a search term")
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	searchURL := strings.TrimRight(c.site, "/") + "/search?q=" + url.QueryEscape(query) + "&offset=0&page_size=200"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/html")
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; urineless/0.1)")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GifCities: HTTP %d", resp.StatusCode)
	}

	html, err := io.ReadAll(io.LimitReader(resp.Body, maxSearchPageBytes+1))
	if err != nil {
		return nil, err
	}
	if len(html) > maxSearchPageBytes {
		return nil, errors.New("GifCities search page is too large")
	}

	matches := blobURLRe.FindAllString(string(html), -1)
	results := make([]Result, 0, min(len(matches), limit))
	seen := make(map[string]struct{}, len(matches))

	for _, mediaURL := range matches {
		if _, ok := seen[mediaURL]; ok {
			continue
		}
		seen[mediaURL] = struct{}{}
		results = append(results, Result{
			URL:         mediaURL,
			PreviewURL:  mediaURL,
			Description: "GifCities sticker",
		})
		if len(results) == limit {
			break
		}
	}

	if len(results) == 0 {
		return nil, errors.New("GifCities hat keine passenden GIFs gefunden.")
	}
	return results, nil
}
