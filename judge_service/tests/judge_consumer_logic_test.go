package judge_service_test

import (
	"context"
	"testing"

	"fuzoj/judge_service/internal/logic"
	"fuzoj/judge_service/internal/svc"
	appErr "fuzoj/pkg/errors"
)

func TestJudgeConsumerLogicHandleMessageRequiresService(t *testing.T) {
	t.Parallel()
	l := logic.NewJudgeConsumerLogic(context.Background(), &svc.ServiceContext{})
	if err := l.HandleMessage(nil); err == nil || !appErr.Is(err, appErr.ServiceUnavailable) {
		t.Fatalf("expected ServiceUnavailable error, got %v", err)
	}
}
