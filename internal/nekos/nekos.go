// Package nekos ist ein minimaler Client für nekos.best (kein API-Key nötig).
// Alle Requests brauchen einen spezifischen User-Agent, sonst blockt Cloudflare.
package nekos

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const (
	apiBase = "https://nekos.best/api/v2"
	// User-Agent muss laut Doku "APP_NAME (CONTACT_INFO)" sein – kein Browser-String.
	userAgent = "urineless/0.1 (+https://github.com/urineless)"
)

// Kategorien, die als GIF verfügbar sind (Stand: nekos.best v2).
var GIFCategories = []string{
	"angry", "baka", "bite", "bleh", "blowkiss", "blush", "bonk", "bored",
	"carry", "clap", "confused", "cry", "cuddle", "dance", "facepalm",
	"feed", "handhold", "handshake", "happy", "highfive", "hug", "kabedon",
	"kick", "kiss", "lappillow", "laugh", "lurk", "nod", "nom", "nope",
	"nyapat", "peck", "poke", "pout", "punch", "run", "salute", "shake",
	"shoot", "shocked", "shrug", "sips", "slap", "sleep", "smile", "smug",
	"spin", "stare", "tableflip", "teehee", "think", "thumbsup", "tickle",
	"wag", "wave", "wink", "yawn", "yeet",
}

// Result ist ein einzelnes Ergebnis von nekos.best.
type Result struct {
	URL       string // direkte GIF-URL
	AnimeName string // optional
}

type Client struct {
	http *http.Client
}

func NewClient() *Client {
	return &Client{http: &http.Client{Timeout: 15 * time.Second}}
}

type apiResp struct {
	Results []struct {
		URL       string `json:"url"`
		AnimeName string `json:"anime_name"`
	} `json:"results"`
}

func (c *Client) fetch(ctx context.Context, path string) ([]Result, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiBase+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nekos: HTTP %d", resp.StatusCode)
	}
	var r apiResp
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}
	out := make([]Result, 0, len(r.Results))
	for _, item := range r.Results {
		if item.URL == "" {
			continue
		}
		out = append(out, Result{URL: item.URL, AnimeName: item.AnimeName})
	}
	return out, nil
}

// Random liefert "amount" zufällige GIFs aus der Kategorie (1–20).
func (c *Client) Random(ctx context.Context, category string, amount int) ([]Result, error) {
	if category == "" {
		category = "hug"
	}
	if amount <= 0 {
		amount = 8
	}
	if amount > 20 {
		amount = 20
	}
	v := url.Values{}
	v.Set("amount", fmt.Sprint(amount))
	return c.fetch(ctx, "/"+category+"?"+v.Encode())
}

// Search durchsucht Metadaten (Anime-Name, Künstler).
func (c *Client) Search(ctx context.Context, query string, amount int) ([]Result, error) {
	if query == "" {
		return nil, errors.New("nekos: leere Suchanfrage")
	}
	if amount <= 0 {
		amount = 20
	}
	if amount > 20 {
		amount = 20
	}
	v := url.Values{}
	v.Set("query", query)
	v.Set("type", "2") // 2 = GIFs
	v.Set("amount", fmt.Sprint(amount))
	return c.fetch(ctx, "/search?"+v.Encode())
}
