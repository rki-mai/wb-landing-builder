package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/xeipuuv/gojsonschema"

	"github.com/rki-mai/wb-landing-builder/storage/config"
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

func (h *DraftHandler) RegisterRoutes(
	mux *http.ServeMux,
	middleware func(http.Handler) http.Handler,
) {
	mux.Handle(
		"POST /api/v1/storage/{project_id}/mutations",
		middleware(http.HandlerFunc(h.applyMutation)),
	)
	mux.Handle(
		"GET /api/v1/storage/{project_id}",
		middleware(http.HandlerFunc(h.sendLatestPage)),
	)
	mux.Handle(
		"GET /api/v1/storage/{project_id}/versions/{version}",
		middleware(http.HandlerFunc(h.sendPage)),
	)
}

func (h *DraftHandler) handleLimit(w http.ResponseWriter, projectID string) bool {
	allowed, _, retryAfter := h.rateLimiter.Allow(projectID)
	if !allowed {
		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(h.rateLimiter.limit))
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(retryAfter).Unix(), 10))
		w.Header().Set("Retry-After", strconv.FormatInt(int64(retryAfter.Seconds()), 10))

		writeJSONError(w, http.StatusTooManyRequests,
			fmt.Sprintf("rate limit exceeded. Limit: %d requests per %v. Retry after: %v",
				h.rateLimiter.limit, h.rateLimiter.window, retryAfter))
		return false
	}
	return true
}

func (h *DraftHandler) applyMutation(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("project_id")

	if projectID == "" {
		writeJSONError(w, http.StatusBadRequest, "invalid URI: missing project_id")
		return
	}

	if !h.handleLimit(w, projectID) {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, MaxBodySize)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeJSONError(w, http.StatusRequestEntityTooLarge, "payload too large")
			return
		}
		writeJSONError(w, http.StatusBadRequest, "failed to read body")
		return
	}

	documentLoader := gojsonschema.NewBytesLoader(body)
	result, err := h.schema.Validate(documentLoader)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("failed to perform json validation: %s", err))
		return
	}

	if !result.Valid() {
		output := "json does not match schema. Errors:\n"
		for i, desc := range result.Errors() {
			output += fmt.Sprintf("\t%d: %s\n", i+1, desc)
		}
		writeJSONError(w, http.StatusBadRequest, output)
		return
	}

	var mutation Mutation
	if err := json.Unmarshal(body, &mutation); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid mutation payload: "+err.Error())
		return
	}

	version, err := h.service.ApplyMutation(r.Context(), projectID, mutation)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to apply mutation: "+err.Error())
		return
	}

	writeJSONResponse(w, http.StatusOK, map[string]string{"status": "ok", "version": strconv.FormatInt(int64(version), 10)})
}

func (h *DraftHandler) sendLatestPage(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("project_id")
	if projectID == "" {
		writeJSONError(w, http.StatusBadRequest, "invalid URI: missing project_id")
		return
	}

	page, err := h.service.GetLatestDraft(r.Context(), projectID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to get page: "+err.Error())
		return
	}

	writeJSONResponse(w, http.StatusOK, page)
}

func (h *DraftHandler) sendPage(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("project_id")
	version, err := strconv.Atoi(r.PathValue("version"))

	if projectID == "" {
		writeJSONError(w, http.StatusBadRequest, "invalid URI: missing project_id")
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid URI: invalid version")
		return
	}

	page, err := h.service.GetDraft(r.Context(), projectID, version)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to get page: "+err.Error())
		return
	}

	writeJSONResponse(w, http.StatusOK, page)
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	writeJSONResponse(w, status, map[string]string{"error": message})
}

func writeJSONResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	switch v := data.(type) {
	case []byte:
		w.Write(v)
	default:
		if err := json.NewEncoder(w).Encode(data); err != nil {
			http.Error(w, "failed to encode response", http.StatusInternalServerError)
		}
	}
}
