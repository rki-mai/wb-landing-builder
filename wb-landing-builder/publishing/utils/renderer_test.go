package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCollectOutputBlobs(t *testing.T) {
	rootDir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(rootDir, "assets"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(rootDir, "index.html"), []byte("<html></html>"), 0o644); err != nil {
		t.Fatalf("WriteFile(index.html) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(rootDir, "assets", "app.js"), []byte("console.log('ok')"), 0o644); err != nil {
		t.Fatalf("WriteFile(app.js) error = %v", err)
	}

	blobs, err := collectOutputBlobs(rootDir)
	if err != nil {
		t.Fatalf("collectOutputBlobs() error = %v", err)
	}
	if len(blobs) != 2 {
		t.Fatalf("collectOutputBlobs() len = %d, want 2", len(blobs))
	}

	byPath := map[string]Blob{}
	for _, blob := range blobs {
		byPath[blob.Path] = blob
	}

	index, ok := byPath["index.html"]
	if !ok {
		t.Fatal("expected index.html in output")
	}
	if index.ContentType != "text/html; charset=utf-8" {
		t.Fatalf("index.html content type = %q, want text/html; charset=utf-8", index.ContentType)
	}

	appJS, ok := byPath["assets/app.js"]
	if !ok {
		t.Fatal("expected assets/app.js in output")
	}
	if appJS.ContentType != "text/javascript; charset=utf-8" && appJS.ContentType != "application/javascript; charset=utf-8" {
		t.Fatalf("app.js content type = %q, want javascript mime type", appJS.ContentType)
	}
}

func TestContentTypeForPath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"index.html", "text/html; charset=utf-8"},
		{"styles.css", "text/css; charset=utf-8"},
		{"assets/app.js", contentTypeForPath("app.js")},
		{"image.webp", "image/webp"},
		{"unknown", "application/octet-stream"},
	}

	for _, tt := range tests {
		got := contentTypeForPath(tt.path)
		if got != tt.want {
			t.Errorf("contentTypeForPath(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}
