# Secure Code Sandbox API

A production-ready API for securely executing user-provided code snippets in isolated MicroVM environments using Firecracker. Designed for AI agents and other applications that need to safely run untrusted code with strict resource limits and network isolation.

## 🎯 Overview

This project implements a secure code execution platform that spins up ephemeral Firecracker microVMs to execute Python and JavaScript code in complete isolation. Each execution runs in its own microVM with:

- **Hardware-level isolation** via Firecracker (same technology powering AWS Lambda)
- **Resource limits** enforced via Linux cgroups (CPU, memory, process count)
- **Network namespace isolation** (prevents scanning internal networks)
- **Sub-150ms cold start times** (when optimized with pre-built artifacts)
- **Automatic cleanup** of resources after execution

## 🏗️ Architecture

```
Client → API Server (Echo/Go) → Executor Service → Firecracker VMM → MicroVM Guest
                                              ↓
                                      Cgroups + Network Namespaces
```

### Key Components

- **API Gateway** (`internal/api/`): REST API endpoint handling with Echo framework
- **Executor Service** (`internal/sandbox/`): Orchestrates Firecracker VM lifecycle
- **Isolation Layer** (`internal/sandbox/isolation_linux.go`): Cgroups and network namespace setup
- **Firecracker Integration**: Direct integration with Firecracker VMM using Go SDK

### Security Features

1. **Cgroups V2** for resource limiting:
   - Memory: 128MB hard limit
   - CPU: 50% of 1 core (quota-based)
   - Process count: Max 20 processes

2. **Network Namespaces**: Each VM runs in an isolated network namespace (implementation in progress)

3. **Read-only RootFS**: Guest filesystem is mounted read-only

4. **VSock Communication**: Uses virtio sockets for host-guest communication (faster than network stack)

## 📋 Prerequisites

### For Development/Testing (Mock Mode)

- **Go 1.24.5+** (or latest stable)
- Any OS (macOS, Linux, Windows) - automatically uses mock executor

### For Production (Firecracker Mode)

- **Linux** (x86_64) with KVM support
- **Firecracker binary** installed at `/usr/local/bin/firecracker` (or set `FC_BIN_PATH`)
- **Kernel image** (`vmlinux`) optimized for Firecracker
- **RootFS image** (`rootfs.ext4`) with:
  - Minimal Linux distribution (e.g., Alpine Linux)
  - Python interpreter (for Python execution)
  - Node.js runtime (for JavaScript execution)
  - Sandbox agent that listens on VSock port 5000
- **Root privileges** or capabilities:
  - `CAP_NET_ADMIN` (for network namespace setup)
  - `CAP_SYS_ADMIN` (for cgroup management)
- **Cgroups V2** mounted at `/sys/fs/cgroup`

## 🚀 Quick Start

### 1. Clone and Build

```bash
git clone <repository-url>
cd sandboxapi
go mod download
go build -o sandbox-api ./cmd/server
```

### 2. Run in Mock Mode (Development)

Mock mode works on any OS and simulates execution without real isolation:

```bash
USE_MOCK=true PORT=8080 ./sandbox-api
```

### 3. Test the API

**Health Check:**
```bash
curl http://localhost:8080/health
```

**Execute Python Code:**
```bash
curl -X POST http://localhost:8080/execute \
  -H "Content-Type: application/json" \
  -d '{
    "language": "python",
    "code_snippet": "print(\"Hello, World!\")",
    "timeout_ms": 5000
  }'
```

**Execute JavaScript Code:**
```bash
curl -X POST http://localhost:8080/execute \
  -H "Content-Type: application/json" \
  -d '{
    "language": "javascript",
    "code_snippet": "console.log(\"Hello, World!\")",
    "timeout_ms": 5000
  }'
```

## ⚙️ Configuration

The server can be configured via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP server port |
| `USE_MOCK` | `false` (auto on non-Linux) | Use mock executor instead of Firecracker |
| `KERNEL_PATH` | `/var/lib/sandbox/vmlinux` | Path to Firecracker kernel image |
| `ROOTFS_PATH` | `/var/lib/sandbox/rootfs.ext4` | Path to root filesystem image |
| `FC_BIN_PATH` | `/usr/local/bin/firecracker` | Path to Firecracker binary |

### Example Configuration

```bash
export PORT=9000
export KERNEL_PATH=/opt/firecracker/vmlinux-5.10.176
export ROOTFS_PATH=/opt/firecracker/rootfs-python.ext4
export FC_BIN_PATH=/usr/bin/firecracker
./sandbox-api
```

## 📡 API Reference

### POST /execute

Execute code in a secure sandbox environment.

**Request Body:**
```json
{
  "language": "python" | "javascript",
  "code_snippet": "print('hello')",
  "timeout_ms": 5000
}
```

**Response (200 OK):**
```json
{
  "stdout": "hello\n",
  "stderr": "",
  "exit_code": 0,
  "duration_ms": 120,
  "error": ""
}
```

**Response Fields:**
- `stdout`: Standard output from code execution
- `stderr`: Standard error output
- `exit_code`: Process exit code (0 = success)
- `duration_ms`: Execution time in milliseconds
- `error`: System error message (if execution failed)

**Error Responses:**
- `400 Bad Request`: Invalid request payload or missing required fields
- `500 Internal Server Error`: Execution failed (VM boot, connection, etc.)

