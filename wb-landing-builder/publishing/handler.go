package publishing

import (
	"encoding/json"
	"net/http"

	"github.com/rki-mai/wb-landing-builder/publishing/utils"
)

const maxBodySize = 1 << 20

// Handler обрабатывает HTTP-запросы, связанные с публикациями.
type Handler struct {
	service *PublicationService
}

// NewPublicationHandler создаёт обработчик публикаций.
func NewPublicationHandler(service *PublicationService) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes регистрирует маршруты публикаций (за middleware авторизации).
func (h *Handler) RegisterRoutes(mux *http.ServeMux, middleware func(http.Handler) http.Handler) {
	mux.Handle("POST /api/v1/publications", middleware(http.HandlerFunc(h.createPublication)))
	mux.Handle("GET /api/v1/publications/{id}", middleware(http.HandlerFunc(h.getPublication)))
	mux.Handle("DELETE /api/v1/publications/{id}", middleware(http.HandlerFunc(h.deletePublication)))
}

// CreatePublication создаёт публикацию по последнему черновику проекта.
// @Summary Создать публикацию
// @Description Загружает последний черновик из storage, рендерит HTML и сохраняет bundle в объектное хранилище.
// @Tags Publications
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body CreatePublicationRequest true "ID проекта для публикации"
// @Success 201 {object} Publication
// @Failure 400 {object} ErrorResponse "Неверный запрос"
// @Failure 401 {object} ErrorResponse "Требуется авторизация"
// @Failure 500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router /api/v1/publications [post]
func (h *Handler) createPublication(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	var req CreatePublicationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ProjectID == "" {
		utils.WriteJSONError(w, http.StatusBadRequest, "project_id is required")
		return
	}

	pub, err := h.service.Create(r.Context(), req.ProjectID)
	if err != nil {
		utils.WriteJSONError(w, http.StatusInternalServerError, "failed to create publication: "+err.Error())
		return
	}

	utils.WriteJSONResponse(w, http.StatusCreated, pub)
}

// GetPublication возвращает метаданные публикации по ID.
// @Summary Получить публикацию
// @Tags Publications
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID публикации"
// @Success 200 {object} Publication
// @Failure 404 {object} ErrorResponse "Публикация не найдена"
// @Failure 500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router /api/v1/publications/{id} [get]
func (h *Handler) getPublication(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		utils.WriteJSONError(w, http.StatusBadRequest, "missing publication id")
		return
	}

	pub, err := h.service.Get(r.Context(), id)
	if err != nil {
		utils.WriteJSONError(w, http.StatusInternalServerError, "failed to get publication: "+err.Error())
		return
	}
	if pub == nil {
		utils.WriteJSONError(w, http.StatusNotFound, "publication not found")
		return
	}

	utils.WriteJSONResponse(w, http.StatusOK, pub)
}

// DeletePublication удаляет публикацию и её bundle из хранилища.
// @Summary Удалить публикацию
// @Tags Publications
// @Security BearerAuth
// @Param id path string true "ID публикации"
// @Success 204 "No Content"
// @Failure 500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router /api/v1/publications/{id} [delete]
func (h *Handler) deletePublication(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		utils.WriteJSONError(w, http.StatusBadRequest, "missing publication id")
		return
	}

	if err := h.service.Delete(r.Context(), id); err != nil {
		utils.WriteJSONError(w, http.StatusInternalServerError, "failed to delete publication: "+err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
