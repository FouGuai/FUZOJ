package contextkey

// key is a private type to avoid context key collisions across packages.
type key string

const (
	TraceID   key = "trace_id"
	RequestID key = "request_id"
	UserID    key = "user_id"
)
