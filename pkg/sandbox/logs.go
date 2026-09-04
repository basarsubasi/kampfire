package sandbox

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/basarsubasi/kampfire/pkg/k8s"

	corev1 "k8s.io/api/core/v1"
)

// LogOptions specifies parameters for retrieving sandbox container logs.
type LogOptions struct {
	Follow     bool
	Tail       int64 // -1 if not set
	Head       int64 // -1 if not set
	Timestamps bool
}

// Validate checks that log options are mutually compatible and within acceptable ranges.
func (opts LogOptions) Validate() error {
	if opts.Tail >= 0 && opts.Head >= 0 {
		return fmt.Errorf("cannot specify both --tail and --head")
	}
	if opts.Tail < -1 {
		return fmt.Errorf("--tail cannot be negative")
	}
	if opts.Head < -1 {
		return fmt.Errorf("--head cannot be negative")
	}
	return nil
}

// CopyHead reads up to head lines from reader and writes them to out.
func CopyHead(r io.Reader, out io.Writer, head int64) error {
	if head <= 0 {
		return nil
	}

	reader := bufio.NewReader(r)
	var count int64

	for count < head {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			if _, writeErr := out.Write(line); writeErr != nil {
				return writeErr
			}
			count++
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
	}

	return nil
}

// StreamLogs streams or fetches logs for the specified sandbox pod.
func StreamLogs(ctx context.Context, client *k8s.Client, podName string, opts LogOptions, out io.Writer) error {
	if err := opts.Validate(); err != nil {
		return err
	}

	podLogOpts := &corev1.PodLogOptions{
		Follow:     opts.Follow,
		Timestamps: opts.Timestamps,
	}

	if opts.Tail >= 0 {
		podLogOpts.TailLines = &opts.Tail
	}

	req := client.Clientset.CoreV1().Pods(client.Namespace).GetLogs(podName, podLogOpts)
	stream, err := req.Stream(ctx)
	if err != nil {
		return fmt.Errorf("failed to open log stream for %s: %w", podName, err)
	}
	defer stream.Close()

	if opts.Head >= 0 {
		return CopyHead(stream, out, opts.Head)
	}

	_, err = io.Copy(out, stream)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("error reading log stream: %w", err)
	}

	return nil
}
