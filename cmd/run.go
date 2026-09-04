package cmd

import (
	"context"
	"fmt"
	"strings"
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

		ui.Info("Provisioning sandbox %s (%s)...", ui.TitleStyle.Render(name), runImage)

		// 1. Create Sandbox CR
		info, err := sandbox.Create(ctx, client, name, runImage, command)
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
			fmt.Println(info.Name)
			return nil
		}

		// Interactive mode
		if runInteractive || runTTY {
			defer func() {
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

	RootCmd.AddCommand(runCmd)
}
