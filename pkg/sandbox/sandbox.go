package sandbox

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/basarsubasi/kampfire/pkg/k8s"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var SandboxGVR = schema.GroupVersionResource{
	Group:    "agents.x-k8s.io",
	Version:  "v1beta1",
	Resource: "sandboxes",
}

// Info holds parsed information about a Sandbox.
type Info struct {
	ID        string
	Name      string
	Image     string
	Status    string
	Age       string
	PodIP     string
	CreatedAt time.Time
}

// GenerateName creates a random human-friendly name for a sandbox.
func GenerateName(image string) string {
	cleanImage := image
	if idx := strings.LastIndex(cleanImage, "/"); idx != -1 {
		cleanImage = cleanImage[idx+1:]
	}
	if idx := strings.Index(cleanImage, ":"); idx != -1 {
		cleanImage = cleanImage[:idx]
	}
	cleanImage = strings.ToLower(cleanImage)
	if cleanImage == "" {
		cleanImage = "sandbox"
	}

	bytes := make([]byte, 2)
	_, _ = rand.Read(bytes)
	suffix := hex.EncodeToString(bytes)

	return fmt.Sprintf("sb-%s-%s", cleanImage, suffix)
}

// Create provisions a new Sandbox custom resource.
func Create(ctx context.Context, client *k8s.Client, name, image string, command []string) (*Info, error) {
	if name == "" {
		name = GenerateName(image)
	}

	if len(command) == 0 {
		// Keep container alive indefinitely
		command = []string{"sleep", "infinity"}
	}

	cmdSlice := make([]interface{}, len(command))
	for i, c := range command {
		cmdSlice[i] = c
	}

	labels := map[string]interface{}{
		"agents.x-k8s.io/created-by": "kampfire",
	}

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "agents.x-k8s.io/v1beta1",
			"kind":       "Sandbox",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": client.Namespace,
				"labels":    labels,
			},
			"spec": map[string]interface{}{
				"operatingMode":  "Running",
				"shutdownPolicy": "Retain",
				"podTemplate": map[string]interface{}{
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"name":    "main",
								"image":   image,
								"command": cmdSlice,
							},
						},
					},
				},
			},
		},
	}

	res, err := client.Dynamic.Resource(SandboxGVR).Namespace(client.Namespace).Create(ctx, obj, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create sandbox: %w", err)
	}

	uid := string(res.GetUID())
	shortID := strings.ReplaceAll(uid, "-", "")
	if len(shortID) > 12 {
		shortID = shortID[:12]
	}
	if shortID == "" {
		shortID = res.GetName()
	}

	return &Info{
		ID:        shortID,
		Name:      res.GetName(),
		Image:     image,
		Status:    "Starting",
		CreatedAt: res.GetCreationTimestamp().Time,
	}, nil
}

// WaitReady polls until the sandbox pod is Ready or the context is cancelled.
func WaitReady(ctx context.Context, client *k8s.Client, name string, onTick func(elapsed time.Duration)) (*Info, error) {
	start := time.Now()
	ticker := time.NewTicker(400 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			if onTick != nil {
				onTick(time.Since(start))
			}

			obj, err := client.Dynamic.Resource(SandboxGVR).Namespace(client.Namespace).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				continue
			}

			// Check conditions
			conditions, found, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
			if found {
				for _, c := range conditions {
					condMap, ok := c.(map[string]interface{})
					if !ok {
						continue
					}
					cType, _, _ := unstructured.NestedString(condMap, "type")
					cStatus, _, _ := unstructured.NestedString(condMap, "status")

					if cType == "Ready" && cStatus == "True" {
						info := extractInfo(obj)
						info.Status = "Running"
						return info, nil
					}
				}
			}
		}
	}
}

// List returns sandboxes in the current namespace.
func List(ctx context.Context, client *k8s.Client, showAll bool) ([]Info, error) {
	list, err := client.Dynamic.Resource(SandboxGVR).Namespace(client.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list sandboxes: %w", err)
	}

	var results []Info
	for _, item := range list.Items {
		info := extractInfo(&item)
		if !showAll && (info.Status == "Terminating" || info.Status == "Failed") {
			continue
		}
		results = append(results, *info)
	}

	return results, nil
}

// Delete removes a Sandbox by name.
func Delete(ctx context.Context, client *k8s.Client, name string) error {
	return client.Dynamic.Resource(SandboxGVR).Namespace(client.Namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// Find searches for a Sandbox by exact name, ID, or ID prefix.
func Find(ctx context.Context, client *k8s.Client, nameOrID string) (*Info, error) {
	// 1. Try exact name get first
	obj, err := client.Dynamic.Resource(SandboxGVR).Namespace(client.Namespace).Get(ctx, nameOrID, metav1.GetOptions{})
	if err == nil {
		return extractInfo(obj), nil
	}

	// 2. Search list for matching name, ID, or ID prefix
	sandboxes, err := List(ctx, client, true)
	if err != nil {
		return nil, fmt.Errorf("failed to search sandboxes: %w", err)
	}

	for _, sb := range sandboxes {
		if sb.Name == nameOrID || sb.ID == nameOrID || strings.HasPrefix(sb.ID, nameOrID) {
			return &sb, nil
		}
	}

	return nil, fmt.Errorf("sandbox '%s' not found in namespace %s", nameOrID, client.Namespace)
}

func extractInfo(obj *unstructured.Unstructured) *Info {
	name := obj.GetName()
	creationTime := obj.GetCreationTimestamp().Time
	age := formatAge(time.Since(creationTime))

	uid := string(obj.GetUID())
	shortID := strings.ReplaceAll(uid, "-", "")
	if len(shortID) > 12 {
		shortID = shortID[:12]
	}
	if shortID == "" {
		shortID = name
	}

	// Get image
	var image string
	containers, found, _ := unstructured.NestedSlice(obj.Object, "spec", "podTemplate", "spec", "containers")
	if found && len(containers) > 0 {
		if cMap, ok := containers[0].(map[string]interface{}); ok {
			image, _, _ = unstructured.NestedString(cMap, "image")
		}
	}

	// Status & IP
	status := "Starting"
	if obj.GetDeletionTimestamp() != nil {
		status = "Terminating"
	} else {
		conditions, found, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
		if found {
			for _, c := range conditions {
				if condMap, ok := c.(map[string]interface{}); ok {
					cType, _, _ := unstructured.NestedString(condMap, "type")
					cStatus, _, _ := unstructured.NestedString(condMap, "status")
					if cType == "Ready" && cStatus == "True" {
						status = "Running"
						break
					}
				}
			}
		}
	}

	var podIP string
	podIPs, found, _ := unstructured.NestedSlice(obj.Object, "status", "podIPs")
	if found && len(podIPs) > 0 {
		if ip, ok := podIPs[0].(string); ok {
			podIP = ip
		}
	}

	return &Info{
		ID:        shortID,
		Name:      name,
		Image:     image,
		Status:    status,
		Age:       age,
		PodIP:     podIP,
		CreatedAt: creationTime,
	}
}

func formatAge(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}
