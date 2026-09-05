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
