// Package duckduckgo provides a client for DuckDuckGo image and GIF search.
package duckduckgo

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

type Result struct {
	URL         string
	PreviewURL  string
	Description string
}

type Client struct {
	http *http.Client
}

func NewClient() *Client {
	return &Client{
		http: &http.Client{Timeout: 15 * time.Second},
	}
}

var vqdRe = regexp.MustCompile(`vqd=['"]?([0-9-]+)['"]?`)

type ddgImageResponse struct {
	Results []struct {
		Image     string `json:"image"`
		Thumbnail string `json:"thumbnail"`
		Title     string `json:"title"`
	} `json:"results"`
}

func (c *Client) Search(ctx context.Context, query string, limit int) ([]Result, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, errors.New("enter a search term")
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	vqd, err := c.fetchVQD(ctx, query)
	if err != nil {
		vqd = ""
	}

	u, err := url.Parse("https://duckduckgo.com/i.js")
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("q", query)
	if vqd != "" {
		q.Set("vqd", vqd)
	}
	q.Set("o", "json")
	q.Set("f", ",,,,type:gif")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	req.Header.Set("Referer", "https://duckduckgo.com/")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("DuckDuckGo: HTTP %d", resp.StatusCode)
	}

	var data ddgImageResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	results := make([]Result, 0, min(len(data.Results), limit))
	seen := make(map[string]struct{}, len(data.Results))

	for _, item := range data.Results {
		imgURL := item.Image
		if imgURL == "" {
			continue
		}
		if _, ok := seen[imgURL]; ok {
			continue
		}
		seen[imgURL] = struct{}{}

		desc := item.Title
		if desc == "" {
			desc = query
		}

		results = append(results, Result{
			URL:         imgURL,
			PreviewURL:  item.Thumbnail,
			Description: desc,
		})
		if len(results) == limit {
			break
		}
	}

	if len(results) == 0 {
		return nil, errors.New("DuckDuckGo hat keine GIFs gefunden.")
	}
	return results, nil
}

func (c *Client) fetchVQD(ctx context.Context, query string) (string, error) {
	u := "https://duckduckgo.com/?q=" + url.QueryEscape(query)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}

	m := vqdRe.FindSubmatch(body)
	if len(m) < 2 {
		return "", errors.New("vqd token not found")
	}
	return string(m[1]), nil
}
