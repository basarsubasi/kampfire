package sandbox

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/basarsubasi/kampfire/pkg/k8s"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

func TestGenerateName(t *testing.T) {
	name := GenerateName("alpine")
	if !strings.HasPrefix(name, "sb-alpine-") {
		t.Errorf("expected name to start with sb-alpine-, got %s", name)
	}

	nameWithTag := GenerateName("python:3.12-slim")
	if !strings.HasPrefix(nameWithTag, "sb-python-") {
		t.Errorf("expected name to start with sb-python-, got %s", nameWithTag)
	}

	nameRegistry := GenerateName("ghcr.io/org/my-agent:latest")
	if !strings.HasPrefix(nameRegistry, "sb-my-agent-") {
		t.Errorf("expected name to start with sb-my-agent-, got %s", nameRegistry)
	}
}

func TestFormatAge(t *testing.T) {
	if got := formatAge(30 * time.Second); got != "30s" {
		t.Errorf("expected 30s, got %s", got)
	}
	if got := formatAge(5 * time.Minute); got != "5m" {
		t.Errorf("expected 5m, got %s", got)
	}
	if got := formatAge(3 * time.Hour); got != "3h" {
		t.Errorf("expected 3h, got %s", got)
	}
	if got := formatAge(48 * time.Hour); got != "2d" {
		t.Errorf("expected 2d, got %s", got)
	}
}

func TestCreateDefaultKeepAliveCommand(t *testing.T) {
	scheme := runtime.NewScheme()
	dynClient := dynamicfake.NewSimpleDynamicClient(scheme)
	fakeK8s := k8sfake.NewSimpleClientset()

	client := &k8s.Client{
		Dynamic:   dynClient,
		Clientset: fakeK8s,
		Namespace: "default",
	}

	info, err := Create(context.Background(), client, "test-box", "alpine", nil)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if info.Name != "test-box" {
		t.Errorf("expected name test-box, got %s", info.Name)
	}

	// Verify the created unstructured object has tail -f /dev/null and NOT sleep infinity
	obj, err := dynClient.Resource(SandboxGVR).Namespace("default").Get(context.Background(), "test-box", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to retrieve created Sandbox: %v", err)
	}

	containers, found, err := unstructured.NestedSlice(obj.Object, "spec", "podTemplate", "spec", "containers")
	if !found || err != nil || len(containers) == 0 {
		t.Fatalf("containers slice not found in spec: %v", err)
	}

	cMap := containers[0].(map[string]interface{})
	cmds, found, _ := unstructured.NestedSlice(cMap, "command")
	if !found || len(cmds) != 3 {
		t.Fatalf("expected 3 command arguments, got %v", cmds)
	}

	if cmds[0] != "tail" || cmds[1] != "-f" || cmds[2] != "/dev/null" {
		t.Errorf("expected ['tail', '-f', '/dev/null'], got %v", cmds)
	}
}

func TestWaitReadyCrashLoopBackOff(t *testing.T) {
	scheme := runtime.NewScheme()
	dynClient := dynamicfake.NewSimpleDynamicClient(scheme)
	fakeK8s := k8sfake.NewSimpleClientset()

	client := &k8s.Client{
		Dynamic:   dynClient,
		Clientset: fakeK8s,
		Namespace: "default",
	}

	// Create Pod with CrashLoopBackOff status
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "crashing-box",
			Namespace: "default",
		},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name: "main",
					State: corev1.ContainerState{
						Waiting: &corev1.ContainerStateWaiting{
							Reason:  "CrashLoopBackOff",
							Message: "back-off 10s restarting failed container",
						},
					},
					LastTerminationState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 1,
							Reason:   "Error",
						},
					},
				},
			},
		},
	}
	_, err := fakeK8s.CoreV1().Pods("default").Create(context.Background(), pod, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create fake pod: %v", err)
	}

	// WaitReady should fail fast and detect the CrashLoopBackOff immediately
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err = WaitReady(ctx, client, "crashing-box", nil)
	if err == nil {
		t.Fatal("expected WaitReady to return an error for crashing container, got nil")
	}
	if !strings.Contains(err.Error(), "CrashLoopBackOff") {
		t.Errorf("expected error to mention CrashLoopBackOff, got: %v", err)
	}
}

func TestWaitReadyImagePullFailure(t *testing.T) {
	scheme := runtime.NewScheme()
	dynClient := dynamicfake.NewSimpleDynamicClient(scheme)
	fakeK8s := k8sfake.NewSimpleClientset()

	client := &k8s.Client{
		Dynamic:   dynClient,
		Clientset: fakeK8s,
		Namespace: "default",
	}

	// Create Pod with ErrImagePull status
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bad-image-box",
			Namespace: "default",
		},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name: "main",
					State: corev1.ContainerState{
						Waiting: &corev1.ContainerStateWaiting{
							Reason:  "ErrImagePull",
							Message: "rpc error: code = NotFound desc = failed to pull image",
						},
					},
				},
			},
		},
	}
	_, err := fakeK8s.CoreV1().Pods("default").Create(context.Background(), pod, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create fake pod: %v", err)
	}

	// WaitReady should fail fast and report image pull failure
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err = WaitReady(ctx, client, "bad-image-box", nil)
	if err == nil {
		t.Fatal("expected WaitReady to return error for image pull failure, got nil")
	}
	if !strings.Contains(err.Error(), "image pull failed") {
		t.Errorf("expected error to mention image pull failed, got: %v", err)
	}
}

func TestWaitReadyStatusUpdates(t *testing.T) {
	scheme := runtime.NewScheme()
	dynClient := dynamicfake.NewSimpleDynamicClient(scheme)
	fakeK8s := k8sfake.NewSimpleClientset()

	client := &k8s.Client{
		Dynamic:   dynClient,
		Clientset: fakeK8s,
		Namespace: "default",
	}

	// Create an event for image pulling
	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pull-event-1",
			Namespace: "default",
		},
		InvolvedObject: corev1.ObjectReference{
			Name: "status-box",
		},
		Reason:  "Pulling",
		Message: "Pulling image \"python:3.12\"",
	}
	_, _ = fakeK8s.CoreV1().Events("default").Create(context.Background(), event, metav1.CreateOptions{})

	// Context with very short timeout to verify StatusUpdate callback was called
	ctx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
	defer cancel()

	var receivedStatus string
	_, _ = WaitReady(ctx, client, "status-box", func(u StatusUpdate) {
		receivedStatus = u.Status
	})

	if receivedStatus != "Pulling image \"python:3.12\"" {
		t.Errorf("expected status 'Pulling image \"python:3.12\"', got: %q", receivedStatus)
	}
}
