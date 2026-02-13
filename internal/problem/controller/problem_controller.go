package controller

import (
	"strconv"
	"time"

	"fuzoj/internal/problem/service"
	"fuzoj/pkg/utils/response"

	"github.com/gin-gonic/gin"
)

// ProblemController handles problem meta HTTP endpoints.
type ProblemController struct {
	problemService *service.ProblemService
}

// NewProblemController creates a new ProblemController.
func NewProblemController(problemService *service.ProblemService) *ProblemController {
	return &ProblemController{problemService: problemService}
}

// Create handles problem creation.
func (h *ProblemController) Create(c *gin.Context) {
	var req CreateProblemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	id, err := h.problemService.CreateProblem(c.Request.Context(), service.CreateInput{
		Title:   req.Title,
		OwnerID: req.OwnerID,
	})
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, CreateProblemResponse{ID: id})
}

// GetLatest handles latest meta query for a problem.
func (h *ProblemController) GetLatest(c *gin.Context) {
	idStr := c.Param("id")
	problemID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || problemID <= 0 {
		response.BadRequest(c, "Invalid problem id")
		return
	}

	meta, err := h.problemService.GetLatestMeta(c.Request.Context(), problemID)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, LatestMetaResponse{
		ProblemID:    meta.ProblemID,
		Version:      meta.Version,
		ManifestHash: meta.ManifestHash,
		DataPackKey:  meta.DataPackKey,
		DataPackHash: meta.DataPackHash,
		UpdatedAt:    meta.UpdatedAt.UTC().Format(time.RFC3339),
	})
}

// Delete handles problem deletion.
func (h *ProblemController) Delete(c *gin.Context) {
	idStr := c.Param("id")
	problemID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || problemID <= 0 {
		response.BadRequest(c, "Invalid problem id")
		return
	}

	if err := h.problemService.DeleteProblem(c.Request.Context(), problemID); err != nil {
		response.Error(c, err)
		return
	}

	response.SuccessWithMessage(c, "Delete success", nil)
}

// CreateProblemRequest defines problem creation payload.
type CreateProblemRequest struct {
	Title   string `json:"title" binding:"required"`
	OwnerID int64  `json:"owner_id"`
}

// CreateProblemResponse defines problem creation response payload.
type CreateProblemResponse struct {
	ID int64 `json:"id"`
}

// LatestMetaResponse defines latest meta response payload.
type LatestMetaResponse struct {
	ProblemID    int64  `json:"problem_id"`
	Version      int32  `json:"version"`
	ManifestHash string `json:"manifest_hash"`
	DataPackKey  string `json:"data_pack_key"`
	DataPackHash string `json:"data_pack_hash"`
	UpdatedAt    string `json:"updated_at"`
}
