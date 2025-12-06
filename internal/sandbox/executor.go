package sandbox

import (
	"context"
	"errors"
)

// Supported languages
const (
	LanguagePython     = "python"
	LanguageJavascript = "javascript"
)

// ExecutionRequest represents the incoming user request
type ExecutionRequest struct {
	Language   string `json:"language"`
	Code       string `json:"code_snippet"`
	TimeoutMs  int    `json:"timeout_ms"`
}

// ExecutionResult represents the secure execution output
type ExecutionResult struct {
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	ExitCode   int    `json:"exit_code"`
	DurationMs int64  `json:"duration_ms"`
	Error      string `json:"error,omitempty"`
}

// Config holds configuration for the Sandbox environment
type Config struct {
	KernelPath       string
	RootFSPath       string
	FirecrackerBin   string
	CgroupParent     string
	NetworkInterface string
}

// Executor defines the interface for running code in a sandbox
type Executor interface {
	// Execute runs the code in a secure environment and returns the result
	Execute(ctx context.Context, req ExecutionRequest) (*ExecutionResult, error)
	
	// Cleanup cleans up any resources (firecracker processes, cgroups, etc)
	Cleanup() error
}

var (
	ErrUnsupportedLanguage = errors.New("unsupported language")
	ErrExecutionFailed     = errors.New("execution failed")
	ErrTimeout             = errors.New("execution timed out")
)
