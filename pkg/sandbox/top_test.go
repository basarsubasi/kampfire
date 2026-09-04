package sandbox

import (
	"context"
	"testing"

	"github.com/basarsubasi/kampfire/pkg/k8s"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

func TestGetMetrics(t *testing.T) {
	scheme := runtime.NewScheme()
	gvrToListKind := map[schema.GroupVersionResource]string{
		SandboxGVR: "SandboxList",
		MetricsGVR: "PodMetricsList",
	}
	dynClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind)
	fakeK8s := k8sfake.NewSimpleClientset()

	client := &k8s.Client{
		Dynamic:   dynClient,
		Clientset: fakeK8s,
		Namespace: "default",
	}

	// 1. Create a Sandbox
	opts := CreateOptions{
		Name:  "top-box",
		Image: "alpine",
	}
	_, err := CreateWithOptions(context.Background(), client, opts)
	if err != nil {
		t.Fatalf("CreateWithOptions() failed: %v", err)
	}

	// 2. Mock PodMetrics object under metrics.k8s.io/v1beta1
	metricObj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "metrics.k8s.io/v1beta1",
			"kind":       "PodMetrics",
			"metadata": map[string]interface{}{
				"name":      "top-box",
				"namespace": "default",
			},
			"containers": []interface{}{
				map[string]interface{}{
					"name": "main",
					"usage": map[string]interface{}{
						"cpu":    "25m",
						"memory": "48Mi",
					},
				},
			},
		},
	}
	_, err = dynClient.Resource(MetricsGVR).Namespace("default").Create(context.Background(), metricObj, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create fake PodMetrics: %v", err)
	}

	// 3. Test GetMetrics for all sandboxes
	metrics, err := GetMetrics(context.Background(), client, "")
	if err != nil {
		t.Fatalf("GetMetrics failed: %v", err)
	}
	if len(metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(metrics))
	}
	if metrics[0].Name != "top-box" {
		t.Errorf("expected metric name 'top-box', got %s", metrics[0].Name)
	}
	if metrics[0].CPU != "25m" {
		t.Errorf("expected CPU 25m, got %s", metrics[0].CPU)
	}
	if metrics[0].Memory != "48Mi" {
		t.Errorf("expected Memory 48Mi, got %s", metrics[0].Memory)
	}

	// 4. Test GetMetrics with specific target
	targetMetrics, err := GetMetrics(context.Background(), client, "top-box")
	if err != nil {
		t.Fatalf("GetMetrics with target failed: %v", err)
	}
	if len(targetMetrics) != 1 {
		t.Fatalf("expected 1 target metric, got %d", len(targetMetrics))
	}

	// 5. Test GetMetrics with non-existent target
	_, err = GetMetrics(context.Background(), client, "non-existent")
	if err == nil {
		t.Errorf("expected error for non-existent target, got nil")
	}
}
