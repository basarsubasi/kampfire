package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/basarsubasi/kampfire/pkg/sandbox"
	"github.com/basarsubasi/kampfire/pkg/ui"

	"github.com/spf13/cobra"
)

var portForwardCmd = &cobra.Command{
	Use:     "port-forward SANDBOX_ID [LOCAL_PORT:]REMOTE_PORT...",
	Aliases: []string{"forward", "pf"},
	Short:   "Forward one or more local ports to a sandbox container",
	Long: `Establishes a secure SPDY port-forward tunnel from your local machine to a sandbox container.
Supports multiple port pairs in the format [LOCAL_PORT:]REMOTE_PORT.
If only REMOTE_PORT is specified (e.g. 8080), LOCAL_PORT defaults to the same value.`,
	Example: `  # Forward local port 8080 to container port 80
  kampfire port-forward my-sandbox 8080:80

  # Forward local port 3000 to container port 3000
  kampfire port-forward my-sandbox 3000

  # Forward multiple ports simultaneously
  kampfire port-forward 0cceb66ac7b7 8080:80 5432:5432`,
	Args:              cobra.MinimumNArgs(2),
	ValidArgsFunction: completeSandboxNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		client, _, err := GetClient()
		if err != nil {
			return err
		}

		target := args[0]
		portArgs := args[1:]

		// 1. Resolve sandbox name or short ID
		sandboxName := target
		if sb, err := sandbox.Find(ctx, client, target); err == nil {
			sandboxName = sb.Name
		}

		// 2. Validate and format port mappings
		var ports []string
		for _, p := range portArgs {
			parts := strings.Split(p, ":")
			if len(parts) == 1 {
				// Single port: 8080 -> 8080:8080
				val, err := strconv.Atoi(parts[0])
				if err != nil || val <= 0 || val > 65535 {
					return fmt.Errorf("invalid port %q: must be between 1 and 65535", parts[0])
				}
				ports = append(ports, fmt.Sprintf("%d:%d", val, val))
			} else if len(parts) == 2 {
				localPort, err1 := strconv.Atoi(parts[0])
				remotePort, err2 := strconv.Atoi(parts[1])
				if err1 != nil || err2 != nil || localPort <= 0 || localPort > 65535 || remotePort <= 0 || remotePort > 65535 {
					return fmt.Errorf("invalid port mapping %q: ports must be between 1 and 65535", p)
				}
				ports = append(ports, fmt.Sprintf("%d:%d", localPort, remotePort))
			} else {
				return fmt.Errorf("invalid port format %q: use [LOCAL_PORT:]REMOTE_PORT", p)
			}
		}

		readyCh := make(chan struct{})
		stopCh := make(chan struct{})
		errCh := make(chan error, 1)

		// 3. Start port forwarding in background goroutine
		go func() {
			if err := client.PortForwardPorts(ctx, sandboxName, ports, readyCh, stopCh, nil, os.Stderr); err != nil {
				errCh <- err
			}
		}()

		select {
		case <-readyCh:
			for _, p := range ports {
				parts := strings.Split(p, ":")
				ui.Success("Forwarding from 127.0.0.1:%s -> %s (sandbox: %s)", parts[0], parts[1], ui.TitleStyle.Render(sandboxName))
			}
			ui.Info("Press Ctrl+C to stop forwarding.")
		case err := <-errCh:
			return fmt.Errorf("port-forward failed: %w", err)
		}

		// 4. Wait for interrupt signal
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

		select {
		case <-sigCh:
			fmt.Println()
			ui.Info("Stopping port-forward...")
			close(stopCh)
		case <-ctx.Done():
			close(stopCh)
		case err := <-errCh:
			return err
		}

		return nil
	},
}

func init() {
	RootCmd.AddCommand(portForwardCmd)
}
