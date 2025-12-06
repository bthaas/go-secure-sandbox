package sandbox

import (
	"context"
	"fmt"
	"time"

	"github.com/labstack/gommon/log"
)

// MockExecutor implements Executor for local development (macOS/Windows)
// It does NOT provide isolation. It simulates execution.
type MockExecutor struct {
}

func NewMockExecutor() *MockExecutor {
	return &MockExecutor{}
}

func (m *MockExecutor) Execute(ctx context.Context, req ExecutionRequest) (*ExecutionResult, error) {
	log.Warn("EXECUTING IN MOCK MODE (NO ISOLATION)")

	// Simulate processing time
	select {
	case <-time.After(100 * time.Millisecond):
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	if req.Language != LanguagePython && req.Language != LanguageJavascript {
		return nil, ErrUnsupportedLanguage
	}

	// Basic simulation logic
	res := &ExecutionResult{
		ExitCode:   0,
		DurationMs: 100,
	}

	switch req.Language {
	case LanguagePython:
		res.Stdout = fmt.Sprintf("MOCK Python Output.\nExecuted: %s", req.Code)
		res.Stderr = ""
	case LanguageJavascript:
		res.Stdout = fmt.Sprintf("MOCK JS Output.\nExecuted: %s", req.Code)
		res.Stderr = ""
	}

	return res, nil
}

func (m *MockExecutor) Cleanup() error {
	return nil
}
