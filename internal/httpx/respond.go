package httpx

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"woason-api/internal/models"
)

type ctxKey int

const principalKey ctxKey = 1

func Param(r *http.Request, name string) string {
	if v := chi.URLParam(r, name); v != "" {
		return v
	}
	return r.PathValue(name)
}

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func Error(w http.ResponseWriter, status int, msg string) {
	WriteJSON(w, status, map[string]string{"error": msg})
}

func Decode(r *http.Request, dst any) error {
	return json.NewDecoder(r.Body).Decode(dst)
}

func IsMultipart(r *http.Request) bool {
	return strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "multipart/")
}

func QueryInt(r *http.Request, key string, def, max int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return def
	}
	if max > 0 && n > max {
		return max
	}
	return n
}

func Page(r *http.Request) (limit, offset int) {
	return PageDefault(r, 24, 100)
}

func PageDefault(r *http.Request, def, max int) (limit, offset int) {
	if def <= 0 {
		def = 24
	}
	limit = QueryInt(r, "limit", def, max)
	if limit <= 0 {
		limit = def
	}
	offset = QueryInt(r, "offset", 0, 0)
	return limit, offset
}

func WithPrincipal(ctx context.Context, p *models.Principal) context.Context {
	return context.WithValue(ctx, principalKey, p)
}

func PrincipalFrom(r *http.Request) *models.Principal {
	v, _ := r.Context().Value(principalKey).(*models.Principal)
	return v
}

func MustPrincipal(r *http.Request) *models.Principal {
	p := PrincipalFrom(r)
	if p == nil {
		return &models.Principal{}
	}
	return p
}

func LogErr(msg string, err error) {
	if err == nil {
		return
	}
	slog.Error(msg, "err", err.Error())
}
