package publishing

import (
	"net/http"
	"strings"

	"github.com/rki-mai/wb-landing-builder/httputil"
)

// Handler обрабатывает HTTP-запросы, связанные с публикациями.
type Handler struct {
	service       *PublicationService
	publicBaseURL string
}

// NewPublicationHandler создаёт обработчик публикаций.
func NewPublicationHandler(service *PublicationService, publicBaseURL string) *Handler {
	return &Handler{service: service, publicBaseURL: publicBaseURL}
}

func (h *Handler) fillPublicURL(pub *Publication) {
	if pub == nil || h.publicBaseURL == "" {
		return
	}
	base := strings.TrimSuffix(h.publicBaseURL, "/")
	pub.PublicURL = base + "/publications/" + pub.ID + "/index.html"
}

// RegisterRoutes регистрирует маршруты публикаций (за middleware авторизации).
func (h *Handler) RegisterRoutes(mux *http.ServeMux, middleware func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/projects/{project_id}/publications", middleware(http.HandlerFunc(h.listPublicationIDs)))
	mux.Handle("POST /api/v1/projects/{project_id}/publications", middleware(http.HandlerFunc(h.createPublication)))
	mux.Handle("GET /api/v1/projects/{project_id}/publications/{id}", middleware(http.HandlerFunc(h.getPublication)))
	mux.Handle("DELETE /api/v1/projects/{project_id}/publications/{id}", middleware(http.HandlerFunc(h.deletePublication)))
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
// @Router /api/v1/projects/{project_id}/publications [get]
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

// CreatePublication ставит публикацию в очередь на рендер.
// @Summary Создать публикацию
// @Description Создаёт запись публикации со статусом PENDING и ставит задачу на рендер в очередь. Доступ только владельцу проекта.
// @Tags Publications
// @Produce json
// @Security BearerAuth
// @Param project_id path string true "ID проекта"
// @Success 201 {object} Publication "Публикация поставлена в очередь (status=PENDING)"
// @Failure 401 {object} ErrorResponse "Требуется авторизация"
// @Failure 403 {object} ErrorResponse "Нет доступа к проекту"
// @Failure 404 {object} ErrorResponse "Черновик не найден"
// @Failure 500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router /api/v1/projects/{project_id}/publications [post]
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

	h.fillPublicURL(pub)
	httputil.WriteJSONResponse(w, http.StatusCreated, pub)
}

// GetPublication возвращает метаданные публикации по ID.
// @Summary Получить публикацию
// @Description Возвращает метаданные публикации и текущий статус (PENDING, PROCESSING, FINISHED, FAILED). Доступ только владельцу проекта.
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
// @Router /api/v1/projects/{project_id}/publications/{id} [get]
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

	h.fillPublicURL(pub)
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
// @Router /api/v1/projects/{project_id}/publications/{id} [delete]
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
