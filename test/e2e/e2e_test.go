package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

var (
	kampfireBin string
	nsCounter   int64
)

const campfireUserClusterRole = `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: kampfire-user
rules:
  - apiGroups: ["agents.x-k8s.io"]
    resources: ["sandboxes"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: ["extensions.agents.x-k8s.io"]
    resources: ["sandboxclaims"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: [""]
    resources: ["pods", "events"]
    verbs: ["get", "list", "watch"]
  - apiGroups: [""]
    resources: ["pods/exec", "pods/portforward", "pods/log"]
    verbs: ["create", "get"]
`

func TestMain(m *testing.M) {
	// Find project root
	root, err := filepath.Abs("../..")
	if err != nil {
		fmt.Printf("failed to determine project root: %v\n", err)
		os.Exit(1)
	}

	kampfireBin = filepath.Join(root, "bin", "kampfire")

	// 1. Ensure campfire binary is compiled
	build := exec.Command("go", "build", "-o", kampfireBin, root)
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Printf("failed to build campfire binary: %s\n", string(out))
		os.Exit(1)
	}

	// 2. Ensure kampfire-user ClusterRole is applied to cluster
	applyRole := exec.Command("kubectl", "apply", "-f", "-")
	applyRole.Stdin = strings.NewReader(campfireUserClusterRole)
	if out, err := applyRole.CombinedOutput(); err != nil {
		fmt.Printf("warning: failed to apply kampfire-user ClusterRole: %s\n", string(out))
	}

	os.Exit(m.Run())
}

// runKampfireWithEnv executes the campfire CLI with optional environment variables.
func runKampfireWithEnv(t *testing.T, env []string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(kampfireBin, args...)
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	output := stdout.String() + stderr.String()
	return output, err
}

// runKampfire executes the campfire CLI with standard environment.
func runKampfire(t *testing.T, args ...string) (string, error) {
	return runKampfireWithEnv(t, nil, args...)
}

// setupNamespace creates a unique ephemeral namespace with atomic collision-free naming and registers cleanup.
func setupNamespace(t *testing.T) string {
	t.Helper()
	id := atomic.AddInt64(&nsCounter, 1)
	ns := fmt.Sprintf("e2e-%d-%d", time.Now().UnixNano()%1000000, id)

	cmd := exec.Command("kubectl", "create", "namespace", ns)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to create test namespace %s: %s", ns, string(out))
	}

	t.Cleanup(func() {
		_ = exec.Command("kubectl", "delete", "namespace", ns).Run()
	})

	return ns
}

