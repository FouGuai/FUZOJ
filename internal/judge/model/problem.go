package model

// ProblemMeta represents the latest published problem meta.
type ProblemMeta struct {
	ProblemID    int64
	Version      int32
	ManifestHash string
	DataPackKey  string
	DataPackHash string
	UpdatedAt    int64
}
