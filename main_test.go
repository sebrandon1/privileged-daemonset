package privilegeddaemonset

import (
	"fmt"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

const (
	testNS       = "test-ns"
	testNodeName = "node1"
)

func setupFakeClient(objects ...runtime.Object) *fake.Clientset {
	cs := fake.NewSimpleClientset(objects...)
	SetDaemonSetClient(cs)
	return cs
}

func newDaemonSet(name, namespace, image string, createdAt time.Time, status appsv1.DaemonSetStatus) *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         namespace,
			CreationTimestamp: metav1.NewTime(createdAt),
		},
		Spec: appsv1.DaemonSetSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Image: image},
					},
				},
			},
		},
		Status: status,
	}
}

func readyStatus(n int32) appsv1.DaemonSetStatus {
	return appsv1.DaemonSetStatus{
		DesiredNumberScheduled: n,
		CurrentNumberScheduled: n,
		NumberAvailable:        n,
		NumberReady:            n,
		NumberMisscheduled:     0,
	}
}

func daemonSetReadyStatusCases() []struct {
	name   string
	status appsv1.DaemonSetStatus
	want   bool
} {
	return []struct {
		name   string
		status appsv1.DaemonSetStatus
		want   bool
	}{
		{name: "all zeros", status: readyStatus(0), want: true},
		{name: "all match", status: readyStatus(3), want: true},
		{
			name: "misscheduled",
			status: appsv1.DaemonSetStatus{
				DesiredNumberScheduled: 3, CurrentNumberScheduled: 3,
				NumberAvailable: 3, NumberReady: 3, NumberMisscheduled: 1,
			},
			want: false,
		},
		{
			name: "not enough ready",
			status: appsv1.DaemonSetStatus{
				DesiredNumberScheduled: 3, CurrentNumberScheduled: 3,
				NumberAvailable: 3, NumberReady: 2,
			},
			want: false,
		},
		{
			name: "not enough available",
			status: appsv1.DaemonSetStatus{
				DesiredNumberScheduled: 3, CurrentNumberScheduled: 3,
				NumberAvailable: 2, NumberReady: 3,
			},
			want: false,
		},
		{
			name: "current less than desired",
			status: appsv1.DaemonSetStatus{
				DesiredNumberScheduled: 3, CurrentNumberScheduled: 2,
				NumberAvailable: 3, NumberReady: 3,
			},
			want: false,
		},
	}
}