// TestE2E_RBACAndTokenAuth verifies tenant ServiceAccount token minting, authorized operations,
// and strictly blocks unauthenticated (401) or unauthorized (403) cross-namespace actions.
func TestE2E_RBACAndTokenAuth(t *testing.T) {
	t.Parallel()
	nsAlice := setupNamespace(t)
	nsBob := setupNamespace(t)
	saAlice := "alice"
	boxAlice := "alice-box"
	boxBob := "bob-box"

	// 1. Setup Tenant Alice with kampfire-user ClusterRole
	cmd := exec.Command("kubectl", "create", "serviceaccount", saAlice, "-n", nsAlice)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to create Alice ServiceAccount: %s", string(out))
	}
	cmd = exec.Command("kubectl", "create", "rolebinding", saAlice+"-campfire",
		"--clusterrole=kampfire-user",
		fmt.Sprintf("--serviceaccount=%s:%s", nsAlice, saAlice),
		"-n", nsAlice)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to create Alice RoleBinding: %s", string(out))
	}
	cmd = exec.Command("kubectl", "create", "token", saAlice, "-n", nsAlice, "--duration=1h")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to mint Alice token: %s", string(out))
	}
	aliceToken := strings.TrimSpace(string(out))
	aliceEnv := []string{"KAMPFIRE_API_TOKEN=" + aliceToken}

	// 2. Setup a sandbox in Tenant Bob's namespace using admin credentials
	_, err = runKampfire(t, "-n", nsBob, "run", "--name", boxBob, "--image", "alpine", "-d")
	if err != nil {
		t.Fatalf("failed to setup Bob sandbox: %v", err)
	}

	// --- Scenario A: Authorized operations for Alice in nsAlice ---
	runOut, err := runKampfireWithEnv(t, aliceEnv, "-n", nsAlice, "run", "--name", boxAlice, "--image", "alpine", "-d")
	if err != nil {
		t.Fatalf("Alice run with token failed in her own namespace: %s (err: %v)", runOut, err)
	}

	execOut, err := runKampfireWithEnv(t, aliceEnv, "-n", nsAlice, "exec", boxAlice, "echo", "token-authenticated")
	if err != nil || !strings.Contains(execOut, "token-authenticated") {
		t.Fatalf("Alice exec with token failed: %s (err: %v)", execOut, err)
	}

	psOut, err := runKampfireWithEnv(t, aliceEnv, "-n", nsAlice, "ps")
	if err != nil || !strings.Contains(psOut, boxAlice) {
		t.Fatalf("Alice ps with token failed: %s (err: %v)", psOut, err)
	}

	// --- Scenario B: Unauthenticated operation with bogus token (401 Unauthorized) ---
	bogusEnv := []string{"KAMPFIRE_API_TOKEN=bogus-invalid-token-12345"}
	unauthOut, err := runKampfireWithEnv(t, bogusEnv, "-n", nsAlice, "ps")
	if err == nil {
		t.Fatalf("expected bogus token to fail with 401 Unauthorized, but succeeded: %s", unauthOut)
	}
	if !strings.Contains(unauthOut, "Unauthorized") && !strings.Contains(unauthOut, "401") && !strings.Contains(unauthOut, "invalid bearer token") {
		t.Errorf("expected 401 Unauthorized error for bogus token, got: %s", unauthOut)
	}

	// --- Scenario C: Unauthorized operation with unprivileged ServiceAccount (403 Forbidden) ---
	saUnpriv := "charlie-unpriv"
	_ = exec.Command("kubectl", "create", "serviceaccount", saUnpriv, "-n", nsAlice).Run()
	tokenCmd := exec.Command("kubectl", "create", "token", saUnpriv, "-n", nsAlice, "--duration=1h")
	tokenOut, _ := tokenCmd.CombinedOutput()
	unprivEnv := []string{"KAMPFIRE_API_TOKEN=" + strings.TrimSpace(string(tokenOut))}

	noRoleOut, err := runKampfireWithEnv(t, unprivEnv, "-n", nsAlice, "ps")
	if err == nil {
		t.Fatalf("expected unprivileged account to fail with 403 Forbidden, but succeeded: %s", noRoleOut)
	}
	if !strings.Contains(noRoleOut, "forbidden") && !strings.Contains(noRoleOut, "Forbidden") && !strings.Contains(noRoleOut, "403") {
		t.Errorf("expected 403 Forbidden for unprivileged account, got: %s", noRoleOut)
	}

	// --- Scenario D: Cross-Tenant Isolation (Alice trying to access or tamper with Bob's namespace) ---
	// 1. Alice tries to list Bob's sandboxes -> 403 Forbidden
	crossPsOut, err := runKampfireWithEnv(t, aliceEnv, "-n", nsBob, "ps")
	if err == nil {
		t.Fatalf("Alice should not be able to list Bob's sandboxes, but succeeded: %s", crossPsOut)
	}
	if !strings.Contains(crossPsOut, "forbidden") && !strings.Contains(crossPsOut, "Forbidden") && !strings.Contains(crossPsOut, "403") {
		t.Errorf("expected 403 Forbidden when Alice lists Bob's namespace, got: %s", crossPsOut)
	}

	// 2. Alice tries to exec into Bob's sandbox -> 403 Forbidden
	crossExecOut, err := runKampfireWithEnv(t, aliceEnv, "-n", nsBob, "exec", boxBob, "uname", "-a")
	if err == nil {
		t.Fatalf("Alice should not be able to exec into Bob's sandbox, but succeeded: %s", crossExecOut)
	}
	if !strings.Contains(crossExecOut, "forbidden") && !strings.Contains(crossExecOut, "Forbidden") && !strings.Contains(crossExecOut, "403") {
		t.Errorf("expected 403 Forbidden when Alice execs into Bob's sandbox, got: %s", crossExecOut)
	}

	// 3. Alice tries to delete Bob's sandbox -> 403 Forbidden
	crossRmOut, err := runKampfireWithEnv(t, aliceEnv, "-n", nsBob, "rm", boxBob)
	if err == nil {
		t.Fatalf("Alice should not be able to delete Bob's sandbox, but succeeded: %s", crossRmOut)
	}
	if !strings.Contains(crossRmOut, "forbidden") && !strings.Contains(crossRmOut, "Forbidden") && !strings.Contains(crossRmOut, "403") {
		t.Errorf("expected 403 Forbidden when Alice deletes Bob's sandbox, got: %s", crossRmOut)
	}

	// 4. Alice tries to port-forward Bob's sandbox -> 403 Forbidden
	crossPfOut, err := runKampfireWithEnv(t, aliceEnv, "-n", nsBob, "port-forward", boxBob, "9999:80")
	if err == nil {
		t.Fatalf("Alice should not be able to port-forward Bob's sandbox, but succeeded: %s", crossPfOut)
	}
	if !strings.Contains(crossPfOut, "forbidden") && !strings.Contains(crossPfOut, "Forbidden") && !strings.Contains(crossPfOut, "403") {
		t.Errorf("expected 403 Forbidden when Alice port-forwards Bob's sandbox, got: %s", crossPfOut)
	}

	// 5. Alice tries to read Bob's sandbox logs -> 403 Forbidden
	crossLogsOut, err := runKampfireWithEnv(t, aliceEnv, "-n", nsBob, "logs", boxBob)
	if err == nil {
		t.Fatalf("Alice should not be able to read Bob's sandbox logs, but succeeded: %s", crossLogsOut)
	}
	if !strings.Contains(crossLogsOut, "forbidden") && !strings.Contains(crossLogsOut, "Forbidden") && !strings.Contains(crossLogsOut, "403") {
		t.Errorf("expected 403 Forbidden when Alice reads Bob's sandbox logs, got: %s", crossLogsOut)
	}
}

