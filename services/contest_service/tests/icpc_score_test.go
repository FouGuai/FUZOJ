package tests

import (
	"testing"
	"time"

	"fuzoj/pkg/contest/score"
)

func TestICPCPenalty(t *testing.T) {
	start := time.Date(2026, 3, 4, 10, 0, 0, 0, time.UTC)
	cases := []struct {
		name       string
		submitAt   time.Time
		wrongCount int
		expect     int64
	}{
		{"first-ac", start.Add(10 * time.Minute), 0, 600},
		{"with-wrong", start.Add(30 * time.Minute), 2, 30*60 + 2*20*60},
		{"before-start", start.Add(-time.Minute), 1, 0 + 20*60},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := score.ICPCPenalty(start, tc.submitAt, tc.wrongCount); got != tc.expect {
				t.Fatalf("penalty mismatch: got=%d expect=%d", got, tc.expect)
			}
		})
	}
}

func TestICPCPenaltyWithMinutes(t *testing.T) {
	start := time.Date(2026, 3, 4, 10, 0, 0, 0, time.UTC)
	cases := []struct {
		name           string
		submitAt       time.Time
		wrongCount     int
		penaltyMinutes int
		expect         int64
	}{
		{"custom-10", start.Add(30 * time.Minute), 2, 10, 30*60 + 2*10*60},
		{"invalid-fallback", start.Add(30 * time.Minute), 2, 0, 30*60 + 2*20*60},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := score.ICPCPenaltyWithMinutes(start, tc.submitAt, tc.wrongCount, tc.penaltyMinutes); got != tc.expect {
				t.Fatalf("penalty with minutes mismatch: got=%d expect=%d", got, tc.expect)
			}
		})
	}
}

func TestSortScore(t *testing.T) {
	cases := []struct {
		name     string
		acCount  int64
		penalty  int64
		expected int64
	}{
		{"zero", 0, 0, 0},
		{"ac1", 1, 100, 1_000_000_000_000 - 100},
		{"ac2", 2, 300, 2_000_000_000_000 - 300},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := score.SortScore(tc.acCount, tc.penalty); got != tc.expected {
				t.Fatalf("sort score mismatch: got=%d expect=%d", got, tc.expected)
			}
		})
	}
}
