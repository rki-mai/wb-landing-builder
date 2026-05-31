package publishing

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rki-mai/wb-landing-builder/publishing/utils"
)

// PublicationService управляет созданием публикаций: постановка задачи и обработка worker'ом.
type PublicationService struct {
	repo      PublicationRepository
	blob      utils.BlobStorage
	render    utils.Renderer
	drafts    utils.DraftReader
	publisher utils.Publisher
}

// NewPublicationService создаёт сервис публикаций с заданными зависимостями.
func NewPublicationService(
	repo PublicationRepository,
	blob utils.BlobStorage,
	render utils.Renderer,
	drafts utils.DraftReader,
	publisher utils.Publisher,
) *PublicationService {
	return &PublicationService{
		repo:      repo,
		blob:      blob,
		render:    render,
		drafts:    drafts,
		publisher: publisher,
	}
}

func (s *PublicationService) ensureProjectAccess(ctx context.Context, projectID, userID string) error {
	return s.drafts.CheckProjectAccess(ctx, projectID, userID)
}

func (s *PublicationService) Create(ctx context.Context, projectID, userID string) (*Publication, error) {
	if err := s.ensureProjectAccess(ctx, projectID, userID); err != nil {
		return nil, err
	}

	if _, err := s.drafts.GetLatestDraft(ctx, projectID, userID); err != nil {
		return nil, err
	}

	id := uuid.NewString()
	pub := Publication{
		ID:        id,
		ProjectID: projectID,
		Version:   0,
		Status:    StatusPending,
		CreatedAt: time.Now().UTC(),
	}

	if err := s.repo.Insert(ctx, pub); err != nil {
		return nil, fmt.Errorf("failed to save publication: %w", err)
	}

	task := utils.PublishTask{
		PublicationID: id,
		ProjectID:     projectID,
		UserID:        userID,
	}
	if err := s.publisher.Publish(ctx, task); err != nil {
		pub.Status = StatusFailed
		pub.ErrorMessage = fmt.Sprintf("failed to enqueue publication task: %v", err)
		_ = s.repo.Update(ctx, pub)
		return nil, fmt.Errorf("failed to enqueue publication: %w", err)
	}

	return &pub, nil
}

// ProcessPublication выполняет рендер и загрузку bundle для задачи из очереди.
func (s *PublicationService) ProcessPublication(ctx context.Context, task utils.PublishTask) error {
	pub, err := s.repo.Get(ctx, task.PublicationID)
	if err != nil {
		return err
	}
	if pub == nil {
		return nil
	}
	if pub.Status == StatusFinished || pub.Status == StatusFailed {
		return nil
	}

	pub.Status = StatusProcessing
	pub.ErrorMessage = ""
	if err := s.repo.Update(ctx, *pub); err != nil {
		return err
	}

	if err := s.renderAndUpload(ctx, task.PublicationID, task.ProjectID, task.UserID, pub); err != nil {
		pub.Status = StatusFailed
		pub.ErrorMessage = err.Error()
		_ = s.repo.Update(ctx, *pub)
		return err
	}

	return nil
}

func (s *PublicationService) renderAndUpload(
	ctx context.Context,
	publicationID, projectID, userID string,
	pub *Publication,
) error {
	draft, err := s.drafts.GetLatestDraft(ctx, projectID, userID)
	if err != nil {
		return fmt.Errorf("failed to load draft: %w", err)
	}

	draftJSON, err := draft.JSON()
	if err != nil {
		return fmt.Errorf("failed to encode draft: %w", err)
	}

	html, err := s.render.Render(ctx, draftJSON)
	if err != nil {
		return fmt.Errorf("failed to render draft: %w", err)
	}

	bundleKey := "publications/" + publicationID
	blobs := []utils.Blob{
		{Path: "index.html", Content: html, ContentType: "text/html; charset=utf-8"},
	}

	if err := s.blob.PutBundle(ctx, bundleKey, blobs); err != nil {
		return fmt.Errorf("failed to upload bundle: %w", err)
	}

	pub.Status = StatusFinished
	pub.AssetsPath = s.blob.URI(bundleKey)
	pub.ErrorMessage = ""
	if err := s.repo.Update(ctx, *pub); err != nil {
		_ = s.blob.DeleteBundle(context.Background(), bundleKey)
		return fmt.Errorf("failed to update publication: %w", err)
	}

	return nil
}

func (s *PublicationService) ListIDsByProject(ctx context.Context, projectID, userID string) ([]string, error) {
	if err := s.ensureProjectAccess(ctx, projectID, userID); err != nil {
		return nil, err
	}
	return s.repo.ListIDsByProject(ctx, projectID)
}

func (s *PublicationService) Get(ctx context.Context, projectID, userID, id string) (*Publication, error) {
	if err := s.ensureProjectAccess(ctx, projectID, userID); err != nil {
		return nil, err
	}

	pub, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if pub == nil || pub.ProjectID != projectID {
		return nil, ErrPublicationNotFound
	}
	return pub, nil
}

func (s *PublicationService) Delete(ctx context.Context, projectID, userID, id string) error {
	if err := s.ensureProjectAccess(ctx, projectID, userID); err != nil {
		return err
	}

	pub, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if pub == nil || pub.ProjectID != projectID {
		return ErrPublicationNotFound
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}

	if pub.AssetsPath != "" {
		bundleKey := "publications/" + id
		if err := s.blob.DeleteBundle(ctx, bundleKey); err != nil {
			return fmt.Errorf("failed to delete bundle: %w", err)
		}
	}

	return nil
}
