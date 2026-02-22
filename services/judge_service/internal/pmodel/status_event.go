package pmodel

// StatusEventType represents the status event type.
type StatusEventType string

const (
	// StatusEventFinal indicates the final status event.
	StatusEventFinal StatusEventType = "final"
)

// StatusEvent carries status updates for async processing.
type StatusEvent struct {
	Type      StatusEventType     `json:"type"`
	Status    JudgeStatusResponse `json:"status"`
	CreatedAt int64               `json:"created_at"`
}
