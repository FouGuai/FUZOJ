// Package observer defines logging and metrics hooks for sandbox execution.
package observer

import "context"

// MetricsRecorder records sandbox metrics.
type MetricsRecorder interface {
	ObserveCompile(ctx context.Context, languageID string, ok bool, timeMs int64, memoryKB int64)
	ObserveRun(ctx context.Context, languageID string, verdict string, timeMs int64, memoryKB int64, outputKB int64)
}
