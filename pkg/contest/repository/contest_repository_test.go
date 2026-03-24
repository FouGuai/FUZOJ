package repository

import "testing"

func TestParsePenaltyMinutes(t *testing.T) {
	cases := []struct {
		name     string
		ruleJSON string
		expect   int
	}{
		{
			name:     "valid penalty minutes",
			ruleJSON: `{"rule_type":"icpc","penalty_minutes":10}`,
			expect:   10,
		},
		{
			name:     "missing penalty minutes uses default",
			ruleJSON: `{"rule_type":"icpc"}`,
			expect:   defaultPenaltyMinutes,
		},
		{
			name:     "invalid json uses default",
			ruleJSON: `{`,
			expect:   defaultPenaltyMinutes,
		},
		{
			name:     "empty uses default",
			ruleJSON: "",
			expect:   defaultPenaltyMinutes,
		},
		{
			name:     "non-positive uses default",
			ruleJSON: `{"penalty_minutes":0}`,
			expect:   defaultPenaltyMinutes,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parsePenaltyMinutes(tc.ruleJSON); got != tc.expect {
				t.Fatalf("parse penalty minutes mismatch: got=%d expect=%d", got, tc.expect)
			}
		})
	}
}
