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
		currentResult  map[string]int64
		expectMeta     map[string]struct {
			maxVersion int64
			maxResult  int64
		}
		wantCount      int
		wantMaxVersion int64
		wantMaxResult  int64
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
			currentResult:  map[string]int64{"c1": 0},
			expectMeta: map[string]struct {
				maxVersion int64
				maxResult  int64
			}{
				"c1": {maxVersion: 11, maxResult: 0},
			},
			wantCount:      1,
			wantMaxVersion: 11,
			wantMaxResult:  0,
			wantErr:        false,
		},
		{
			name: "filters stale result ids and sorts",
			events: []pmodel.RankUpdateEvent{
				{ContestID: "c1", MemberID: "m1", ResultID: 9, UpdatedAt: 90},
				{ContestID: "c1", MemberID: "m1", ResultID: 11, UpdatedAt: 110},
				{ContestID: "c1", MemberID: "m1", ResultID: 10, UpdatedAt: 100},
			},
			currentVersion: map[string]int64{"c1": 10},
			currentResult:  map[string]int64{"c1": 10},
			expectMeta: map[string]struct {
				maxVersion int64
				maxResult  int64
			}{
				"c1": {maxVersion: 11, maxResult: 11},
			},
			wantCount:      1,
			wantMaxVersion: 11,
			wantMaxResult:  11,
			wantErr:        false,
		},
		{
			name: "mix result and version prefers result filter",
			events: []pmodel.RankUpdateEvent{
				{ContestID: "c1", MemberID: "m1", ResultID: 2, UpdatedAt: 120, Version: "5"},
				{ContestID: "c1", MemberID: "m1", Version: "6", UpdatedAt: 130},
				{ContestID: "c1", MemberID: "m1", ResultID: 1, UpdatedAt: 110},
			},
			currentVersion: map[string]int64{"c1": 0},
			currentResult:  map[string]int64{"c1": 1},
			expectMeta: map[string]struct {
				maxVersion int64
				maxResult  int64
			}{
				"c1": {maxVersion: 6, maxResult: 2},
			},
			wantCount:      2,
			wantMaxVersion: 6,
			wantMaxResult:  2,
			wantErr:        false,
		},
		{
			name: "multi contest independent",
			events: []pmodel.RankUpdateEvent{
				{ContestID: "c1", MemberID: "m1", ResultID: 1, UpdatedAt: 10},
				{ContestID: "c2", MemberID: "m2", ResultID: 5, UpdatedAt: 20},
				{ContestID: "c2", MemberID: "m2", ResultID: 4, UpdatedAt: 15},
			},
			currentVersion: map[string]int64{"c1": 0, "c2": 0},
			currentResult:  map[string]int64{"c1": 0, "c2": 4},
			expectMeta: map[string]struct {
				maxVersion int64
				maxResult  int64
			}{
				"c1": {maxVersion: 1, maxResult: 1},
				"c2": {maxVersion: 5, maxResult: 5},
			},
			wantCount:      2,
			wantMaxVersion: 5,
			wantMaxResult:  5,
			wantErr:        false,
		},
		{
			name: "invalid contest id",
			events: []pmodel.RankUpdateEvent{
				{ContestID: "", MemberID: "m1", Version: "1"},
			},
			currentVersion: map[string]int64{"": 0},
			currentResult:  map[string]int64{"": 0},
			wantErr:        true,
		},
		{
			name: "stale result ids filtered",
			events: []pmodel.RankUpdateEvent{
				{ContestID: "c1", MemberID: "m1", ResultID: 3, UpdatedAt: 30},
				{ContestID: "c1", MemberID: "m1", ResultID: 2, UpdatedAt: 20},
			},
			currentVersion: map[string]int64{"c1": 0},
			currentResult:  map[string]int64{"c1": 3},
			expectMeta: map[string]struct {
				maxVersion int64
				maxResult  int64
			}{
				"c1": {maxVersion: 0, maxResult: 0},
			},
			wantCount:      0,
			wantMaxVersion: 0,
			wantMaxResult:  0,
			wantErr:        false,
		},
		{
			name: "invalid version",
			events: []pmodel.RankUpdateEvent{
				{ContestID: "c1", MemberID: "m1", Version: "bad"},
			},
			currentVersion: map[string]int64{"c1": 0},
			currentResult:  map[string]int64{"c1": 0},
			expectMeta:     nil,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered, meta, err := repository.SortAndFilterRankUpdates(tt.events, tt.currentVersion, tt.currentResult)
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
			if len(tt.expectMeta) > 0 {
				for contestID, expected := range tt.expectMeta {
					metaInfo := meta[contestID]
					if metaInfo.MaxVersion != expected.maxVersion {
						t.Fatalf("expected max version %d, got %d", expected.maxVersion, metaInfo.MaxVersion)
					}
					if metaInfo.MaxResultID != expected.maxResult {
						t.Fatalf("expected max result id %d, got %d", expected.maxResult, metaInfo.MaxResultID)
					}
				}
				return
			}
			if tt.wantCount > 0 {
				metaInfo := meta["c1"]
				if metaInfo.MaxVersion != tt.wantMaxVersion {
					t.Fatalf("expected max version %d, got %d", tt.wantMaxVersion, metaInfo.MaxVersion)
				}
				if metaInfo.MaxResultID != tt.wantMaxResult {
					t.Fatalf("expected max result id %d, got %d", tt.wantMaxResult, metaInfo.MaxResultID)
				}
			}
		})
	}
}
