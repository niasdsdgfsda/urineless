package linkpreview

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchParsesOpenGraph(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`
			<html>
			<head>
				<meta property="og:title" content="Sample Video Title" />
				<meta property="og:description" content="An awesome sample video preview" />
				<meta property="og:image" content="https://example.com/thumb.jpg" />
				<meta property="og:site_name" content="VideoTube" />
			</head>
			</html>
		`))
	}))
	defer server.Close()

	p, err := Fetch(context.Background(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if p.Title != "Sample Video Title" || p.Description != "An awesome sample video preview" || p.ImageURL != "https://example.com/thumb.jpg" || p.SiteName != "VideoTube" {
		t.Fatalf("unexpected preview: %#v", p)
	}
}
