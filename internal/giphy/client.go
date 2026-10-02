// Package giphy provides a client for Giphy's GIF search API.
package giphy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const apiBase = "https://api.giphy.com/v1/gifs"
const defaultPublicKey = "dc6zaTOxFJmzC"

type Result struct {
	URL         string
	PreviewURL  string
	Description string
}

type Client struct {
	apiKey string
	http   *http.Client
	base   string
}

func NewClient(apiKey string) *Client {
	if strings.TrimSpace(apiKey) == "" {
		apiKey = defaultPublicKey
	}
	return &Client{
		apiKey: strings.TrimSpace(apiKey),
		http:   &http.Client{Timeout: 15 * time.Second},
		base:   apiBase,
	}
}

type apiResponse struct {
	Data []struct {
		Title  string `json:"title"`
		Images struct {
			FixedHght struct {
				URL string `json:"url"`
			} `json:"fixed_height"`
			FixedHghtSmall struct {
				URL string `json:"url"`
			} `json:"fixed_height_small"`
		} `json:"images"`
	} `json:"data"`
}

func (c *Client) Search(ctx context.Context, query string, limit int) ([]Result, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, errors.New("enter a search term")
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	u, err := url.Parse(c.base + "/search")
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("api_key", c.apiKey)
	q.Set("q", query)
	q.Set("limit", fmt.Sprint(limit))
	q.Set("rating", "g")
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
		return nil, fmt.Errorf("Giphy: HTTP %d", resp.StatusCode)
	}

	var body apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}

	results := make([]Result, 0, len(body.Data))
	for _, item := range body.Data {
		gif := item.Images.FixedHght.URL
		preview := item.Images.FixedHghtSmall.URL
		if gif == "" {
			gif = preview
		}
		if preview == "" {
			preview = gif
		}
		if gif == "" {
			continue
		}
		desc := item.Title
		if desc == "" {
			desc = "Giphy GIF"
		}
		results = append(results, Result{
			URL:         gif,
			PreviewURL:  preview,
			Description: desc,
		})
	}

	if len(results) == 0 {
		return nil, errors.New("Giphy hat keine passenden GIFs gefunden.")
	}
	return results, nil
}
