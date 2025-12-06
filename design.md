# Secure Code Sandbox API - Design Document

## 1. High-Level Architecture

The system follows a synchronous execution model designed for low latency.

```mermaid
graph LR
    User[User/Client] -- HTTPS POST /execute --> LB[Load Balancer]
    LB --> API[API Gateway / Routing]
    
    subgraph Host [Executor Host]
        API -- "gRPC/Internal" --> Executor[Executor Service (Go)]
        
        Executor -- "Start VMM (fork/exec)" --> VMM_Process[Firecracker VMM Process]
        
        subgraph Isolation [Isolated Environment]
            direction TB
            Cgroups[Cgroups Limit (CPU/RAM)]
            NetNS[Network Namespace]
            
            VMM_Process --> MicroVM[MicroVM Guest]
        end
        
        Executor -- "Config (HTTP/Unix Socket)" --> VMM_Process
        Executor -- "Code/Output (VSock)" --> MicroVM
    end
```

### Components
*   **API Gateway**: Handles authentication, validation, and routing.
*   **Executor Service**:Written in **Go**. Orchestrates the lifecycle of Firecracker VMs. It sets up namespaces, cgroups, configures the VMM, and manages the result retrieval.
*   **Firecracker VMM**: The KVM-based VMM daemon provided by AWS.
*   **MicroVM**: Minimal Linux guest (e.g., Alpine-based) running a lightweight **Sandbox Agent**.

## 2. Firecracker Initialization Flow (<150ms Cold Start)

To achieve the <150ms target without a pre-warmed pool, we optimize the boot sequence:

1.  **Request Reception**: Executor receives `ExecutionJob`.
2.  **Resource Prep (Parallel)**:
    *   **Network**: Create a `veth` pair. One end in host, one moved to a new Network Namespace.
    *   **FS**: Create a copy-on-write overlay or snapshot of the rootfs (ext4) for ephemeral use.
3.  **VMM Launch**:
    *   Executor starts `firecracker` process inside a new **cgroup** and **network namespace**.
    *   *Optimization*: Use a stripped-down Linux kernel (`vmlinux`) optimized for Firecracker (no USB, minimal drivers).
4.  **VMM Configuration (via Unix Socket)**:
    *   **Boot Source**: Point to the kernel image.
    *   **Drives**: point to the ephemeral rootfs (read-mostly + overlay).
    *   **VSock**: Configure `vsock` device for communication.
    *   **Network**: Configure network interface using the tap device created in the namespace.
5.  **Instance Start**: Send `InstanceStart`.
6.  **Agent Ready**: The in-VM agent starts immediately (PID 1 or called by init) and listens on the VSock.

**Key Latency Factors**:
*   **Kernel**: Small uncompressed kernel (vmlinux).
*   **Init System**: Do not use systemd. Use a custom init or extremely minimal OpenRC.
*   **VMM API**: Send configuration commands in parallel or bulk if possible (standard FC API is sequential).

## 3. Security Implementation Plan

### Network Namespace (NetNS)
Isolate the network stack. The VM sees only its own loopback and the tap interface.

**Setup Plan:**
1.  Create a new netns: `ip netns add <sandbox_id>`
2.  Create veth pair: `ip link add veth_host type veth peer name veth_guest`
3.  Move guest end to netns: `ip link set veth_guest netns <sandbox_id>`
4.  Configure IP/Routing inside netns.
5.  Setup IPTables/NFTables rules on Host to Masquerade (NAT) if internet is required, or DROP by default.

### Control Groups (Cgroups V2)
Limit resource usage to prevent DoS.

**Pseudo-code / Setup:**
```bash
# Create a parent slice for all sandboxes
mkdir -p /sys/fs/cgroup/sandbox.slice

# Create a group for this specific execution
mkdir -p /sys/fs/cgroup/sandbox.slice/sandbox-<id>

# Apply Limits
# Memory: 128MB Hard Limit
echo "134217728" > /sys/fs/cgroup/sandbox.slice/sandbox-<id>/memory.max
# CPU: 50% of 1 Core (Quota/Period)
echo "50000 100000" > /sys/fs/cgroup/sandbox.slice/sandbox-<id>/cpu.max
# PIDs: Max 20 processes
echo "20" > /sys/fs/cgroup/sandbox.slice/sandbox-<id>/pids.max

# When starting Firecracker, move the process into this cgroup
echo <FIRECRACKER_PID> > /sys/fs/cgroup/sandbox.slice/sandbox-<id>/cgroup.procs
```

### Linux Capabilities
The Executor Service needs `CAP_NET_ADMIN` (for network orchestration) and potentially `CAP_SYS_ADMIN` (for mount/unmount if handling rootfs overlays directly), though running as root is often the simplest path for the VMM spawner. Ideally, the *Firecracker process* itself is then jailed using the `jailer` binary which drops privileges and `chroot`s.

## 4. Communication and Output Capture

We will use **VSock (Virtio Socket)**. It provides a standard socket interface between host and guest without network stack overhead.

1.  **Host Side**: The Executor listens on a Unix socket that maps to the VSock port (e.g., via Firecracker configuration).
2.  **Guest Side**: The Sandbox Agent connects/listens on `AF_VSOCK`.
3.  **Protocol**:
    *   Executor CONNECTs to Guest OS Agent.
    *   Send Payload: `{ "code": "...", "lang": "python" }`
    *   Agent executes code (spawning `python -c ...`).
    *   Agent captures `stdout` and `stderr` pipes.
    *   Agent streams response back: `{ "stdout": "...", "stderr": "...", "ret": 0 }`

This avoids the overhead of mounting/unmounting block devices for every small request and is faster than a virtualized TCP/IP network.

## 5. Data Structures (Go)

```go
package sandbox

// ExecutionRequest represents the incoming user request
type ExecutionRequest struct {
    Language   string `json:"language" validate:"required,oneof=python javascript"`
    Code       string `json:"code_snippet" validate:"required,max=10000"` // Max 10KB code
    TimeoutMs  int    `json:"timeout_ms,omitempty"` // Default 5000ms
}

// ExecutionResult represents the secure execution output
type ExecutionResult struct {
    Stdout     string `json:"stdout"`
    Stderr     string `json:"stderr"`
    ExitCode   int    `json:"exit_code"`
    DurationMs int64  `json:"duration_ms"` // Execution time inside VM
    Error      string `json:"error,omitempty"` // System errors (not code errors)
}

// Internal config for the MicroVM
type VMConfig struct {
    KernelPath  string
    RootFSPath  string
    CPUCount    int
    MemSizeMib  int
    NetworkTap  string
}
```
