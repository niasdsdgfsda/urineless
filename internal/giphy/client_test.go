package giphy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSearchParsesGiphyResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("q"); got != "happy cat" {
			t.Fatalf("q = %q, want happy cat", got)
		}
		if got := r.URL.Query().Get("rating"); got != "g" {
			t.Fatalf("rating = %q, want g", got)
		}
		_, _ = w.Write([]byte(`{"data":[{"title":"Happy Cat","images":{"fixed_height":{"url":"https://media.giphy.com/full.gif"},"fixed_height_small":{"url":"https://media.giphy.com/preview.gif"}}}]}`))
	}))
	defer server.Close()

	client := NewClient("test-key")
	client.base = server.URL
	results, err := client.Search(context.Background(), "happy cat", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].URL != "https://media.giphy.com/full.gif" ||
		results[0].PreviewURL != "https://media.giphy.com/preview.gif" ||
		results[0].Description != "Happy Cat" {
		t.Fatalf("unexpected result: %#v", results[0])
	}
}

func TestSearchRequiresQuery(t *testing.T) {
	_, err := NewClient("").Search(context.Background(), "  ", 10)
	if err == nil {
		t.Fatal("expected empty query to fail")
	}
}