// TestE2E_RunAndExec tests sandbox creation and command execution.
func TestE2E_RunAndExec(t *testing.T) {
	t.Parallel()
	ns := setupNamespace(t)
	boxName := "exec-box"

	// 1. Run sandbox in detached mode
	out, err := runKampfire(t, "-n", ns, "run", "--name", boxName, "--image", "alpine", "-d")
	if err != nil {
		t.Fatalf("campfire run failed: %s (err: %v)", out, err)
	}
	if !strings.Contains(out, boxName) {
		t.Errorf("expected output to mention %s, got: %s", boxName, out)
	}

	// 2. Exec command inside sandbox
	out, err = runKampfire(t, "-n", ns, "exec", boxName, "uname", "-a")
	if err != nil {
		t.Fatalf("campfire exec failed: %s (err: %v)", out, err)
	}
	if !strings.Contains(out, "Linux") {
		t.Errorf("expected 'Linux' in exec output, got: %s", out)
	}
}

// TestE2E_PSTableAndShortIDs tests ps listing, column alignment, and short ID resolution.
func TestE2E_PSTableAndShortIDs(t *testing.T) {
	t.Parallel()
	ns := setupNamespace(t)
	boxName := "ps-box"

	// Create sandbox
	_, err := runKampfire(t, "-n", ns, "run", "--name", boxName, "--image", "alpine", "-d")
	if err != nil {
		t.Fatalf("campfire run failed: %v", err)
	}

	// 1. Check ps table output
	out, err := runKampfire(t, "-n", ns, "ps")
	if err != nil {
		t.Fatalf("campfire ps failed: %s", out)
	}
	for _, header := range []string{"SANDBOX ID", "NAME", "IMAGE", "STATUS", "AGE", "IP", "PORTS"} {
		if !strings.Contains(out, header) {
			t.Errorf("ps output missing expected column %q: %s", header, out)
		}
	}
	if !strings.Contains(out, boxName) || !strings.Contains(out, "alpine") {
		t.Errorf("ps output missing sandbox details: %s", out)
	}

	// 2. Check ps -q output returns 12-char short ID
	out, err = runKampfire(t, "-n", ns, "ps", "-q")
	if err != nil {
		t.Fatalf("campfire ps -q failed: %s", out)
	}
	shortID := strings.TrimSpace(out)
	if len(shortID) != 12 {
		t.Fatalf("expected 12-character short ID, got: %q", shortID)
	}

	// 3. Verify exec works using short ID
	out, err = runKampfire(t, "-n", ns, "exec", shortID, "echo", "verified-short-id")
	if err != nil || !strings.Contains(out, "verified-short-id") {
		t.Fatalf("campfire exec with short ID failed: %s (err: %v)", out, err)
	}
}

// TestE2E_Copy tests bidirectional file transfer between host and sandbox.
func TestE2E_Copy(t *testing.T) {
	t.Parallel()
	ns := setupNamespace(t)
	boxName := "cp-box"

	_, err := runKampfire(t, "-n", ns, "run", "--name", boxName, "--image", "alpine", "-d")
	if err != nil {
		t.Fatalf("failed to provision sandbox: %v", err)
	}

	// 1. Create a local test file
	testData := "campfire-e2e-transfer-payload-12345"
	localSrc := filepath.Join(t.TempDir(), "local-file.txt")
	if err := os.WriteFile(localSrc, []byte(testData), 0644); err != nil {
		t.Fatalf("failed to write local file: %v", err)
	}

	// 2. Copy local -> sandbox
	remotePath := fmt.Sprintf("%s:/tmp/remote-file.txt", boxName)
	out, err := runKampfire(t, "-n", ns, "cp", localSrc, remotePath)
	if err != nil {
		t.Fatalf("campfire cp to sandbox failed: %s (err: %v)", out, err)
	}

	// 3. Verify file content inside container
	out, err = runKampfire(t, "-n", ns, "exec", boxName, "cat", "/tmp/remote-file.txt")
	if err != nil || !strings.Contains(out, testData) {
		t.Fatalf("content inside sandbox did not match: %s", out)
	}

	// 4. Copy sandbox -> local
	localDest := filepath.Join(t.TempDir(), "downloaded-file.txt")
	out, err = runKampfire(t, "-n", ns, "cp", remotePath, localDest)
	if err != nil {
		t.Fatalf("campfire cp from sandbox failed: %s (err: %v)", out, err)
	}

	readBack, err := os.ReadFile(localDest)
	if err != nil {
		t.Fatalf("failed to read downloaded file: %v", err)
	}
	if string(readBack) != testData {
		t.Errorf("content mismatch: expected %q, got %q", testData, string(readBack))
	}
}

// TestE2E_Remove tests sandbox deletion and confirmation in ps.
func TestE2E_Remove(t *testing.T) {
	t.Parallel()
	ns := setupNamespace(t)
	boxName := "rm-box"

	_, _ = runKampfire(t, "-n", ns, "run", "--name", boxName, "--image", "alpine", "-d")

	// Remove sandbox
	out, err := runKampfire(t, "-n", ns, "rm", boxName)
	if err != nil {
		t.Fatalf("campfire rm failed: %s (err: %v)", out, err)
	}
	if !strings.Contains(out, boxName) {
		t.Errorf("expected rm output to confirm deletion of %s, got: %s", boxName, out)
	}

	// Verify ps reflects empty namespace
	out, _ = runKampfire(t, "-n", ns, "ps")
	if !strings.Contains(out, "No sandboxes found") {
		t.Errorf("expected 'No sandboxes found', got: %s", out)
	}
}


