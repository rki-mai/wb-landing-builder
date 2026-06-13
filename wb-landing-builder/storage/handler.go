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

	"github.com/google/uuid"
	"github.com/xeipuuv/gojsonschema"

	"github.com/rki-mai/wb-landing-builder/auth"
	"github.com/rki-mai/wb-landing-builder/config"
	"github.com/rki-mai/wb-landing-builder/httputil"
)

// MaxBodySize ограничивает размер тела запроса в 1 МБ
const MaxBodySize = 1 << 20

// DraftHandler обрабатывает HTTP-запросы, связанные с черновиками страниц.
type DraftHandler struct {
	service     DraftService
	schema      *gojsonschema.Schema
	rateLimiter *RateLimiter
}

// RateLimiter реализует алгоритм скользящего окна для ограничения частоты запросов.
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

// RegisterRoutes регистрирует маршруты для обработчика черновиков.
func (h *DraftHandler) RegisterRoutes(
	mux *http.ServeMux,
	middleware func(http.Handler) http.Handler,
) {
	mux.Handle(
		"GET /api/v1/projects",
		middleware(http.HandlerFunc(h.getUserProjects)),
	)
	mux.Handle(
		"POST /api/v1/projects",
		middleware(http.HandlerFunc(h.createProject)),
	)
	mux.Handle(
		"PATCH /api/v1/projects/{project_id}",
		middleware(http.HandlerFunc(h.updateProjectName)),
	)
	mux.Handle(
		"POST /api/v1/projects/{project_id}/draft/mutations",
		middleware(http.HandlerFunc(h.applyMutation)),
	)
	mux.Handle(
		"GET /api/v1/projects/{project_id}/draft",
		middleware(http.HandlerFunc(h.sendLatestPage)),
	)
	mux.Handle(
		"GET /api/v1/projects/{project_id}/draft/versions/{version}",
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

		httputil.WriteJSONError(w, http.StatusTooManyRequests,
			fmt.Sprintf("rate limit exceeded. Limit: %d requests per %v. Retry after: %v",
				h.rateLimiter.limit, h.rateLimiter.window, retryAfter))
		return false
	}
	return true
}

