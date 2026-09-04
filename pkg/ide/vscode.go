package ide

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/basarsubasi/kampfire/pkg/k8s"
	"github.com/basarsubasi/kampfire/pkg/terminal"
	"github.com/basarsubasi/kampfire/pkg/ui"
)

const remotePort = 13337

var (
	checkScript = `
export PATH="/usr/local/bin:/usr/bin:$HOME/.local/bin:/root/.local/bin:$PATH"
if command -v code-server >/dev/null 2>&1; then
    code-server --version >/dev/null 2>&1
    exit $?
fi
if [ -x /usr/local/bin/code-server ]; then
    /usr/local/bin/code-server --version >/dev/null 2>&1
    exit $?
fi
if [ -x /usr/bin/code-server ]; then
    /usr/bin/code-server --version >/dev/null 2>&1
    exit $?
fi
exit 1
`

	installScript = `
set -e
export PATH="/usr/local/bin:/usr/bin:$HOME/.local/bin:/root/.local/bin:$PATH"
if [ -f /etc/alpine-release ]; then
    echo "Error: Alpine Linux uses musl libc and is not supported by code-server." >&2
    echo "Please use a glibc-based distribution like debian:bookworm-slim or ubuntu:latest." >&2
    exit 1
fi
if ! command -v curl >/dev/null 2>&1 && ! command -v wget >/dev/null 2>&1; then
    if command -v apt-get >/dev/null 2>&1; then
        apt-get update >/dev/null 2>&1 && apt-get install -y --no-install-recommends curl ca-certificates >/dev/null 2>&1 || true
    elif command -v dnf >/dev/null 2>&1; then
        dnf install -y curl ca-certificates >/dev/null 2>&1 || true
    elif command -v yum >/dev/null 2>&1; then
        yum install -y curl ca-certificates >/dev/null 2>&1 || true
    elif command -v pacman >/dev/null 2>&1; then
        pacman -S --noconfirm curl ca-certificates >/dev/null 2>&1 || true
    else
        echo "Error: neither curl nor wget found and package manager not installed" >&2
        exit 1
    fi
fi
if command -v curl >/dev/null 2>&1; then
    curl -fsSL https://code-server.dev/install.sh | sh -s -- --method=standalone --prefix=/usr/local
elif command -v wget >/dev/null 2>&1; then
    wget -qO- https://code-server.dev/install.sh | sh -s -- --method=standalone --prefix=/usr/local
else
    echo "Error: neither curl nor wget found to download code-server" >&2
    exit 1
fi
`

	startScript = `
export PATH="/usr/local/bin:/usr/bin:$HOME/.local/bin:/root/.local/bin:$PATH"

# Function to check if port 13337 is actively listening (prefers ss and /proc/net/tcp over nc)
is_port_listening() {
    ss -tlpn 2>/dev/null | grep -q ":13337" || \
    grep -q ":3419 " /proc/net/tcp /proc/net/tcp6 2>/dev/null || \
    netstat -tlpn 2>/dev/null | grep -q ":13337" || \
    nc -z 127.0.0.1 13337 2>/dev/null
}

# If already listening on port 13337, exit early
if is_port_listening; then
    exit 0
fi

# Locate the code-server binary
CODE_BIN=""
for p in "code-server" \
         "/usr/local/bin/code-server" \
         "/usr/bin/code-server" \
         "$HOME/.local/bin/code-server" \
         "/root/.local/bin/code-server"; do
    if command -v "$p" >/dev/null 2>&1 || [ -x "$p" ]; then
        CODE_BIN="$p"
        break
    fi
done

if [ -z "$CODE_BIN" ]; then
    CODE_BIN=$(find / -name "code-server" -type f -perm -111 2>/dev/null | grep bin/code-server | head -n 1)
fi

if [ -z "$CODE_BIN" ]; then
    echo "Error: code-server binary not found in container" >&2
    exit 1
fi

# Verify the binary is executable on this architecture / libc
if ! "$CODE_BIN" --version >/tmp/code-server-check.log 2>&1; then
    echo "Error: failed to execute $CODE_BIN:" >&2
    cat /tmp/code-server-check.log >&2
    exit 1
fi

# Launch daemon in background (do not use pgrep -f which matches this script's own command line)
"$CODE_BIN" --auth none --bind-addr 0.0.0.0:13337 >/tmp/code-server.log 2>&1 &
echo $! > /tmp/code-server.pid

# Wait up to 15 seconds for code-server to bind to port 13337
READY=0
for i in $(seq 1 30); do
    if is_port_listening; then
        READY=1
        break
    fi
    sleep 0.5
done

if [ "$READY" -ne 1 ]; then
    echo "code-server failed to bind to port 13337. Output log:" >&2
    cat /tmp/code-server.log >&2 2>/dev/null || echo "No /tmp/code-server.log found" >&2
    exit 1
fi
`
)

