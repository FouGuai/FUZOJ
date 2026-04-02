package statusmonotonic

import "strings"

const (
	stageUnknown    = -1
	stageQueueing   = 0
	stageValidating = 1
	stagePending    = 2
	stageCompile    = 3
	stageRunning    = 4
	stageJudging    = 5
	stageFinal      = 6
)

// ShouldAccept returns whether next status can overwrite current status.
// Rules:
// 1. status stage must not go backwards.
// 2. once current status is final, no further update is accepted.
// 3. when stage is equal, progress cannot decrease.
func ShouldAccept(
	currentStatus string,
	currentDoneTests, currentTotalTests int,
	nextStatus string,
	nextDoneTests, nextTotalTests int,
) (bool, string) {
	currentStage := statusStage(currentStatus)
	nextStage := statusStage(nextStatus)
	if nextStage == stageUnknown {
		return false, "next status is unknown"
	}
	if currentStage == stageUnknown {
		return true, ""
	}
	if currentStage == stageFinal {
		return false, "current status is already final"
	}
	if nextStage < currentStage {
		return false, "status stage regressed"
	}
	if nextStage == currentStage {
		if nextDoneTests < currentDoneTests {
			return false, "done_tests regressed"
		}
		if nextTotalTests < currentTotalTests {
			return false, "total_tests regressed"
		}
		if nextDoneTests == currentDoneTests && nextTotalTests == currentTotalTests {
			return false, "progress is not strictly increasing"
		}
	}
	return true, ""
}

func statusStage(status string) int {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "queueing":
		return stageQueueing
	case "validating":
		return stageValidating
	case "pending":
		return stagePending
	case "compiling":
		return stageCompile
	case "running":
		return stageRunning
	case "judging":
		return stageJudging
	case "finished", "failed":
		return stageFinal
	default:
		return stageUnknown
	}
}
