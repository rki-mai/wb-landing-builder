package utils

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNginxCacheFileNameIsMd5Hex(t *testing.T) {
	name := nginxCacheFileName("/publications/pub-1/index.html")
	if len(name) != 32 {
		t.Fatalf("cache file name length = %d, want 32", len(name))
	}
}

func TestFileCDNCachePurgerRemovesCacheFile(t *testing.T) {
	cacheDir := t.TempDir()
	cacheKey := "/publications/pub-123/index.html"
	cacheFileName := nginxCacheFileName(cacheKey)
	cacheFile := filepath.Join(cacheDir, "a", "bc", cacheFileName)

	if err := os.MkdirAll(filepath.Dir(cacheFile), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(cacheFile, []byte("cached"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	purger := NewFileCDNCachePurger(cacheDir)
	if err := purger.PurgePublication(context.Background(), "pub-123"); err != nil {
		t.Fatalf("PurgePublication() error = %v", err)
	}

	if _, err := os.Stat(cacheFile); !os.IsNotExist(err) {
		t.Fatalf("cache file still exists: %v", err)
	}
}

func TestFileCDNCachePurgerNoopWhenPathMissing(t *testing.T) {
	purger := NewFileCDNCachePurger("")
	if err := purger.PurgePublication(context.Background(), "pub-123"); err != nil {
		t.Fatalf("PurgePublication() error = %v", err)
	}
}
