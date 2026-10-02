// Package tenor provides a small client for Tenor's v2 GIF search API.
package tenor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const apiBase = "https://tenor.googleapis.com/v2"
const siteBase = "https://tenor.com"
const maxSearchPageBytes = 5 << 20

var mediaURLRe = regexp.MustCompile(`https://(?:media[0-9]*|c)\.tenor\.com/[A-Za-z0-9_./-]+\.gif(?:\?[A-Za-z0-9_=&%-]+)?`)

type Result struct {
	URL         string
	PreviewURL  string
	Description string
}

type Client struct {
	apiKey string
	http   *http.Client
	base   string
	site   string
}

func NewClient(apiKey string) *Client {
	return &Client{
		apiKey: strings.TrimSpace(apiKey),
		http:   &http.Client{Timeout: 15 * time.Second},
		base:   apiBase,
		site:   siteBase,
	}
}

type apiResponse struct {
	Results []struct {
		ContentDescription string `json:"content_description"`
		MediaFormats       map[string]struct {
			URL string `json:"url"`
		} `json:"media_formats"`
	} `json:"results"`
}

func (c *Client) Search(ctx context.Context, query string, limit int) ([]Result, error) {
	if strings.TrimSpace(query) == "" {
		return nil, errors.New("enter a search term")
	}
	if limit <= 0 || limit > 50 {
		limit = 12
	}
	if c.apiKey == "" {
		return c.searchPage(ctx, query, limit)
	}
	return c.searchAPI(ctx, query, limit)
}

func (c *Client) searchAPI(ctx context.Context, query string, limit int) ([]Result, error) {
	u, err := url.Parse(c.base + "/search")
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("key", c.apiKey)
	q.Set("client_key", "urineless")
	q.Set("q", query)
	q.Set("limit", fmt.Sprint(limit))
	q.Set("media_filter", "tinygif,gif")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Tenor: HTTP %d", resp.StatusCode)
	}

	var body apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	results := make([]Result, 0, len(body.Results))
	for _, item := range body.Results {
		gif := item.MediaFormats["gif"].URL
		preview := item.MediaFormats["tinygif"].URL
		if gif == "" {
			gif = preview
		}
		if preview == "" {
			preview = gif
		}
		if gif == "" {
			continue
		}
		results = append(results, Result{
			URL:         gif,
			PreviewURL:  preview,
			Description: item.ContentDescription,
		})
	}
	return results, nil
}

func (c *Client) searchPage(ctx context.Context, query string, limit int) ([]Result, error) {
	slug := strings.Join(strings.Fields(strings.TrimSpace(query)), "-")
	pageURL := strings.TrimRight(c.site, "/") + "/search/" + url.PathEscape(slug) + "-gifs?format=stickers"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
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
		return nil, fmt.Errorf("Tenor search page: HTTP %d", resp.StatusCode)
	}
	html, err := io.ReadAll(io.LimitReader(resp.Body, maxSearchPageBytes+1))
	if err != nil {
		return nil, err
	}
	if len(html) > maxSearchPageBytes {
		return nil, errors.New("Tenor search page is too large")
	}

	matches := mediaURLRe.FindAllString(string(html), -1)
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
			Description: "Tenor sticker",
		})
		if len(results) == limit {
			break
		}
	}
	if len(results) == 0 {
		return nil, errors.New("no direct GIF links found on Tenor search page; its page format may have changed")
	}
	return results, nil
}
