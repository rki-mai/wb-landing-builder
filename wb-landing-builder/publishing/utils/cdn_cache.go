package utils

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CachePurger сбрасывает закэшированные публичные страницы публикаций.
type CachePurger interface {
	PurgePublication(ctx context.Context, publicationID string) error
}

// FileCDNCachePurger удаляет запись nginx proxy_cache по ключу $request_uri.
type FileCDNCachePurger struct {
	cachePath string
}

// NewFileCDNCachePurger создаёт purger для локального nginx cache (shared volume).
func NewFileCDNCachePurger(cachePath string) *FileCDNCachePurger {
	return &FileCDNCachePurger{
		cachePath: strings.TrimSpace(cachePath),
	}
}

func (p *FileCDNCachePurger) PurgePublication(_ context.Context, publicationID string) error {
	if p == nil || p.cachePath == "" {
		return nil
	}

	cacheKey := "/publications/" + publicationID + "/index.html"
	cacheFileName := nginxCacheFileName(cacheKey)

	var (
		removed bool
		err     error
	)
	err = filepath.WalkDir(p.cachePath, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || entry.Name() != cacheFileName {
			return nil
		}

		if removeErr := os.Remove(path); removeErr != nil {
			return removeErr
		}
		removed = true
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to purge cdn cache file: %w", err)
	}
	if !removed {
		return nil
	}

	return nil
}

func nginxCacheFileName(cacheKey string) string {
	sum := md5.Sum([]byte(cacheKey))
	return hex.EncodeToString(sum[:])
}
