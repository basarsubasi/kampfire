package cmd

import (
	"context"
	"fmt"

	"github.com/basarsubasi/kampfire/pkg/sandbox"
	"github.com/basarsubasi/kampfire/pkg/ui"

	"github.com/spf13/cobra"
)

var stopAll bool

var stopCmd = &cobra.Command{
	Use:   "stop [flags] SANDBOX_ID [SANDBOX_ID...]",
	Short: "Stop one or more running sandboxes",
	Long: `Stops the specified running sandboxes by suspending their pods.
This reduces CPU and Memory consumption to zero while retaining the sandbox definition,
network settings, and any persistent volumes.`,
	Example: `  # Stop a single sandbox
  kampfire stop my-sandbox

  # Stop multiple sandboxes
  kampfire stop sb-1 sb-2 sb-3

  # Stop all running sandboxes in current namespace
  kampfire stop -a`,
	Args: func(cmd *cobra.Command, args []string) error {
		if !stopAll && len(args) == 0 {
			return fmt.Errorf("requires at least 1 arg(s), or use -a/--all to stop all sandboxes")
		}
		return nil
	},
	ValidArgsFunction: completeSandboxNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		client, _, err := GetClient()
		if err != nil {
			return err
		}

		var targets []string
		if stopAll {
			sandboxes, err := sandbox.List(ctx, client, false)
			if err != nil {
				return err
			}
			for _, sb := range sandboxes {
				if sb.Status != "Stopped" && sb.Status != "Terminating" {
					targets = append(targets, sb.Name)
				}
			}
			if len(targets) == 0 {
				ui.Info("No running sandboxes to stop in namespace %s.", client.Namespace)
				return nil
			}
		} else {
			targets = args
		}

		var hasError bool
		for _, target := range targets {
			_, err := sandbox.Stop(ctx, client, target)
			if err != nil {
				ui.Error("Failed to stop sandbox %s: %s", target, err)
				hasError = true
			} else {
				fmt.Println(target)
			}
		}

		if hasError {
			return fmt.Errorf("some sandboxes could not be stopped")
		}
		return nil
	},
}

func init() {
	stopCmd.Flags().BoolVarP(&stopAll, "all", "a", false, "Stop all running sandboxes in current namespace")
	RootCmd.AddCommand(stopCmd)
}