// BuildK8sContainerURI constructs the vscode-remote URI for attaching to a Kubernetes container.
func BuildK8sContainerURI(client *k8s.Client, podName, homeDir string) string {
	if homeDir == "" {
		homeDir = "/root"
	}
	if !strings.HasPrefix(homeDir, "/") {
		homeDir = "/" + homeDir
	}
	if client.Context != "" {
		return fmt.Sprintf("vscode-remote://k8s-container+context=%s+podname=%s+namespace=%s+name=main%s",
			client.Context, podName, client.Namespace, homeDir)
	}
	return fmt.Sprintf("vscode-remote://k8s-container+podname=%s+namespace=%s+name=main%s",
		podName, client.Namespace, homeDir)
}

// ResolveContainerHome inspects the container's environment to find its home directory.
func ResolveContainerHome(ctx context.Context, client *k8s.Client, podName string) string {
	homeCmd := []string{"sh", "-c", "echo -n \"${HOME:-/root}\""}
	if stdout, _, err := terminal.ExecSimple(ctx, client, podName, homeCmd); err == nil && strings.TrimSpace(stdout) != "" {
		return strings.TrimSpace(stdout)
	}
	return "/"
}

// OpenVSCode handles connecting desktop VS Code via kubectl exec, or browser mode via code-server.
func OpenVSCode(ctx context.Context, client *k8s.Client, podName string, openInBrowser bool) error {
	if openInBrowser {
		return OpenBrowserCodeServer(ctx, client, podName)
	}

	ui.Info("Connecting to sandbox %s via kubectl exec...", ui.TitleStyle.Render(podName))
	homeDir := ResolveContainerHome(ctx, client, podName)
	uri := BuildK8sContainerURI(client, podName, homeDir)

	ui.Info("Working Directory: %s", ui.TitleStyle.Render(homeDir))
	ui.Info("Connection URI:    %s", ui.TitleStyle.Render(uri))

	if err := OpenDesktopVSCode(uri); err != nil {
		ui.Error("Could not launch 'code' binary in PATH (%v).", err)
		ui.Info("You can connect manually:")
		ui.Info("  code --folder-uri %s", uri)
		return nil
	}
	ui.Success("Desktop VS Code launched.")
	return nil
}

// OpenAntigravity handles connecting Antigravity IDE via kubectl exec, or browser mode via code-server.
func OpenAntigravity(ctx context.Context, client *k8s.Client, podName string, openInBrowser bool) error {
	if openInBrowser {
		return OpenBrowserCodeServer(ctx, client, podName)
	}

	ui.Info("Connecting to sandbox %s via kubectl exec...", ui.TitleStyle.Render(podName))
	homeDir := ResolveContainerHome(ctx, client, podName)
	uri := BuildK8sContainerURI(client, podName, homeDir)

	ui.Info("Working Directory: %s", ui.TitleStyle.Render(homeDir))
	ui.Info("Connection URI:    %s", ui.TitleStyle.Render(uri))

	if err := OpenDesktopAntigravity(uri); err != nil {
		ui.Error("Could not launch 'antigravity-ide' binary in PATH (%v).", err)
		ui.Info("You can connect manually:")
		ui.Info("  antigravity-ide --folder-uri %s", uri)
		return nil
	}
	ui.Success("Antigravity IDE launched.")
	return nil
}

