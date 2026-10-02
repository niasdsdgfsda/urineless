package tenor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSearchParsesGIFResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("q"); got != "hello cat" {
			t.Fatalf("query = %q, want hello cat", got)
		}
		if got := r.URL.Query().Get("media_filter"); got != "tinygif,gif" {
			t.Fatalf("media_filter = %q", got)
		}
		_, _ = w.Write([]byte(`{"results":[{"content_description":"wave","media_formats":{"gif":{"url":"https://media.tenor.com/full.gif"},"tinygif":{"url":"https://media.tenor.com/preview.gif"}}},{"media_formats":{}}]}`))
	}))
	defer server.Close()

	client := NewClient("test-key")
	client.base = server.URL
	results, err := client.Search(context.Background(), "hello cat", 12)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].URL != "https://media.tenor.com/full.gif" ||
		results[0].PreviewURL != "https://media.tenor.com/preview.gif" ||
		results[0].Description != "wave" {
		t.Fatalf("unexpected result: %#v", results[0])
	}
}

func TestSearchScrapesPublicTenorPageWithoutAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search/cat-stickers-gifs" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("format"); got != "stickers" {
			t.Fatalf("format = %q, want stickers", got)
		}
		_, _ = w.Write([]byte(`<html><meta property="og:image" content="https://media1.tenor.com/m/abc/cat.gif"><img src="https://media1.tenor.com/m/abc/cat.gif"><img src="https://media2.tenor.com/m/xyz/dog.gif"></html>`))
	}))
	defer server.Close()

	client := NewClient("")
	client.site = server.URL
	results, err := client.Search(context.Background(), "cat stickers", 12)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	if results[0].URL != "https://media1.tenor.com/m/abc/cat.gif" {
		t.Fatalf("unexpected first result: %#v", results[0])
	}
	if results[1].URL != "https://media2.tenor.com/m/xyz/dog.gif" {
		t.Fatalf("unexpected second result: %#v", results[1])
	}
}

func TestSearchRequiresQuery(t *testing.T) {
	_, err := NewClient("").Search(context.Background(), "  ", 12)
	if err == nil {
		t.Fatal("expected empty query to fail")
	}
}