// TestE2E_PortForward tests port forwarding from host to container.
func TestE2E_PortForward(t *testing.T) {
	t.Parallel()
	ns := setupNamespace(t)
	boxName := "pf-box"

	// 1. Run sandbox
	_, err := runKampfire(t, "-n", ns, "run", "--name", boxName, "--image", "alpine", "-d")
	if err != nil {
		t.Fatalf("failed to run sandbox: %v", err)
	}

	// 2. Start a simple HTTP listener inside sandbox using busybox nc
	startServerScript := `nohup sh -c 'while true; do echo -e "HTTP/1.1 200 OK\r\nContent-Length: 14\r\n\r\nhello-campfire" | nc -l -p 8080; done' >/dev/null 2>&1 &`
	_, err = runKampfire(t, "-n", ns, "exec", boxName, "sh", "-c", startServerScript)
	if err != nil {
		t.Fatalf("failed to start server in container: %v", err)
	}

	// 3. Find a free local port
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find free port: %v", err)
	}
	localPort := l.Addr().(*net.TCPAddr).Port
	l.Close()

	// 4. Start campfire port-forward in background
	pfCmd := exec.Command(kampfireBin, "-n", ns, "port-forward", boxName, fmt.Sprintf("%d:8080", localPort))
	var pfStdout, pfStderr bytes.Buffer
	pfCmd.Stdout = &pfStdout
	pfCmd.Stderr = &pfStderr

	if err := pfCmd.Start(); err != nil {
		t.Fatalf("failed to start port-forward command: %v", err)
	}
	t.Cleanup(func() {
		if pfCmd.Process != nil {
			_ = pfCmd.Process.Kill()
		}
	})

	// 5. Poll localPort with HTTP client
	targetURL := fmt.Sprintf("http://127.0.0.1:%d", localPort)
	client := &http.Client{Timeout: 2 * time.Second}
	var respBody string
	var success bool

	for i := 0; i < 20; i++ {
		time.Sleep(500 * time.Millisecond)
		resp, err := client.Get(targetURL)
		if err == nil {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if strings.Contains(string(body), "hello-campfire") {
				respBody = string(body)
				success = true
				break
			}
		}
	}

	if !success {
		t.Fatalf("port-forward request failed: did not receive expected response from %s (stdout: %s, stderr: %s)", targetURL, pfStdout.String(), pfStderr.String())
	}

	if !strings.Contains(respBody, "hello-campfire") {
		t.Errorf("expected response to contain 'hello-campfire', got: %s", respBody)
	}
}

// TestE2E_ConfigSetTokenAndKubeconfig tests configuring token and kubeconfig via campfire config set,
// and verifies that environment variables (KAMPFIRE_API_TOKEN, KUBECONFIG) take precedence.
func TestE2E_ConfigSetTokenAndKubeconfig(t *testing.T) {
	t.Parallel()
	ns := setupNamespace(t)
	sa := "config-tester"

	// 1. Setup ServiceAccount with kampfire-user ClusterRole
	cmd := exec.Command("kubectl", "create", "serviceaccount", sa, "-n", ns)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to create ServiceAccount: %s", string(out))
	}
	cmd = exec.Command("kubectl", "create", "rolebinding", sa+"-campfire",
		"--clusterrole=kampfire-user",
		fmt.Sprintf("--serviceaccount=%s:%s", ns, sa),
		"-n", ns)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to create RoleBinding: %s", string(out))
	}
	cmd = exec.Command("kubectl", "create", "token", sa, "-n", ns, "--duration=1h")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to mint token: %s", string(out))
	}
	validToken := strings.TrimSpace(string(out))

	tempDir := t.TempDir()
	tempCfg := filepath.Join(tempDir, "config.json")

	// 2. Set token via config set --token
	setOut, err := runKampfire(t, "config", "set", "--token", validToken, "--config", tempCfg)
	if err != nil {
		t.Fatalf("failed to set token via config: %s (err: %v)", setOut, err)
	}

	// 3. Verify campfire config shows token (from config)
	cfgOut, err := runKampfireWithEnv(t, []string{"KAMPFIRE_API_TOKEN="}, "config", "--config", tempCfg)
	if err != nil || !strings.Contains(cfgOut, "(from config)") {
		t.Fatalf("expected token (from config), got: %s", cfgOut)
	}

	// 4. Verify campfire operations succeed using the configured token
	psOut, err := runKampfireWithEnv(t, []string{"KAMPFIRE_API_TOKEN="}, "-n", ns, "--config", tempCfg, "ps")
	if err != nil {
		t.Fatalf("ps failed using configured token: %s (err: %v)", psOut, err)
	}

	// 5. Verify KAMPFIRE_API_TOKEN env var takes precedence over config
	bogusEnv := []string{"KAMPFIRE_API_TOKEN=bogus-invalid-token-12345"}
	unauthOut, err := runKampfireWithEnv(t, bogusEnv, "-n", ns, "--config", tempCfg, "ps")
	if err == nil {
		t.Fatalf("expected bogus KAMPFIRE_API_TOKEN to override config token and fail, but succeeded: %s", unauthOut)
	}
	if !strings.Contains(unauthOut, "Unauthorized") && !strings.Contains(unauthOut, "401") && !strings.Contains(unauthOut, "invalid bearer token") {
		t.Errorf("expected 401 error when env var overrides config, got: %s", unauthOut)
	}

	// 6. Test kubeconfig set and precedence
	currentKubeconfig := os.Getenv("KUBECONFIG")
	if currentKubeconfig == "" {
		home, _ := os.UserHomeDir()
		currentKubeconfig = filepath.Join(home, ".kube", "config")
	}

	// Set valid kubeconfig path in config
	setKubeOut, err := runKampfire(t, "config", "set", "--kubeconfig", currentKubeconfig, "--config", tempCfg)
	if err != nil {
		t.Fatalf("failed to set kubeconfig via config: %s (err: %v)", setKubeOut, err)
	}

	// Verify campfire config shows kubeconfig (from config) when KUBECONFIG env var is unset
	cfgKubeOut, err := runKampfireWithEnv(t, []string{"KUBECONFIG=", "KAMPFIRE_API_TOKEN="}, "config", "--config", tempCfg)
	if err != nil || !strings.Contains(cfgKubeOut, "(from config)") {
		t.Fatalf("expected kubeconfig (from config), got: %s", cfgKubeOut)
	}

	// Verify KUBECONFIG env var takes precedence when exported
	nonExistentKube := filepath.Join(tempDir, "non-existent-kubeconfig")
	envOverrideOut, err := runKampfireWithEnv(t, []string{"KUBECONFIG=" + nonExistentKube, "KAMPFIRE_API_TOKEN="}, "-n", ns, "--config", tempCfg, "ps")
	if err == nil {
		t.Fatalf("expected non-existent KUBECONFIG env var to override config and fail, but succeeded: %s", envOverrideOut)
	}

	// 7. Verify config.json does NOT hold server or namespace (retrieved dynamically from active context)
	data, err := os.ReadFile(tempCfg)
	if err != nil {
		t.Fatalf("failed to read config file: %v", err)
	}
	var rawMap map[string]interface{}
	if err := json.Unmarshal(data, &rawMap); err != nil {
		t.Fatalf("failed to parse json config: %v", err)
	}
	if _, exists := rawMap["server"]; exists {
		t.Errorf("expected server to NOT be stored in config.json, but found it")
	}
	if _, exists := rawMap["namespace"]; exists {
		t.Errorf("expected namespace to NOT be stored in config.json, but found it")
	}

	// 8. Verify campfire config displays active Server and Namespace dynamically from kubeconfig context
	viewOut, err := runKampfire(t, "config", "--config", tempCfg)
	if err != nil {
		t.Fatalf("failed to view config: %s (err: %v)", viewOut, err)
	}
	if !strings.Contains(viewOut, "API Server") || !strings.Contains(viewOut, "Namespace") {
		t.Errorf("expected campfire config to display API Server and Namespace from context, got:\n%s", viewOut)
	}
}

