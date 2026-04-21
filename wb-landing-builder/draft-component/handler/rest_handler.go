package handler

import (
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rki-mai/wb-landing-builder/draft-component/registry"
	"github.com/rki-mai/wb-landing-builder/draft-component/service"
)

const MaxBodySize = 1 << 20

type Handler struct {
	service service.DraftService
}

func NewHandler(svc service.DraftService) *Handler {
	return &Handler{service: svc}
}

func (h *Handler) RegisterRoutes(r *gin.Engine) {
	api := r.Group("/api/v1/drafts")
	{
		api.POST("/:draft_id/mutations", h.applyMutation)
		api.GET("/:draft_id", h.sendLatestPage)
		api.GET("/:draft_id/versions/:version", h.sendPage)
	}
}

type DraftIDURI struct {
	DraftID string `uri:"draft_id" binding:"required"`
}

type DraftIDVersionURI struct {
	DraftID string `uri:"draft_id" binding:"required"`
	Version string `uri:"version" binding:"required"`
}

func (h *Handler) applyMutation(c *gin.Context) {
	var uri DraftIDURI
	if err := c.ShouldBindUri(&uri); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid URI: " + err.Error()})
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxBodySize)

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "payload too large"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
		return
	}

	mutation, err := registry.ParseMutation(body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid mutation payload: " + err.Error()})
		return
	}

	err = mutation.Apply(h.service, uri.DraftID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to apply mutation: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) sendLatestPage(c *gin.Context) {
	var uri DraftIDURI
	if err := c.ShouldBindUri(&uri); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid URI: " + err.Error()})
		return
	}
	service.GetLatestPage(uri.DraftID)
}

func (h *Handler) sendPage(c *gin.Context) {
	var uri DraftIDVersionURI
	if err := c.ShouldBindUri(&uri); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid URI: " + err.Error()})
		return
	}
	service.GetPage(uri.DraftID, uri.Version)
}
