package ide

import (
	"fmt"
	"net"
	"strings"
	"testing"

	"github.com/basarsubasi/kampfire/pkg/k8s"
)

func TestGetFreePort(t *testing.T) {
	port, err := getFreePort()
	if err != nil {
		t.Fatalf("getFreePort() failed: %v", err)
	}

	if port <= 0 || port > 65535 {
		t.Fatalf("getFreePort() returned invalid port: %d", port)
	}

	// Verify that the port can be bound on 0.0.0.0
	addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		t.Fatalf("failed to resolve address on 0.0.0.0:%d: %v", port, err)
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		t.Fatalf("failed to listen on 0.0.0.0:%d: %v", port, err)
	}
	defer l.Close()
}

func TestScripts_AlpineGlibcNotice(t *testing.T) {
	// Must branch on /etc/alpine-release
	if !strings.Contains(installScript, "/etc/alpine-release") {
		t.Errorf("installScript must check for /etc/alpine-release")
	}

	// Must notify user about musl libc and prompt switching to glibc distros
	if !strings.Contains(installScript, "debian:bookworm-slim") {
		t.Errorf("installScript should recommend a glibc-based image like debian:bookworm-slim")
	}
}

func TestScripts_GlibcStandaloneInstallation(t *testing.T) {
	// Must retain the official standalone installer for non-Alpine (glibc) distros
	if !strings.Contains(installScript, "https://code-server.dev/install.sh") {
		t.Errorf("installScript must include official code-server.dev installer for non-Alpine systems")
	}
	if !strings.Contains(installScript, "--method=standalone") {
		t.Errorf("installScript must specify --method=standalone for glibc distributions")
	}
}

func TestScripts_StartScriptPortListening(t *testing.T) {
	// Must check port 13337 in hex (:3419) in /proc/net/tcp
	if !strings.Contains(startScript, ":3419 ") {
		t.Errorf("startScript must check hex 3419 (13337) in /proc/net/tcp")
	}

	// Must check ss command
	if !strings.Contains(startScript, "ss -tlpn") {
		t.Errorf("startScript should check ss -tlpn")
	}

	// Must look for code-server in /usr/bin as well as /usr/local/bin
	if !strings.Contains(startScript, "/usr/bin/code-server") {
		t.Errorf("startScript must search /usr/bin/code-server (npm default)")
	}
}

func TestScripts_CheckScript(t *testing.T) {
	// Check script must verify that code-server is actually runnable with --version
	if !strings.Contains(checkScript, "code-server --version") {
		t.Errorf("checkScript should verify code-server --version to detect broken binaries")
	}
}

func TestBuildK8sContainerURI(t *testing.T) {
	tests := []struct {
		name     string
		context  string
		podName  string
		ns       string
		homeDir  string
		expected string
	}{
		{
			name:     "with context and root home",
			context:  "kind-cluster",
			podName:  "sb-alpine-1234",
			ns:       "default",
			homeDir:  "/root",
			expected: "vscode-remote://k8s-container+context=kind-cluster+podname=sb-alpine-1234+namespace=default+name=main/root",
		},
		{
			name:     "empty context defaults gracefully",
			context:  "",
			podName:  "sb-debian-5678",
			ns:       "user-tenant",
			homeDir:  "/root",
			expected: "vscode-remote://k8s-container+podname=sb-debian-5678+namespace=user-tenant+name=main/root",
		},
		{
			name:     "custom user home directory",
			context:  "prod-cluster",
			podName:  "sb-node-9999",
			ns:       "my-ns",
			homeDir:  "/home/node",
			expected: "vscode-remote://k8s-container+context=prod-cluster+podname=sb-node-9999+namespace=my-ns+name=main/home/node",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &k8s.Client{
				Context:   tt.context,
				Namespace: tt.ns,
			}
			uri := BuildK8sContainerURI(client, tt.podName, tt.homeDir)
			if uri != tt.expected {
				t.Errorf("BuildK8sContainerURI() = %q, want %q", uri, tt.expected)
			}
		})
	}
}

