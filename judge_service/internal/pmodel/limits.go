package pmodel

import "fuzoj/judge_service/internal/sandbox/spec"

// MergeLimits merges override with defaults using non-zero fields.
func MergeLimits(override *ResourceLimit, defaults ResourceLimit) ResourceLimit {
	if override == nil {
		return defaults
	}
	out := defaults
	if override.CPUTimeMs > 0 {
		out.CPUTimeMs = override.CPUTimeMs
	}
	if override.WallTimeMs > 0 {
		out.WallTimeMs = override.WallTimeMs
	}
	if override.MemoryMB > 0 {
		out.MemoryMB = override.MemoryMB
	}
	if override.StackMB > 0 {
		out.StackMB = override.StackMB
	}
	if override.OutputMB > 0 {
		out.OutputMB = override.OutputMB
	}
	if override.PIDs > 0 {
		out.PIDs = override.PIDs
	}
	return out
}

// ToSandboxLimit converts to sandbox spec.
func ToSandboxLimit(limit ResourceLimit) spec.ResourceLimit {
	return spec.ResourceLimit{
		CPUTimeMs:  limit.CPUTimeMs,
		WallTimeMs: limit.WallTimeMs,
		MemoryMB:   limit.MemoryMB,
		StackMB:    limit.StackMB,
		OutputMB:   limit.OutputMB,
		PIDs:       limit.PIDs,
	}
}
