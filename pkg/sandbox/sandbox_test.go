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

	// 1. When command is nil, command should default to tail -f /dev/null keep-alive
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

	// Verify stdin and tty are omitted by default when not specified
	if _, ok := cMap["stdin"]; ok {
		t.Errorf("expected stdin to be omitted when not specified, got %v", cMap["stdin"])
	}
	if _, ok := cMap["tty"]; ok {
		t.Errorf("expected tty to be omitted when not specified, got %v", cMap["tty"])
	}

	// Verify default UTF-8 and terminal environment variables
	envs, found, err := unstructured.NestedSlice(cMap, "env")
	if !found || err != nil || len(envs) == 0 {
		t.Fatalf("expected env slice in container, got found=%v err=%v", found, err)
	}
	envMap := make(map[string]string)
	for _, e := range envs {
		if em, ok := e.(map[string]interface{}); ok {
			envMap[em["name"].(string)] = em["value"].(string)
		}
	}
	if envMap["LANG"] != "C.UTF-8" {
		t.Errorf("expected LANG=C.UTF-8, got %q", envMap["LANG"])
	}
	if envMap["LC_ALL"] != "C.UTF-8" {
		t.Errorf("expected LC_ALL=C.UTF-8, got %q", envMap["LC_ALL"])
	}
	if envMap["TERM"] != "xterm-256color" {
		t.Errorf("expected TERM=xterm-256color, got %q", envMap["TERM"])
	}

	// Verify labels on Sandbox metadata
	if createdBy, _, _ := unstructured.NestedString(obj.Object, "metadata", "labels", "agents.x-k8s.io/created-by"); createdBy != "kampfire" {
		t.Errorf("expected Sandbox metadata label agents.x-k8s.io/created-by=kampfire, got %s", createdBy)
	}

	// Verify labels on podTemplate metadata
	if createdBy, _, _ := unstructured.NestedString(obj.Object, "spec", "podTemplate", "metadata", "labels", "agents.x-k8s.io/created-by"); createdBy != "kampfire" {
		t.Errorf("expected podTemplate metadata label agents.x-k8s.io/created-by=kampfire, got %s", createdBy)
	}

	// 2. When command and stdin/tty are specified, they should be set
	optsCustom := CreateOptions{
		Name:    "custom-box",
		Image:   "alpine",
		Command: []string{"python3", "app.py"},
		Stdin:   true,
		TTY:     true,
	}
	infoCustom, err := CreateWithOptions(context.Background(), client, optsCustom)
	if err != nil {
		t.Fatalf("CreateWithOptions failed: %v", err)
	}
	objCustom, err := dynClient.Resource(SandboxGVR).Namespace("default").Get(context.Background(), infoCustom.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to retrieve custom Sandbox: %v", err)
	}
	customContainers, _, _ := unstructured.NestedSlice(objCustom.Object, "spec", "podTemplate", "spec", "containers")
	customCMap := customContainers[0].(map[string]interface{})
	customCmds, found, _ := unstructured.NestedSlice(customCMap, "command")
	if !found || len(customCmds) != 2 || customCmds[0] != "python3" || customCmds[1] != "app.py" {
		t.Errorf("expected ['python3', 'app.py'], got %v", customCmds)
	}
	if stdin, ok := customCMap["stdin"].(bool); !ok || !stdin {
		t.Errorf("expected stdin=true when specified, got %v", customCMap["stdin"])
	}
	if tty, ok := customCMap["tty"].(bool); !ok || !tty {
		t.Errorf("expected tty=true when specified, got %v", customCMap["tty"])
	}

	// 3. When NoKeepAlive is true and command is nil, command should be omitted from container spec
	optsNoKeepAlive := CreateOptions{
		Name:        "nokeepalive-box",
		Image:       "alpine",
		NoKeepAlive: true,
	}
	infoNoKeepAlive, err := CreateWithOptions(context.Background(), client, optsNoKeepAlive)
	if err != nil {
		t.Fatalf("CreateWithOptions failed: %v", err)
	}
	objNoKeepAlive, err := dynClient.Resource(SandboxGVR).Namespace("default").Get(context.Background(), infoNoKeepAlive.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to retrieve nokeepalive Sandbox: %v", err)
	}
	nkaContainers, _, _ := unstructured.NestedSlice(objNoKeepAlive.Object, "spec", "podTemplate", "spec", "containers")
	nkaCMap := nkaContainers[0].(map[string]interface{})
	if _, foundCmd := nkaCMap["command"]; foundCmd {
		t.Errorf("expected command to be omitted when NoKeepAlive is true, got: %v", nkaCMap["command"])
	}

	// 4. When NoKeepAlive is true and command/args are provided, command is omitted and args is populated
	optsNKAArgs := CreateOptions{
		Name:        "nka-args-box",
		Image:       "curlimages/curl",
		NoKeepAlive: true,
		Command:     []string{"https://example.com"},
	}
	infoNKAArgs, err := CreateWithOptions(context.Background(), client, optsNKAArgs)
	if err != nil {
		t.Fatalf("CreateWithOptions failed: %v", err)
	}
	objNKAArgs, err := dynClient.Resource(SandboxGVR).Namespace("default").Get(context.Background(), infoNKAArgs.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to retrieve nka-args Sandbox: %v", err)
	}
	nkaArgsContainers, _, _ := unstructured.NestedSlice(objNKAArgs.Object, "spec", "podTemplate", "spec", "containers")
	nkaArgsCMap := nkaArgsContainers[0].(map[string]interface{})
	if _, foundCmd := nkaArgsCMap["command"]; foundCmd {
		t.Errorf("expected command to be omitted when NoKeepAlive is true with args, got: %v", nkaArgsCMap["command"])
	}
	argsSlice, foundArgs, _ := unstructured.NestedSlice(nkaArgsCMap, "args")
	if !foundArgs || len(argsSlice) != 1 || argsSlice[0] != "https://example.com" {
		t.Errorf("expected args to be ['https://example.com'], got: %v", argsSlice)
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

func TestCreateWithOptions_CPU_Memory_Ports(t *testing.T) {
	scheme := runtime.NewScheme()
	dynClient := dynamicfake.NewSimpleDynamicClient(scheme)
	fakeK8s := k8sfake.NewSimpleClientset()

	client := &k8s.Client{
		Dynamic:   dynClient,
		Clientset: fakeK8s,
		Namespace: "default",
	}

	opts := CreateOptions{
		Name:           "custom-box",
		Image:          "alpine:latest",
		CPU:            "500m",
		Memory:         "256Mi",
		PublishedPorts: []string{"8080:80", "3000"},
	}

	info, err := CreateWithOptions(context.Background(), client, opts)
	if err != nil {
		t.Fatalf("CreateWithOptions() failed: %v", err)
	}
	if info.Name != "custom-box" {
		t.Errorf("expected name custom-box, got %s", info.Name)
	}

	// Verify the created resource has resources and ports
	obj, err := dynClient.Resource(SandboxGVR).Namespace("default").Get(context.Background(), "custom-box", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to retrieve created Sandbox: %v", err)
	}

	containers, found, _ := unstructured.NestedSlice(obj.Object, "spec", "podTemplate", "spec", "containers")
	if !found || len(containers) == 0 {
		t.Fatalf("expected containers slice in Sandbox spec")
	}

	cMap := containers[0].(map[string]interface{})

	// Check CPU and Memory
	cpuReq, _, _ := unstructured.NestedString(cMap, "resources", "requests", "cpu")
	if cpuReq != "500m" {
		t.Errorf("expected cpu request 500m, got %s", cpuReq)
	}
	memReq, _, _ := unstructured.NestedString(cMap, "resources", "requests", "memory")
	if memReq != "256Mi" {
		t.Errorf("expected memory request 256Mi, got %s", memReq)
	}

	// Check Ports
	ports, found, _ := unstructured.NestedSlice(cMap, "ports")
	if !found || len(ports) != 2 {
		t.Fatalf("expected 2 container ports, got %d", len(ports))
	}
	p0 := ports[0].(map[string]interface{})
	if p0["containerPort"] != int64(80) {
		t.Errorf("expected port 0 containerPort 80, got %v", p0["containerPort"])
	}
	p1 := ports[1].(map[string]interface{})
	if p1["containerPort"] != int64(3000) {
		t.Errorf("expected port 1 containerPort 3000, got %v", p1["containerPort"])
	}

	// Verify extractInfo parses ports into Info.Ports
	extracted := extractInfo(obj)
	if extracted.Ports != "80/TCP, 3000/TCP" {
		t.Errorf("expected extracted.Ports to be '80/TCP, 3000/TCP', got %q", extracted.Ports)
	}
}

func TestCreateWithOptions_Persistence(t *testing.T) {
	scheme := runtime.NewScheme()
	dynClient := dynamicfake.NewSimpleDynamicClient(scheme)
	fakeK8s := k8sfake.NewSimpleClientset()

	client := &k8s.Client{
		Dynamic:   dynClient,
		Clientset: fakeK8s,
		Namespace: "default",
	}

	opts := CreateOptions{
		Name:        "persist-box",
		Image:       "python:3.12",
		PersistPath: "/workspace",
		PersistSize: "10Gi",
	}

	info, err := CreateWithOptions(context.Background(), client, opts)
	if err != nil {
		t.Fatalf("CreateWithOptions() failed: %v", err)
	}
	if info.Name != "persist-box" {
		t.Errorf("expected name persist-box, got %s", info.Name)
	}

	obj, err := dynClient.Resource(SandboxGVR).Namespace("default").Get(context.Background(), "persist-box", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to retrieve created Sandbox: %v", err)
	}

	// Verify volumeClaimTemplates
	vcts, found, err := unstructured.NestedSlice(obj.Object, "spec", "volumeClaimTemplates")
	if !found || err != nil || len(vcts) == 0 {
		t.Fatalf("expected volumeClaimTemplates in spec, found=%v, err=%v", found, err)
	}

	vct := vcts[0].(map[string]interface{})
	vctName, _, _ := unstructured.NestedString(vct, "metadata", "name")
	if vctName != "workspace-storage" {
		t.Errorf("expected vct name workspace-storage, got %s", vctName)
	}

	storageReq, _, _ := unstructured.NestedString(vct, "spec", "resources", "requests", "storage")
	if storageReq != "10Gi" {
		t.Errorf("expected storage request 10Gi, got %s", storageReq)
	}

	// Verify volumeMounts in container
	containers, _, _ := unstructured.NestedSlice(obj.Object, "spec", "podTemplate", "spec", "containers")
	cMap := containers[0].(map[string]interface{})
	mounts, found, _ := unstructured.NestedSlice(cMap, "volumeMounts")
	if !found || len(mounts) == 0 {
		t.Fatalf("expected volumeMounts in container")
	}

	m0 := mounts[0].(map[string]interface{})
	if m0["name"] != "workspace-storage" {
		t.Errorf("expected mount name workspace-storage, got %v", m0["name"])
	}
	if m0["mountPath"] != "/workspace" {
		t.Errorf("expected mountPath /workspace, got %v", m0["mountPath"])
	}
}

func TestStopAndStart(t *testing.T) {
	scheme := runtime.NewScheme()
	dynClient := dynamicfake.NewSimpleDynamicClient(scheme)
	fakeK8s := k8sfake.NewSimpleClientset()

	client := &k8s.Client{
		Dynamic:   dynClient,
		Clientset: fakeK8s,
		Namespace: "default",
	}

	// 1. Create a sandbox
	opts := CreateOptions{
		Name:  "lifecycle-box",
		Image: "alpine",
	}
	_, err := CreateWithOptions(context.Background(), client, opts)
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	// 2. Stop the sandbox
	sb, err := Stop(context.Background(), client, "lifecycle-box")
	if err != nil {
		t.Fatalf("Stop() failed: %v", err)
	}
	if sb.Status != "Stopped" {
		t.Errorf("expected status Stopped, got %s", sb.Status)
	}

	// Verify operatingMode in resource is Suspended
	obj, err := dynClient.Resource(SandboxGVR).Namespace("default").Get(context.Background(), "lifecycle-box", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get sandbox: %v", err)
	}
	opMode, _, _ := unstructured.NestedString(obj.Object, "spec", "operatingMode")
	if opMode != "Suspended" {
		t.Errorf("expected operatingMode Suspended, got %s", opMode)
	}

	// Verify extractInfo returns Stopped
	extracted := extractInfo(obj)
	if extracted.Status != "Stopped" {
		t.Errorf("expected extracted.Status to be Stopped, got %s", extracted.Status)
	}

	// 3. Start the sandbox
	sbStart, err := Start(context.Background(), client, "lifecycle-box")
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	if sbStart.Status != "Starting" {
		t.Errorf("expected status Starting, got %s", sbStart.Status)
	}

	// Verify operatingMode in resource is Running
	obj, err = dynClient.Resource(SandboxGVR).Namespace("default").Get(context.Background(), "lifecycle-box", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get sandbox: %v", err)
	}
	opMode, _, _ = unstructured.NestedString(obj.Object, "spec", "operatingMode")
	if opMode != "Running" {
		t.Errorf("expected operatingMode Running, got %s", opMode)
	}
}

func TestCreateWithOptions_PullSecret(t *testing.T) {
	scheme := runtime.NewScheme()
	dynClient := dynamicfake.NewSimpleDynamicClient(scheme)
	fakeK8s := k8sfake.NewSimpleClientset()

	client := &k8s.Client{
		Dynamic:   dynClient,
		Clientset: fakeK8s,
		Namespace: "default",
	}

	opts := CreateOptions{
		Name:       "pullsecret-box",
		Image:      "ghcr.io/private/agent:latest",
		PullSecret: "ghcr-creds,internal-creds",
	}

	info, err := CreateWithOptions(context.Background(), client, opts)
	if err != nil {
		t.Fatalf("CreateWithOptions() failed: %v", err)
	}
	if info.Name != "pullsecret-box" {
		t.Errorf("expected name pullsecret-box, got %s", info.Name)
	}

	obj, err := dynClient.Resource(SandboxGVR).Namespace("default").Get(context.Background(), "pullsecret-box", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to retrieve created Sandbox: %v", err)
	}

	// Verify imagePullSecrets in podTemplate.spec
	pullSecrets, found, err := unstructured.NestedSlice(obj.Object, "spec", "podTemplate", "spec", "imagePullSecrets")
	if !found || err != nil || len(pullSecrets) != 2 {
		t.Fatalf("expected 2 imagePullSecrets, found=%v, len=%d, err=%v", found, len(pullSecrets), err)
	}

	s0 := pullSecrets[0].(map[string]interface{})
	if s0["name"] != "ghcr-creds" {
		t.Errorf("expected secret 0 name ghcr-creds, got %v", s0["name"])
	}

	s1 := pullSecrets[1].(map[string]interface{})
	if s1["name"] != "internal-creds" {
		t.Errorf("expected secret 1 name internal-creds, got %v", s1["name"])
	}
}

func TestCreateWithOptions_CustomEnv(t *testing.T) {
	scheme := runtime.NewScheme()
	dynClient := dynamicfake.NewSimpleDynamicClient(scheme)
	fakeK8s := k8sfake.NewSimpleClientset()

	client := &k8s.Client{
		Dynamic:   dynClient,
		Clientset: fakeK8s,
		Namespace: "default",
	}

	t.Setenv("MY_HOST_VAR", "host_secret_123")

	opts := CreateOptions{
		Name:  "env-test-box",
		Image: "alpine",
		Env: []string{
			"CUSTOM_KEY=custom_val",
			"LANG=en_US.UTF-8", // Override default LANG
			"MY_HOST_VAR",     // Inherit from host
		},
	}

	info, err := CreateWithOptions(context.Background(), client, opts)
	if err != nil {
		t.Fatalf("CreateWithOptions() failed: %v", err)
	}

	obj, err := dynClient.Resource(SandboxGVR).Namespace("default").Get(context.Background(), info.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to retrieve created Sandbox: %v", err)
	}

	containers, found, err := unstructured.NestedSlice(obj.Object, "spec", "podTemplate", "spec", "containers")
	if !found || err != nil || len(containers) == 0 {
		t.Fatalf("containers not found in spec: %v", err)
	}
	cMap := containers[0].(map[string]interface{})
	envs, found, _ := unstructured.NestedSlice(cMap, "env")
	if !found {
		t.Fatalf("expected env in container, got none")
	}

	envMap := make(map[string]string)
	for _, e := range envs {
		em := e.(map[string]interface{})
		envMap[em["name"].(string)] = em["value"].(string)
	}

	if envMap["CUSTOM_KEY"] != "custom_val" {
		t.Errorf("expected CUSTOM_KEY=custom_val, got %q", envMap["CUSTOM_KEY"])
	}
	if envMap["LANG"] != "en_US.UTF-8" {
		t.Errorf("expected LANG=en_US.UTF-8 (override), got %q", envMap["LANG"])
	}
	if envMap["LC_ALL"] != "C.UTF-8" {
		t.Errorf("expected default LC_ALL=C.UTF-8, got %q", envMap["LC_ALL"])
	}
	if envMap["TERM"] != "xterm-256color" {
		t.Errorf("expected default TERM=xterm-256color, got %q", envMap["TERM"])
	}
	if envMap["MY_HOST_VAR"] != "host_secret_123" {
		t.Errorf("expected MY_HOST_VAR=host_secret_123, got %q", envMap["MY_HOST_VAR"])
	}
}



