package auth

import (
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"unicode"
)

var phoneDigits = regexp.MustCompile(`\d+`)

func ValidEmail(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	_, err := mail.ParseAddress(s)
	return err == nil
}

func ValidPhone(s string) bool {
	digits := strings.Join(phoneDigits.FindAllString(s, -1), "")
	return len(digits) >= 10 && len(digits) <= 15
}

func ValidPhotoURL(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	u, err := url.Parse(s)
	if err != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	if u.Host == "" {
		return false
	}
	return true
}

// ValidAssetRef принимает http(s) URL (после /uploads) или старые текстовые логотипы (эмодзи).
// blob: и data: отклоняются — их нельзя сохранять.
func ValidAssetRef(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return true
	}
	lower := strings.ToLower(s)
	if strings.HasPrefix(lower, "blob:") || strings.HasPrefix(lower, "data:") {
		return false
	}
	if strings.Contains(s, "://") {
		return ValidPhotoURL(s)
	}
	if strings.HasPrefix(s, "/uploads/") && !strings.Contains(s, "..") {
		return true
	}
	return !strings.ContainsAny(s, "/\\")
}

func ValidPassword(s string) bool {
	return len([]rune(s)) >= 4
}

func ValidName(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
