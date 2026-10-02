package chat

import "testing"

func TestParseAttachmentAcceptsTenorDirectLinks(t *testing.T) {
	tests := []string{
		"https://media.tenor.com/abc123/tenor.gif",
		"https://media1.tenor.com/m/abc/cat.gif",
		"https://media12.tenor.com/m/xyz/dog.gif",
		"https://c.tenor.com/abc123/sticker.webp",
		"https://media.tenor.com/abc123",
	}
	for _, text := range tests {
		att := ParseAttachment(text)
		if att == nil {
			t.Errorf("ParseAttachment(%q) = nil", text)
			continue
		}
		if att.URL != text {
			t.Errorf("attachment URL = %q, want %q", att.URL, text)
		}
		if !att.AutoLoad() {
			t.Errorf("attachment from %q should auto-load", text)
		}
		if !att.Sticker {
			t.Errorf("attachment from %q should use sticker presentation", text)
		}
	}
}

func TestParseAttachmentKeepsLegacyImageLinks(t *testing.T) {
	att := ParseAttachment("[img] https://example.com/image.gif")
	if att == nil || att.URL != "https://example.com/image.gif" {
		t.Fatalf("legacy [img] link was not parsed: %#v", att)
	}
}

func TestParseStickerWormhole(t *testing.T) {
	const wire = "[sticker] [wormhole] 1234-aardvark-banana sticker.gif"
	att := ParseAttachment(wire)
	if att == nil || !att.Sticker || att.Code != "1234-aardvark-banana" {
		t.Fatalf("sticker wormhole not parsed: %#v", att)
	}
	if got := FormatStickerAttachment(att.Code, att.Name); got != wire {
		t.Fatalf("formatted sticker = %q, want %q", got, wire)
	}
}

func TestParseAttachmentDoesNotTreatArbitraryURLsAsImages(t *testing.T) {
	for _, text := range []string{
		"https://example.com/page.html",
		"https://google.com",
	} {
		if att := ParseAttachment(text); att != nil {
			t.Errorf("ParseAttachment(%q) = %#v, want nil", text, att)
		}
	}
}