### GET /health

Health check endpoint for monitoring and load balancers.

**Response:**
```
OK
```

## 🔧 Building RootFS and Kernel

For production use, you need to prepare Firecracker artifacts. Here's a high-level overview:

### 1. Build Kernel Image

```bash
# Clone Linux kernel source
# Configure for Firecracker (minimal drivers, no USB, etc.)
# Build vmlinux (uncompressed kernel)
# Place at /var/lib/sandbox/vmlinux
```

### 2. Build RootFS

```bash
# Create Alpine Linux rootfs
# Install Python, Node.js
# Create sandbox agent that:
#   - Listens on VSock port 5000
#   - Receives ExecutionRequest JSON
#   - Executes code in subprocess
#   - Returns ExecutionResult JSON
# Create ext4 image: mkfs.ext4 rootfs.ext4
# Place at /var/lib/sandbox/rootfs.ext4
```

See `design.md` for detailed architecture and implementation notes.

## 🧪 Testing

Run the test suite:

```bash
go test ./...
```

Run with verbose output:

```bash
go test -v ./...
```

### Test Coverage

The project includes unit tests for the API handler using the mock executor. Integration tests with Firecracker would require a Linux environment with KVM.

## 🐳 Docker

A Dockerfile is provided for containerized deployment. Note that Firecracker requires KVM access, so the container must run with:

- `--privileged` flag, or
- Specific capabilities and `/dev/kvm` device access

```bash
docker build -t sandbox-api .
docker run --privileged -p 8080:8080 sandbox-api
```

For production, consider using Firecracker's jailer for additional security.

## 🛠️ Development

### Project Structure

```
sandboxapi/
├── cmd/server/          # Main application entry point
├── internal/
│   ├── api/            # HTTP handlers and routes
│   └── sandbox/        # Executor implementations
│       ├── executor.go              # Interface definition
│       ├── firecracker_executor_linux.go  # Firecracker implementation
│       ├── mock_executor.go         # Mock for development
│       └── isolation_linux.go       # Cgroups/NetNS setup
├── design.md           # Detailed architecture document
└── Dockerfile          # Container image definition
```

### Adding New Languages

1. Update `Language*` constants in `internal/sandbox/executor.go`
2. Modify the sandbox agent in the RootFS to handle the new language
3. Update validation logic if needed

### Mock Executor

The mock executor is automatically used when:
- Running on non-Linux OS (macOS, Windows)
- `USE_MOCK=true` environment variable is set

**Warning**: Mock executor provides **NO isolation** and should only be used for development/testing.

## 🔒 Security Considerations

### Production Checklist

- [ ] Run as non-root user with minimal capabilities where possible
- [ ] Use Firecracker Jailer for additional process isolation
- [ ] Implement rate limiting on API endpoints
- [ ] Add authentication/authorization (not included)
- [ ] Enable HTTPS/TLS (use reverse proxy like nginx)
- [ ] Monitor resource usage and set global quotas
- [ ] Implement request size limits (currently code size not validated)
- [ ] Add network policies to prevent VM-to-VM communication
- [ ] Regular security updates for kernel and rootfs images
- [ ] Audit logging for all executions

### Known Limitations

- Network namespace setup is partially implemented (cgroups are fully functional)
- No authentication/authorization built-in
- No rate limiting
- RootFS agent implementation not included (must be built separately)

## 📊 Performance

### Expected Metrics

- **Cold Start**: <150ms (with optimized kernel and rootfs)
- **Memory per VM**: ~128MB
- **CPU**: 50% of 1 core per execution
- **Concurrent Executions**: Limited by host resources

### Optimization Tips

1. Use stripped-down kernel (`vmlinux`) without compression
2. Minimal init system (avoid systemd, use custom init)
3. Pre-warm VM pool for faster response times
4. Use copy-on-write filesystems for RootFS snapshots

## 🐛 Troubleshooting

### "Failed to setup isolation"

- Ensure running on Linux with cgroups V2
- Check permissions: need write access to `/sys/fs/cgroup/sandbox.slice`
- Verify cgroups are mounted: `mount | grep cgroup`

### "Failed to start machine"

- Verify Firecracker binary exists and is executable
- Check kernel and rootfs paths are correct
- Ensure KVM is available: `ls -l /dev/kvm`
- Check Firecracker logs in temporary directory

### "Failed to connect to sandbox agent"

- Verify RootFS contains a sandbox agent listening on VSock port 5000
- Check agent logs inside the VM (requires console access)
- Increase connection timeout if VM boot is slow

### Mock Mode Warnings

If you see "EXECUTING IN MOCK MODE (NO ISOLATION)" in logs:
- This is expected on macOS/Windows or when `USE_MOCK=true`
- Mock mode is safe for development but provides no security

## 📝 License

MIT License - see [LICENSE](LICENSE) file for details.

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## 📚 Additional Resources

- [Firecracker Documentation](https://firecracker-microvm.github.io/)
- [Firecracker Go SDK](https://github.com/firecracker-microvm/firecracker-go-sdk)
- [Cgroups V2 Documentation](https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v2.html)
- See `design.md` for detailed architecture and design decisions

---

**Built with ❤️ for secure, fast code execution**

