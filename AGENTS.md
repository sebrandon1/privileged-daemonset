# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository Overview

This is a Go library for managing privileged DaemonSets on Kubernetes/OpenShift clusters. It provides functionality to:

- Create DaemonSets running in privileged mode with full host access
- Delete DaemonSets from specified namespaces
- Wait for DaemonSets to become ready within a specified timeout
- Configure privileged service accounts with necessary RBAC permissions

The library is designed to be imported by other projects (notably the certsuite) that need to run privileged operations on cluster nodes. The DaemonSet pods have:
- Privileged security context running as root
- Host network, IPC, and PID namespaces
- Host filesystem mounted at `/host`
- Tolerations for master/control-plane nodes

## Build Commands

```bash
make deps           # Download and tidy go module dependencies
make vet            # Run go vet on all packages
make lint           # Run golangci-lint on all packages (10 minute timeout)
make test           # Run go test with verbose output on all packages
make install-lint   # Install golangci-lint v2.1.6 if not present
make clean          # Clean build artifacts (placeholder)
make help           # Show available targets
```

## Test Commands

```bash
make test           # Run all unit tests with verbose output
go test -v ./...    # Alternative: run tests directly
```

## Code Organization

The repository has a simple structure with a single main package:

```
privileged-daemonset/
├── main.go           # Core library implementation (all exported functions)
├── go.mod            # Go module definition
├── go.sum            # Dependency checksums
├── Makefile          # Build automation
├── .golangci.yml     # Linter configuration
├── .github/
│   └── workflows/
│       └── pre-main.yaml   # CI workflow for linting and vetting
├── LICENSE           # Apache 2.0 license
└── README.md         # Basic usage documentation
```

### Exported Functions

The library exports these primary functions:

- `SetDaemonSetClient(k8sClient kubernetes.Interface)` - Set the Kubernetes client
- `CreateDaemonSet(name, namespace, containerName, image string, labels map[string]string, timeout time.Duration, cpuReq, cpuLim, memReq, memLim string, pullPolicy corev1.PullPolicy) (*corev1.PodList, error)` - Create a privileged DaemonSet
- `DeleteDaemonSet(name, namespace string) error` - Delete a DaemonSet
- `WaitDaemonsetReady(namespace, name string, timeout time.Duration) error` - Wait for DaemonSet to be ready
- `IsDaemonSetReady(name, namespace, image string) bool` - Check if DaemonSet is ready
- `ConfigurePrivilegedServiceAccount(namespace string) error` - Set up RBAC for privileged access
- `DeleteNamespaceIfPresent(namespace string) error` - Clean up namespace

## Key Dependencies

Primary dependencies (from go.mod):
- `k8s.io/api v0.34.1` - Kubernetes API types
- `k8s.io/apimachinery v0.34.1` - Kubernetes API machinery utilities
- `k8s.io/client-go v0.34.1` - Kubernetes client library
- `k8s.io/utils` - Kubernetes utility functions (pointer helpers)

## Development Guidelines

### Go Version
This repository uses Go 1.25.3. Ensure your local environment matches this version.

### Linting
The project uses golangci-lint with extensive configuration in `.golangci.yml`. Enabled linters include:
- `bodyclose`, `copyloopvar`, `dogsled`, `exhaustive`
- `funlen`, `goconst`, `gocritic`, `gocyclo`
- `goprintffuncname`, `gosec`, `lll`, `misspell`
- `mnd`, `nakedret`, `nolintlint`, `revive`
- `rowserrcheck`, `unconvert`, `unparam`, `whitespace`

Run `make lint` before committing changes.

### CI/CD
The GitHub Actions workflow (`.github/workflows/pre-main.yaml`) runs on:
- Pushes to `main` branch
- Pull requests targeting `main`
- Manual workflow dispatch

It performs:
1. Checkout code
2. Set up Go (version from go.mod)
3. Run golangci-lint with 10 minute timeout
4. Run `make vet`

### Usage Pattern

```go
import k8sPriviledgedDs "github.com/redhat-best-practices-for-k8s/privileged-daemonset"

// 1. Set the Kubernetes client
k8sPriviledgedDs.SetDaemonSetClient(myK8sInterface)

// 2. Create a privileged DaemonSet
pods, err := k8sPriviledgedDs.CreateDaemonSet(
    "my-ds",           // DaemonSet name
    "my-namespace",    // Namespace
    "container-name",  // Container name
    "image:tag",       // Container image
    labels,            // Pod labels
    5*time.Minute,     // Timeout
    "100m", "200m",    // CPU request/limit
    "64Mi", "128Mi",   // Memory request/limit
    corev1.PullAlways, // Image pull policy
)

// 3. Delete when done
err = k8sPriviledgedDs.DeleteDaemonSet("my-ds", "my-namespace")
```

### Important Implementation Details

- The library creates a namespace with RBAC resources (Role, RoleBinding, ServiceAccount) for privileged access
- DaemonSets are configured with OpenShift-specific SCC annotations (`node-exporter`)
- Tolerations allow scheduling on master/control-plane nodes
- The `IsDaemonSetReady` function considers a DaemonSet stale if it's been running for more than 7 days
- Namespace initialization deletes and recreates the namespace to ensure clean state
