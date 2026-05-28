package utils

import (
	"context"
	"fmt"

	"github.com/rki-mai/wb-landing-builder/storage"
)

// DraftReader загружает черновики и проверяет доступ к проекту через storage.
type DraftReader interface {
	CheckProjectAccess(ctx context.Context, projectID, userID string) error
	GetLatestDraft(ctx context.Context, projectID, userID string) (*Draft, error)
}

type storageDraftReader struct {
	drafts storage.DraftService
}

// NewStorageDraftReader создаёт читатель черновиков поверх DraftService.
func NewStorageDraftReader(drafts storage.DraftService) DraftReader {
	return &storageDraftReader{drafts: drafts}
}

func (r *storageDraftReader) CheckProjectAccess(ctx context.Context, projectID, userID string) error {
	return r.drafts.CheckOwnership(ctx, projectID, userID)
}

func (r *storageDraftReader) GetLatestDraft(ctx context.Context, projectID, userID string) (*Draft, error) {
	data, err := r.drafts.GetLatestDraft(ctx, projectID, userID)
	if err != nil {
		return nil, err
	}

	draft, err := parseDraftSnapshot(data)
	if err != nil {
		return nil, fmt.Errorf("failed to load draft for project %s: %w", projectID, err)
	}
	return draft, nil
}
