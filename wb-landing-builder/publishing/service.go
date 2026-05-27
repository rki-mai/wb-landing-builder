package publishing

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rki-mai/wb-landing-builder/publishing/utils"
)

// PublicationService управляет созданием публикаций: черновик, рендер, загрузка в хранилище.
type PublicationService struct {
	repo   PublicationRepository
	blob   utils.BlobStorage
	render utils.Renderer
	drafts utils.DraftReader
}

// NewPublicationService создаёт сервис публикаций с заданными зависимостями.
func NewPublicationService(
	repo PublicationRepository,
	blob utils.BlobStorage,
	render utils.Renderer,
	drafts utils.DraftReader,
) *PublicationService {
	return &PublicationService{
		repo:   repo,
		blob:   blob,
		render: render,
		drafts: drafts,
	}
}

func (s *PublicationService) Create(ctx context.Context, projectID string) (*Publication, error) {
	draft, err := s.drafts.GetLatestDraft(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to load draft: %w", err)
	}

	draftJSON, err := draft.JSON()
	if err != nil {
		return nil, fmt.Errorf("failed to encode draft: %w", err)
	}

	html, err := s.render.Render(ctx, draftJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to render draft: %w", err)
	}

	id := uuid.NewString()
	bundleKey := "publications/" + id
	blobs := []utils.Blob{
		{Path: "index.json", Content: draftJSON, ContentType: "application/json"},
		{Path: "index.html", Content: html, ContentType: "text/html; charset=utf-8"},
	}

	if err := s.blob.PutBundle(ctx, bundleKey, blobs); err != nil {
		return nil, fmt.Errorf("failed to upload bundle: %w", err)
	}

	pub := Publication{
		ID:         id,
		ProjectID:  projectID,
		Version:    0,
		AssetsPath: s.blob.URI(bundleKey),
		Status:     StatusFinished,
		CreatedAt:  time.Now().UTC(),
	}

	if err := s.repo.Insert(ctx, pub); err != nil {
		_ = s.blob.DeleteBundle(context.Background(), bundleKey)
		return nil, fmt.Errorf("failed to save publication: %w", err)
	}

	return &pub, nil
}

func (s *PublicationService) ListIDsByProject(ctx context.Context, projectID string) ([]string, error) {
	return s.repo.ListIDsByProject(ctx, projectID)
}

func (s *PublicationService) Get(ctx context.Context, id string) (*Publication, error) {
	pub, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return pub, nil
}

func (s *PublicationService) Delete(ctx context.Context, id string) error {
	pub, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if pub == nil {
		return nil
	}

	bundleKey := "publications/" + id
	if err := s.blob.DeleteBundle(ctx, bundleKey); err != nil {
		return fmt.Errorf("failed to delete bundle: %w", err)
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	return nil
}
