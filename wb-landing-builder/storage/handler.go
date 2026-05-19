package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/xeipuuv/gojsonschema"

	config "github.com/rki-mai/wb-landing-builder/configs"
)

const MaxBodySize = 1 << 20

type DraftHandler struct {
	service     DraftService
	schema      *gojsonschema.Schema
	rateLimiter *RateLimiter
}

type RateLimiter struct {
	mu            sync.RWMutex
	requests      map[string][]time.Time
	limit         int
	window        time.Duration
	cleanupTicker *time.Ticker
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		requests:      make(map[string][]time.Time),
		limit:         limit,
		window:        window,
		cleanupTicker: time.NewTicker(window),
	}

	go func() {
		for range rl.cleanupTicker.C {
			rl.cleanup()
		}
	}()

	return rl
}

func (rl *RateLimiter) Allow(projectID string) (bool, int, time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-rl.window)

	timestamps, exists := rl.requests[projectID]
	if !exists {
		rl.requests[projectID] = []time.Time{now}
		return true, rl.limit - 1, rl.window
	}

	filtered := make([]time.Time, 0, len(timestamps))
	for _, ts := range timestamps {
		if ts.After(windowStart) {
			filtered = append(filtered, ts)
		}
	}

	if len(filtered) >= rl.limit {
		oldest := filtered[0]
		retryAfter := rl.window - now.Sub(oldest)
		return false, 0, retryAfter
	}

	filtered = append(filtered, now)
	rl.requests[projectID] = filtered
	remaining := rl.limit - len(filtered)

	return true, remaining, rl.window - (now.Sub(filtered[0]))
}

func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-rl.window)

	for projectID, timestamps := range rl.requests {
		filtered := make([]time.Time, 0, len(timestamps))
		for _, ts := range timestamps {
			if ts.After(windowStart) {
				filtered = append(filtered, ts)
			}
		}
		if len(filtered) == 0 {
			delete(rl.requests, projectID)
		} else {
			rl.requests[projectID] = filtered
		}
	}
}

func (rl *RateLimiter) Stop() {
	rl.cleanupTicker.Stop()
}

func NewDraftHandler(svc DraftService, cfg *config.Config) (*DraftHandler, error) {
	schemaLoader := gojsonschema.NewReferenceLoader("file:///app/storage/schema.json")
	schema, err := gojsonschema.NewSchema(schemaLoader)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema file: %w", err)
	}

	return &DraftHandler{
		service:     svc,
		schema:      schema,
		rateLimiter: NewRateLimiter(cfg.RateLimit, time.Minute),
	}, nil
}

func (h *DraftHandler) RegisterRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "applyMutation",
		Method:      http.MethodPost,
		Path:        "/api/v1/storage/{project_id}/mutations",
		Summary:     "Apply mutation",
		Tags:        []string{"Storage"},
	}, h.applyMutation)

	huma.Register(api, huma.Operation{
		OperationID: "getLatestDraft",
		Method:      http.MethodGet,
		Path:        "/api/v1/storage/{project_id}",
		Summary:     "Get latest draft",
		Tags:        []string{"Storage"},
	}, h.sendLatestPage)

	huma.Register(api, huma.Operation{
		OperationID: "getDraftByVersion",
		Method:      http.MethodGet,
		Path:        "/api/v1/storage/{project_id}/versions/{version}",
		Summary:     "Get draft by version",
		Tags:        []string{"Storage"},
	}, h.sendPage)
}

func (h *DraftHandler) applyMutation(ctx context.Context, input *applyMutationInput) (*ApplyMutationResponse, error) {
	if input.ProjectID == "" {
		return nil, huma.Error400BadRequest("invalid URI: missing project_id")
	}

	allowed, _, retryAfter := h.rateLimiter.Allow(input.ProjectID)
	if !allowed {
		return nil, huma.Error429TooManyRequests(fmt.Sprintf(
			"rate limit exceeded. Limit: %d requests per %v. Retry after: %v",
			h.rateLimiter.limit, h.rateLimiter.window, retryAfter))
	}

	bodyBytes, err := json.Marshal(input.Body)
	if err != nil {
		return nil, huma.Error400BadRequest("failed to marshal mutation: " + err.Error())
	}

	if len(bodyBytes) > MaxBodySize {
		return nil, huma.Error413RequestEntityTooLarge("payload too large")
	}

	documentLoader := gojsonschema.NewBytesLoader(bodyBytes)
	result, err := h.schema.Validate(documentLoader)
	if err != nil {
		return nil, huma.Error400BadRequest(fmt.Sprintf("failed to perform json validation: %s", err))
	}

	if !result.Valid() {
		output := "json does not match schema. Errors:\n"
		for i, desc := range result.Errors() {
			output += fmt.Sprintf("\t%d: %s\n", i+1, desc)
		}
		return nil, huma.Error400BadRequest(output)
	}

	version, err := h.service.ApplyMutation(ctx, input.ProjectID, input.Body)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to apply mutation: " + err.Error())
	}

	return &ApplyMutationResponse{
		Body: struct {
			Status  string `json:"status" example:"success" doc:"Operation status"`
			Version string `json:"version" example:"1" doc:"Current version after mutation"`
		}{
			Status:  "ok",
			Version: strconv.FormatInt(int64(version), 10),
		},
	}, nil
}

func (h *DraftHandler) sendLatestPage(ctx context.Context, input *getLatestDraftInput) (*GetDraftResponse, error) {
	if input.ProjectID == "" {
		return nil, huma.Error400BadRequest("invalid URI: missing project_id")
	}

	page, err := h.service.GetLatestDraft(ctx, input.ProjectID)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to get page: " + err.Error())
	}

	return &GetDraftResponse{Body: Draft{Mutations: string(page)}}, nil
}

func (h *DraftHandler) sendPage(ctx context.Context, input *getDraftByVersionInput) (*GetDraftResponse, error) {
	if input.ProjectID == "" {
		return nil, huma.Error400BadRequest("invalid URI: missing project_id")
	}

	if input.Version <= 0 {
		return nil, huma.Error400BadRequest("invalid URI: invalid version")
	}

	page, err := h.service.GetDraft(ctx, input.ProjectID, input.Version)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to get page: " + err.Error())
	}

	return &GetDraftResponse{Body: Draft{Mutations: string(page)}}, nil
}
