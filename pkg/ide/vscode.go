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

// OpenVSCode handles auto-installing code-server, launching it, port-forwarding, and opening desktop VS Code.
func OpenVSCode(ctx context.Context, client *k8s.Client, podName string, openInBrowser bool) error {
	ui.Info("Checking VS Code server in sandbox %s...", ui.TitleStyle.Render(podName))

	// 1. Check if code-server is installed
	checkCmd := []string{"sh", "-c", "command -v code-server || [ -x ~/.local/bin/code-server ]"}
	_, _, err := terminal.ExecSimple(ctx, client, podName, checkCmd)

	if err != nil {
		ui.Info("VS Code server not found. Installing standalone code-server inside container...")
		installScript := `
set -e
if command -v curl >/dev/null 2>&1; then
    curl -fsSL https://code-server.dev/install.sh | sh -s -- --method=standalone
elif command -v wget >/dev/null 2>&1; then
    wget -qO- https://code-server.dev/install.sh | sh -s -- --method=standalone
else
    echo "Error: neither curl nor wget found to download code-server" >&2
    exit 1
fi
`
		_, stderr, err := terminal.ExecSimple(ctx, client, podName, []string{"sh", "-c", installScript})
		if err != nil {
			return fmt.Errorf("failed to install code-server in container: %s: %w", stderr, err)
		}
		ui.Success("VS Code server installed successfully.")
	}

	// 2. Start code-server if not already running
	ui.Info("Starting VS Code server process...")
	startScript := `
export PATH="$HOME/.local/bin:$PATH"
if ! pgrep -f "code-server" >/dev/null 2>&1; then
    nohup code-server --auth none --bind-addr 0.0.0.0:13337 >/tmp/code-server.log 2>&1 &
    sleep 1
fi
`
	_, stderr, err := terminal.ExecSimple(ctx, client, podName, []string{"sh", "-c", startScript})
	if err != nil && !strings.Contains(stderr, "already running") {
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
		ui.Success("VS Code server tunnel established on port %d", localPort)
		if openInBrowser {
			ui.Info("Opening in default browser (%s)...", webURL)
			_ = openBrowser(webURL)
		} else {
			ui.Info("Launching desktop VS Code...")
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
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
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
