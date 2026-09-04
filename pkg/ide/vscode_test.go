package ide

import (
	"fmt"
	"net"
	"strings"
	"testing"
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

func TestScripts_AlpineNpmInstallation(t *testing.T) {
	// Must branch on /etc/alpine-release
	if !strings.Contains(installScript, "/etc/alpine-release") {
		t.Errorf("installScript must check for /etc/alpine-release")
	}

	// Must install native C compilation tools and Python for node-gyp on Alpine
	requiredDeps := []string{"nodejs", "npm", "alpine-sdk", "libstdc++", "libc6-compat", "python3", "krb5-dev"}
	for _, dep := range requiredDeps {
		if !strings.Contains(installScript, dep) {
			t.Errorf("installScript missing Alpine dependency: %s", dep)
		}
	}

	// Must clean up incompatible standalone binaries from previous runs
	if !strings.Contains(installScript, "rm -rf /usr/local/bin/code-server") {
		t.Errorf("installScript should clean up old standalone code-server on Alpine")
	}

	// Must install code-server via npm with --unsafe-perm
	if !strings.Contains(installScript, "npm install --global code-server --unsafe-perm") {
		t.Errorf("installScript must run npm install --global code-server --unsafe-perm")
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

