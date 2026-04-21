package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/rki-mai/wb-landing-builder/draft-component/models"
	"github.com/rki-mai/wb-landing-builder/draft-component/service"
)

const MaxBodySize = 1 << 20

type Handler struct {
	service service.DraftService
}

func NewHandler(svc service.DraftService) *Handler {
	return &Handler{service: svc}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/drafts/{project_id}/mutations", h.applyMutation)
	mux.HandleFunc("GET /api/v1/drafts/{project_id}", h.sendLatestPage)
	mux.HandleFunc("GET /api/v1/drafts/{project_id}/versions/{version}", h.sendPage)
}

func (h *Handler) applyMutation(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("project_id")
	if projectID == "" {
		writeJSONError(w, http.StatusBadRequest, "invalid URI: missing project_id")
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

	var mutation models.Mutation
	if err := json.Unmarshal(body, &mutation); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid mutation payload: "+err.Error())
		return
	}

	version, err := h.service.ApplyMutation(r.Context(), projectID, mutation)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to apply mutation: "+err.Error())
		return
	}

	writeJSONResponse(w, http.StatusOK, map[string]string{"status": "ok", "version": strconv.FormatInt(version, 10)})
}

func (h *Handler) sendLatestPage(w http.ResponseWriter, r *http.Request) {
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

func (h *Handler) sendPage(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("project_id")
	version, err := strconv.ParseInt(r.PathValue("version"), 10, 64)

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
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}
