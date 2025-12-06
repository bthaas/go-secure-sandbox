# Build Stage
FROM golang:1.23-bullseye AS builder

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build for Linux
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o sandbox-api ./cmd/server

# Final Stage (Minimal Runtime)
FROM debian:bullseye-slim

WORKDIR /app

# Install dependencies (Firecracker requires KVM access, usually provided by host)
# installing curl for healthcheck
RUN apt-get update && apt-get install -y curl && rm -rf /var/lib/apt/lists/*

# Copy binary
COPY --from=builder /app/sandbox-api /usr/local/bin/sandbox-api

# Create cgroup directories
RUN mkdir -p /sys/fs/cgroup/sandbox.slice

# Create directories for artifacts
RUN mkdir -p /var/lib/sandbox

EXPOSE 8080

CMD ["sandbox-api"]
