package gifcities

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSearchScrapesGifCitiesPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("q"); got != "cat" {
			t.Fatalf("q = %q, want cat", got)
		}
		_, _ = w.Write([]byte(`<html><img src="https://blob.gifcities.org/gifcities/ABC123XYZ.gif"><img src="https://blob.gifcities.org/gifcities/DEF456UVW.gif"></html>`))
	}))
	defer server.Close()

	client := NewClient()
	client.site = server.URL
	results, err := client.Search(context.Background(), "cat", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	if results[0].URL != "https://blob.gifcities.org/gifcities/ABC123XYZ.gif" {
		t.Fatalf("unexpected first result: %#v", results[0])
	}
	if results[1].URL != "https://blob.gifcities.org/gifcities/DEF456UVW.gif" {
		t.Fatalf("unexpected second result: %#v", results[1])
	}
}

func TestSearchRequiresQuery(t *testing.T) {
	_, err := NewClient().Search(context.Background(), "  ", 10)
	if err == nil {
		t.Fatal("expected empty query to fail")
	}
}
