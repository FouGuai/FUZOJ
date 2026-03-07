package logic

import (
	"strings"

	"fuzoj/services/contest_service/internal/types"
)

func mergeRuleUpdate(existing types.ContestRulePayload, update types.ContestRulePayload) (types.ContestRulePayload, bool) {
	merged := existing
	changed := false

	if strings.TrimSpace(update.RuleType) != "" {
		merged.RuleType = update.RuleType
		changed = true
	}
	if update.PenaltyMinutes != 0 {
		merged.PenaltyMinutes = update.PenaltyMinutes
		changed = true
	}
	if strings.TrimSpace(update.PenaltyFormula) != "" {
		merged.PenaltyFormula = update.PenaltyFormula
		changed = true
	}
	if update.PenaltyCapMinutes != 0 {
		merged.PenaltyCapMinutes = update.PenaltyCapMinutes
		changed = true
	}
	if update.FreezeMinutesBeforeEnd != 0 {
		merged.FreezeMinutesBeforeEnd = update.FreezeMinutesBeforeEnd
		changed = true
	}
	if update.AllowHack {
		merged.AllowHack = true
		changed = true
	}
	if update.HackReward != 0 {
		merged.HackReward = update.HackReward
		changed = true
	}
	if update.HackPenalty != 0 {
		merged.HackPenalty = update.HackPenalty
		changed = true
	}
	if update.MaxSubmissionsPerProblem != 0 {
		merged.MaxSubmissionsPerProblem = update.MaxSubmissionsPerProblem
		changed = true
	}
	if strings.TrimSpace(update.ScoreMode) != "" {
		merged.ScoreMode = update.ScoreMode
		changed = true
	}
	if update.PublishSolutionsAfterEnd {
		merged.PublishSolutionsAfterEnd = true
		changed = true
	}
	if update.VirtualParticipationEnabled {
		merged.VirtualParticipationEnabled = true
		changed = true
	}
	return merged, changed
}
