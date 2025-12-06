//go:build !linux

package sandbox

import (
	"context"
	"errors"
)

// FirecrackerExecutor is a stub for non-Linux systems
type FirecrackerExecutor struct{}

func NewFirecrackerExecutor(cfg Config) (*FirecrackerExecutor, error) {
	return nil, errors.New("firecracker executor only supported on linux")
}

func (f *FirecrackerExecutor) Execute(ctx context.Context, req ExecutionRequest) (*ExecutionResult, error) {
	return nil, errors.New("not implemented on this OS")
}

func (f *FirecrackerExecutor) Cleanup() error {
	return nil
}
