package consumer

import "testing"

func TestUpdateRankOutboxFullScanStreak(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		current    int
		contests   int
		scanBatch  int
		wantStreak int
	}{
		{
			name:       "full batch increments",
			current:    0,
			contests:   32,
			scanBatch:  32,
			wantStreak: 1,
		},
		{
			name:       "caps at threshold",
			current:    3,
			contests:   32,
			scanBatch:  32,
			wantStreak: 3,
		},
		{
			name:       "empty resets",
			current:    2,
			contests:   0,
			scanBatch:  32,
			wantStreak: 0,
		},
		{
			name:       "partial resets",
			current:    2,
			contests:   31,
			scanBatch:  32,
			wantStreak: 0,
		},
		{
			name:       "invalid batch resets",
			current:    2,
			contests:   32,
			scanBatch:  0,
			wantStreak: 0,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := updateRankOutboxFullScanStreak(tt.current, tt.contests, tt.scanBatch)
			if got != tt.wantStreak {
				t.Fatalf("unexpected streak, got=%d want=%d", got, tt.wantStreak)
			}
		})
	}
}

func TestShouldRankOutboxSleep(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		streak    int
		wantSleep bool
	}{
		{
			name:      "below threshold should sleep",
			streak:    2,
			wantSleep: true,
		},
		{
			name:      "at threshold should not sleep",
			streak:    3,
			wantSleep: false,
		},
		{
			name:      "above threshold should not sleep",
			streak:    4,
			wantSleep: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := shouldRankOutboxSleep(tt.streak)
			if got != tt.wantSleep {
				t.Fatalf("unexpected sleep decision, got=%v want=%v", got, tt.wantSleep)
			}
		})
	}
}

func TestRankOutboxFullScanHeuristicSequence(t *testing.T) {
	t.Parallel()

	type step struct {
		contests int
		err      bool
		want     bool
	}

	sequence := []step{
		{contests: 32, want: true},
		{contests: 32, want: true},
		{contests: 32, want: false},
		{contests: 31, want: true},
		{contests: 32, want: true},
		{contests: 32, want: true},
		{contests: 32, want: false},
		{err: true, want: true},
		{contests: 32, want: true},
		{contests: 32, want: true},
		{contests: 32, want: false},
	}

	streak := 0
	for i, item := range sequence {
		if item.err {
			streak = 0
		} else {
			streak = updateRankOutboxFullScanStreak(streak, item.contests, 32)
		}
		got := shouldRankOutboxSleep(streak)
		if got != item.want {
			t.Fatalf("step %d unexpected decision, got=%v want=%v streak=%d", i+1, got, item.want, streak)
		}
	}
}
