package utils

import (
	"context"
	"fmt"

	"github.com/rki-mai/wb-landing-builder/storage"
)

// DraftReader загружает актуальный снимок черновика из компонента storage.
type DraftReader interface {
	GetLatestDraft(ctx context.Context, projectID string) (*Draft, error)
}

type storageDraftReader struct {
	drafts storage.DraftService
}

// NewStorageDraftReader создаёт читатель черновиков поверх DraftService.
func NewStorageDraftReader(drafts storage.DraftService) DraftReader {
	return &storageDraftReader{drafts: drafts}
}

func (r *storageDraftReader) GetLatestDraft(ctx context.Context, projectID string) (*Draft, error) {
	data, err := r.drafts.GetLatestDraft(ctx, projectID)
	if err != nil {
		return nil, err
	}

	draft, err := parseDraftSnapshot(data)
	if err != nil {
		return nil, fmt.Errorf("failed to load draft for project %s: %w", projectID, err)
	}
	return draft, nil
}
