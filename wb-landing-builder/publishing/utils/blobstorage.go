package utils

import "context"

// Blob — один файл внутри bundle публикации.
type Blob struct {
	Path        string
	Content     []byte
	ContentType string
}

// BlobStorage сохраняет bundle публикации (один или несколько файлов).
type BlobStorage interface {
	PutBundle(ctx context.Context, bundleKey string, blobs []Blob) error
	DeleteBundle(ctx context.Context, bundleKey string) error
	URI(bundleKey string) string
}
