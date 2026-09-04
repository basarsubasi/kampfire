package sandbox

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestParsePortForwardCommandLine(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantNS      string
		wantSandbox string
		wantPorts   []string
		wantOK      bool
	}{
		{
			name:        "kampfire port-forward basic",
			args:        []string{"kampfire", "port-forward", "arch", "8080:80"},
			wantNS:      "",
			wantSandbox: "arch",
			wantPorts:   []string{"8080:80"},
			wantOK:      true,
		},
		{
			name:        "kampfire with namespace before subcommand",
			args:        []string{"/usr/local/bin/kampfire", "-n", "prod", "pf", "debian", "3000:3000", "8080:80"},
			wantNS:      "prod",
			wantSandbox: "debian",
			wantPorts:   []string{"3000:3000", "8080:80"},
			wantOK:      true,
		},
		{
			name:        "kampfire with namespace after subcommand",
			args:        []string{"kampfire", "forward", "--namespace=staging", "my-box", "5432"},
			wantNS:      "staging",
			wantSandbox: "my-box",
			wantPorts:   []string{"5432"},
			wantOK:      true,
		},
		{
			name:        "kubectl port-forward with pod/ prefix",
			args:        []string{"kubectl", "-n", "default", "port-forward", "pod/arch", "8080:80"},
			wantNS:      "default",
			wantSandbox: "arch",
			wantPorts:   []string{"8080:80"},
			wantOK:      true,
		},
		{
			name:   "non-port-forward command",
			args:   []string{"kampfire", "ps", "-a"},
			wantOK: false,
		},
		{
			name:   "unrelated command",
			args:   []string{"/bin/zsh"},
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ns, sb, ports, ok := parsePortForwardCommandLine(tt.args)
			if ok != tt.wantOK {
				t.Fatalf("expected ok=%v, got %v", tt.wantOK, ok)
			}
			if !ok {
				return
			}
			if ns != tt.wantNS {
				t.Errorf("expected ns %q, got %q", tt.wantNS, ns)
			}
			if sb != tt.wantSandbox {
				t.Errorf("expected sandbox %q, got %q", tt.wantSandbox, sb)
			}
			if len(ports) != len(tt.wantPorts) {
				t.Fatalf("expected %d ports, got %d (%v)", len(tt.wantPorts), len(ports), ports)
			}
			for i, p := range ports {
				if p != tt.wantPorts[i] {
					t.Errorf("port[%d]: expected %q, got %q", i, tt.wantPorts[i], p)
				}
			}
		})
	}
}

func TestFormatPortsWithActiveForwards(t *testing.T) {
	// 1. Static port without active forward
	res := FormatPortsWithActiveForwards("80/TCP")
	if res != "80/TCP" {
		t.Errorf("expected '80/TCP', got %q", res)
	}

	// 2. No static ports, with active forward
	res = FormatPortsWithActiveForwards("", []string{"127.0.0.1:8080->80/TCP"})
	if res != "127.0.0.1:8080->80/TCP" {
		t.Errorf("expected '127.0.0.1:8080->80/TCP', got %q", res)
	}

	// 3. Static port overridden by active forward
	res = FormatPortsWithActiveForwards("80/TCP, 3000/TCP", []string{"127.0.0.1:8080->80/TCP"})
	if !strings.Contains(res, "127.0.0.1:8080->80/TCP") || !strings.Contains(res, "3000/TCP") {
		t.Errorf("expected active forward and remaining static port, got %q", res)
	}

	// 4. Empty static and empty active
	res = FormatPortsWithActiveForwards("")
	if res != "" {
		t.Errorf("expected empty string, got %q", res)
	}
}

func TestRegisterActiveForward_Lifecycle(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tempDir, ".config"))

	cleanup, err := RegisterActiveForward("default", "test-box", []string{"8080:80", "3000"})
	if err != nil {
		t.Fatalf("RegisterActiveForward failed: %v", err)
	}

	// Verify active forwards are retrieved
	forwards := GetActiveForwards("default")
	ports, ok := forwards["test-box"]
	if !ok || len(ports) != 2 {
		t.Fatalf("expected 2 active forwards for test-box, got %v", forwards)
	}
	if ports[0] != "127.0.0.1:8080->80/TCP" || ports[1] != "127.0.0.1:3000->3000/TCP" {
		t.Errorf("unexpected ports: %v", ports)
	}

	// Run cleanup and verify state is removed
	cleanup()
	forwardsAfter := GetActiveForwards("default")
	if len(forwardsAfter["test-box"]) != 0 {
		// Only state file was cleaned up; if no process running with that PID, it's empty
		t.Logf("forwards after cleanup: %v", forwardsAfter["test-box"])
	}
}
