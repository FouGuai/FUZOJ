package controller

import (
	"fuzoj/internal/judge/repository"
	"fuzoj/pkg/utils/response"

	"github.com/gin-gonic/gin"
)

// JudgeController handles judge status requests.
type JudgeController struct {
	repo *repository.StatusRepository
}

// NewJudgeController creates a new controller.
func NewJudgeController(repo *repository.StatusRepository) *JudgeController {
	return &JudgeController{repo: repo}
}

// GetStatus returns status for one submission.
func (h *JudgeController) GetStatus(c *gin.Context) {
	submissionID := c.Param("id")
	if submissionID == "" {
		response.BadRequest(c, "Invalid submission id")
		return
	}
	status, err := h.repo.Get(c.Request.Context(), submissionID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, status)
}
