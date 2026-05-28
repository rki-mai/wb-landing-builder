package publishing

import (
	"net/http"

	"github.com/rki-mai/wb-landing-builder/httputil"
)

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
	mux.Handle("GET /api/v1/storage/{project_id}/publications", middleware(http.HandlerFunc(h.listPublicationIDs)))
	mux.Handle("POST /api/v1/storage/{project_id}/publications", middleware(http.HandlerFunc(h.createPublication)))
	mux.Handle("GET /api/v1/storage/{project_id}/publications/{id}", middleware(http.HandlerFunc(h.getPublication)))
	mux.Handle("DELETE /api/v1/storage/{project_id}/publications/{id}", middleware(http.HandlerFunc(h.deletePublication)))
}

// ListPublicationIDs возвращает ID публикаций проекта.
// @Summary Список ID публикаций проекта
// @Description Возвращает ID всех публикаций для указанного проекта (от новых к старым). Доступ только владельцу проекта.
// @Tags Publications
// @Produce json
// @Security BearerAuth
// @Param project_id path string true "ID проекта"
// @Success 200 {object} PublicationIDsResponse "Список ID публикаций"
// @Failure 401 {object} ErrorResponse "Требуется авторизация"
// @Failure 403 {object} ErrorResponse "Нет доступа к проекту"
// @Failure 404 {object} ErrorResponse "Проект не найден"
// @Failure 500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router /api/v1/storage/{project_id}/publications [get]
func (h *Handler) listPublicationIDs(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("project_id")

	userID, ok := userIDFromRequest(r)
	if !ok {
		httputil.WriteJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	ids, err := h.service.ListIDsByProject(r.Context(), projectID, userID)
	if err != nil {
		writePublicationError(w, err)
		return
	}

	httputil.WriteJSONResponse(w, http.StatusOK, PublicationIDsResponse{IDs: ids})
}

// CreatePublication создаёт публикацию по последнему черновику проекта.
// @Summary Создать публикацию
// @Description Загружает последний черновик из storage, рендерит HTML и сохраняет bundle в объектное хранилище. Доступ только владельцу проекта.
// @Tags Publications
// @Produce json
// @Security BearerAuth
// @Param project_id path string true "ID проекта"
// @Success 201 {object} Publication "Публикация создана"
// @Failure 401 {object} ErrorResponse "Требуется авторизация"
// @Failure 403 {object} ErrorResponse "Нет доступа к проекту"
// @Failure 404 {object} ErrorResponse "Черновик не найден"
// @Failure 500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router /api/v1/storage/{project_id}/publications [post]
func (h *Handler) createPublication(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("project_id")

	userID, ok := userIDFromRequest(r)
	if !ok {
		httputil.WriteJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	pub, err := h.service.Create(r.Context(), projectID, userID)
	if err != nil {
		writePublicationError(w, err)
		return
	}

	httputil.WriteJSONResponse(w, http.StatusCreated, pub)
}

// GetPublication возвращает метаданные публикации по ID.
// @Summary Получить публикацию
// @Description Возвращает метаданные публикации. Доступ только владельцу проекта.
// @Tags Publications
// @Produce json
// @Security BearerAuth
// @Param project_id path string true "ID проекта"
// @Param id path string true "ID публикации"
// @Success 200 {object} Publication "Метаданные публикации"
// @Failure 401 {object} ErrorResponse "Требуется авторизация"
// @Failure 403 {object} ErrorResponse "Нет доступа к проекту"
// @Failure 404 {object} ErrorResponse "Публикация или проект не найдены"
// @Failure 500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router /api/v1/storage/{project_id}/publications/{id} [get]
func (h *Handler) getPublication(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("project_id")
	id := r.PathValue("id")

	userID, ok := userIDFromRequest(r)
	if !ok {
		httputil.WriteJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	pub, err := h.service.Get(r.Context(), projectID, userID, id)
	if err != nil {
		writePublicationError(w, err)
		return
	}

	httputil.WriteJSONResponse(w, http.StatusOK, pub)
}

// DeletePublication удаляет публикацию и её bundle из хранилища.
// @Summary Удалить публикацию
// @Description Удаляет публикацию и связанные файлы. Доступ только владельцу проекта.
// @Tags Publications
// @Security BearerAuth
// @Param project_id path string true "ID проекта"
// @Param id path string true "ID публикации"
// @Success 204 "Публикация удалена"
// @Failure 401 {object} ErrorResponse "Требуется авторизация"
// @Failure 403 {object} ErrorResponse "Нет доступа к проекту"
// @Failure 404 {object} ErrorResponse "Публикация или проект не найдены"
// @Failure 500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router /api/v1/storage/{project_id}/publications/{id} [delete]
func (h *Handler) deletePublication(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("project_id")
	id := r.PathValue("id")

	userID, ok := userIDFromRequest(r)
	if !ok {
		httputil.WriteJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if err := h.service.Delete(r.Context(), projectID, userID, id); err != nil {
		writePublicationError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
