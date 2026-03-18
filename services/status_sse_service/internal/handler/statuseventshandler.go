package handler

import (
	"net/http"
	"strconv"
	"strings"

	appErr "fuzoj/pkg/errors"
	"fuzoj/pkg/handlerx"
	"fuzoj/services/status_sse_service/internal/sse"
	"fuzoj/services/status_sse_service/internal/svc"
	"fuzoj/services/status_sse_service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func StatusEventsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.StatusEventsRequest
		if err := httpx.Parse(r, &req); err != nil {
			handlerx.WriteError(w, r, handlerx.BadRequestError())
			return
		}
		if strings.TrimSpace(req.Id) == "" {
			handlerx.WriteError(w, r, appErr.ValidationError("submission_id", "required"))
			return
		}
		if svcCtx.Hub == nil {
			handlerx.WriteError(w, r, appErr.New(appErr.ServiceUnavailable).WithMessage("status sse hub is not configured"))
			return
		}

		userID, err := parseUserID(r.Header.Get("X-User-Id"))
		if err != nil {
			handlerx.WriteError(w, r, err)
			return
		}
		if svcCtx.StatusRepo == nil {
			handlerx.WriteError(w, r, appErr.New(appErr.ServiceUnavailable).WithMessage("status repository is not configured"))
			return
		}
		if err := svcCtx.StatusRepo.CheckSubmissionOwner(r.Context(), req.Id, userID); err != nil {
			handlerx.WriteError(w, r, err)
			return
		}

		flusher, ok := w.(http.Flusher)
		if !ok {
			handlerx.WriteError(w, r, appErr.New(appErr.ServiceUnavailable).WithMessage("streaming is not supported"))
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()

		sender := sse.NewSender(w, flusher)
		if err := svcCtx.Hub.SubscribeAuthorized(r.Context(), req.Id, req.Include, sender); err != nil {
			logx.WithContext(r.Context()).Errorf("subscribe status sse failed: %v", err)
			return
		}
		<-r.Context().Done()
	}
}

func parseUserID(raw string) (int64, error) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return 0, appErr.New(appErr.Unauthorized).WithMessage("user is not authenticated")
	}
	userID, err := strconv.ParseInt(v, 10, 64)
	if err != nil || userID <= 0 {
		return 0, appErr.New(appErr.Unauthorized).WithMessage("user is not authenticated")
	}
	return userID, nil
}
