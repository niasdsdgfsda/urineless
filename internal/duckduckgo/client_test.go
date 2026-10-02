package duckduckgo

import (
	"context"
	"testing"
)

func TestSearchRequiresQuery(t *testing.T) {
	_, err := NewClient().Search(context.Background(), "  ", 10)
	if err == nil {
		t.Fatal("expected empty query to fail")
	}
}

func TestLiveDuckDuckGoSearch(t *testing.T) {
	client := NewClient()
	results, err := client.Search(context.Background(), "cat", 5)
	if err != nil {
		t.Logf("Live search failed (network dependent): %v", err)
		return
	}
	t.Logf("Found %d results", len(results))
	for _, r := range results {
		t.Logf("Result: URL=%s, Title=%s", r.URL, r.Description)
	}
}
