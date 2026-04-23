package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"

	"github.com/mohae/deepcopy"
	"github.com/rki-mai/wb-landing-builder/storage/config"
	"github.com/rki-mai/wb-landing-builder/storage/models"
	"github.com/rki-mai/wb-landing-builder/storage/repository"
	"go.mongodb.org/mongo-driver/bson"
)

const (
	CollapsThreshould = 20
)

type DraftService struct {
	repo      repository.DraftRepository
	semaphore chan struct{}
}

func NewDraftService(repo repository.DraftRepository, cfg *config.Config) DraftService {
	return DraftService{
		repo:      repo,
		semaphore: make(chan struct{}, cfg.DBConfig.MaxConnections),
	}
}

func (s *DraftService) mergeBSON(dst, src map[string]interface{}) {
	for key, srcVal := range src {
		if dstVal, exists := dst[key]; exists {

			srcMap, srcIsMap := toMap(srcVal)
			dstMap, dstIsMap := toMap(dstVal)

			if srcIsMap && dstIsMap {
				s.mergeBSON(dstMap, srcMap)
				dst[key] = dstMap
				continue
			}
		}

		dst[key] = srcVal
	}
}

func toMap(v interface{}) (map[string]interface{}, bool) {
	switch m := v.(type) {
	case map[string]interface{}:
		return m, true
	case bson.M:
		return m, true
	default:
		return nil, false
	}
}

func (s *DraftService) ApplyMutation(ctx context.Context, projectID string, mutation models.Mutation) (int, error) {
	s.semaphore <- struct{}{}
	defer func() { <-s.semaphore }()
	mutationToInsert := mutation.Data
	mutationToInsert["deleted"] = mutation.Operation == models.OperationDelete
	if mutation.Operation == models.OperationUpdate {
		latestMutation, err := s.repo.GetLatestMutationForID(ctx, projectID, mutation.Data["id"].(string))
		if err != nil {
			return 0, err
		}
		var ok bool
		mutationToInsert, ok = deepcopy.Copy(latestMutation).(bson.M)
		if !ok {
			return 0, fmt.Errorf("mutation update failed: copy error")
		}
		delete(mutationToInsert, "_id")
		fieldsBson := bson.M(mutation.Data["fields"].(map[string]interface{}))
		s.mergeBSON(mutationToInsert, fieldsBson)
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

func (s *DraftService) collapseMutations(ctx context.Context, projectID string, version int) ([]bson.M, error) {
	latestDraft, err := s.repo.GetDraft(ctx, projectID, version)
	if err != nil {
		return nil, err
	}
	fromVersion := latestDraft["version"].(int)
	if fromVersion == version {
		return latestDraft["mutations"].([]bson.M), nil
	}
	mutations, err := s.repo.GetMutationsInRange(ctx, projectID, fromVersion+1, version)
	if err != nil {
		return nil, err
	}
	seenIDs := make(map[string]bool)
	uniqueMutations := make([]bson.M, 0)
	combined := append(latestDraft["mutations"].([]bson.M), *mutations...)
	for i := len(combined) - 1; i >= 0; i-- {
		mutation := combined[i]
		id := mutation["id"].(string)
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

func (s *DraftService) GetDraft(ctx context.Context, projectID string, version int) ([]byte, error) {
	s.semaphore <- struct{}{}
	defer func() { <-s.semaphore }()
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
	jsonData, err := s.GetDraft(ctx, projectID, math.MaxInt)
	if err != nil {
		return nil, err
	}
	return jsonData, nil
}