// TestE2E_RunWithPrivateSSHKeys verifies that campfire run --with-private-ssh-keys
// injects host SSH keys into the container with correct permissions.
func TestE2E_RunWithPrivateSSHKeys(t *testing.T) {
	t.Parallel()
	ns := setupNamespace(t)
	boxName := "ssh-box"

	out, err := runKampfire(t, "-n", ns, "run", "--name", boxName, "--image", "alpine", "--with-private-ssh-keys", "-d")
	if err != nil {
		t.Fatalf("failed to run with --with-private-ssh-keys: %s (err: %v)", out, err)
	}

	// Verify that ~/.ssh exists inside the container and files were copied
	lsOut, err := runKampfire(t, "-n", ns, "exec", boxName, "ls", "-la", "/root/.ssh")
	if err != nil {
		t.Fatalf("exec ls ~/.ssh failed: %s (err: %v)", lsOut, err)
	}

	if !strings.Contains(lsOut, "id_ed25519") && !strings.Contains(lsOut, "id_rsa") {
		t.Errorf("expected SSH keys to be present in /root/.ssh, got:\n%s", lsOut)
	}

	// Verify permissions on .ssh dir
	statOut, err := runKampfire(t, "-n", ns, "exec", boxName, "stat", "-c", "%a", "/root/.ssh")
	if err == nil {
		perm := strings.TrimSpace(statOut)
		if perm != "700" {
			t.Errorf("expected .ssh dir permission 700, got: %s", perm)
		}
	}
}

// TestE2E_RunCloneRepo verifies that campfire run --clone-repo attempts git clone in sandbox home.
func TestE2E_RunCloneRepo(t *testing.T) {
	t.Parallel()
	ns := setupNamespace(t)
	boxName := "git-box"

	// In plain alpine without git installed, campfire should report that git clone was attempted and failed
	out, err := runKampfire(t, "-n", ns, "run", "--name", boxName, "--image", "alpine", "--clone-repo", "https://github.com/example/repo.git", "-d")
	if err == nil {
		t.Fatalf("expected alpine without git to fail cloning, but succeeded: %s", out)
	}
	if !strings.Contains(out, "failed to clone repository") && !strings.Contains(out, "git") {
		t.Errorf("expected clone failure output, got: %s", out)
	}
}

