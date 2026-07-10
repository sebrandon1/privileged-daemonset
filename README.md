# K8s Daemonset in privilege mode

## Requirements

- Go 1.26.0 or later

## Functions

- Creation of the daemonset in privilege mode
- Deletion of a daemonset in a specified namespace
- Check if a daemonset is ready within a specified time
- Check if a daemonset is ready and not stale
- Configure a privileged service account with RBAC permissions
- Delete a namespace if it exists

## Usage

1. Import the library

```go
import k8sPrivilegedDs "github.com/redhat-best-practices-for-k8s/privileged-daemonset"
```

2. Set the K8s client to act on `Daemonset` object

```go
k8sPrivilegedDs.SetDaemonSetClient(myK8sInterface) // myK8sInterface is of type kubernetes.Interface
```

3. Invoke the exported functions in a specified namespace with a specified imagename

**To create**

```go
daemonSetRunningPods, err := k8sPrivilegedDs.CreateDaemonSet(
    daemonSetName,        // DaemonSet name (string)
    namespace,            // Namespace (string)
    containerName,        // Container name (string)
    imageWithVersion,     // Container image (string)
    labels,               // Pod labels (map[string]string)
    timeout,              // Timeout (time.Duration)
    cpuReq,               // CPU request, e.g. "100m" (string)
    cpuLim,               // CPU limit, e.g. "200m" (string)
    memReq,               // Memory request, e.g. "64Mi" (string)
    memLim,               // Memory limit, e.g. "128Mi" (string)
    pullPolicy,           // Image pull policy (corev1.PullPolicy)
)
```

**To delete**

```go
err := k8sPrivilegedDs.DeleteDaemonSet(daemonSetName, namespace)
```

**To wait for the daemonset to be ready**

```go
err := k8sPrivilegedDs.WaitDaemonsetReady(namespace, daemonSetName, timeout)
```

**To check if the daemonset is ready and not stale**

```go
ready := k8sPrivilegedDs.IsDaemonSetReady(daemonSetName, namespace, image)
```

Returns `false` if the DaemonSet does not exist, has been running for more than 7 days, the container image does not match, or the DaemonSet is not healthy.

**To configure a privileged service account**

```go
err := k8sPrivilegedDs.ConfigurePrivilegedServiceAccount(namespace)
```

Creates a Role, RoleBinding, and ServiceAccount in the specified namespace with permissions to use the `privileged` SecurityContextConstraint.

**To delete a namespace if present**

```go
err := k8sPrivilegedDs.DeleteNamespaceIfPresent(namespace)
```

Deletes the namespace and waits for its removal from the cluster.
