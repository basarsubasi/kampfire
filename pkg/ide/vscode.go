package ide

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/basarsubasi/kampfire/pkg/k8s"
	"github.com/basarsubasi/kampfire/pkg/terminal"
	"github.com/basarsubasi/kampfire/pkg/ui"
)

const remotePort = 13337

// OpenVSCode handles auto-installing code-server, launching it, port-forwarding, and opening desktop VS Code.
func OpenVSCode(ctx context.Context, client *k8s.Client, podName string, openInBrowser bool) error {
	ui.Info("Checking VS Code server in sandbox %s...", ui.TitleStyle.Render(podName))

	// 1. Check if code-server is installed
	checkCmd := []string{"sh", "-c", "command -v code-server || [ -x /usr/local/bin/code-server ] || [ -x ~/.local/bin/code-server ] || [ -x /root/.local/bin/code-server ]"}
	_, _, err := terminal.ExecSimple(ctx, client, podName, checkCmd)

	if err != nil {
		ui.Info("VS Code server not found. Installing standalone code-server inside container...")
		installScript := `
set -e
if [ -f /etc/alpine-release ]; then
    apk update >/dev/null 2>&1 || true
    apk add --no-cache nodejs curl wget gcompat libstdc++ libgcc procps iproute2 >/dev/null 2>&1 || true
fi
if command -v curl >/dev/null 2>&1; then
    curl -fsSL https://code-server.dev/install.sh | sh -s -- --method=standalone --prefix=/usr/local
elif command -v wget >/dev/null 2>&1; then
    wget -qO- https://code-server.dev/install.sh | sh -s -- --method=standalone --prefix=/usr/local
else
    echo "Error: neither curl nor wget found to download code-server" >&2
    exit 1
fi
if [ -f /etc/alpine-release ] && command -v node >/dev/null 2>&1; then
    SYS_NODE="$(command -v node)"
    for n in $(find /usr/local/lib /root/.local/lib ~/.local/lib /usr/lib -name node -path "*/code-server*/lib/node" 2>/dev/null); do
        ln -sf "$SYS_NODE" "$n"
    done
fi
`
		_, stderr, err := terminal.ExecSimple(ctx, client, podName, []string{"sh", "-c", installScript})
		if err != nil {
			return fmt.Errorf("failed to install code-server in container: %s: %w", stderr, err)
		}
		ui.Success("VS Code server installed successfully.")
	}

	// 2. Start code-server if not already running and wait for port 13337 readiness
	ui.Info("Starting VS Code server process...")
	startScript := `
export PATH="/usr/local/bin:$HOME/.local/bin:/root/.local/bin:$PATH"

# If on Alpine, install dependencies and replace bundled glibc node with Alpine's native musl node
if [ -f /etc/alpine-release ]; then
    apk update >/dev/null 2>&1 || true
    apk add --no-cache nodejs curl wget gcompat libstdc++ libgcc procps iproute2 >/dev/null 2>&1 || true
    if command -v node >/dev/null 2>&1; then
        SYS_NODE="$(command -v node)"
        for n in $(find /usr/local/lib /root/.local/lib ~/.local/lib /usr/lib -name node -path "*/code-server*/lib/node" 2>/dev/null); do
            ln -sf "$SYS_NODE" "$n"
        done
    fi
fi

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
    if grep -q "fcntl64" /tmp/code-server-check.log || [ -f /etc/alpine-release ]; then
        echo "" >&2
        echo "Hint: Alpine Linux uses musl libc, which lacks glibc symbols (like fcntl64) required by code-server." >&2
        echo "To use VS Code IDE with full compatibility, launch a glibc-based sandbox:" >&2
        echo "  kampfire run --image debian:bookworm-slim -d" >&2
        echo "  kampfire ide vscode <sandbox>" >&2
    fi
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
		vscodeURI := fmt.Sprintf("vscode://ms-vscode.remote-server/open?url=%s", webURL)
		ui.Success("VS Code server tunnel established on 0.0.0.0:%d", localPort)
		if openInBrowser {
			ui.Info("Opening in default browser (%s)...", webURL)
			ui.Info("VS Code URI:      %s", ui.TitleStyle.Render(vscodeURI))
			_ = openBrowser(webURL)
		} else {
			ui.Info("Launching desktop VS Code...")
			ui.Info("VS Code URI:      %s", ui.TitleStyle.Render(vscodeURI))
			ui.Info("Web fallback URL: %s", ui.TitleStyle.Render(webURL))
			_ = openDesktopVSCode(localPort)
		}
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

func openDesktopVSCode(localPort int) error {
	webURL := fmt.Sprintf("http://localhost:%d", localPort)
	vscodeURI := fmt.Sprintf("vscode://ms-vscode.remote-server/open?url=%s", webURL)

	// 1. If "code" CLI is in PATH, try launching directly
	if codePath, err := exec.LookPath("code"); err == nil {
		if err := exec.Command(codePath, "--open-url", vscodeURI).Start(); err == nil {
			return nil
		}
	}

	// 2. Open via OS protocol handler
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", vscodeURI}
	case "darwin":
		cmd = "open"
		args = []string{vscodeURI}
	default:
		cmd = "xdg-open"
		args = []string{vscodeURI}
	}

	return exec.Command(cmd, args...).Start()
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