// TestE2E_Logs verifies retrieving container logs with --tail, --head, and standard options.
func TestE2E_Logs(t *testing.T) {
	t.Parallel()
	ns := setupNamespace(t)
	boxName := "logs-box"

	// Launch a sandbox running a command that outputs 5 distinct lines and sleeps
	out, err := runKampfire(t, "-n", ns, "run", "--name", boxName, "--image", "alpine", "-d", "--", "sh", "-c", "echo line1; echo line2; echo line3; echo line4; echo line5; sleep 3600")
	if err != nil {
		t.Fatalf("failed to launch logs sandbox: %s (err: %v)", out, err)
	}

	// 1. Full logs
	allLogs, err := runKampfire(t, "-n", ns, "logs", boxName)
	if err != nil {
		t.Fatalf("failed to retrieve all logs: %s (err: %v)", allLogs, err)
	}
	for _, l := range []string{"line1", "line2", "line3", "line4", "line5"} {
		if !strings.Contains(allLogs, l) {
			t.Errorf("expected all logs to contain %q, got: %s", l, allLogs)
		}
	}

	// 2. Head logs (--head 2)
	headLogs, err := runKampfire(t, "-n", ns, "logs", "--head", "2", boxName)
	if err != nil {
		t.Fatalf("failed to retrieve head logs: %s (err: %v)", headLogs, err)
	}
	if !strings.Contains(headLogs, "line1") || !strings.Contains(headLogs, "line2") {
		t.Errorf("expected head logs to contain line1 and line2, got: %s", headLogs)
	}
	if strings.Contains(headLogs, "line3") || strings.Contains(headLogs, "line4") || strings.Contains(headLogs, "line5") {
		t.Errorf("head logs contained unexpected later lines: %s", headLogs)
	}

	// 3. Tail logs (--tail 2)
	tailLogs, err := runKampfire(t, "-n", ns, "logs", "--tail", "2", boxName)
	if err != nil {
		t.Fatalf("failed to retrieve tail logs: %s (err: %v)", tailLogs, err)
	}
	if !strings.Contains(tailLogs, "line4") || !strings.Contains(tailLogs, "line5") {
		t.Errorf("expected tail logs to contain line4 and line5, got: %s", tailLogs)
	}
	if strings.Contains(tailLogs, "line1") || strings.Contains(tailLogs, "line2") {
		t.Errorf("tail logs contained unexpected earlier lines: %s", tailLogs)
	}

	// 4. Timestamps flag (-t / --timestamps)
	tsLogs, err := runKampfire(t, "-n", ns, "logs", "-t", boxName)
	if err != nil {
		t.Fatalf("failed to retrieve logs with timestamps: %s (err: %v)", tsLogs, err)
	}
	tsRegex := regexp.MustCompile(`[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}`)
	if !tsRegex.MatchString(tsLogs) {
		t.Errorf("expected logs with -t to contain RFC3339 timestamps, got:\n%s", tsLogs)
	}
	if !strings.Contains(tsLogs, "line1") || !strings.Contains(tsLogs, "line5") {
		t.Errorf("expected logs with -t to contain log content, got:\n%s", tsLogs)
	}

	// 5. Follow flag (-f) combined with --head 2 (terminates promptly after head lines)
	headFollowLogs, err := runKampfire(t, "-n", ns, "logs", "-f", "--head", "2", boxName)
	if err != nil {
		t.Fatalf("failed to run follow with --head: %s (err: %v)", headFollowLogs, err)
	}
	if !strings.Contains(headFollowLogs, "line1") || !strings.Contains(headFollowLogs, "line2") {
		t.Errorf("expected follow with --head to contain line1 and line2, got: %s", headFollowLogs)
	}
	if strings.Contains(headFollowLogs, "line3") {
		t.Errorf("expected follow with --head to stop after 2 lines, got: %s", headFollowLogs)
	}

	// 6. Follow flag (-f) streaming dynamic log events
	streamBox := "stream-box"
	out, err = runKampfire(t, "-n", ns, "run", "--name", streamBox, "--image", "alpine", "-d", "--", "sh", "-c", "echo stream-start; sleep 1; echo stream-delayed; sleep 3600")
	if err != nil {
		t.Fatalf("failed to launch streaming sandbox: %s (err: %v)", out, err)
	}

	followCmd := exec.Command(kampfireBin, "-n", ns, "logs", "-f", streamBox)
	var followBuf bytes.Buffer
	followCmd.Stdout = &followBuf
	followCmd.Stderr = &followBuf

	if err := followCmd.Start(); err != nil {
		t.Fatalf("failed to start follow command: %v", err)
	}
	t.Cleanup(func() {
		if followCmd.Process != nil {
			_ = followCmd.Process.Kill()
		}
	})

	// Wait up to 10 seconds for the delayed log line to be streamed
	var receivedDelayed bool
	for i := 0; i < 20; i++ {
		time.Sleep(500 * time.Millisecond)
		if strings.Contains(followBuf.String(), "stream-delayed") {
			receivedDelayed = true
			break
		}
	}

	// Terminate follow command gracefully
	if followCmd.Process != nil {
		_ = followCmd.Process.Signal(os.Interrupt)
	}

	if !receivedDelayed {
		t.Fatalf("timed out waiting for stream-delayed log in follow stream (buffer: %s)", followBuf.String())
	}
	if !strings.Contains(followBuf.String(), "stream-start") {
		t.Errorf("expected follow stream to also contain stream-start, got: %s", followBuf.String())
	}
}

