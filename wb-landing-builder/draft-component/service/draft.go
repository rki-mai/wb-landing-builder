package service

import (
	"context"
	"encoding/json"
	"math"

	"github.com/jinzhu/copier"
	"github.com/rki-mai/wb-landing-builder/draft-component/models"
	"github.com/rki-mai/wb-landing-builder/draft-component/repository"
	"go.mongodb.org/mongo-driver/bson"
)

const (
	CollapsThreshould = 20
)

type DraftService struct {
	repo repository.DraftRepository
}

func NewDraftService(repo repository.DraftRepository) *DraftService {
	return &DraftService{
		repo: repo,
	}
}

func (s *DraftService) mergeBSON(dst, src bson.M) {
	for key, srcVal := range src {
		if dstVal, exists := dst[key]; exists {
			srcMap, srcIsMap := srcVal.(bson.M)
			dstMap, dstIsMap := dstVal.(bson.M)

			if srcIsMap && dstIsMap {
				s.mergeBSON(dstMap, srcMap)
				continue
			}
		}
		dst[key] = srcVal
	}
}

func (s *DraftService) ApplyMutation(ctx context.Context, projectID string, mutation models.Mutation) (int64, error) {
	mutationToInsert := mutation.Data
	if mutation.Operation == models.OperationUpdate {
		latestMutation, err := s.repo.GetLatestMutationForID(ctx, projectID, mutation.Data["id"].(string))
		if err != nil {
			return 0, err
		}
		copier.CopyWithOption(&mutationToInsert, latestMutation, copier.Option{DeepCopy: true})
		s.mergeBSON(mutationToInsert, mutation.Data)
	}
	version, err := s.repo.InsertMutation(ctx, projectID, mutationToInsert)
	if err != nil {
		return 0, err
	}
	if version%CollapsThreshould == 0 {
		mutations, err := s.collapseMutations(ctx, projectID, version)
		if err != nil {
			return 0, err
		}
		s.repo.InsertDraft(ctx, projectID, mutations, version)
	}
	return version, nil
}

func (s *DraftService) collapseMutations(ctx context.Context, projectID string, version int64) ([]bson.M, error) {
	latestDraft, err := s.repo.GetDraft(ctx, projectID, version)
	if err != nil {
		return nil, err
	}
	prevVersion := latestDraft["version"].(int64) + 1
	if prevVersion >= version {
		return latestDraft["mutations"].([]bson.M), nil
	}
	mutations, err := s.repo.GetMutationsInRange(ctx, projectID, latestDraft["version"].(int64)+1, version)
	if err != nil {
		return nil, err
	}
	seenIDs := make(map[string]bool)
	uniqueMutations := make([]bson.M, 0)
	for _, mutation := range append(latestDraft["mutations"].([]bson.M), *mutations...) {
		id := mutation["element_id"].(string)
		if mutation["deleted"].(bool) {
			seenIDs[id] = true
		}
		if !seenIDs[id] {
			seenIDs[id] = true
			uniqueMutations = append(uniqueMutations, mutation)
		}
	}
	return uniqueMutations, nil
}

func (s *DraftService) GetDraft(ctx context.Context, projectID string, version int64) ([]byte, error) {
	mutations, err := s.collapseMutations(ctx, projectID, version)
	if err != nil {
		return nil, err
	}
	jsonData, err := json.Marshal(mutations)
	if err != nil {
		return nil, err
	}
	return jsonData, nil
}

func (s *DraftService) GetLatestDraft(ctx context.Context, projectID string) ([]byte, error) {
	jsonData, err := s.GetDraft(ctx, projectID, math.MaxInt64)
	if err != nil {
		return nil, err
	}
	return jsonData, nil
}
