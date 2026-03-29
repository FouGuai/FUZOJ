package statusmonotonic

import "testing"

func TestShouldAccept_StrictSameStageProgress(t *testing.T) {
	tests := []struct {
		name               string
		currentStatus      string
		currentDone        int
		currentTotal       int
		nextStatus         string
		nextDone           int
		nextTotal          int
		wantAccept         bool
		wantReasonNonEmpty bool
	}{
		{
			name:               "same stage no progress should reject",
			currentStatus:      "running",
			currentDone:        1,
			currentTotal:       10,
			nextStatus:         "running",
			nextDone:           1,
			nextTotal:          10,
			wantAccept:         false,
			wantReasonNonEmpty: true,
		},
		{
			name:               "same stage increasing done should accept",
			currentStatus:      "running",
			currentDone:        1,
			currentTotal:       10,
			nextStatus:         "running",
			nextDone:           2,
			nextTotal:          10,
			wantAccept:         true,
			wantReasonNonEmpty: false,
		},
		{
			name:               "same stage regressing done should reject",
			currentStatus:      "running",
			currentDone:        2,
			currentTotal:       10,
			nextStatus:         "running",
			nextDone:           1,
			nextTotal:          10,
			wantAccept:         false,
			wantReasonNonEmpty: true,
		},
		{
			name:               "same stage regressing total should reject",
			currentStatus:      "running",
			currentDone:        2,
			currentTotal:       10,
			nextStatus:         "running",
			nextDone:           2,
			nextTotal:          9,
			wantAccept:         false,
			wantReasonNonEmpty: true,
		},
		{
			name:               "stage forward should accept",
			currentStatus:      "running",
			currentDone:        5,
			currentTotal:       10,
			nextStatus:         "finished",
			nextDone:           5,
			nextTotal:          10,
			wantAccept:         true,
			wantReasonNonEmpty: false,
		},
		{
			name:               "unknown next should reject",
			currentStatus:      "running",
			currentDone:        1,
			currentTotal:       10,
			nextStatus:         "mystatus",
			nextDone:           1,
			nextTotal:          10,
			wantAccept:         false,
			wantReasonNonEmpty: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			accept, reason := ShouldAccept(
				tc.currentStatus,
				tc.currentDone,
				tc.currentTotal,
				tc.nextStatus,
				tc.nextDone,
				tc.nextTotal,
			)
			if accept != tc.wantAccept {
				t.Fatalf("accept mismatch, got=%v want=%v reason=%s", accept, tc.wantAccept, reason)
			}
			if tc.wantReasonNonEmpty && reason == "" {
				t.Fatalf("expected non-empty reason")
			}
			if !tc.wantReasonNonEmpty && reason != "" {
				t.Fatalf("expected empty reason, got=%s", reason)
			}
		})
	}
}
