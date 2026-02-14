package controller

import (
	"encoding/json"
	"strconv"
	"time"

	"fuzoj/internal/problem/service"
	"fuzoj/pkg/utils/response"

	"github.com/gin-gonic/gin"
)

// ProblemUploadController handles data pack upload endpoints.
type ProblemUploadController struct {
	uploadService *service.ProblemUploadService
}

func NewProblemUploadController(uploadService *service.ProblemUploadService) *ProblemUploadController {
	return &ProblemUploadController{uploadService: uploadService}
}

func (h *ProblemUploadController) Prepare(c *gin.Context) {
	problemID, ok := parseProblemID(c)
	if !ok {
		response.BadRequest(c, "Invalid problem id")
		return
	}

	idemKey := c.GetHeader("Idempotency-Key")
	if idemKey == "" {
		response.BadRequest(c, "Idempotency-Key is required")
		return
	}

	var req PrepareUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	out, err := h.uploadService.PrepareDataPackUpload(c.Request.Context(), service.PrepareUploadInput{
		ProblemID:         problemID,
		IdempotencyKey:    idemKey,
		ExpectedSizeBytes: req.ExpectedSizeBytes,
		ExpectedSHA256:    req.ExpectedSHA256,
		ContentType:       req.ContentType,
		CreatedBy:         req.CreatedBy,
		ClientType:        req.ClientType,
		UploadStrategy:    req.UploadStrategy,
	})
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, PrepareUploadResponse{
		UploadID:          out.UploadSessionID,
		ProblemID:         out.ProblemID,
		Version:           out.Version,
		Bucket:            out.Bucket,
		ObjectKey:         out.ObjectKey,
		MultipartUploadID: out.MultipartUploadID,
		PartSizeBytes:     out.PartSizeBytes,
		ExpiresAt:         out.ExpiresAt.UTC().Format(time.RFC3339),
	})
}

func (h *ProblemUploadController) Sign(c *gin.Context) {
	problemID, ok := parseProblemID(c)
	if !ok {
		response.BadRequest(c, "Invalid problem id")
		return
	}

	uploadIDStr := c.Param("upload_id")
	uploadID, err := strconv.ParseInt(uploadIDStr, 10, 64)
	if err != nil || uploadID <= 0 {
		response.BadRequest(c, "Invalid upload id")
		return
	}

	var req SignPartsRequest
	if err := c.ShouldBindJSON(&req); err != nil || len(req.PartNumbers) == 0 {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	out, err := h.uploadService.SignUploadParts(c.Request.Context(), service.SignPartsInput{
		ProblemID:       problemID,
		UploadSessionID: uploadID,
		PartNumbers:     req.PartNumbers,
	})
	if err != nil {
		response.Error(c, err)
		return
	}

	urls := make(map[string]string, len(out.URLs))
	for n, u := range out.URLs {
		urls[strconv.Itoa(n)] = u
	}

	response.Success(c, SignPartsResponse{
		URLs:             urls,
		ExpiresInSeconds: out.ExpiresInSeconds,
	})
}

func (h *ProblemUploadController) Complete(c *gin.Context) {
	problemID, ok := parseProblemID(c)
	if !ok {
		response.BadRequest(c, "Invalid problem id")
		return
	}

	uploadIDStr := c.Param("upload_id")
	uploadID, err := strconv.ParseInt(uploadIDStr, 10, 64)
	if err != nil || uploadID <= 0 {
		response.BadRequest(c, "Invalid upload id")
		return
	}

	var req CompleteUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	out, err := h.uploadService.CompleteDataPackUpload(c.Request.Context(), service.CompleteUploadInput{
		ProblemID:       problemID,
		UploadSessionID: uploadID,
		Parts:           req.Parts,
		ManifestJSON:    req.ManifestJSON,
		ConfigJSON:      req.ConfigJSON,
		ManifestHash:    req.ManifestHash,
		DataPackHash:    req.DataPackHash,
	})
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, CompleteUploadResponse{
		ProblemID:    out.ProblemID,
		Version:      out.Version,
		ManifestHash: out.ManifestHash,
		DataPackKey:  out.DataPackKey,
		DataPackHash: out.DataPackHash,
	})
}

func (h *ProblemUploadController) Abort(c *gin.Context) {
	problemID, ok := parseProblemID(c)
	if !ok {
		response.BadRequest(c, "Invalid problem id")
		return
	}

	uploadIDStr := c.Param("upload_id")
	uploadID, err := strconv.ParseInt(uploadIDStr, 10, 64)
	if err != nil || uploadID <= 0 {
		response.BadRequest(c, "Invalid upload id")
		return
	}

	if err := h.uploadService.AbortDataPackUpload(c.Request.Context(), service.AbortUploadInput{
		ProblemID:       problemID,
		UploadSessionID: uploadID,
	}); err != nil {
		response.Error(c, err)
		return
	}

	response.SuccessWithMessage(c, "Abort success", nil)
}

func (h *ProblemUploadController) Publish(c *gin.Context) {
	problemID, ok := parseProblemID(c)
	if !ok {
		response.BadRequest(c, "Invalid problem id")
		return
	}

	versionStr := c.Param("version")
	v, err := strconv.ParseInt(versionStr, 10, 32)
	if err != nil || v <= 0 {
		response.BadRequest(c, "Invalid version")
		return
	}

	if err := h.uploadService.PublishVersion(c.Request.Context(), service.PublishInput{
		ProblemID: problemID,
		Version:   int32(v),
	}); err != nil {
		response.Error(c, err)
		return
	}
	response.SuccessWithMessage(c, "Publish success", nil)
}

func parseProblemID(c *gin.Context) (int64, bool) {
	idStr := c.Param("id")
	problemID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || problemID <= 0 {
		return 0, false
	}
	return problemID, true
}

type PrepareUploadRequest struct {
	ExpectedSizeBytes int64  `json:"expected_size_bytes"`
	ExpectedSHA256    string `json:"expected_sha256"`
	ContentType       string `json:"content_type"`
	CreatedBy         int64  `json:"created_by"`

	// Reserved for future web uploads.
	ClientType     string `json:"client_type"`
	UploadStrategy string `json:"upload_strategy"`
}

type PrepareUploadResponse struct {
	UploadID          int64  `json:"upload_id"`
	ProblemID         int64  `json:"problem_id"`
	Version           int32  `json:"version"`
	Bucket            string `json:"bucket"`
	ObjectKey         string `json:"object_key"`
	MultipartUploadID string `json:"multipart_upload_id"`
	PartSizeBytes     int64  `json:"part_size_bytes"`
	ExpiresAt         string `json:"expires_at"`
}

type SignPartsRequest struct {
	PartNumbers []int `json:"part_numbers" binding:"required"`
}

type SignPartsResponse struct {
	URLs             map[string]string `json:"urls"`
	ExpiresInSeconds int64             `json:"expires_in_seconds"`
}

type CompleteUploadRequest struct {
	Parts        []service.CompletedPartInput `json:"parts" binding:"required"`
	ManifestJSON json.RawMessage              `json:"manifest_json" binding:"required"`
	ConfigJSON   json.RawMessage              `json:"config_json" binding:"required"`
	ManifestHash string                       `json:"manifest_hash" binding:"required"`
	DataPackHash string                       `json:"data_pack_hash" binding:"required"`
}

type CompleteUploadResponse struct {
	ProblemID    int64  `json:"problem_id"`
	Version      int32  `json:"version"`
	ManifestHash string `json:"manifest_hash"`
	DataPackKey  string `json:"data_pack_key"`
	DataPackHash string `json:"data_pack_hash"`
}
