package repository

import "fuzoj/services/submit_service/internal/domain"

const (
	LogTypeCompileLog   = "compile_log"
	LogTypeCompileError = "compile_error"
	LogTypeRuntime      = "runtime_log"
	LogTypeChecker      = "checker_log"
	LogTypeStdout       = "stdout"
	LogTypeStderr       = "stderr"
)

// LogRecord captures one log entry extracted from judge status.
type LogRecord struct {
	SubmissionID string
	LogType      string
	TestID       string
	Content      string
}

// ExtractLogs removes log fields from status and returns sanitized status + logs.
func ExtractLogs(status domain.JudgeStatusPayload) (domain.JudgeStatusPayload, []LogRecord) {
	clean := status
	logs := make([]LogRecord, 0)

	if status.Compile != nil {
		compile := *status.Compile
		if compile.Log != "" {
			logs = append(logs, LogRecord{
				SubmissionID: status.SubmissionID,
				LogType:      LogTypeCompileLog,
				Content:      compile.Log,
			})
		}
		if compile.Error != "" {
			logs = append(logs, LogRecord{
				SubmissionID: status.SubmissionID,
				LogType:      LogTypeCompileError,
				Content:      compile.Error,
			})
		}
		compile.Log = ""
		compile.Error = ""
		clean.Compile = &compile
	}

	if len(status.Tests) == 0 {
		return clean, logs
	}
	tests := make([]domain.TestcaseResult, 0, len(status.Tests))
	for _, test := range status.Tests {
		item := test
		if item.RuntimeLog != "" {
			logs = append(logs, LogRecord{
				SubmissionID: status.SubmissionID,
				LogType:      LogTypeRuntime,
				TestID:       item.TestID,
				Content:      item.RuntimeLog,
			})
		}
		if item.CheckerLog != "" {
			logs = append(logs, LogRecord{
				SubmissionID: status.SubmissionID,
				LogType:      LogTypeChecker,
				TestID:       item.TestID,
				Content:      item.CheckerLog,
			})
		}
		if item.Stdout != "" {
			logs = append(logs, LogRecord{
				SubmissionID: status.SubmissionID,
				LogType:      LogTypeStdout,
				TestID:       item.TestID,
				Content:      item.Stdout,
			})
		}
		if item.Stderr != "" {
			logs = append(logs, LogRecord{
				SubmissionID: status.SubmissionID,
				LogType:      LogTypeStderr,
				TestID:       item.TestID,
				Content:      item.Stderr,
			})
		}
		item.RuntimeLog = ""
		item.CheckerLog = ""
		item.Stdout = ""
		item.Stderr = ""
		tests = append(tests, item)
	}
	clean.Tests = tests
	return clean, logs
}
