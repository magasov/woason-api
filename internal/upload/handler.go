package upload

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"

	httpx "woason-api/internal/httpx"
)

type Handler struct {
	Dir           string
	PublicBaseURL string
}

type item struct {
	URL      string `json:"url"`
	Filename string `json:"filename"`
}

func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	p := httpx.MustPrincipal(r)
	if !safeUserDir(p.ID) {
		httpx.Error(w, http.StatusBadRequest, "некорректный пользователь")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			httpx.Error(w, http.StatusBadRequest, "файл слишком большой (максимум 10 МБ)")
			return
		}
		httpx.Error(w, http.StatusBadRequest, "ожидается multipart/form-data с полем files")
		return
	}

	kind := strings.TrimSpace(r.FormValue("kind"))
	if !ValidKind(kind) {
		httpx.Error(w, http.StatusBadRequest, "kind: product, avatar, banner, story или review")
		return
	}

	headers := r.MultipartForm.File["files"]
	if len(headers) == 0 {
		httpx.Error(w, http.StatusBadRequest, "добавьте файлы в поле files")
		return
	}
	maxFiles := MaxFilesForKind(kind)
	if len(headers) > maxFiles {
		httpx.Error(w, http.StatusBadRequest, "слишком много файлов (максимум "+strconv.Itoa(maxFiles)+")")
		return
	}

	dir := filepath.Join(h.uploadDir(), kind, p.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		httpx.LogErr("upload mkdir", err)
		httpx.Error(w, http.StatusInternalServerError, "не удалось сохранить файлы")
		return
	}

	base := strings.TrimRight(h.PublicBaseURL, "/")
	if base == "" {
		base = "http://localhost:8080"
	}

	urls := make([]string, 0, len(headers))
	items := make([]item, 0, len(headers))
	written := make([]string, 0, len(headers))
	cleanup := func() {
		for _, path := range written {
			_ = os.Remove(path)
		}
	}

	for _, fh := range headers {
		if fh.Size > MaxFileSize {
			cleanup()
			httpx.Error(w, http.StatusBadRequest, "файл слишком большой (максимум 10 МБ)")
			return
		}
		src, err := fh.Open()
		if err != nil {
			cleanup()
			httpx.Error(w, http.StatusBadRequest, "не удалось прочитать файл")
			return
		}
		head := make([]byte, 512)
		n, err := io.ReadFull(src, head)
		if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
			src.Close()
			cleanup()
			httpx.Error(w, http.StatusBadRequest, "не удалось прочитать файл")
			return
		}
		head = head[:n]
		_, ext, ok := sniffImage(head)
		if !ok {
			src.Close()
			cleanup()
			httpx.Error(w, http.StatusBadRequest, "можно загружать только изображения (jpeg, png, webp, gif, avif)")
			return
		}

		name := uuid.NewString() + ext
		dstPath := filepath.Join(dir, name)
		dst, err := os.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			src.Close()
			cleanup()
			httpx.LogErr("upload create", err)
			httpx.Error(w, http.StatusInternalServerError, "не удалось сохранить файлы")
			return
		}
		written = append(written, dstPath)

		rest := io.LimitReader(src, MaxFileSize+1-int64(len(head)))
		wn, err := dst.Write(head)
		if err == nil {
			var cn int64
			cn, err = io.Copy(dst, rest)
			wn += int(cn)
		}
		src.Close()
		closeErr := dst.Close()
		if int64(wn) > MaxFileSize {
			cleanup()
			httpx.Error(w, http.StatusBadRequest, "файл слишком большой (максимум 10 МБ)")
			return
		}
		if err != nil || closeErr != nil {
			cleanup()
			httpx.LogErr("upload write", err)
			httpx.Error(w, http.StatusInternalServerError, "не удалось сохранить файлы")
			return
		}

		public := base + "/uploads/" + kind + "/" + p.ID + "/" + name
		urls = append(urls, public)
		items = append(items, item{URL: public, Filename: name})
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"urls":  urls,
		"items": items,
	})
}

func (h *Handler) uploadDir() string {
	if strings.TrimSpace(h.Dir) != "" {
		return h.Dir
	}
	return "uploads"
}
