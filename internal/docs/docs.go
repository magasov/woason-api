package docs

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

func Register(r chi.Router) {
	r.Get("/docs", serve("portal.html", "text/html; charset=utf-8"))
	r.Get("/docs/", serve("portal.html", "text/html; charset=utf-8"))
	r.Get("/docs/openapi.yaml", serve("openapi.yaml", "application/yaml; charset=utf-8"))
	r.Get("/docs/terms", serve("terms.html", "text/html; charset=utf-8"))
}

func serve(name, ctype string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		b, err := files.ReadFile(name)
		if err != nil {
			http.Error(w, "документ недоступен", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", ctype)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "public, max-age=120")
		if strings.HasSuffix(name, ".html") {
			w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' https://cdn.jsdelivr.net; style-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net https://fonts.googleapis.com; font-src https://fonts.gstatic.com https://cdn.jsdelivr.net; connect-src 'self' https://cdn.jsdelivr.net; img-src 'self' data:; worker-src blob: 'self'")
		}
		_, _ = w.Write(b)
	}
}
