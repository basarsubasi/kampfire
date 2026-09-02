package e2e

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var campfireBin string

func TestMain(m *testing.M) {
	// Find project root
	root, err := filepath.Abs("../..")
	if err != nil {
		fmt.Printf("failed to determine project root: %v\n", err)
		os.Exit(1)
	}

	campfireBin = filepath.Join(root, "bin", "campfire")

	// Ensure campfire binary is compiled
	build := exec.Command("go", "build", "-o", campfireBin, root)
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Printf("failed to build campfire binary: %s\n", string(out))
		os.Exit(1)
	}

	os.Exit(m.Run())
}

// runCampfire executes the campfire binary with the provided arguments.
func runCampfire(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(campfireBin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	output := stdout.String() + stderr.String()
	return output, err
}

// setupNamespace creates a unique ephemeral namespace and registers cleanup.
func setupNamespace(t *testing.T) string {
	t.Helper()
	ns := fmt.Sprintf("e2e-%d", time.Now().UnixNano()%1000000+int64(rand.Intn(1000)))

	cmd := exec.Command("kubectl", "create", "namespace", ns)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to create test namespace %s: %s", ns, string(out))
	}

	t.Cleanup(func() {
		_ = exec.Command("kubectl", "delete", "namespace", ns).Run()
	})

	return ns
}

// TestE2E_RunAndExec tests sandbox creation and command execution.
func TestE2E_RunAndExec(t *testing.T) {
	ns := setupNamespace(t)
	boxName := "exec-box"

	// 1. Run sandbox in detached mode
	out, err := runCampfire(t, "-n", ns, "run", "--name", boxName, "--image", "alpine", "-d")
	if err != nil {
		t.Fatalf("campfire run failed: %s (err: %v)", out, err)
	}
	if !strings.Contains(out, boxName) {
		t.Errorf("expected output to mention %s, got: %s", boxName, out)
	}

	// 2. Exec command inside sandbox
	out, err = runCampfire(t, "-n", ns, "exec", boxName, "uname", "-a")
	if err != nil {
		t.Fatalf("campfire exec failed: %s (err: %v)", out, err)
	}
	if !strings.Contains(out, "Linux") {
		t.Errorf("expected 'Linux' in exec output, got: %s", out)
	}
}

// TestE2E_PSTableAndShortIDs tests ps listing, column alignment, and short ID resolution.
func TestE2E_PSTableAndShortIDs(t *testing.T) {
	ns := setupNamespace(t)
	boxName := "ps-box"

	// Create sandbox
	_, err := runCampfire(t, "-n", ns, "run", "--name", boxName, "--image", "alpine", "-d")
	if err != nil {
		t.Fatalf("campfire run failed: %v", err)
	}

	// 1. Check ps table output
	out, err := runCampfire(t, "-n", ns, "ps")
	if err != nil {
		t.Fatalf("campfire ps failed: %s", out)
	}
	for _, header := range []string{"SANDBOX ID", "NAME", "IMAGE", "STATUS", "AGE", "IP"} {
		if !strings.Contains(out, header) {
			t.Errorf("ps output missing expected column %q: %s", header, out)
		}
	}
	if !strings.Contains(out, boxName) || !strings.Contains(out, "alpine") {
		t.Errorf("ps output missing sandbox details: %s", out)
	}

	// 2. Check ps -q output returns 12-char short ID
	out, err = runCampfire(t, "-n", ns, "ps", "-q")
	if err != nil {
		t.Fatalf("campfire ps -q failed: %s", out)
	}
	shortID := strings.TrimSpace(out)
	if len(shortID) != 12 {
		t.Fatalf("expected 12-character short ID, got: %q", shortID)
	}

	// 3. Verify exec works using short ID
	out, err = runCampfire(t, "-n", ns, "exec", shortID, "echo", "verified-short-id")
	if err != nil || !strings.Contains(out, "verified-short-id") {
		t.Fatalf("campfire exec with short ID failed: %s (err: %v)", out, err)
	}
}

