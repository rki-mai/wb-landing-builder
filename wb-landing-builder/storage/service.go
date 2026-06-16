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

// CheckOwnership проверяет, что userID совпадает с владельцем проекта.
func (s *DraftService) CheckOwnership(ctx context.Context, projectID, userID string) error {
	project, err := s.repo.GetProject(ctx, projectID)
	if err != nil {
		return err
	}
	if project == nil {
		return ErrProjectNotFound
	}

	ownerID, ok := project["owner_id"].(string)
	if !ok || ownerID == "" {
		return ErrProjectNotFound
	}

	if ownerID != userID {
		return ErrForbidden
	}

	return nil
}

func (s *DraftService) ApplyMutation(ctx context.Context, projectID string, userID string, mutation Mutation) (int, error) {
	s.semaphore <- struct{}{}
	defer func() { <-s.semaphore }()
	if err := s.CheckOwnership(ctx, projectID, userID); err != nil {
		return 0, err
	}

	var mutationToInsert bson.M
	switch mutation.Operation {
	case OperationRevert:
		count, err := revertCountFromData(mutation.Data)
		if err != nil {
			return 0, err
		}
		mutationToInsert = bson.M{"revert": count}
	case OperationDelete:
		mutationToInsert = mutation.Data
		mutationToInsert["deleted"] = true
	case OperationUpdate:
		mutationID, ok := mutation.Data["id"].(string)
		if !ok || mutationID == "" {
			return 0, ErrInvalidMutation
		}
		effectiveMutation, err := s.getEffectiveElementMutation(ctx, projectID, mutationID)
		if err != nil {
			return 0, err
		}
		var okCopy bool
		mutationToInsert, okCopy = deepcopy.Copy(effectiveMutation).(bson.M)
		if !okCopy {
			return 0, fmt.Errorf("mutation update failed: copy error")
		}
		delete(mutationToInsert, "_id")
		fields, ok := mutation.Data["fields"].(map[string]interface{})
		if !ok {
			return 0, ErrInvalidMutation
		}
		fieldsBson := bson.M(fields)
		s.mergeBSON(mutationToInsert, fieldsBson)
		mutationToInsert["deleted"] = false
	default:
		mutationToInsert = mutation.Data
		mutationToInsert["deleted"] = false
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
		if err := s.repo.InsertDraft(ctx, projectID, userID, mutations, version); err != nil {
			return 0, err
		}
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
	combined := append(draftMutations, *mutations...)
	return dedupElementMutations(resolveMutations(combined)), nil
}

func (s *DraftService) getEffectiveElementMutation(ctx context.Context, projectID, elementID string) (bson.M, error) {
	version, err := s.repo.GetLatestMutationVersion(ctx, projectID)
	if err != nil {
		return nil, err
	}
	elements, err := s.collapseMutations(ctx, projectID, version)
	if err != nil {
		return nil, err
	}
	for _, element := range elements {
		if id, ok := element["id"].(string); ok && id == elementID {
			return element, nil
		}
	}
	return nil, ErrMutationNotFound
}

func revertCountFromData(data bson.M) (int, error) {
	if data == nil {
		return 0, ErrInvalidMutation
	}
	count, ok := intFromBSON(data["count"])
	if !ok || count < 1 {
		return 0, ErrInvalidMutation
	}
	return count, nil
}

func intFromBSON(value interface{}) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int32:
		return int(v), true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	default:
		return 0, false
	}
}

func isRevertMutation(mutation bson.M) bool {
	count, ok := intFromBSON(mutation["revert"])
	return ok && count > 0
}

func isElementMutation(mutation bson.M) bool {
	id, ok := mutation["id"].(string)
	return ok && id != ""
}

// resolveMutations восстанавливает эффективную историю (issue #52).
// combined — мутации в порядке версий (v1, v2, …). Collapse идёт от vn к v1;
// revert(N) на версии i отменяет N предшествующих записей (i-1, i-2, …).
func resolveMutations(combined []bson.M) []bson.M {
	active := make([]bool, len(combined))
	for i := range active {
		active[i] = true
	}

	for i := len(combined) - 1; i >= 0; i-- {
		if !active[i] || !isRevertMutation(combined[i]) {
			continue
		}
		revertN, _ := intFromBSON(combined[i]["revert"])
		for j := i - 1; j >= 0 && revertN > 0; j-- {
			if !active[j] {
				continue
			}
			active[j] = false
			revertN--
		}
		active[i] = false
	}

	survivors := make([]bson.M, 0, len(combined))
	for i, mutation := range combined {
		if active[i] && isElementMutation(mutation) {
			survivors = append(survivors, mutation)
		}
	}
	return survivors
}

func dedupElementMutations(mutations []bson.M) []bson.M {
	seenIDs := make(map[string]bool)
	uniqueMutations := make([]bson.M, 0, len(mutations))
	for i := len(mutations) - 1; i >= 0; i-- {
		mutation := mutations[i]
		id := mutation["id"].(string)
		deleted, _ := mutation["deleted"].(bool)
		if deleted {
			seenIDs[id] = true
		}
		if !seenIDs[id] {
			seenIDs[id] = true
			uniqueMutations = append(uniqueMutations, mutation)
		}
	}
	return uniqueMutations
}

func (s *DraftService) GetDraft(ctx context.Context, projectID string, userID string, version int) ([]byte, error) {
	s.semaphore <- struct{}{}
	defer func() { <-s.semaphore }()
	if err := s.CheckOwnership(ctx, projectID, userID); err != nil {
		return nil, err
	}
	elements, err := s.collapseMutations(ctx, projectID, version)
	if err != nil {
		return nil, err
	}
	actualVersion := version
	if version == math.MaxInt {
		actualVersion, err = s.repo.GetLatestMutationVersion(ctx, projectID)
		if err != nil {
			return nil, err
		}
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
	return s.GetDraft(ctx, projectID, userID, math.MaxInt)
}

func (s *DraftService) CreateProject(ctx context.Context, projectID string, ownerID string, name string) error {
	s.semaphore <- struct{}{}
	defer func() { <-s.semaphore }()

	project, err := s.repo.GetProject(ctx, projectID)
	if err != nil {
		return err
	}

	if project != nil {
		return ErrProjectAlreadyExists
	}

	return s.repo.CreateProject(ctx, projectID, ownerID, name)
}

func (s *DraftService) UpdateProjectName(ctx context.Context, projectID string, userID string, name string) error {
	if err := s.CheckOwnership(ctx, projectID, userID); err != nil {
		return err
	}
	return s.repo.UpdateProjectName(ctx, projectID, name)
}

func (s *DraftService) GetProject(ctx context.Context, projectID string) (bson.M, error) {
	s.semaphore <- struct{}{}
	defer func() { <-s.semaphore }()

	return s.repo.GetProject(ctx, projectID)
}

func (s *DraftService) GetUserProjects(ctx context.Context, userID string) ([]map[string]any, error) {
	return s.repo.GetUserProjects(ctx, userID)
}
