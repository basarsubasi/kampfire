package sandbox

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
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
	Ports     string
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

// CreateOptions specifies optional configuration when creating a sandbox.
type CreateOptions struct {
	Name           string
	Image          string
	Command        []string
	CPU            string
	Memory         string
	PublishedPorts []string
}

// CreateWithOptions provisions a new Sandbox custom resource with granular options (CPU, Memory, Ports).
func CreateWithOptions(ctx context.Context, client *k8s.Client, opts CreateOptions) (*Info, error) {
	name := opts.Name
	if name == "" {
		name = GenerateName(opts.Image)
	}

	command := opts.Command
	if len(command) == 0 {
		// Keep container alive indefinitely across all Linux distributions (Alpine, Debian, Ubuntu)
		command = []string{"tail", "-f", "/dev/null"}
	}

	cmdSlice := make([]interface{}, len(command))
	for i, c := range command {
		cmdSlice[i] = c
	}

	containerObj := map[string]interface{}{
		"name":    "main",
		"image":   opts.Image,
		"command": cmdSlice,
	}

	// Resources: CPU and Memory
	if opts.CPU != "" || opts.Memory != "" {
		resources := map[string]interface{}{}
		req := map[string]interface{}{}
		lim := map[string]interface{}{}
		if opts.CPU != "" {
			req["cpu"] = opts.CPU
			lim["cpu"] = opts.CPU
		}
		if opts.Memory != "" {
			req["memory"] = opts.Memory
			lim["memory"] = opts.Memory
		}
		if len(req) > 0 {
			resources["requests"] = req
		}
		if len(lim) > 0 {
			resources["limits"] = lim
		}
		containerObj["resources"] = resources
	}

	// Ports: containerPort definitions from PublishedPorts
	if len(opts.PublishedPorts) > 0 {
		var ports []interface{}
		for _, p := range opts.PublishedPorts {
			parts := strings.Split(p, ":")
			remotePortStr := parts[0]
			if len(parts) == 2 {
				remotePortStr = parts[1]
			}
			if portNum, err := strconv.Atoi(remotePortStr); err == nil && portNum > 0 {
				ports = append(ports, map[string]interface{}{
					"containerPort": int64(portNum),
				})
			}
		}
		if len(ports) > 0 {
			containerObj["ports"] = ports
		}
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
							containerObj,
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
		Image:     opts.Image,
		Status:    "Starting",
		Ports:     strings.Join(opts.PublishedPorts, ", "),
		CreatedAt: res.GetCreationTimestamp().Time,
	}, nil
}

// Create provisions a new Sandbox custom resource with default options.
func Create(ctx context.Context, client *k8s.Client, name, image string, command []string) (*Info, error) {
	return CreateWithOptions(ctx, client, CreateOptions{
		Name:    name,
		Image:   image,
		Command: command,
	})
}

// StatusUpdate represents real-time sandbox status and elapsed time during startup.
type StatusUpdate struct {
	Status  string
	Elapsed time.Duration
}

// WaitReady polls until the sandbox pod is Ready, encounters a fatal container crash, or the timeout expires.
func WaitReady(ctx context.Context, client *k8s.Client, name string, onStatus func(StatusUpdate)) (*Info, error) {
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 120*time.Second)
		defer cancel()
	}

	start := time.Now()
	ticker := time.NewTicker(400 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			statusText := "Waiting for container to start..."

			// 1. Check Kubernetes events for image pull progress
			if events, err := client.Clientset.CoreV1().Events(client.Namespace).List(ctx, metav1.ListOptions{
				FieldSelector: "involvedObject.name=" + name,
			}); err == nil && len(events.Items) > 0 {
				for i := len(events.Items) - 1; i >= 0; i-- {
					ev := events.Items[i]
					switch ev.Reason {
					case "Pulling":
						statusText = ev.Message
					case "Pulled":
						statusText = "Image pulled, starting container..."
					case "Created":
						statusText = "Container created..."
					case "Started":
						statusText = "Container started..."
					case "Failed":
						if strings.Contains(ev.Message, "pull") || strings.Contains(ev.Message, "image") {
							return nil, fmt.Errorf("image pull failed: %s", ev.Message)
						}
					}
					if statusText != "Waiting for container to start..." {
						break
					}
				}
			}

			// 2. Check underlying Pod status for container crash loops or premature exits
			if pod, err := client.Clientset.CoreV1().Pods(client.Namespace).Get(ctx, name, metav1.GetOptions{}); err == nil {
				for _, cs := range pod.Status.ContainerStatuses {
					if cs.State.Waiting != nil {
						reason := cs.State.Waiting.Reason
						if reason == "ImagePullBackOff" || reason == "ErrImagePull" {
							detail := cs.State.Waiting.Message
							if detail == "" {
								detail = "image not found or pull access denied"
							}
							return nil, fmt.Errorf("image pull failed: %s (%s)", reason, detail)
						}
						if reason == "CrashLoopBackOff" {
							detail := cs.State.Waiting.Message
							if detail == "" && cs.LastTerminationState.Terminated != nil {
								detail = fmt.Sprintf("exit code %d: %s", cs.LastTerminationState.Terminated.ExitCode, cs.LastTerminationState.Terminated.Reason)
							}
							return nil, fmt.Errorf("container failed to start: %s (%s)", reason, detail)
						}
						if reason == "ContainerCreating" && statusText == "Waiting for container to start..." {
							statusText = "Container creating..."
						}
					}
					if cs.State.Terminated != nil && cs.State.Terminated.ExitCode != 0 {
						return nil, fmt.Errorf("container terminated prematurely with exit code %d: %s", cs.State.Terminated.ExitCode, cs.State.Terminated.Reason)
					}
				}
			}

			if onStatus != nil {
				onStatus(StatusUpdate{
					Status:  statusText,
					Elapsed: time.Since(start),
				})
			}

			// 3. Check Sandbox custom resource conditions
			obj, err := client.Dynamic.Resource(SandboxGVR).Namespace(client.Namespace).Get(ctx, name, metav1.GetOptions{})
			if err == nil {
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

	// Get image and ports
	var image string
	var portsList []string
	containers, found, _ := unstructured.NestedSlice(obj.Object, "spec", "podTemplate", "spec", "containers")
	if found && len(containers) > 0 {
		if cMap, ok := containers[0].(map[string]interface{}); ok {
			image, _, _ = unstructured.NestedString(cMap, "image")
			if ports, found, _ := unstructured.NestedSlice(cMap, "ports"); found {
				for _, p := range ports {
					if pMap, ok := p.(map[string]interface{}); ok {
						if cPort, ok := pMap["containerPort"].(int64); ok {
							portsList = append(portsList, fmt.Sprintf("%d/TCP", cPort))
						} else if cPortFloat, ok := pMap["containerPort"].(float64); ok {
							portsList = append(portsList, fmt.Sprintf("%d/TCP", int64(cPortFloat)))
						}
					}
				}
			}
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
		Ports:     strings.Join(portsList, ", "),
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
