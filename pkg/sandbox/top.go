package sandbox

import (
	"context"
	"fmt"
	"strings"

	"github.com/basarsubasi/kampfire/pkg/k8s"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// MetricsGVR identifies the metrics.k8s.io PodMetrics API resource.
var MetricsGVR = schema.GroupVersionResource{
	Group:    "metrics.k8s.io",
	Version:  "v1beta1",
	Resource: "pods",
}

// PodMetric represents resource usage for a sandbox.
type PodMetric struct {
	ID     string
	Name   string
	CPU    string
	Memory string
	Status string
}

// GetMetrics fetches live CPU and memory metrics for sandboxes in the active namespace.
func GetMetrics(ctx context.Context, client *k8s.Client, target string) ([]PodMetric, error) {
	// 1. Fetch sandboxes in the namespace
	sandboxes, err := List(ctx, client, true)
	if err != nil {
		return nil, fmt.Errorf("failed to list sandboxes: %w", err)
	}

	if len(sandboxes) == 0 {
		return nil, nil
	}

	// Filter if target was specified (matches name or ID)
	var filtered []Info
	if target != "" {
		for _, sb := range sandboxes {
			if sb.Name == target || sb.ID == target || strings.HasPrefix(sb.ID, target) {
				filtered = append(filtered, sb)
				break
			}
		}
		if len(filtered) == 0 {
			return nil, fmt.Errorf("sandbox %q not found in namespace %s", target, client.Namespace)
		}
	} else {
		filtered = sandboxes
	}

	// 2. Query metrics.k8s.io/v1beta1 API for pod metrics in namespace
	metricMap := make(map[string]map[string]string)
	metricList, err := client.Dynamic.Resource(MetricsGVR).Namespace(client.Namespace).List(ctx, metav1.ListOptions{})
	if err == nil && metricList != nil {
		for _, item := range metricList.Items {
			podName := item.GetName()
			containers, found, _ := unstructured.NestedSlice(item.Object, "containers")
			if found && len(containers) > 0 {
				var cpuUsage, memUsage string
				for _, c := range containers {
					if cMap, ok := c.(map[string]interface{}); ok {
						cName, _, _ := unstructured.NestedString(cMap, "name")
						cpu, _, _ := unstructured.NestedString(cMap, "usage", "cpu")
						mem, _, _ := unstructured.NestedString(cMap, "usage", "memory")
						if cName == "main" || cpuUsage == "" {
							cpuUsage = cpu
							memUsage = mem
						}
					}
				}
				metricMap[podName] = map[string]string{
					"cpu":    cpuUsage,
					"memory": memUsage,
				}
			}
		}
	}

	var results []PodMetric
	for _, sb := range filtered {
		cpu := "<unknown>"
		mem := "<unknown>"
		if m, ok := metricMap[sb.Name]; ok {
			if m["cpu"] != "" {
				cpu = m["cpu"]
			}
			if m["memory"] != "" {
				mem = m["memory"]
			}
		}
		results = append(results, PodMetric{
			ID:     sb.ID,
			Name:   sb.Name,
			CPU:    cpu,
			Memory: mem,
			Status: sb.Status,
		})
	}

	return results, nil
}