// TestE2E_Copy tests bidirectional file transfer between host and sandbox.
func TestE2E_Copy(t *testing.T) {
	ns := setupNamespace(t)
	boxName := "cp-box"

	_, err := runCampfire(t, "-n", ns, "run", "--name", boxName, "--image", "alpine", "-d")
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
	out, err := runCampfire(t, "-n", ns, "cp", localSrc, remotePath)
	if err != nil {
		t.Fatalf("campfire cp to sandbox failed: %s (err: %v)", out, err)
	}

	// 3. Verify file content inside container
	out, err = runCampfire(t, "-n", ns, "exec", boxName, "cat", "/tmp/remote-file.txt")
	if err != nil || !strings.Contains(out, testData) {
		t.Fatalf("content inside sandbox did not match: %s", out)
	}

	// 4. Copy sandbox -> local
	localDest := filepath.Join(t.TempDir(), "downloaded-file.txt")
	out, err = runCampfire(t, "-n", ns, "cp", remotePath, localDest)
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
	ns := setupNamespace(t)
	boxName := "rm-box"

	_, _ = runCampfire(t, "-n", ns, "run", "--name", boxName, "--image", "alpine", "-d")

	// Remove sandbox
	out, err := runCampfire(t, "-n", ns, "rm", boxName)
	if err != nil {
		t.Fatalf("campfire rm failed: %s (err: %v)", out, err)
	}
	if !strings.Contains(out, boxName) {
		t.Errorf("expected rm output to confirm deletion of %s, got: %s", boxName, out)
	}

	// Verify ps reflects empty namespace
	out, _ = runCampfire(t, "-n", ns, "ps")
	if !strings.Contains(out, "No sandboxes found") {
		t.Errorf("expected 'No sandboxes found', got: %s", out)
	}
}

// TestE2E_ResourceQuotaLimit tests that Kubernetes ResourceQuotas are enforced and formatted properly.
func TestE2E_ResourceQuotaLimit(t *testing.T) {
	ns := setupNamespace(t)

	// Apply a hard limit of 1 sandbox
	quotaManifest := fmt.Sprintf(`
apiVersion: v1
kind: ResourceQuota
metadata:
  name: test-quota
  namespace: %s
spec:
  hard:
    count/sandboxes.agents.x-k8s.io: "1"
`, ns)

	cmd := exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(quotaManifest)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to apply ResourceQuota: %s", string(out))
	}

	// 1st sandbox: should succeed
	out, err := runCampfire(t, "-n", ns, "run", "--name", "quota-box1", "--image", "alpine", "-d")
	if err != nil {
		t.Fatalf("first sandbox should succeed under quota: %s", out)
	}

	// 2nd sandbox: MUST fail due to quota
	out, err = runCampfire(t, "-n", ns, "run", "--name", "quota-box2", "--image", "alpine", "-d")
	if err == nil {
		t.Fatalf("expected second sandbox to fail quota, but succeeded: %s", out)
	}
	if !strings.Contains(out, "ResourceQuota exceeded") && !strings.Contains(out, "limit reached") {
		t.Errorf("expected clean quota error message, got: %s", out)
	}
}

// TestE2E_PortForward tests port forwarding from host to container.
func TestE2E_PortForward(t *testing.T) {
	ns := setupNamespace(t)
	boxName := "pf-box"

	// 1. Run sandbox
	_, err := runCampfire(t, "-n", ns, "run", "--name", boxName, "--image", "alpine", "-d")
	if err != nil {
		t.Fatalf("failed to run sandbox: %v", err)
	}

	// 2. Start a simple HTTP listener inside sandbox using busybox nc
	startServerScript := `nohup sh -c 'while true; do echo -e "HTTP/1.1 200 OK\r\nContent-Length: 14\r\n\r\nhello-campfire" | nc -l -p 8080; done' >/dev/null 2>&1 &`
	_, err = runCampfire(t, "-n", ns, "exec", boxName, "sh", "-c", startServerScript)
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
	pfCmd := exec.Command(campfireBin, "-n", ns, "port-forward", boxName, fmt.Sprintf("%d:8080", localPort))
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
