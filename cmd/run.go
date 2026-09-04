package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/basarsubasi/kampfire/pkg/sandbox"
	"github.com/basarsubasi/kampfire/pkg/terminal"
	"github.com/basarsubasi/kampfire/pkg/transfer"
	"github.com/basarsubasi/kampfire/pkg/ui"

	"github.com/spf13/cobra"
)

var (
	runImage              string
	runName               string
	runInteractive        bool
	runTTY                bool
	runDetach             bool
	runAutoRemove         bool
	runWithPrivateSSHKeys bool
	runCloneRepo          string
	runCPU                string
	runMemory             string
	runPublish            []string
)

var runCmd = &cobra.Command{
	Use:   "run [NAME] --image <IMAGE> [flags] [-- COMMAND...]",
	Short: "Run a command in a new sandbox (or start an interactive session)",
	Long: `Provisions an isolated Sandbox container in your configured namespace.
When run with -it, automatically drops into an interactive shell as soon as the container boots.`,
	Example: `  # Launch an interactive Alpine sandbox with full PTY terminal
  kampfire run --image alpine -it

  # Launch a named sandbox in detached mode
  kampfire run my-box --image python:3.12 -d

  # Launch an ephemeral sandbox that is deleted on exit
  kampfire run --image alpine --rm -it`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		client, _, err := GetClient()
		if err != nil {
			return err
		}

		name := runName
		var command []string

		// Check if first arg is name or image if --image is not provided
		if len(args) > 0 {
			if runImage == "" {
				runImage = args[0]
				args = args[1:]
			} else if name == "" && !runInteractive {
				name = args[0]
				args = args[1:]
			}
		}

		if runImage == "" {
			return fmt.Errorf("required flag --image not specified (e.g. --image alpine)")
		}

		if len(args) > 0 {
			command = args
		}

		if name == "" {
			name = sandbox.GenerateName(runImage)
		}

		// Validate published ports if provided
		var formattedPorts []string
		for _, p := range runPublish {
			parts := strings.Split(p, ":")
			if len(parts) == 1 {
				val, err := strconv.Atoi(parts[0])
				if err != nil || val <= 0 || val > 65535 {
					return fmt.Errorf("invalid port %q: must be between 1 and 65535", parts[0])
				}
				formattedPorts = append(formattedPorts, fmt.Sprintf("%d:%d", val, val))
			} else if len(parts) == 2 {
				localPort, err1 := strconv.Atoi(parts[0])
				remotePort, err2 := strconv.Atoi(parts[1])
				if err1 != nil || err2 != nil || localPort <= 0 || localPort > 65535 || remotePort <= 0 || remotePort > 65535 {
					return fmt.Errorf("invalid port mapping %q: ports must be between 1 and 65535", p)
				}
				formattedPorts = append(formattedPorts, fmt.Sprintf("%d:%d", localPort, remotePort))
			} else {
				return fmt.Errorf("invalid port format %q: use [LOCAL_PORT:]REMOTE_PORT", p)
			}
		}

		ui.Info("Provisioning sandbox %s (%s)...", ui.TitleStyle.Render(name), runImage)

		// 1. Create Sandbox CR with granular options (CPU, Memory, Ports)
		opts := sandbox.CreateOptions{
			Name:           name,
			Image:          runImage,
			Command:        command,
			CPU:            runCPU,
			Memory:         runMemory,
			PublishedPorts: formattedPorts,
		}
		info, err := sandbox.CreateWithOptions(ctx, client, opts)
		if err != nil {
			if strings.Contains(err.Error(), "exceeded quota") {
				return fmt.Errorf("sandbox limit reached in namespace %s (ResourceQuota exceeded)\n  Use 'kampfire ps' and 'kampfire rm' to free up capacity", client.Namespace)
			}
			return fmt.Errorf("failed to create sandbox: %w", err)
		}

		// 2. Wait for Ready condition
		start := time.Now()
		var lastStatusLen int
		_, err = sandbox.WaitReady(ctx, client, info.Name, func(update sandbox.StatusUpdate) {
			text := fmt.Sprintf("\r  %s %s [%.1fs]", ui.ColorAmber, update.Status, update.Elapsed.Seconds())
			pad := 0
			if lastStatusLen > len(text) {
				pad = lastStatusLen - len(text)
			}
			lastStatusLen = len(text)
			fmt.Printf("%s%s", text, stringsRepeat(" ", pad))
		})
		fmt.Print("\r" + stringsRepeat(" ", 85) + "\r")

		if err != nil {
			return fmt.Errorf("sandbox did not become ready: %w", err)
		}

		ui.Success("Sandbox %s is running in namespace %s (took %.1fs)",
			ui.TitleStyle.Render(info.Name), client.Namespace, time.Since(start).Seconds())

		// Inject SSH keys if requested
		if runWithPrivateSSHKeys {
			ui.Info("Injecting private SSH keys into sandbox %s...", info.Name)
			if err := transfer.InjectSSHKeys(ctx, client, info.Name); err != nil {
				return fmt.Errorf("failed to inject SSH keys: %w", err)
			}
			ui.Success("SSH keys injected into sandbox")
		}

		// Clone git repository if requested (executed last, after SSH keys are in place)
		if runCloneRepo != "" {
			ui.Info("Cloning %s into sandbox home...", runCloneRepo)
			stdout, stderr, err := terminal.ExecSimple(ctx, client, info.Name, []string{
				"sh", "-c", "cd \"$HOME\" && git clone \"$1\"", "--", runCloneRepo,
			})
			if err != nil {
				output := strings.TrimSpace(stdout + " " + stderr)
				return fmt.Errorf("failed to clone repository inside sandbox: %s (err: %w)", output, err)
			}
			ui.Success("Cloned repository into sandbox home")
		}

		// Detached mode
		if runDetach {
			if len(formattedPorts) > 0 {
				for _, fp := range formattedPorts {
					parts := strings.Split(fp, ":")
					ui.Info("Port published: 127.0.0.1:%s -> %s (connect: kampfire port-forward %s %s)",
						parts[0], parts[1], info.Name, fp)
				}
			}
			fmt.Println(info.Name)
			return nil
		}

		// Interactive mode
		if runInteractive || runTTY {
			var stopPortForward chan struct{}
			if len(formattedPorts) > 0 {
				stopPortForward = make(chan struct{})
				readyCh := make(chan struct{})
				go func() {
					_ = client.PortForwardPorts(ctx, info.Name, formattedPorts, readyCh, stopPortForward, nil, io.Discard)
				}()
				select {
				case <-readyCh:
					for _, fp := range formattedPorts {
						parts := strings.Split(fp, ":")
						ui.Success("Port forward active: 127.0.0.1:%s -> %s", parts[0], parts[1])
					}
				case <-time.After(2 * time.Second):
				}
			}

			defer func() {
				if stopPortForward != nil {
					close(stopPortForward)
				}
				if runAutoRemove {
					ui.Info("Cleaning up sandbox %s (--rm)...", info.Name)
					_ = sandbox.Delete(context.Background(), client, info.Name)
				}
			}()

			shellCmd := command
			if len(shellCmd) == 0 {
				shellCmd = []string{"/bin/sh"}
			}

			err = terminal.RunInteractiveSession(ctx, client, info.Name, shellCmd)
			if err != nil {
				return err
			}
			return nil
		}

		// Non-interactive, non-detached with published ports: keep forward alive until interrupted
		if len(formattedPorts) > 0 {
			readyCh := make(chan struct{})
			stopCh := make(chan struct{})
			errCh := make(chan error, 1)

			go func() {
				if err := client.PortForwardPorts(ctx, info.Name, formattedPorts, readyCh, stopCh, nil, os.Stderr); err != nil {
					errCh <- err
				}
			}()

			select {
			case <-readyCh:
				for _, fp := range formattedPorts {
					parts := strings.Split(fp, ":")
					ui.Success("Forwarding from 127.0.0.1:%s -> %s (sandbox: %s)", parts[0], parts[1], ui.TitleStyle.Render(info.Name))
				}
				ui.Info("Press Ctrl+C to stop forwarding.")
			case err := <-errCh:
				return fmt.Errorf("port-forward failed: %w", err)
			}

			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
			defer signal.Stop(sigCh)

			select {
			case <-sigCh:
				close(stopCh)
				if runAutoRemove {
					ui.Info("Cleaning up sandbox %s (--rm)...", info.Name)
					_ = sandbox.Delete(context.Background(), client, info.Name)
				}
			case <-ctx.Done():
				close(stopCh)
			case err := <-errCh:
				return err
			}
		}

		return nil
	},
}