// TestE2E_RunKeepAliveAndFailureDetection verifies default keep-alive on Alpine (without sleep infinity)
// and ensures fast failure reporting on image pull errors or crashing commands.
func TestE2E_RunKeepAliveAndFailureDetection(t *testing.T) {
	t.Parallel()
	ns := setupNamespace(t)

	// 1. Launch detached Alpine sandbox without command: must not hang or crash on sleep infinity
	boxKeepAlive := "keepalive-box"
	out, err := runKampfire(t, "-n", ns, "run", "--name", boxKeepAlive, "--image", "alpine", "-d")
	if err != nil {
		t.Fatalf("expected detached alpine sandbox with default keep-alive to start successfully, got error: %s (err: %v)", out, err)
	}

	// Verify sandbox is listed as Running
	psOut, err := runKampfire(t, "-n", ns, "ps")
	if err != nil || !strings.Contains(psOut, boxKeepAlive) {
		t.Errorf("expected ps to list %s, got: %s", boxKeepAlive, psOut)
	}

	// 2. Launch with nonexistent image: must fail fast with image pull error rather than hanging
	boxBadImage := "bad-image-box"
	out, err = runKampfire(t, "-n", ns, "run", "--name", boxBadImage, "--image", "kampfire-fake-registry.invalid/no-such-image:99999", "-d")
	if err == nil {
		t.Fatalf("expected run with invalid image to fail, but succeeded: %s", out)
	}
	if !strings.Contains(out, "image pull failed") && !strings.Contains(out, "ErrImagePull") && !strings.Contains(out, "ImagePullBackOff") {
		t.Errorf("expected output to mention image pull failure, got:\n%s", out)
	}

	// 3. Launch with crashing command: must fail fast reporting premature termination rather than hanging
	boxCrash := "crash-box"
	out, err = runKampfire(t, "-n", ns, "run", "--name", boxCrash, "--image", "alpine", "-d", "--", "/bin/sh", "-c", "exit 42")
	if err == nil {
		t.Fatalf("expected run with crashing command to fail, but succeeded: %s", out)
	}
	if !strings.Contains(out, "terminated prematurely") && !strings.Contains(out, "42") && !strings.Contains(out, "CrashLoopBackOff") {
		t.Errorf("expected output to mention premature termination/exit code, got:\n%s", out)
	}
}

// TestE2E_IDE_VSCode tests the `kampfire ide vscode` workflow:
// verifying binary resolution, readiness detection, tunnel binding on 0.0.0.0,
// vscode:// protocol URI emission, and HTTP accessibility.
func TestE2E_IDE_VSCode(t *testing.T) {
	t.Parallel()
	ns := setupNamespace(t)
	boxName := "vscode-box"

	// 1. Run Alpine sandbox
	_, err := runKampfire(t, "-n", ns, "run", "--name", boxName, "--image", "alpine", "-d")
	if err != nil {
		t.Fatalf("failed to run sandbox: %v", err)
	}

	// 2. Set up simulated code-server in /usr/local/bin to avoid external 80MB download during CI
	mockScript := `cat << 'EOF' > /usr/local/bin/code-server
#!/bin/sh
if [ "$1" = "--version" ]; then
    echo "4.96.4 mock-version"
    exit 0
fi
while true; do
    echo -e "HTTP/1.1 200 OK\r\nContent-Type: text/html\r\nContent-Length: 22\r\n\r\n<h1>code-server</h1>" | nc -l -p 13337 2>/dev/null || sleep 1
done
EOF
chmod +x /usr/local/bin/code-server`
	_, err = runKampfire(t, "-n", ns, "exec", boxName, "sh", "-c", mockScript)
	if err != nil {
		t.Fatalf("failed to install mock code-server in container: %v", err)
	}

	// 3. Start kampfire ide vscode --browser in background
	ideCmd := exec.Command(kampfireBin, "-n", ns, "ide", "vscode", boxName, "--browser")
	var ideStdout, ideStderr bytes.Buffer
	ideCmd.Stdout = &ideStdout
	ideCmd.Stderr = &ideStderr

	if err := ideCmd.Start(); err != nil {
		t.Fatalf("failed to start ide vscode command: %v", err)
	}
	t.Cleanup(func() {
		if ideCmd.Process != nil {
			_ = ideCmd.Process.Kill()
		}
	})

	// 4. Wait for tunnel established message with port and vscode:// URI
	portRegex := regexp.MustCompile(`0\.0\.0\.0:([0-9]+)`)
	var localPort string
	var success bool

	for i := 0; i < 30; i++ {
		time.Sleep(500 * time.Millisecond)
		out := ideStdout.String()
		if match := portRegex.FindStringSubmatch(out); len(match) > 1 {
			localPort = match[1]
			success = true
			break
		}
	}

	if !success {
		t.Fatalf("timed out waiting for VS Code tunnel establishment. stdout:\n%s\nstderr:\n%s", ideStdout.String(), ideStderr.String())
	}

	// 5. Verify vscode:// URI was printed in terminal output
	out := ideStdout.String()
	expectedURI := fmt.Sprintf("vscode://ms-vscode.remote-server/open?url=http://localhost:%s", localPort)
	if !strings.Contains(out, expectedURI) {
		t.Errorf("expected stdout to contain VS Code URI %q, got:\n%s", expectedURI, out)
	}

	// 6. Verify HTTP accessibility on localPort
	targetURL := fmt.Sprintf("http://127.0.0.1:%s", localPort)
	client := &http.Client{Timeout: 2 * time.Second}
	var respBody string
	var httpSuccess bool

	for i := 0; i < 20; i++ {
		time.Sleep(500 * time.Millisecond)
		resp, err := client.Get(targetURL)
		if err == nil {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if strings.Contains(string(body), "code-server") {
				respBody = string(body)
				httpSuccess = true
				break
			}
		}
	}

	if !httpSuccess {
		t.Fatalf("HTTP request to %s failed (resp: %s, stderr: %s)", targetURL, respBody, ideStderr.String())
	}

	// 7. Terminate gracefully with SIGINT
	if ideCmd.Process != nil {
		_ = ideCmd.Process.Signal(os.Interrupt)
	}
}

