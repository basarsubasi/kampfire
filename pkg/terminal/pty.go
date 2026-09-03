package terminal

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/basarsubasi/kampfire/pkg/k8s"

	"golang.org/x/term"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

// SizeQueue implements remotecommand.TerminalSizeQueue to forward window resizing in real-time.
type SizeQueue struct {
	resizeChan chan remotecommand.TerminalSize
	stopChan   chan struct{}
}

// NewSizeQueue creates and starts a dynamic terminal size monitor.
func NewSizeQueue() *SizeQueue {
	sq := &SizeQueue{
		resizeChan: make(chan remotecommand.TerminalSize, 10),
		stopChan:   make(chan struct{}),
	}

	// Send initial terminal size
	if width, height, err := term.GetSize(int(os.Stdin.Fd())); err == nil {
		sq.resizeChan <- remotecommand.TerminalSize{
			Width:  uint16(width),
			Height: uint16(height),
		}
	}

	// Listen for SIGWINCH (window resize)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGWINCH)

	go func() {
		defer signal.Stop(sigChan)
		for {
			select {
			case <-sq.stopChan:
				return
			case <-sigChan:
				if width, height, err := term.GetSize(int(os.Stdin.Fd())); err == nil {
					sq.resizeChan <- remotecommand.TerminalSize{
						Width:  uint16(width),
						Height: uint16(height),
					}
				}
			}
		}
	}()

	return sq
}

// Next returns the next terminal size event.
func (s *SizeQueue) Next() *remotecommand.TerminalSize {
	size, ok := <-s.resizeChan
	if !ok {
		return nil
	}
	return &size
}

// Stop terminates the signal listener.
func (s *SizeQueue) Stop() {
	close(s.stopChan)
}

// ExecOptions configures an execution request.
type ExecOptions struct {
	PodName       string
	ContainerName string
	Command       []string
	Stdin         io.Reader
	Stdout        io.Writer
	Stderr        io.Writer
	TTY           bool
	Interactive   bool
}

// RunInteractiveSession connects an interactive PTY session with raw terminal mode and dynamic resizing.
func RunInteractiveSession(ctx context.Context, client *k8s.Client, podName string, command []string) error {
	if len(command) == 0 {
		// Automatically check or fallback: bash -> sh
		command = []string{"/bin/sh"}
	}

	// Check if stdin is a terminal
	isTerminal := term.IsTerminal(int(os.Stdin.Fd()))
	var oldState *term.State
	var sizeQueue *SizeQueue

	if isTerminal {
		var err error
		oldState, err = term.MakeRaw(int(os.Stdin.Fd()))
		if err != nil {
			return fmt.Errorf("failed to set raw terminal mode: %w", err)
		}
		defer func() {
			_ = term.Restore(int(os.Stdin.Fd()), oldState)
		}()

		sizeQueue = NewSizeQueue()
		defer sizeQueue.Stop()
	}

	req := client.Clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(client.Namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Command: command,
			Stdin:   true,
			Stdout:  true,
			Stderr:  true,
			TTY:     isTerminal,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(client.RestConfig, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("failed to create executor: %w", err)
	}

	streamOpts := remotecommand.StreamOptions{
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Tty:    isTerminal,
	}
	if sizeQueue != nil {
		streamOpts.TerminalSizeQueue = sizeQueue
	}

	return exec.StreamWithContext(ctx, streamOpts)
}

// ExecSimple runs a command non-interactively in a pod and returns stdout and stderr.
func ExecSimple(ctx context.Context, client *k8s.Client, podName string, command []string) (string, string, error) {
	req := client.Clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(client.Namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Command: command,
			Stdin:   false,
			Stdout:  true,
			Stderr:  true,
			TTY:     false,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(client.RestConfig, "POST", req.URL())
	if err != nil {
		return "", "", fmt.Errorf("failed to create SPDY executor: %w", err)
	}

	var stdout, stderr bytes.Buffer
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
		Tty:    false,
	})

	return stdout.String(), stderr.String(), err
}
