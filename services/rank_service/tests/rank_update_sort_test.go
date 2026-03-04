package tests

import (
	"testing"

	appErr "fuzoj/pkg/errors"
	"fuzoj/services/rank_service/internal/pmodel"
	"fuzoj/services/rank_service/internal/repository"
)

func TestSortAndFilterRankUpdates(t *testing.T) {
	tests := []struct {
		name           string
		events         []pmodel.RankUpdateEvent
		currentVersion map[string]int64
		wantCount      int
		wantMaxVersion int64
		wantErr        bool
	}{
		{
			name: "filters stale versions and sorts",
			events: []pmodel.RankUpdateEvent{
				{ContestID: "c1", MemberID: "m1", Version: "9", UpdatedAt: 90},
				{ContestID: "c1", MemberID: "m1", Version: "11", UpdatedAt: 110},
				{ContestID: "c1", MemberID: "m1", Version: "10", UpdatedAt: 100},
			},
			currentVersion: map[string]int64{"c1": 10},
			wantCount:      1,
			wantMaxVersion: 11,
			wantErr:        false,
		},
		{
			name: "invalid version",
			events: []pmodel.RankUpdateEvent{
				{ContestID: "c1", MemberID: "m1", Version: "bad"},
			},
			currentVersion: map[string]int64{"c1": 0},
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered, meta, err := repository.SortAndFilterRankUpdates(tt.events, tt.currentVersion)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				if appErr.GetCode(err) != appErr.ValidationFailed {
					t.Fatalf("unexpected error code: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(filtered) != tt.wantCount {
				t.Fatalf("expected %d events, got %d", tt.wantCount, len(filtered))
			}
			if tt.wantCount > 0 {
				metaInfo := meta["c1"]
				if metaInfo.MaxVersion != tt.wantMaxVersion {
					t.Fatalf("expected max version %d, got %d", tt.wantMaxVersion, metaInfo.MaxVersion)
				}
			}
		})
	}
}
