package controller

import (
	"strings"

	"fuzoj/internal/submit/service"
	"fuzoj/judge_service/internal/model"
	"fuzoj/pkg/utils/response"

	"github.com/gin-gonic/gin"
)

// SubmitController handles submission HTTP endpoints.
type SubmitController struct {
	submitService *service.SubmitService
}

// NewSubmitController creates a new SubmitController.
func NewSubmitController(submitService *service.SubmitService) *SubmitController {
	return &SubmitController{submitService: submitService}
}

// Create handles submission requests.
func (h *SubmitController) Create(c *gin.Context) {
	var req SubmitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	idempotencyKey := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	submissionID, status, err := h.submitService.Submit(c.Request.Context(), service.SubmitInput{
		ProblemID:         req.ProblemID,
		UserID:            req.UserID,
		LanguageID:        req.LanguageID,
		SourceCode:        req.SourceCode,
		ContestID:         req.ContestID,
		Scene:             req.Scene,
		ExtraCompileFlags: req.ExtraCompileFlags,
		IdempotencyKey:    idempotencyKey,
		ClientIP:          c.ClientIP(),
	})
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, SubmitResponse{
		SubmissionID: submissionID,
		Status:       string(status.Status),
		ReceivedAt:   status.Timestamps.ReceivedAt,
	})
}

// GetStatus returns status for one submission.
func (h *SubmitController) GetStatus(c *gin.Context) {
	submissionID := c.Param("id")
	if submissionID == "" {
		response.BadRequest(c, "Invalid submission id")
		return
	}
	status, err := h.submitService.GetStatus(c.Request.Context(), submissionID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, status)
}

// BatchStatus returns statuses for multiple submissions.
func (h *SubmitController) BatchStatus(c *gin.Context) {
	var req BatchStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil || len(req.SubmissionIDs) == 0 {
		response.BadRequest(c, "Invalid request parameters")
		return
	}
	statuses, missing, err := h.submitService.GetStatusBatch(c.Request.Context(), req.SubmissionIDs)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, BatchStatusResponse{
		Items:   statuses,
		Missing: missing,
	})
}

// GetSource returns submission source code.
func (h *SubmitController) GetSource(c *gin.Context) {
	submissionID := c.Param("id")
	if submissionID == "" {
		response.BadRequest(c, "Invalid submission id")
		return
	}
	submission, err := h.submitService.GetSource(c.Request.Context(), submissionID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, SourceResponse{
		SubmissionID: submission.SubmissionID,
		ProblemID:    submission.ProblemID,
		UserID:       submission.UserID,
		ContestID:    submission.ContestID,
		LanguageID:   submission.LanguageID,
		SourceCode:   submission.SourceCode,
		CreatedAt:    submission.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	})
}

// SubmitRequest defines submission payload.
type SubmitRequest struct {
	ProblemID         int64    `json:"problem_id" binding:"required"`
	UserID            int64    `json:"user_id" binding:"required"`
	LanguageID        string   `json:"language_id" binding:"required"`
	SourceCode        string   `json:"source_code" binding:"required"`
	ContestID         string   `json:"contest_id"`
	Scene             string   `json:"scene"`
	ExtraCompileFlags []string `json:"extra_compile_flags"`
}

// SubmitResponse defines submission response payload.
type SubmitResponse struct {
	SubmissionID string `json:"submission_id"`
	Status       string `json:"status"`
	ReceivedAt   int64  `json:"received_at"`
}

// BatchStatusRequest defines batch status payload.
type BatchStatusRequest struct {
	SubmissionIDs []string `json:"submission_ids" binding:"required"`
}

// BatchStatusResponse defines batch status response payload.
type BatchStatusResponse struct {
	Items   []model.JudgeStatusResponse `json:"items"`
	Missing []string                    `json:"missing"`
}

// SourceResponse defines source query response payload.
type SourceResponse struct {
	SubmissionID string `json:"submission_id"`
	ProblemID    int64  `json:"problem_id"`
	UserID       int64  `json:"user_id"`
	ContestID    string `json:"contest_id,omitempty"`
	LanguageID   string `json:"language_id"`
	SourceCode   string `json:"source_code"`
	CreatedAt    string `json:"created_at"`
}
