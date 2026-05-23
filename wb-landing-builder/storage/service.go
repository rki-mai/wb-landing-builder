package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"math"

	"github.com/mohae/deepcopy"
	"github.com/rki-mai/wb-landing-builder/config"
	"go.mongodb.org/mongo-driver/bson"
)

const (
	CollapsThreshould = 20
)

type DraftService struct {
	repo      DraftRepository
	semaphore chan struct{}
}

func NewDraftService(repo DraftRepository, cfg *config.Config) DraftService {
	return DraftService{
		repo:      repo,
		semaphore: make(chan struct{}, cfg.DBConfig.MaxConnections),
	}
}

func (s *DraftService) mergeBSON(dst, src map[string]interface{}) {
	if dst == nil {
		dst = make(map[string]interface{})
	}
	if src == nil {
		return
	}
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

func (s *DraftService) checkOwnership(ctx context.Context, projectID string, userID string) error {
	ownerID, err := s.repo.GetDraftOwner(ctx, projectID)
	if err != nil {
		return err
	}
	if ownerID == "" {
		return fmt.Errorf("draft not found")
	}
	if ownerID != userID {
		return fmt.Errorf("forbidden")
	}
	return nil
}

func (s *DraftService) ApplyMutation(ctx context.Context, projectID string, userID string, mutation Mutation) (int, error) {
	s.semaphore <- struct{}{}
	defer func() { <-s.semaphore }()
	ownerID, err := s.repo.GetDraftOwner(ctx, projectID)
	if err != nil {
		return 0, err
	}
	if ownerID != "" && ownerID != userID {
		return 0, fmt.Errorf("forbidden")
	}
	mutationToInsert := mutation.Data
	mutationToInsert["deleted"] = mutation.Operation == OperationDelete
	if mutation.Operation == OperationUpdate {
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
	version, err := s.repo.InsertMutation(ctx, projectID, userID, mutationToInsert)
	if err != nil {
		return 0, err
	}
	if version%CollapsThreshould == 0 {
		mutations, err := s.collapseMutations(ctx, projectID, version)
		if err != nil {
			return 0, err
		}
		s.repo.InsertDraft(ctx, projectID, userID, mutations, version)
	}
	return version, nil
}

func toBSONMArraySafe(arr bson.A) []bson.M {
	result := make([]bson.M, 0, len(arr))
	for _, v := range arr {
		if m, ok := v.(bson.M); ok {
			result = append(result, m)
		}
	}
	return result
}

func (s *DraftService) collapseMutations(ctx context.Context, projectID string, version int) ([]bson.M, error) {
	latestDraft, err := s.repo.GetDraft(ctx, projectID, version)
	if err != nil {
		return nil, err
	}
	var fromVersion int
	if m, ok := latestDraft["version"].(int32); ok {
		fromVersion = int(m)
	} else {
		fromVersion = latestDraft["version"].(int)
	}
	draftMutations := toBSONMArraySafe(latestDraft["mutations"].(bson.A))
	if fromVersion == version {
		return draftMutations, nil
	}
	mutations, err := s.repo.GetMutationsInRange(ctx, projectID, fromVersion+1, version)
	if err != nil {
		return nil, err
	}
	seenIDs := make(map[string]bool)
	uniqueMutations := make([]bson.M, 0)
	combined := append(draftMutations, *mutations...)
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

func (s *DraftService) GetDraft(ctx context.Context, projectID string, userID string, version int) ([]byte, error) {
	s.semaphore <- struct{}{}
	defer func() { <-s.semaphore }()
	if err := s.checkOwnership(ctx, projectID, userID); err != nil {
		return nil, err
	}
	elements, err := s.collapseMutations(ctx, projectID, version)
	if err != nil {
		return nil, err
	}
	actualVersion := version
	if version == math.MaxInt {
		actualVersion = getMaxVersion(elements)
	}
	response := struct {
		Version  int      `json:"version"`
		Elements []bson.M `json:"elements"`
	}{
		Version:  actualVersion,
		Elements: elements,
	}
	jsonData, err := json.Marshal(response)
	if err != nil {
		return nil, err
	}
	return jsonData, nil
}

func (s *DraftService) GetLatestDraft(ctx context.Context, projectID string, userID string) ([]byte, error) {
	jsonData, err := s.GetDraft(ctx, projectID, userID, math.MaxInt)
	if err != nil {
		return nil, err
	}
	return jsonData, nil
}

func getMaxVersion(elements []bson.M) int {
	if len(elements) == 0 {
		return 0
	}
	maxV := elements[0]["version"].(int32)
	for _, e := range elements[1:] {
		if e["version"].(int32) > maxV {
			maxV = e["version"].(int32)
		}
	}
	return int(maxV)
}