// TestE2E_Run_ResourceLimits tests running a sandbox with --cpu and --memory flags.
func TestE2E_Run_ResourceLimits(t *testing.T) {
	t.Parallel()
	ns := setupNamespace(t)
	boxName := "res-box"

	out, err := runKampfire(t, "-n", ns, "run", "--name", boxName, "--image", "alpine", "--cpu", "100m", "--memory", "128Mi", "-d")
	if err != nil {
		t.Fatalf("failed to run sandbox with resource limits: %v (output: %s)", err, out)
	}

	// Verify sandbox is listed and running
	psOut, err := runKampfire(t, "-n", ns, "ps")
	if err != nil {
		t.Fatalf("failed to list sandboxes: %v (output: %s)", err, psOut)
	}
	if !strings.Contains(psOut, boxName) {
		t.Errorf("expected ps output to contain %s, got:\n%s", boxName, psOut)
	}
}

// TestE2E_Run_PublishPort tests running a sandbox with published port flag (-p).
func TestE2E_Run_PublishPort(t *testing.T) {
	t.Parallel()
	ns := setupNamespace(t)
	boxName := "publish-box"

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find free port: %v", err)
	}
	freePort := l.Addr().(*net.TCPAddr).Port
	l.Close()

	portFlag := fmt.Sprintf("%d:8080", freePort)
	out, err := runKampfire(t, "-n", ns, "run", "--name", boxName, "--image", "alpine", "-p", portFlag, "-d")
	if err != nil {
		t.Fatalf("failed to run sandbox with -p %s: %v (output: %s)", portFlag, err, out)
	}
	if !strings.Contains(out, fmt.Sprintf("127.0.0.1:%d -> 8080", freePort)) {
		t.Errorf("expected output to mention published port 127.0.0.1:%d -> 8080, got:\n%s", freePort, out)
	}

	// Verify that ps displays the published port in the PORTS column
	psOut, err := runKampfire(t, "-n", ns, "ps")
	if err != nil {
		t.Fatalf("kampfire ps failed: %v (output: %s)", err, psOut)
	}
	if !strings.Contains(psOut, "8080/TCP") {
		t.Errorf("expected kampfire ps to contain 8080/TCP, got:\n%s", psOut)
	}
}

// TestE2E_Top tests the `kampfire top` command.
func TestE2E_Top(t *testing.T) {
	t.Parallel()
	ns := setupNamespace(t)
	boxName := "top-test-box"

	// 1. Run a sandbox
	_, err := runKampfire(t, "-n", ns, "run", "--name", boxName, "--image", "alpine", "-d")
	if err != nil {
		t.Fatalf("failed to run sandbox: %v", err)
	}

	// 2. Run top for the namespace
	out, err := runKampfire(t, "-n", ns, "top")
	if err != nil {
		t.Fatalf("kampfire top failed: %v (output: %s)", err, out)
	}
	if !strings.Contains(out, "SANDBOX ID") || !strings.Contains(out, "CPU") || !strings.Contains(out, "MEMORY") {
		t.Errorf("expected top output to include headers SANDBOX ID, CPU, MEMORY, got:\n%s", out)
	}
	if !strings.Contains(out, boxName) {
		t.Errorf("expected top output to include sandbox %s, got:\n%s", boxName, out)
	}

	// 3. Run top targeting specific sandbox
	targetOut, err := runKampfire(t, "-n", ns, "top", boxName)
	if err != nil {
		t.Fatalf("kampfire top %s failed: %v (output: %s)", boxName, err, targetOut)
	}
	if !strings.Contains(targetOut, boxName) {
		t.Errorf("expected target top output to contain %s, got:\n%s", boxName, targetOut)
	}
}

// TestE2E_Completion tests the `kampfire completion` command and dynamic autocompletion.
func TestE2E_Completion(t *testing.T) {
	t.Parallel()
	ns := setupNamespace(t)
	boxName := "comp-test-box"

	// 1. Run a sandbox to populate autocompletions
	_, err := runKampfire(t, "-n", ns, "run", "--name", boxName, "--image", "alpine", "-d")
	if err != nil {
		t.Fatalf("failed to run sandbox: %v", err)
	}

	// 2. Test completion scripts generation
	for _, shell := range []string{"bash", "zsh", "fish"} {
		out, err := runKampfire(t, "completion", shell)
		if err != nil {
			t.Fatalf("kampfire completion %s failed: %v", shell, err)
		}
		if len(out) == 0 {
			t.Errorf("expected non-empty completion output for %s", shell)
		}
	}

	// 3. Test dynamic sandbox name completion via Cobra's __complete internal command
	compOut, err := runKampfire(t, "-n", ns, "__complete", "exec", "")
	if err != nil {
		t.Fatalf("dynamic completion failed: %v (output: %s)", err, compOut)
	}
	if !strings.Contains(compOut, boxName) {
		t.Errorf("expected dynamic completions to contain sandbox name %s, got:\n%s", boxName, compOut)
	}
}