func stringsRepeat(s string, count int) string {
	var res string
	for i := 0; i < count; i++ {
		res += s
	}
	return res
}

func init() {
	runCmd.Flags().SetInterspersed(false)
	runCmd.Flags().StringVar(&runImage, "image", "", "Container image (e.g. alpine, python:3.12)")
	runCmd.Flags().StringVar(&runName, "name", "", "Custom name for the sandbox")
	runCmd.Flags().BoolVarP(&runInteractive, "interactive", "i", false, "Keep STDIN open")
	runCmd.Flags().BoolVarP(&runTTY, "tty", "t", false, "Allocate a pseudo-TTY")
	runCmd.Flags().BoolVarP(&runDetach, "detach", "d", false, "Run sandbox in background and print ID")
	runCmd.Flags().BoolVar(&runAutoRemove, "rm", false, "Automatically remove the sandbox when it exits")
	runCmd.Flags().BoolVar(&runWithPrivateSSHKeys, "with-private-ssh-keys", false, "Copy host private SSH keys into sandbox upon creation")
	runCmd.Flags().StringVar(&runCloneRepo, "clone-repo", "", "Clone a git repository to home upon sandbox creation")
	runCmd.Flags().StringVar(&runCPU, "cpu", "", "Container CPU request and limit (e.g. 500m, 2)")
	runCmd.Flags().StringVar(&runMemory, "memory", "", "Container memory request and limit (e.g. 256Mi, 2Gi)")
	runCmd.Flags().StringArrayVarP(&runPublish, "publish", "p", nil, "Publish container port(s) to host (e.g. 8080:80, 3000)")

	RootCmd.AddCommand(runCmd)
}
