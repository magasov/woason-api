package upload

import (
	"bytes"
	"testing"
)

var png1x1 = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d,
	0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4, 0x89, 0x00, 0x00, 0x00,
	0x0a, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00,
	0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00, 0x00, 0x00, 0x00, 0x49,
	0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
}

func TestSniffImage(t *testing.T) {
	jpeg := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 0x4a, 0x46, 0x49, 0x46}
	gif := []byte("GIF89a")
	webp := append([]byte("RIFF"), bytes.Repeat([]byte{0}, 4)...)
	webp = append(webp[:8], []byte("WEBPVP8 ")...)
	avif := make([]byte, 24)
	copy(avif[4:], []byte("ftypavif"))

	cases := []struct {
		name string
		in   []byte
		mime string
		ext  string
		ok   bool
	}{
		{"png", png1x1, "image/png", ".png", true},
		{"jpeg", jpeg, "image/jpeg", ".jpg", true},
		{"gif", gif, "image/gif", ".gif", true},
		{"webp", webp, "image/webp", ".webp", true},
		{"avif", avif, "image/avif", ".avif", true},
		{"txt", []byte("hello world not an image"), "", "", false},
		{"empty", nil, "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mime, ext, ok := sniffImage(tc.in)
			if ok != tc.ok || mime != tc.mime || ext != tc.ext {
				t.Fatalf("sniff(%s)=%s %s %v, want %s %s %v", tc.name, mime, ext, ok, tc.mime, tc.ext, tc.ok)
			}
		})
	}
}

func TestValidKind(t *testing.T) {
	for _, k := range []string{"product", "avatar", "banner", "story", "review"} {
		if !ValidKind(k) {
			t.Fatalf("kind %s should be valid", k)
		}
	}
	if ValidKind("video") || ValidKind("") {
		t.Fatal("unexpected kind")
	}
}

func TestMaxFilesForKind(t *testing.T) {
	if MaxFilesForKind("review") != 4 {
		t.Fatal("review max files")
	}
	if MaxFilesForKind("product") != 12 {
		t.Fatal("product max files")
	}
}

func TestSafeUserDir(t *testing.T) {
	if !safeUserDir("user-ab12cd34") || safeUserDir("../etc") || safeUserDir("user/id") {
		t.Fatal("safeUserDir")
	}
}
