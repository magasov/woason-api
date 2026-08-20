package upload

import (
	"bytes"
	"net/http"
	"strings"
)

const (
	MaxFiles    = 12
	MaxFileSize = 10 << 20
	maxBodySize = MaxFiles*MaxFileSize + 1<<20
)

var allowedKinds = map[string]struct{}{
	"product": {},
	"avatar":  {},
	"banner":  {},
	"story":   {},
	"review":  {},
}

func MaxFilesForKind(kind string) int {
	if strings.TrimSpace(kind) == "review" {
		return 4
	}
	return MaxFiles
}

var mimeExt = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
	"image/gif":  ".gif",
	"image/avif": ".avif",
}

func ValidKind(kind string) bool {
	_, ok := allowedKinds[strings.TrimSpace(kind)]
	return ok
}

func sniffImage(head []byte) (mime, ext string, ok bool) {
	if isAVIF(head) {
		return "image/avif", ".avif", true
	}
	ct := http.DetectContentType(head)
	if ext, ok := mimeExt[ct]; ok {
		return ct, ext, true
	}
	if isWEBP(head) {
		return "image/webp", ".webp", true
	}
	return "", "", false
}

func isWEBP(b []byte) bool {
	return len(b) >= 12 && string(b[:4]) == "RIFF" && string(b[8:12]) == "WEBP"
}

func isAVIF(b []byte) bool {
	if len(b) < 12 || string(b[4:8]) != "ftyp" {
		return false
	}
	n := min(len(b), 64)
	return bytes.Contains(b[:n], []byte("avif")) || bytes.Contains(b[:n], []byte("avis"))
}

func safeUserDir(id string) bool {
	if id == "" || strings.Contains(id, "..") {
		return false
	}
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			continue
		}
		return false
	}
	return true
}
