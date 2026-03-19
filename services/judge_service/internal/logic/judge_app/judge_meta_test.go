package judge_app

import (
	"testing"
	"time"

	"fuzoj/services/judge_service/internal/pmodel"
)

func TestInvalidateProblemMeta(t *testing.T) {
	app := &JudgeApp{
		metaCache: map[int64]metaEntry{
			1: {
				meta: pmodel.ProblemMeta{
					ProblemID:    1,
					Version:      2,
					ManifestHash: "m1",
					DataPackKey:  "k1",
					DataPackHash: "d1",
				},
				expiresAt: time.Now().Add(time.Minute),
			},
			2: {
				meta: pmodel.ProblemMeta{
					ProblemID:    2,
					Version:      1,
					ManifestHash: "m2",
					DataPackKey:  "k2",
					DataPackHash: "d2",
				},
				expiresAt: time.Now().Add(time.Minute),
			},
		},
	}

	app.InvalidateProblemMeta(1)

	if _, ok := app.metaCache[1]; ok {
		t.Fatalf("expected problem 1 cache entry to be removed")
	}
	if _, ok := app.metaCache[2]; !ok {
		t.Fatalf("expected unrelated cache entry to remain")
	}
}
