//go:build linux

package sandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"

	fc "github.com/firecracker-microvm/firecracker-go-sdk"
	models "github.com/firecracker-microvm/firecracker-go-sdk/client/models"
	"github.com/google/uuid"
	"github.com/labstack/gommon/log"
)

type FirecrackerExecutor struct {
	Config Config
}

func NewFirecrackerExecutor(cfg Config) (*FirecrackerExecutor, error) {
	return &FirecrackerExecutor{Config: cfg}, nil
}

func (f *FirecrackerExecutor) Execute(ctx context.Context, req ExecutionRequest) (*ExecutionResult, error) {
	// 1. Generate Sandbox ID
	id := uuid.New().String()
	socketPath := filepath.Join(os.TempDir(), fmt.Sprintf("fc-sock-%s", id))
	vsockPath := filepath.Join(os.TempDir(), fmt.Sprintf("fc-vsock-%s", id))
	logPath := filepath.Join(os.TempDir(), fmt.Sprintf("fc-log-%s", id))

	// Ensure cleanup of socket files
	defer os.Remove(socketPath)
	defer os.Remove(vsockPath)
	defer os.Remove(logPath)

	// 2. Prepare Isolation (Cgroups)
	_, err := SetupIsolation(id)
	if err != nil {
		return nil, fmt.Errorf("failed to setup isolation: %w", err)
	}
	defer CleanupIsolation(id)

	// 3. Configure Firecracker
	cfg := fc.Config{
		SocketPath:      socketPath,
		KernelImagePath: f.Config.KernelPath,
		MachineCfg: models.MachineConfiguration{
			VcpuCount:  fc.Int64(1),
			MemSizeMib: fc.Int64(128),
			Smt:        fc.Bool(false),
		},
		Drives: []models.Drive{
			{
				DriveID:      fc.String("rootfs"),
				PathOnHost:   fc.String(f.Config.RootFSPath),
				IsRootDevice: fc.Bool(true),
				IsReadOnly:   fc.Bool(true),
			},
		},
		VsockDevices: []fc.VsockDevice{
			{
				Path: vsockPath,
				CID:  uint32(3),
			},
		},
		LogPath:  logPath,
		LogLevel: "Warn",
	}

	// 4. Create Command builder
	cmd := fc.VMCommandBuilder{}.
		WithBin(f.Config.FirecrackerBin).
		WithSocketPath(socketPath).
		Build(ctx)

	// 5. Start Machine
	m, err := fc.NewMachine(ctx, cfg, fc.WithProcessRunner(cmd))
	if err != nil {
		return nil, fmt.Errorf("failed to create machine: %w", err)
	}

	// Start the VMM
	if err := m.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start machine: %w", err)
	}
	defer m.StopVMM()

	// 5b. Attach to Cgroup
	pid, _ := m.PID()
	if pid > 0 {
		if err := AttachToCgroup(id, pid); err != nil {
			log.Errorf("Failed to attach PID %d to cgroup: %v", pid, err)
			// Proceeding, but logging error
		}
	}

	// 6. Connect to VSock (Host side of the socket)
	// Retry loop until connection succeeds (guest agent up)
	conn, err := tryConnectVsock(ctx, vsockPath)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to sandbox agent: %w", err)
	}
	defer conn.Close()

	// 7. Send Request
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// 8. Read Response
	var result ExecutionResult
	if err := json.NewDecoder(conn).Decode(&result); err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("sandbox closed connection unexpectedly")
		}
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

func (f *FirecrackerExecutor) Cleanup() error {
	return nil
}

// tryConnectVsock attempts to dial the Unix socket and perform the CONNECT handshake
func tryConnectVsock(ctx context.Context, path string) (net.Conn, error) {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	// Try for up to 500ms for MicroVM cold start
	timeout := time.After(500 * time.Millisecond)

	var conn net.Conn
	var err error

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeout:
			return nil, fmt.Errorf("timeout waiting for vsock socket at %s", path)
		case <-ticker.C:
			// 1. Dial the Unix Socket
			conn, err = net.Dial("unix", path)
			if err == nil {
				goto Handshake
			}
		}
	}

Handshake:
	// 2. Perform Handshake: "CONNECT 5000\n"
	// We assume Guest Agent listens on port 5000
	// "CONNECT 5000\n"
	// Expect "OK 5000\n"
	targetPort := "5000"
	cmd := fmt.Sprintf("CONNECT %s\n", targetPort)
	if _, err := conn.Write([]byte(cmd)); err != nil {
		conn.Close()
		return nil, fmt.Errorf("handshake write failed: %w", err)
	}

	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("handshake read failed: %w", err)
	}

	resp := string(buf[:n])
	expected := fmt.Sprintf("OK %s", targetPort)
	// Check prefix because response might have newline
	if len(resp) < len(expected) || resp[:len(expected)] != expected {
		conn.Close()
		return nil, fmt.Errorf("handshake failed, got: %s", resp)
	}

	return conn, nil
}
