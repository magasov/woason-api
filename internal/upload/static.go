package upload

import (
	"net/http"
	"os"
	"strings"
)

func FileServer(dir string) http.Handler {
	if strings.TrimSpace(dir) == "" {
		dir = "uploads"
	}
	fs := http.StripPrefix("/uploads/", http.FileServer(noListDir{http.Dir(dir)}))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cross-Origin-Resource-Policy", "cross-origin")
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		fs.ServeHTTP(w, r)
	})
}

type noListDir struct {
	fs http.FileSystem
}

func (n noListDir) Open(name string) (http.File, error) {
	f, err := n.fs.Open(name)
	if err != nil {
		return nil, err
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	if st.IsDir() {
		f.Close()
		return nil, os.ErrNotExist
	}
	return f, nil
}