func TestIsDaemonSetReadyStatus(t *testing.T) {
	for _, tc := range daemonSetReadyStatusCases() {
		t.Run(tc.name, func(t *testing.T) {
			got := isDaemonSetReady(&tc.status)
			if got != tc.want {
				t.Errorf("isDaemonSetReady() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestIsDaemonSetReady(t *testing.T) {
	const (
		dsName = "test-ds"
		image  = "test-image:latest"
	)

	tests := []struct {
		name    string
		objects []runtime.Object
		want    bool
	}{
		{
			name:    "ds not found",
			objects: nil,
			want:    false,
		},
		{
			name: "ds older than 7 days",
			objects: []runtime.Object{
				newDaemonSet(dsName, testNS, image, time.Now().Add(-8*24*time.Hour), readyStatus(1)),
			},
			want: false,
		},
		{
			name: "image mismatch",
			objects: []runtime.Object{
				newDaemonSet(dsName, testNS, "wrong-image:v1", time.Now(), readyStatus(1)),
			},
			want: false,
		},
		{
			name: "ds not healthy",
			objects: []runtime.Object{
				newDaemonSet(dsName, testNS, image, time.Now(), appsv1.DaemonSetStatus{
					DesiredNumberScheduled: 3,
					CurrentNumberScheduled: 3,
					NumberAvailable:        2,
					NumberReady:            2,
				}),
			},
			want: false,
		},
		{
			name: "ds ready",
			objects: []runtime.Object{
				newDaemonSet(dsName, testNS, image, time.Now(), readyStatus(3)),
			},
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			setupFakeClient(tc.objects...)
			got := IsDaemonSetReady(dsName, testNS, image)
			if got != tc.want {
				t.Errorf("IsDaemonSetReady() = %v, want %v", got, tc.want)
			}
		})
	}
}

func buildTestTemplate() (*appsv1.DaemonSet, map[string]string) {
	labels := map[string]string{"app": "myapp", "env": "test"}
	ds := createDaemonSetsTemplate(
		"my-ds", "my-ns", "my-container", "my-image:v1",
		labels, "100m", "200m", "64Mi", "128Mi", corev1.PullAlways,
	)
	return ds, labels
}

func TestCreateDaemonSetsTemplateMetadata(t *testing.T) {
	ds, labels := buildTestTemplate()

	if ds.Name != "my-ds" {
		t.Errorf("Name = %q, want %q", ds.Name, "my-ds")
	}
	if ds.Namespace != "my-ns" {
		t.Errorf("Namespace = %q, want %q", ds.Namespace, "my-ns")
	}
	if ds.Annotations["openshift.io/scc"] != "node-exporter" {
		t.Error("missing openshift.io/scc annotation")
	}

	matchLabels := ds.Spec.Selector.MatchLabels
	if matchLabels["name"] != "my-ds" {
		t.Error("matchLabels missing 'name' key")
	}
	for k, v := range labels {
		if matchLabels[k] != v {
			t.Errorf("matchLabels[%q] = %q, want %q", k, matchLabels[k], v)
		}
	}
}

func TestCreateDaemonSetsTemplatePodSpec(t *testing.T) {
	ds, _ := buildTestTemplate()
	podSpec := ds.Spec.Template.Spec

	if podSpec.ServiceAccountName != roleSaName {
		t.Errorf("ServiceAccountName = %q, want %q", podSpec.ServiceAccountName, roleSaName)
	}
	if !podSpec.HostNetwork {
		t.Error("HostNetwork should be true")
	}
	if !podSpec.HostIPC {
		t.Error("HostIPC should be true")
	}
	if !podSpec.HostPID {
		t.Error("HostPID should be true")
	}
	if len(podSpec.Tolerations) != 4 {
		t.Errorf("expected 4 tolerations, got %d", len(podSpec.Tolerations))
	}
	if len(podSpec.Volumes) != 1 || podSpec.Volumes[0].Name != hostVolumeName {
		t.Error("expected host volume")
	}
}

func TestCreateDaemonSetsTemplateContainer(t *testing.T) {
	ds, _ := buildTestTemplate()
	podSpec := ds.Spec.Template.Spec

	if len(podSpec.Containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(podSpec.Containers))
	}
	c := podSpec.Containers[0]

	if c.Name != "my-container" {
		t.Errorf("container name = %q, want %q", c.Name, "my-container")
	}
	if c.Image != "my-image:v1" {
		t.Errorf("container image = %q, want %q", c.Image, "my-image:v1")
	}
	if c.ImagePullPolicy != corev1.PullAlways {
		t.Errorf("pull policy = %v, want PullAlways", c.ImagePullPolicy)
	}
	if c.SecurityContext == nil || c.SecurityContext.Privileged == nil || !*c.SecurityContext.Privileged {
		t.Error("container should be privileged")
	}
	if c.SecurityContext.RunAsUser == nil || *c.SecurityContext.RunAsUser != 0 {
		t.Error("container should run as root (uid 0)")
	}
	if len(c.VolumeMounts) != 1 || c.VolumeMounts[0].MountPath != "/host" {
		t.Error("expected volume mount at /host")
	}
}

func TestCreateDaemonSetsTemplateResources(t *testing.T) {
	ds, _ := buildTestTemplate()
	c := ds.Spec.Template.Spec.Containers[0]

	cpuReq := c.Resources.Requests[corev1.ResourceCPU]
	if cpuReq.String() != "100m" {
		t.Errorf("CPU request = %q, want %q", cpuReq.String(), "100m")
	}
	cpuLim := c.Resources.Limits[corev1.ResourceCPU]
	if cpuLim.String() != "200m" {
		t.Errorf("CPU limit = %q, want %q", cpuLim.String(), "200m")
	}
	memReq := c.Resources.Requests[corev1.ResourceMemory]
	if memReq.String() != "64Mi" {
		t.Errorf("Memory request = %q, want %q", memReq.String(), "64Mi")
	}
	memLim := c.Resources.Limits[corev1.ResourceMemory]
	if memLim.String() != "128Mi" {
		t.Errorf("Memory limit = %q, want %q", memLim.String(), "128Mi")
	}
}

func TestCreateDaemonSetsTemplateResourcesWithoutLimits(t *testing.T) {
	// Create DaemonSet with empty limit strings
	ds := createDaemonSetsTemplate(
		"my-ds", "my-ns", "my-container", "my-image:v1",
		map[string]string{"app": "test"},
		"100m", "", "100M", "", // Empty CPU and memory limits
		corev1.PullAlways,
	)
	c := ds.Spec.Template.Spec.Containers[0]

	// Verify requests are set
	cpuReq := c.Resources.Requests[corev1.ResourceCPU]
	if cpuReq.String() != "100m" {
		t.Errorf("CPU request = %q, want %q", cpuReq.String(), "100m")
	}
	memReq := c.Resources.Requests[corev1.ResourceMemory]
	if memReq.String() != "100M" {
		t.Errorf("Memory request = %q, want %q", memReq.String(), "100M")
	}

	// Verify limits are NOT set
	if _, exists := c.Resources.Limits[corev1.ResourceCPU]; exists {
		t.Error("CPU limit should not be set when empty string is passed")
	}
	if _, exists := c.Resources.Limits[corev1.ResourceMemory]; exists {
		t.Error("Memory limit should not be set when empty string is passed")
	}
}

func TestCreateDaemonSetsTemplateNilLabels(t *testing.T) {
	ds := createDaemonSetsTemplate(
		"ds", "ns", "c", "img:v1",
		nil, "100m", "200m", "64Mi", "128Mi", corev1.PullIfNotPresent,
	)
	if ds.Spec.Selector.MatchLabels["name"] != "ds" {
		t.Error("matchLabels should contain 'name' even with nil input labels")
	}
}

func TestConfigurePrivilegedServiceAccount(t *testing.T) {
	tests := []struct {
		name      string
		failOn    string
		wantErr   bool
		errSubstr string
	}{
		{
			name: "success",
		},
		{
			name:      "role create fails",
			failOn:    "roles",
			wantErr:   true,
			errSubstr: "error creating role",
		},
		{
			name:      "rolebinding create fails",
			failOn:    "rolebindings",
			wantErr:   true,
			errSubstr: "error creating role bindings",
		},
		{
			name:      "service account create fails",
			failOn:    "serviceaccounts",
			wantErr:   true,
			errSubstr: "error creating service account",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cs := setupFakeClient()
			if tc.failOn != "" {
				cs.PrependReactor("create", tc.failOn, func(k8stesting.Action) (bool, runtime.Object, error) {
					return true, nil, fmt.Errorf("injected error")
				})
			}

			err := ConfigurePrivilegedServiceAccount(testNS)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tc.errSubstr) {
					t.Errorf("error %q should contain %q", err, tc.errSubstr)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestDeleteDaemonSet(t *testing.T) {
	t.Run("delete error", func(t *testing.T) {
		cs := setupFakeClient()
		cs.PrependReactor("delete", "daemonsets", func(k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, fmt.Errorf("injected delete error")
		})

		err := DeleteDaemonSet("ds", "ns")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "deletion failed") {
			t.Errorf("error %q should mention deletion failure", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		ds := newDaemonSet("ds", "ns", "img", time.Now(), readyStatus(1))
		setupFakeClient(ds)

		err := DeleteDaemonSet("ds", "ns")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestNamespaceCreate(t *testing.T) {
	tests := []struct {
		name    string
		reactor func(k8stesting.Action) (bool, runtime.Object, error)
		wantErr bool
	}{
		{
			name: "success",
		},
		{
			name: "already exists returns nil",
			reactor: func(k8stesting.Action) (bool, runtime.Object, error) {
				return true, nil, k8serrors.NewAlreadyExists(
					schema.GroupResource{Resource: "namespaces"}, testNS)
			},
		},
		{
			name: "other error",
			reactor: func(k8stesting.Action) (bool, runtime.Object, error) {
				return true, nil, fmt.Errorf("injected error")
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cs := setupFakeClient()
			if tc.reactor != nil {
				cs.PrependReactor("create", "namespaces", tc.reactor)
			}

			err := namespaceCreate(testNS)
			if tc.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestDeleteNamespaceIfPresent(t *testing.T) {
	t.Run("not present returns nil", func(t *testing.T) {
		setupFakeClient()
		err := DeleteNamespaceIfPresent("nonexistent")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("delete error", func(t *testing.T) {
		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNS}}
		cs := setupFakeClient(ns)
		cs.PrependReactor("delete", "namespaces", func(k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, fmt.Errorf("injected delete error")
		})

		err := DeleteNamespaceIfPresent(testNS)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "could not delete namespace") {
			t.Errorf("error %q should mention namespace deletion", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNS}}
		setupFakeClient(ns)

		err := DeleteNamespaceIfPresent(testNS)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestDoesDaemonSetExist(t *testing.T) {
	t.Run("exists", func(t *testing.T) {
		ds := newDaemonSet("ds", "ns", "img", time.Now(), readyStatus(1))
		setupFakeClient(ds)
		if !doesDaemonSetExist("ds", "ns") {
			t.Error("expected true for existing daemonset")
		}
	})

	t.Run("not found", func(t *testing.T) {
		setupFakeClient()
		if doesDaemonSetExist("ds", "ns") {
			t.Error("expected false for non-existing daemonset")
		}
	})
}

func TestNamespaceIsPresent(t *testing.T) {
	t.Run("present", func(t *testing.T) {
		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNS}}
		setupFakeClient(ns)
		if !namespaceIsPresent(testNS) {
			t.Error("expected true for existing namespace")
		}
	})

	t.Run("not found", func(t *testing.T) {
		setupFakeClient()
		if namespaceIsPresent(testNS) {
			t.Error("expected false for non-existing namespace")
		}
	})
}

func TestWaitDaemonsetReady(t *testing.T) {
	t.Run("node list error", func(t *testing.T) {
		cs := setupFakeClient()
		cs.PrependReactor("list", "nodes", func(k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, fmt.Errorf("injected node list error")
		})

		err := WaitDaemonsetReady("ns", "ds", time.Second)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "failed to get node list") {
			t.Errorf("error %q should mention node list", err)
		}
	})

	t.Run("ds get error", func(t *testing.T) {
		node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: testNodeName}}
		cs := setupFakeClient(node)
		cs.PrependReactor("get", "daemonsets", func(k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, fmt.Errorf("injected get error")
		})

		err := WaitDaemonsetReady("ns", "ds", time.Second)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "failed to get daemonset") {
			t.Errorf("error %q should mention daemonset get failure", err)
		}
	})

	t.Run("timeout", func(t *testing.T) {
		node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: testNodeName}}
		ds := newDaemonSet("ds", "ns", "img", time.Now(), appsv1.DaemonSetStatus{
			DesiredNumberScheduled: 1,
			CurrentNumberScheduled: 0,
		})
		setupFakeClient(node, ds)

		err := WaitDaemonsetReady("ns", "ds", 100*time.Millisecond)
		if err == nil {
			t.Fatal("expected timeout error, got nil")
		}
		if !strings.Contains(err.Error(), "timed out") {
			t.Errorf("error %q should mention timeout", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: testNodeName}}
		ds := newDaemonSet("ds", "ns", "img", time.Now(), readyStatus(1))
		setupFakeClient(node, ds)

		err := WaitDaemonsetReady("ns", "ds", 5*time.Second)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestSetDaemonSetClient(t *testing.T) {
	cs := fake.NewSimpleClientset()
	SetDaemonSetClient(cs)
	if daemonsetClient.K8sClient != cs {
		t.Error("SetDaemonSetClient did not set the client")
	}
}

