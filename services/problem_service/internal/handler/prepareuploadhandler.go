// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package handler

import (
	"net/http"

	"fuzoj/pkg/handlerx"

	"fuzoj/services/problem_service/internal/logic"
	"fuzoj/services/problem_service/internal/svc"
	"fuzoj/services/problem_service/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func PrepareUploadHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logx.WithContext(r.Context()).Infof("prepare upload request headers idempotency_key=%s content_type=%s", r.Header.Get("Idempotency-Key"), r.Header.Get("Content-Type"))
		var req types.PrepareUploadRequest
		if err := httpx.Parse(r, &req); err != nil {
			logx.WithContext(r.Context()).Errorf("prepare upload parse failed path=%s err=%v", r.URL.Path, err)
			handlerx.WriteError(w, r, handlerx.BadRequestError())
			return
		}
		logx.WithContext(r.Context()).Infof("prepare upload request parsed problem_id=%d idempotency_key=%s", req.Id, req.IdempotencyKey)

		l := logic.NewPrepareUploadLogic(r.Context(), svcCtx)
		resp, err := l.PrepareUpload(&req)
		if err != nil {
			handlerx.WriteError(w, r, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
