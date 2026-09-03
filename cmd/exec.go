package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/basarsubasi/kampfire/pkg/sandbox"
	"github.com/basarsubasi/kampfire/pkg/terminal"

	"github.com/spf13/cobra"
)

var (
	execInteractive bool
	execTTY         bool
)

var execCmd = &cobra.Command{
	Use:   "exec [flags] SANDBOX_ID [COMMAND...]",
	Short: "Execute a command or start an interactive shell in a running sandbox",
	Long: `Runs a command inside an existing, running sandbox container with full PTY support.
If no command is specified when using -it, defaults to an interactive /bin/sh session.`,
	Example: `  # Start an interactive shell
  kampfire exec -it my-sandbox /bin/sh

  # Run a command non-interactively
  kampfire exec my-sandbox uname -a`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		client, _, err := GetClient()
		if err != nil {
			return err
		}

		target := args[0]
		command := args[1:]

		sandboxName := target
		if sb, err := sandbox.Find(ctx, client, target); err == nil {
			sandboxName = sb.Name
		}

		if execInteractive || execTTY {
			if len(command) == 0 {
				command = []string{"/bin/sh"}
			}
			return terminal.RunInteractiveSession(ctx, client, sandboxName, command)
		}

		if len(command) == 0 {
			return fmt.Errorf("no command specified for exec")
		}

		stdout, stderr, err := terminal.ExecSimple(ctx, client, sandboxName, command)
		if stdout != "" {
			fmt.Print(stdout)
		}
		if stderr != "" {
			fmt.Fprint(os.Stderr, stderr)
		}
		return err
	},
}

func init() {
	execCmd.Flags().SetInterspersed(false)
	execCmd.Flags().BoolVarP(&execInteractive, "interactive", "i", false, "Keep STDIN open")
	execCmd.Flags().BoolVarP(&execTTY, "tty", "t", false, "Allocate a pseudo-TTY")

	RootCmd.AddCommand(execCmd)
}
