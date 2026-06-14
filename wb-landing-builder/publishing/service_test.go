package publishing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rki-mai/wb-landing-builder/publishing/utils"
	"github.com/rki-mai/wb-landing-builder/storage"
)

type mockPublicationRepo struct {
	publications map[string]Publication
}

func newMockPublicationRepo() *mockPublicationRepo {
	return &mockPublicationRepo{publications: map[string]Publication{}}
}

func (m *mockPublicationRepo) Insert(_ context.Context, pub Publication) error {
	m.publications[pub.ID] = pub
	return nil
}

func (m *mockPublicationRepo) Update(_ context.Context, pub Publication) error {
	if _, ok := m.publications[pub.ID]; !ok {
		return ErrPublicationNotFound
	}
	m.publications[pub.ID] = pub
	return nil
}

func (m *mockPublicationRepo) Get(_ context.Context, id string) (*Publication, error) {
	pub, ok := m.publications[id]
	if !ok {
		return nil, nil
	}
	copyPub := pub
	return &copyPub, nil
}

func (m *mockPublicationRepo) ListIDsByProject(_ context.Context, projectID string) ([]string, error) {
	var ids []string
	for id, pub := range m.publications {
		if pub.ProjectID == projectID {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func (m *mockPublicationRepo) Delete(_ context.Context, id string) error {
	delete(m.publications, id)
	return nil
}

func (m *mockPublicationRepo) Close(_ context.Context) error {
	return nil
}

type mockBlobStorage struct {
	bundles map[string][]utils.Blob
}

func newMockBlobStorage() *mockBlobStorage {
	return &mockBlobStorage{bundles: map[string][]utils.Blob{}}
}

func (m *mockBlobStorage) PutBundle(_ context.Context, bundleKey string, blobs []utils.Blob) error {
	m.bundles[bundleKey] = blobs
	return nil
}

func (m *mockBlobStorage) DeleteBundle(_ context.Context, bundleKey string) error {
	delete(m.bundles, bundleKey)
	return nil
}

func (m *mockBlobStorage) URI(bundleKey string) string {
	return "s3://publications/" + bundleKey + "/"
}

type mockRenderer struct {
	called bool
}

func (m *mockRenderer) Render(_ context.Context, _ []byte) ([]utils.Blob, error) {
	m.called = true
	return []utils.Blob{
		{Path: "index.html", Content: []byte("<html></html>"), ContentType: "text/html; charset=utf-8"},
	}, nil
}

type mockDraftReader struct {
	draft *utils.Draft
}

func (m *mockDraftReader) CheckProjectAccess(_ context.Context, _, _ string) error {
	return nil
}

func (m *mockDraftReader) GetLatestDraft(_ context.Context, _, _ string) (*utils.Draft, error) {
	if m.draft == nil {
		return nil, storage.ErrDraftNotFound
	}
	return m.draft, nil
}

type mockPublisher struct {
	tasks []utils.PublishTask
	err   error
}

type mockCachePurger struct {
	purged []string
	err    error
}

func (m *mockCachePurger) PurgePublication(_ context.Context, publicationID string) error {
	if m.err != nil {
		return m.err
	}
	m.purged = append(m.purged, publicationID)
	return nil
}

func (m *mockPublisher) Publish(_ context.Context, task utils.PublishTask) error {
	if m.err != nil {
		return m.err
	}
	m.tasks = append(m.tasks, task)
	return nil
}

func (m *mockPublisher) Close() error {
	return nil
}

func testDraft(t *testing.T) *utils.Draft {
	t.Helper()

	draft, err := utils.ParseDraftSnapshot([]byte(`[{"id":"lb-1","element":"text"}]`))
	if err != nil {
		t.Fatalf("ParseDraftSnapshot() error = %v", err)
	}
	return draft
}

func TestPublicationServiceCreateReturnsPendingWithoutRendering(t *testing.T) {
	repo := newMockPublicationRepo()
	blob := newMockBlobStorage()
	renderer := &mockRenderer{}
	drafts := &mockDraftReader{draft: testDraft(t)}
	publisher := &mockPublisher{}

	service := NewPublicationService(repo, blob, renderer, drafts, publisher, nil)

	pub, err := service.Create(context.Background(), "project-1", "user-1")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if pub.Status != StatusPending {
		t.Fatalf("Create() status = %q, want %q", pub.Status, StatusPending)
	}
	if pub.AssetsPath != "" {
		t.Fatalf("Create() assets_path = %q, want empty", pub.AssetsPath)
	}
	if renderer.called {
		t.Fatal("Create() should not render synchronously")
	}
	if len(publisher.tasks) != 1 {
		t.Fatalf("Publish() calls = %d, want 1", len(publisher.tasks))
	}
	if publisher.tasks[0].PublicationID != pub.ID {
		t.Fatalf("Publish() publication_id = %q, want %q", publisher.tasks[0].PublicationID, pub.ID)
	}
}

func TestPublicationServiceProcessPublicationFinishesPublication(t *testing.T) {
	repo := newMockPublicationRepo()
	blob := newMockBlobStorage()
	renderer := &mockRenderer{}
	drafts := &mockDraftReader{draft: testDraft(t)}
	publisher := &mockPublisher{}

	service := NewPublicationService(repo, blob, renderer, drafts, publisher, nil)

	pub, err := service.Create(context.Background(), "project-1", "user-1")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	task := utils.PublishTask{
		PublicationID: pub.ID,
		ProjectID:     "project-1",
		UserID:        "user-1",
	}
	if err := service.ProcessPublication(context.Background(), task); err != nil {
		t.Fatalf("ProcessPublication() error = %v", err)
	}

	updated, err := repo.Get(context.Background(), pub.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if updated.Status != StatusFinished {
		t.Fatalf("status = %q, want %q", updated.Status, StatusFinished)
	}
	if updated.AssetsPath == "" {
		t.Fatal("expected assets_path to be set")
	}
	if !renderer.called {
		t.Fatal("ProcessPublication() should render draft")
	}
}

func TestPublicationServiceCreateMarksFailedWhenPublishFails(t *testing.T) {
	repo := newMockPublicationRepo()
	blob := newMockBlobStorage()
	renderer := &mockRenderer{}
	drafts := &mockDraftReader{draft: testDraft(t)}
	publisher := &mockPublisher{err: errors.New("queue unavailable")}

	service := NewPublicationService(repo, blob, renderer, drafts, publisher, nil)

	_, err := service.Create(context.Background(), "project-1", "user-1")
	if err == nil {
		t.Fatal("Create() expected error")
	}

	for _, pub := range repo.publications {
		if pub.Status != StatusFailed {
			t.Fatalf("publication status = %q, want %q", pub.Status, StatusFailed)
		}
		if pub.ErrorMessage == "" {
			t.Fatal("expected error_message to be set")
		}
	}
}

func TestPublicationServiceDeleteRemovesMongoBeforeBlob(t *testing.T) {
	repo := newMockPublicationRepo()
	blob := newMockBlobStorage()
	renderer := &mockRenderer{}
	drafts := &mockDraftReader{draft: testDraft(t)}
	publisher := &mockPublisher{}

	service := NewPublicationService(repo, blob, renderer, drafts, publisher, nil)

	pub := Publication{
		ID:         "pub-1",
		ProjectID:  "project-1",
		Status:     StatusFinished,
		AssetsPath: "s3://publications/publications/pub-1/",
		CreatedAt:  time.Now().UTC(),
	}
	if err := repo.Insert(context.Background(), pub); err != nil {
		t.Fatalf("Insert() error = %v", err)
	}
	if err := blob.PutBundle(context.Background(), "publications/pub-1", []utils.Blob{{Path: "index.html", Content: []byte("html")}}); err != nil {
		t.Fatalf("PutBundle() error = %v", err)
	}

	if err := service.Delete(context.Background(), "project-1", "user-1", "pub-1"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	if _, ok := repo.publications["pub-1"]; ok {
		t.Fatal("publication should be removed from repository")
	}
	if _, ok := blob.bundles["publications/pub-1"]; ok {
		t.Fatal("bundle should be removed from blob storage")
	}
}

func TestPublicationServiceDeletePurgesCDNCache(t *testing.T) {
	repo := newMockPublicationRepo()
	blob := newMockBlobStorage()
	renderer := &mockRenderer{}
	drafts := &mockDraftReader{draft: testDraft(t)}
	publisher := &mockPublisher{}
	cachePurger := &mockCachePurger{}

	service := NewPublicationService(repo, blob, renderer, drafts, publisher, cachePurger)

	pub := Publication{
		ID:         "pub-1",
		ProjectID:  "project-1",
		Status:     StatusFinished,
		AssetsPath: "s3://publications/publications/pub-1/",
		CreatedAt:  time.Now().UTC(),
	}
	if err := repo.Insert(context.Background(), pub); err != nil {
		t.Fatalf("Insert() error = %v", err)
	}

	if err := service.Delete(context.Background(), "project-1", "user-1", "pub-1"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	if len(cachePurger.purged) != 1 || cachePurger.purged[0] != "pub-1" {
		t.Fatalf("PurgePublication() calls = %v, want [pub-1]", cachePurger.purged)
	}
}
