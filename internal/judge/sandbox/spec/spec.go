// Package spec defines the execution specification and resource limits.
package spec

// ResourceLimit describes hard limits enforced by the sandbox.
type ResourceLimit struct {
	CPUTimeMs  int64
	WallTimeMs int64
	MemoryMB   int64
	StackMB    int64
	OutputMB   int64
	PIDs       int64
}

// MountSpec describes a bind mount inside the sandbox.
type MountSpec struct {
	Source   string
	Target   string
	ReadOnly bool
}

// RunSpec is the unified execution specification for one task.
type RunSpec struct {
	WorkDir    string
	Cmd        []string
	Env        []string
	StdinPath  string
	StdoutPath string
	StderrPath string
	BindMounts []MountSpec
	Profile    string
	Limits     ResourceLimit
}
