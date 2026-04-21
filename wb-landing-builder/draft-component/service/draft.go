package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/rki-mai/wb-landing-builder/draft-service/internal/model"
	"github.com/rki-mai/wb-landing-builder/draft-service/internal/repository"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/wI2L/jsondiff"
)

type DraftService interface {
	saveDraft(ctx context.Context, projectID int64) error
	savePatch(ctx context.Context, projectID int64) error
	GetAsDraft(ctx context.Context, draftID string, version int64) (*model.Draft, error)
}

type draftService struct {
	repo     repository.DraftRepository
	writeSem chan struct{}
}

func NewDraftService(repo repository.DraftRepository, maxConcurrentWrites int) DraftService {
	return &draftService{
		repo:     repo,
		writeSem: make(chan struct{}, maxConcurrentWrites),
	}
}

const (
	patchEfficiencyThreshold = 0.65
	FullCopyInterval         = 10

	patchesShrinkThreshold = 20
)

type ElementMeta struct {
	ID        string
	IsDeleted bool
}

func extractMetaFromContent(content []byte) (ElementMeta, error) {
	var data map[string]json.RawMessage
	if err := json.Unmarshal(content, &data); err != nil {
		return ElementMeta{}, err
	}

	for _, rawValue := range data {
		var innerObj map[string]interface{}
		if err := json.Unmarshal(rawValue, &innerObj); err != nil {
			continue
		}

		idVal, ok := innerObj["id"]
		if !ok {
			continue
		}
		idStr, ok := idVal.(string)
		if !ok {
			continue
		}

		isDeleted := false
		if delVal, ok := innerObj["isDeleted"]; ok {
			if b, ok := delVal.(bool); ok {
				isDeleted = b
			}
		}

		return ElementMeta{ID: idStr, IsDeleted: isDeleted}, nil
	}
	return ElementMeta{}, fmt.Errorf("no ID found")
}

func (s *draftService) saveDraft(ctx context.Context, projectID int64) error {
	drafts, err := s.repo.GetDrafts(ctx, projectID)
	if err != nil {
		return fmt.Errorf("failed to get drafts %w", err)
	}
	var oldPatches []model.Patch
	if err := json.Unmarshal(drafts[0].Content, &oldPatches); err != nil {
		return fmt.Errorf("error parsing content: %w", err)
	}
	var lastPatchVersion int64
	if len(drafts) > 0 {
		lastPatchVersion = drafts[0].Version
	}
	newPatches, err := s.repo.GetPatchesInRange(ctx, projectID, lastPatchVersion+1, lastPatchVersion+patchesShrinkThreshold)
	if err != nil {
		return fmt.Errorf("failed to get patches %w", err)
	}
	patches := append(oldPatches, newPatches...)
	seenIDs := make(map[string]bool)
	uniquePatches := make([]model.Patch, 0)
	for _, patch := range patches {
		meta, err := extractMetaFromContent(patch.Content)
		if err != nil {
			continue
		}
		if meta.IsDeleted {
			seenIDs[meta.ID] = true
		}
		if !seenIDs[meta.ID] {
			seenIDs[meta.ID] = true
			uniquePatches = append(uniquePatches, patch)
		}
	}
	newContent, err := json.Marshal(uniquePatches)
	if err != nil {
		return fmt.Errorf("failed to encode json %w", err)
	}
	s.writeSem <- struct{}{}
	defer func() { <-s.writeSem }()
	s.repo.SaveDraft(ctx, &model.Draft{
		ProjectID: projectID,
		Content:   newContent,
		Version:   uniquePatches[len(uniquePatches)-1].Version,
	})
	return nil
}

// parseDraftID вынесен для чистоты основной функции
func (s *draftService) parseDraftID(draftID string) (primitive.ObjectID, error) {
	if draftID == "" {
		return primitive.NewObjectID(), nil
	}
	objectID, err := primitive.ObjectIDFromHex(draftID)
	if err != nil {
		return primitive.NilObjectID, fmt.Errorf("invalid draft ID format: %w", err)
	}
	return objectID, nil
}

// createVersionModel инкапсулирует логику выбора Patch vs Snapshot
func (s *draftService) createVersionModel(
	objectID primitive.ObjectID,
	newVersion int64,
	latestContent, contentBytes []byte,
	currentVersion int64,
) (*model.Version, error) {

	// Условие для принудительного снапшота
	isSnapshotInterval := currentVersion > 0 && int(currentVersion)%FullCopyInterval == 0

	if isSnapshotInterval {
		return &model.Version{
			DraftID:  objectID,
			Version:  newVersion,
			Type:     model.VersionTypeSnapshot,
			Snapshot: contentBytes,
		}, nil
	}

	// Попытка создать патч
	patch, err := jsondiff.CompareJSON(latestContent, contentBytes)
	if err != nil {
		// Если не удалось сравнить (например, невалидный JSON), лучше сохранить как снапшот, чем упасть с ошибкой,
		// либо вернуть ошибку, если это критично. В твоем коде была ошибка, оставим её.
		return nil, fmt.Errorf("failed to create patch between drafts: %w", err)
	}

	// Проверка эффективности патча
	patchSize := len(patch)
	contentSize := len(contentBytes)

	// Защита от деления на ноль, хотя contentSize обычно > 0
	if contentSize == 0 {
		return &model.Version{
			DraftID:  objectID,
			Version:  newVersion,
			Type:     model.VersionTypeSnapshot,
			Snapshot: contentBytes,
		}, nil
	}

	isEfficientPatch := float64(patchSize)/float64(contentSize) < patchEfficiencyThreshold && patchSize > 0

	if isEfficientPatch {
		return &model.Version{
			DraftID: objectID,
			Version: newVersion,
			Type:    model.VersionTypePatch,
			Patch:   patch,
		}, nil
	}

	return &model.Version{
		DraftID:  objectID,
		Version:  newVersion,
		Type:     model.VersionTypeSnapshot,
		Snapshot: contentBytes,
	}, nil
}

func (s *draftService) GetDraft(ctx context.Context, draftID string, version int64) (*model.Draft, error) {
	objID, err := primitive.ObjectIDFromHex(draftID)
	if err != nil {
		return nil, fmt.Errorf("invalid draft ID format: %w", err)
	}
	return s.repo.GetByVersion(ctx, objID, version)
}
