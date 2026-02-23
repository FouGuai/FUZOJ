package judge_service

import (
	"context"
	"testing"

	appErr "fuzoj/pkg/errors"
	"fuzoj/services/judge_service/internal/logic"
	"fuzoj/services/judge_service/internal/svc"
)

func TestJudgeConsumerLogicHandleMessageRequiresService(t *testing.T) {
	t.Parallel()
	l := logic.NewJudgeConsumerLogic(context.Background(), &svc.ServiceContext{})
	if err := l.Consume(context.Background(), "", ""); err == nil || !appErr.Is(err, appErr.ServiceUnavailable) {
		t.Fatalf("expected ServiceUnavailable error, got %v", err)
	}
}
