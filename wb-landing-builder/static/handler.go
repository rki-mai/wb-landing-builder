package static

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Handler отдаёт файлы из корневой директории по URL-пути.
type Handler struct {
	root string
}

// NewHandler создаёт handler для существующей директории.
func NewHandler(dir string) (*Handler, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, os.ErrNotExist
	}

	return &Handler{root: abs}, nil
}

// ServeHTTP отдаёт файл или HTML-листинг директории.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.NotFound(w, r)
		return
	}

	if strings.Contains(r.URL.Path, "..") {
		http.NotFound(w, r)
		return
	}

	rel := strings.TrimPrefix(r.URL.Path, "/")
	full := h.root
	if rel != "" {
		full = filepath.Join(h.root, rel)
	}

	if !isUnderRoot(h.root, full) {
		http.NotFound(w, r)
		return
	}

	info, err := os.Stat(full)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if info.IsDir() {
		if err := writeDirectoryListing(w, r, r.URL.Path, os.DirFS(full)); err != nil {
			http.Error(w, "failed to read directory", http.StatusInternalServerError)
		}
		return
	}

	file, err := os.Open(full)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()

	http.ServeContent(w, r, info.Name(), info.ModTime(), file)
}

// TryRegister подключает отдачу статики, если директория существует.
// При отсутствии директории handler не регистрируется.
func TryRegister(mux *http.ServeMux, dir string) {
	if dir == "" {
		return
	}

	handler, err := NewHandler(dir)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("Static files: directory %q not found, skipping", dir)
			return
		}
		log.Printf("Static files: skipping %q: %v", dir, err)
		return
	}

	mux.Handle("GET /", handler)
	mux.Handle("HEAD /", handler)
	mux.Handle("GET /{path...}", handler)
	mux.Handle("HEAD /{path...}", handler)
	log.Printf("Static files: serving from %s", handler.root)
}

func isUnderRoot(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
