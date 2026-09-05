package terminal

import (
	"testing"
	"time"

	"k8s.io/client-go/tools/remotecommand"
)

func TestSizeQueueLifecycle(t *testing.T) {
	sq := &SizeQueue{
		resizeChan: make(chan remotecommand.TerminalSize, 10),
		stopChan:   make(chan struct{}),
	}

	testSize := remotecommand.TerminalSize{
		Width:  120,
		Height: 40,
	}

	sq.resizeChan <- testSize

	got := sq.Next()
	if got == nil {
		t.Fatalf("expected terminal size, got nil")
	}
	if got.Width != 120 || got.Height != 40 {
		t.Errorf("expected 120x40, got %dx%d", got.Width, got.Height)
	}

	sq.Stop()
	select {
	case <-sq.stopChan:
		// success
	case <-time.After(1 * time.Second):
		t.Fatalf("timed out waiting for stopChan to close")
	}
}

func TestExecOptionsDefaults(t *testing.T) {
	opts := ExecOptions{
		PodName: "test-pod",
		Command: []string{"echo", "hello"},
		TTY:     true,
	}

	if opts.PodName != "test-pod" {
		t.Errorf("expected pod name 'test-pod', got %q", opts.PodName)
	}
	if len(opts.Command) != 2 || opts.Command[0] != "echo" {
		t.Errorf("unexpected command: %v", opts.Command)
	}
	if !opts.TTY {
		t.Errorf("expected TTY to be true")
	}
}

func TestBuildInteractiveCommand_Empty(t *testing.T) {
	cmd := BuildInteractiveCommand(nil)
	if len(cmd) != 3 {
		t.Fatalf("expected 3 arguments for empty command (/bin/sh, -c, script), got %d: %v", len(cmd), cmd)
	}
	if cmd[0] != "/bin/sh" || cmd[1] != "-c" {
		t.Errorf("expected [/bin/sh, -c, ...], got %v", cmd)
	}
	script := cmd[2]
	if !containsStr(script, "LANG=C.UTF-8") || !containsStr(script, "LC_ALL=C.UTF-8") {
		t.Errorf("expected script to configure UTF-8 locale, got: %s", script)
	}
	if !containsStr(script, "command -v bash") {
		t.Errorf("expected script to test for bash availability, got: %s", script)
	}
}

func TestBuildInteractiveCommand_WithCustomCommand(t *testing.T) {
	cmd := BuildInteractiveCommand([]string{"python3", "-i"})
	if len(cmd) < 5 {
		t.Fatalf("expected at least 5 arguments for custom command, got %d: %v", len(cmd), cmd)
	}
	if cmd[0] != "/bin/sh" || cmd[1] != "-c" {
		t.Errorf("expected [/bin/sh -c ...], got %v", cmd)
	}
	if cmd[3] != "--" || cmd[4] != "python3" || cmd[5] != "-i" {
		t.Errorf("expected trailing [-- python3 -i], got %v", cmd[3:])
	}
	script := cmd[2]
	if !containsStr(script, "LANG=C.UTF-8") || !containsStr(script, "LC_ALL=C.UTF-8") {
		t.Errorf("expected script to configure UTF-8 locale, got: %s", script)
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(s) > len(substr) && searchSubstr(s, substr)))
}

func searchSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

