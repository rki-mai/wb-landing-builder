package utils

import (
	"context"

	"github.com/rki-mai/wb-landing-builder/storage"
)

// DraftReader загружает актуальный снимок черновика из компонента storage.
type DraftReader interface {
	GetLatestDraft(ctx context.Context, projectID string) ([]byte, error)
}

type storageDraftReader struct {
	drafts storage.DraftService
}

// NewStorageDraftReader создаёт читатель черновиков поверх DraftService.
func NewStorageDraftReader(drafts storage.DraftService) DraftReader {
	return &storageDraftReader{drafts: drafts}
}

func (r *storageDraftReader) GetLatestDraft(ctx context.Context, projectID string) ([]byte, error) {
	return r.drafts.GetLatestDraft(ctx, projectID)
}
