package logic

import (
	"encoding/json"
	"strings"

	"fuzoj/services/contest_service/internal/types"
)

const (
	defaultRuleType       = "icpc"
	defaultPenaltyMinutes = 20
)

func normalizeRule(rule types.ContestRulePayload) types.ContestRulePayload {
	if strings.TrimSpace(rule.RuleType) == "" {
		rule.RuleType = defaultRuleType
	}
	if rule.PenaltyMinutes == 0 {
		rule.PenaltyMinutes = defaultPenaltyMinutes
	}
	return rule
}

func parseRuleJSON(ruleJSON string) (types.ContestRulePayload, error) {
	if strings.TrimSpace(ruleJSON) == "" {
		return types.ContestRulePayload{}, nil
	}
	var rule types.ContestRulePayload
	if err := json.Unmarshal([]byte(ruleJSON), &rule); err != nil {
		return types.ContestRulePayload{}, err
	}
	return rule, nil
}

func ruleTypeFromJSON(ruleJSON string) string {
	if strings.TrimSpace(ruleJSON) == "" {
		return ""
	}
	var rule struct {
		RuleType string `json:"rule_type"`
	}
	if err := json.Unmarshal([]byte(ruleJSON), &rule); err != nil {
		return ""
	}
	return rule.RuleType
}

func hasRuleUpdate(rule types.ContestRulePayload) bool {
	if strings.TrimSpace(rule.RuleType) != "" {
		return true
	}
	if strings.TrimSpace(rule.PenaltyFormula) != "" {
		return true
	}
	if strings.TrimSpace(rule.ScoreMode) != "" {
		return true
	}
	if rule.PenaltyMinutes != 0 || rule.PenaltyCapMinutes != 0 || rule.FreezeMinutesBeforeEnd != 0 || rule.HackReward != 0 || rule.HackPenalty != 0 || rule.MaxSubmissionsPerProblem != 0 {
		return true
	}
	if rule.AllowHack || rule.PublishSolutionsAfterEnd || rule.VirtualParticipationEnabled {
		return true
	}
	return false
}
