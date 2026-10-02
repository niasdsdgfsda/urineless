// Package fixupx resolves Twitter/X and FixupX links using the VxTwitter API.
package fixupx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"time"
)

var statusRe = regexp.MustCompile(`(?:status|statuses)/([0-9]+)`)

type Result struct {
	URL      string
	IsVideo  bool
	VideoURL string
}

type Client struct {
	http *http.Client
}

func NewClient() *Client {
	return &Client{
		http: &http.Client{Timeout: 15 * time.Second},
	}
}

type vxResponse struct {
	MediaURLs    []string `json:"media_urls"`
	MediaIsVideo bool     `json:"media_isVideo"`
	VideoURL     string   `json:"video_url"`
}

func (c *Client) Resolve(ctx context.Context, tweetURL string) (Result, error) {
	m := statusRe.FindStringSubmatch(tweetURL)
	if len(m) < 2 {
		return Result{}, errors.New("keine Tweet-ID in URL gefunden")
	}
	tweetID := m[1]
	apiURL := fmt.Sprintf("https://api.vxtwitter.com/Twitter/status/%s", tweetID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "urineless/0.1")

	resp, err := c.http.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Result{}, fmt.Errorf("VxTwitter API: HTTP %d", resp.StatusCode)
	}

	var data vxResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return Result{}, err
	}

	res := Result{
		IsVideo:  data.MediaIsVideo,
		VideoURL: data.VideoURL,
	}
	if len(data.MediaURLs) > 0 {
		res.URL = data.MediaURLs[0]
	}
	if res.URL == "" && res.VideoURL != "" {
		res.URL = res.VideoURL
	}
	if res.URL == "" {
		return Result{}, errors.New("keine Medien in diesem Tweet gefunden")
	}
	return res, nil
}