// OpenDesktopVSCode launches the desktop VS Code application with the given folder URI.
func OpenDesktopVSCode(uri string) error {
	// 1. If "code" CLI is in PATH, try launching directly
	if codePath, err := exec.LookPath("code"); err == nil {
		if err := exec.Command(codePath, "--folder-uri", uri).Start(); err == nil {
			return nil
		}
	}
	return fmt.Errorf("could not find or launch 'code'")
}

// OpenDesktopAntigravity launches the Antigravity IDE desktop application with the given folder URI.
func OpenDesktopAntigravity(uri string) error {
	// 1. Check if antigravity-ide is available in PATH
	if binPath, err := exec.LookPath("antigravity-ide"); err == nil {
		if err := exec.Command(binPath, "--folder-uri", uri).Start(); err == nil {
			return nil
		}
	}
	return fmt.Errorf("could not find or launch 'antigravity-ide'")
}

// OpenBrowserCodeServer starts code-server in the container, establishes port-forwarding, and opens the default browser.
func OpenBrowserCodeServer(ctx context.Context, client *k8s.Client, podName string) error {
	ui.Info("Checking VS Code server in sandbox %s...", ui.TitleStyle.Render(podName))

	// 1. Check if code-server is installed and runnable
	checkCmd := []string{"sh", "-c", checkScript}
	_, _, err := terminal.ExecSimple(ctx, client, podName, checkCmd)

	if err != nil {
		// Alpine Linux uses musl libc, which lacks glibc symbols (like fcntl64) required by code-server.
		if _, _, aErr := terminal.ExecSimple(ctx, client, podName, []string{"sh", "-c", "[ -f /etc/alpine-release ]"}); aErr == nil {
			return fmt.Errorf("code-server web IDE is not supported on Alpine Linux (musl libc).\nPlease switch to a glibc-based distribution instead (e.g. debian:bookworm-slim or ubuntu:latest):\n  kampfire run --image debian:bookworm-slim -d\n  kampfire ide vscode %s --browser", podName)
		}

		ui.Info("VS Code server not found. Installing standalone code-server inside container...")
		_, stderr, err := terminal.ExecSimple(ctx, client, podName, []string{"sh", "-c", installScript})
		if err != nil {
			return fmt.Errorf("failed to install code-server in container: %s: %w", stderr, err)
		}
		ui.Success("VS Code server installed successfully.")
	}

	// 2. Start code-server if not already running and wait for port 13337 readiness
	ui.Info("Starting VS Code server process...")
	_, stderr, err := terminal.ExecSimple(ctx, client, podName, []string{"sh", "-c", startScript})
	if err != nil {
		return fmt.Errorf("failed to start code-server: %s: %w", stderr, err)
	}

	// 3. Find a free local port
	localPort, err := getFreePort()
	if err != nil {
		localPort = remotePort
	}

	// 4. Start PortForward
	readyCh := make(chan struct{})
	stopCh := make(chan struct{})
	errCh := make(chan error, 1)

	go func() {
		if err := client.PortForward(ctx, podName, localPort, remotePort, readyCh, stopCh); err != nil {
			errCh <- err
		}
	}()

	select {
	case <-readyCh:
		webURL := fmt.Sprintf("http://localhost:%d", localPort)
		ui.Success("VS Code server tunnel established on 0.0.0.0:%d", localPort)
		ui.Info("Opening in default browser (%s)...", webURL)
		_ = openBrowser(webURL)
		ui.Info("Press Ctrl+C to disconnect the tunnel.")
	case err := <-errCh:
		return fmt.Errorf("port-forward failed: %w", err)
	case <-time.After(15 * time.Second):
		return fmt.Errorf("timed out waiting for port-forward tunnel")
	}

	// Wait for user exit (SIGINT/SIGTERM)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	select {
	case <-sigCh:
		ui.Info("\nDisconnecting IDE tunnel...")
		close(stopCh)
	case <-ctx.Done():
		close(stopCh)
	}

	return nil
}

func getFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "0.0.0.0:0")
	if err != nil {
		return 0, err
	}
	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}


func openBrowser(url string) error {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", url}
	case "darwin":
		cmd = "open"
		args = []string{url}
	default: // "linux", "freebsd", "openbsd", "netbsd"
		cmd = "xdg-open"
		args = []string{url}
	}
	return exec.Command(cmd, args...).Start()
}