// GetUserProjects получает список проектов текущего пользователя.
// @Summary Получить проекты пользователя
// @Description Возвращает список ID проектов, в которых участвует текущий авторизованный пользователь
// @Tags Storage
// @Accept json
// @Security BearerAuth
// @Produce json
// @Success 200 {object} map[string][]string "Список ID проектов пользователя"
// @Failure 401 {object} ErrorResponse "Пользователь не авторизован"
// @Failure 500 {object} ErrorResponse "Ошибка получения списка проектов"
// @Router /api/v1/projects [get]
func (h *DraftHandler) getUserProjects(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(auth.UserIDKey).(string)
	if !ok {
		httputil.WriteJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	projectIDs, err := h.service.GetUserProjects(r.Context(), userID)
	if err != nil {
		httputil.WriteJSONError(w, http.StatusInternalServerError, "failed to get projects: "+err.Error())
		return
	}

	httputil.WriteJSONResponse(w, http.StatusOK, map[string]interface{}{
		"projects": projectIDs,
	})
}

// CreateProject создает новый пустой проект.
// @Summary Создать проект
// @Description Создает новый проект и назначает владельцем текущего авторизованного пользователя.
// @Tags Storage
// @Accept json
// @Security BearerAuth
// @Produce json
// @Success 201 {object} map[string]string "ID созданного проекта"
// @Failure 401 {object} ErrorResponse "Пользователь не авторизован"
// @Failure 409 {object} ErrorResponse "Проект уже существует"
// @Failure 500 {object} ErrorResponse "Ошибка создания проекта"
// @Router /api/v1/projects [post]
func (h *DraftHandler) createProject(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(auth.UserIDKey).(string)

	if !ok {
		httputil.WriteJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req CreateProjectRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		httputil.WriteJSONError(w, http.StatusBadRequest, "project name is required")
		return
	}

	projectID := uuid.NewString()
	err := h.service.CreateProject(r.Context(), projectID, userID, req.Name)
	if err != nil {
		if errors.Is(err, ErrProjectAlreadyExists) {
			httputil.WriteJSONError(w, http.StatusConflict, err.Error())
			return
		}
		httputil.WriteJSONError(w, http.StatusInternalServerError, "failed to create project: "+err.Error())
		return
	}

	httputil.WriteJSONResponse(w, http.StatusCreated, map[string]string{"project_id": projectID})
}

// UpdateProjectName изменяет имя проекта.
// @Summary Изменить имя проекта
// @Description Обновляет человекочитаемое имя существующего проекта.
// @Tags Storage
// @Accept json
// @Security BearerAuth
// @Produce json
// @Param project_id path string true "ID проекта"
// @Param request body UpdateProjectNameRequest true "Новое имя проекта"
// @Success 200 {object} map[string]string "Имя проекта успешно обновлено"
// @Failure 400 {object} ErrorResponse "Ошибка валидации или неверный запрос"
// @Failure 401 {object} ErrorResponse "Пользователь не авторизован"
// @Failure 403 {object} ErrorResponse "Доступ запрещен"
// @Failure 404 {object} ErrorResponse "Проект не найден"
// @Failure 500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router /api/v1/projects/{project_id} [patch]
func (h *DraftHandler) updateProjectName(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(auth.UserIDKey).(string)
	if !ok {
		httputil.WriteJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	projectID := r.PathValue("project_id")
	if projectID == "" {
		httputil.WriteJSONError(w, http.StatusBadRequest, "invalid URI: missing project_id")
		return
	}

	var req UpdateProjectNameRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		httputil.WriteJSONError(w, http.StatusBadRequest, "project name is required")
		return
	}

	err := h.service.UpdateProjectName(r.Context(), projectID, userID, req.Name)
	if err != nil {
		if errors.Is(err, ErrForbidden) {
			httputil.WriteJSONError(w, http.StatusForbidden, err.Error())
			return
		}
		if errors.Is(err, ErrProjectNotFound) {
			httputil.WriteJSONError(w, http.StatusNotFound, err.Error())
			return
		}
		httputil.WriteJSONError(w, http.StatusInternalServerError, "failed to create project: "+err.Error())
		return
	}

	httputil.WriteJSONResponse(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ApplyMutation применяет мутацию к черновику страницы проекта.
// @Summary Применить мутацию
// @Description Применяет JSON-мутацию к указанному проекту после проверки схемы и лимитов.
// @Tags Storage
// @Accept json
// @Security BearerAuth
// @Produce json
// @Param project_id path string true "ID проекта"
// @Param mutation body Mutation true "Объект мутации"
// @Success 200 {object} ErrorResponse "Успешное применение, возвращает версию"
// @Failure 400 {object} ErrorResponse "Ошибка валидации или неверный запрос"
// @Failure 401 {object} ErrorResponse "Пользователь не авторизован"
// @Failure 403 {object} ErrorResponse "Доступ запрещен"
// @Failure 404 {object} ErrorResponse "Мутация или проект не найдены"
// @Failure 413 {object} ErrorResponse "Превышен размер payload"
// @Failure 429 {object} ErrorResponse "Превышен лимит запросов"
// @Failure 500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router /api/v1/projects/{project_id}/draft/mutations [post]
func (h *DraftHandler) applyMutation(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("project_id")

	if projectID == "" {
		httputil.WriteJSONError(w, http.StatusBadRequest, "invalid URI: missing project_id")
		return
	}

	userID, ok := r.Context().Value(auth.UserIDKey).(string)
	if !ok {
		httputil.WriteJSONError(w, http.StatusUnauthorized, "unauthorized")
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
			httputil.WriteJSONError(w, http.StatusRequestEntityTooLarge, "payload too large")
			return
		}
		httputil.WriteJSONError(w, http.StatusBadRequest, "failed to read body")
		return
	}

	documentLoader := gojsonschema.NewBytesLoader(body)
	result, err := h.schema.Validate(documentLoader)
	if err != nil {
		httputil.WriteJSONError(w, http.StatusBadRequest, fmt.Sprintf("failed to perform json validation: %s", err))
		return
	}

	if !result.Valid() {
		output := "json does not match schema. Errors:\n"
		for i, desc := range result.Errors() {
			output += fmt.Sprintf("\t%d: %s\n", i+1, desc)
		}
		httputil.WriteJSONError(w, http.StatusBadRequest, output)
		return
	}

	var mutation Mutation
	if err := json.Unmarshal(body, &mutation); err != nil {
		httputil.WriteJSONError(w, http.StatusBadRequest, "invalid mutation payload: "+err.Error())
		return
	}

	version, err := h.service.ApplyMutation(r.Context(), projectID, userID, mutation)
	if err != nil {
		if errors.Is(err, ErrMutationNotFound) {
			httputil.WriteJSONError(w, http.StatusNotFound, err.Error())
			return
		}
		if errors.Is(err, ErrProjectNotFound) {
			httputil.WriteJSONError(w, http.StatusNotFound, err.Error())
			return
		}
		if errors.Is(err, ErrForbidden) {
			httputil.WriteJSONError(w, http.StatusForbidden, err.Error())
			return
		}
		if errors.Is(err, ErrInvalidMutation) {
			httputil.WriteJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		httputil.WriteJSONError(w, http.StatusInternalServerError, "failed to apply mutation: "+err.Error())
		return
	}

	httputil.WriteJSONResponse(w, http.StatusOK, map[string]string{"status": "ok", "version": strconv.FormatInt(int64(version), 10)})
}

// GetLatestDraft получает последнюю версию черновика страницы.
// @Summary Получить последний черновик
// @Description Возвращает актуальную версию страницы для указанного проекта.
// @Tags Storage
// @Accept json
// @Security BearerAuth
// @Produce json
// @Param project_id path string true "ID проекта"
// @Success 200 {object} Mutation "JSON контент страницы"
// @Failure 400 {object} ErrorResponse "Отсутствует project_id"
// @Failure 401 {object} ErrorResponse "Пользователь не авторизован"
// @Failure 403 {object} ErrorResponse "Доступ запрещен"
// @Failure 404 {object} ErrorResponse "Проект не найден"
// @Failure 500 {object} ErrorResponse "Ошибка получения данных"
// @Router /api/v1/projects/{project_id}/draft [get]
func (h *DraftHandler) sendLatestPage(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("project_id")
	if projectID == "" {
		httputil.WriteJSONError(w, http.StatusBadRequest, "invalid URI: missing project_id")
		return
	}

	userID, ok := r.Context().Value(auth.UserIDKey).(string)
	if !ok {
		httputil.WriteJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	page, err := h.service.GetLatestDraft(r.Context(), projectID, userID)
	if err != nil {
		if errors.Is(err, ErrProjectNotFound) {
			httputil.WriteJSONError(w, http.StatusNotFound, err.Error())
			return
		}
		if errors.Is(err, ErrForbidden) {
			httputil.WriteJSONError(w, http.StatusForbidden, err.Error())
			return
		}
		httputil.WriteJSONError(w, http.StatusInternalServerError, "failed to get page: "+err.Error())
		return
	}

	httputil.WriteJSONResponse(w, http.StatusOK, page)
}

// GetDraftByVersion получает конкретную версию черновика страницы.
// @Summary Получить версию черновика
// @Description Возвращает страницу указанной версии для проекта.
// @Tags Storage
// @Accept json
// @Security BearerAuth
// @Produce json
// @Param project_id path string true "ID проекта"
// @Param version path int true "Номер версии"
// @Success 200 {object} string "JSON контент страницы"
// @Failure 400 {object} ErrorResponse "Неверный ID или версия"
// @Failure 401 {object} ErrorResponse "Пользователь не авторизован"
// @Failure 403 {object} ErrorResponse "Доступ запрещен"
// @Failure 404 {object} ErrorResponse "Проект не найден"
// @Failure 500 {object} ErrorResponse "Ошибка получения данных"
// @Router /api/v1/projects/{project_id}/draft/versions/{version} [get]
func (h *DraftHandler) sendPage(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("project_id")
	version, err := strconv.Atoi(r.PathValue("version"))

	if projectID == "" {
		httputil.WriteJSONError(w, http.StatusBadRequest, "invalid URI: missing project_id")
		return
	}
	if err != nil {
		httputil.WriteJSONError(w, http.StatusBadRequest, "invalid URI: invalid version")
		return
	}
	userID, ok := r.Context().Value(auth.UserIDKey).(string)
	if !ok {
		httputil.WriteJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	page, err := h.service.GetDraft(r.Context(), projectID, userID, version)
	if err != nil {
		if errors.Is(err, ErrProjectNotFound) {
			httputil.WriteJSONError(w, http.StatusNotFound, err.Error())
			return
		}
		if errors.Is(err, ErrForbidden) {
			httputil.WriteJSONError(w, http.StatusForbidden, err.Error())
			return
		}
		httputil.WriteJSONError(w, http.StatusInternalServerError, "failed to get page: "+err.Error())
		return
	}

	httputil.WriteJSONResponse(w, http.StatusOK, page)
}
