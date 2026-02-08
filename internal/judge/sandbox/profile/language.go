// Package profile defines language and task profiles used by the sandbox.
package profile

// LanguageSpec defines how to compile and run a language.
type LanguageSpec struct {
	ID               string
	Name             string
	Version          string
	SourceFile       string
	BinaryFile       string
	CompileEnabled   bool
	CompileCmdTpl    string
	RunCmdTpl        string
	Env              []string
	TimeMultiplier   float64
	MemoryMultiplier float64
}
