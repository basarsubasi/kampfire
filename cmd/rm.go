package cmd

import (
	"context"
	"fmt"

	"github.com/basarsubasi/kampfire/pkg/sandbox"
	"github.com/basarsubasi/kampfire/pkg/ui"

	"github.com/spf13/cobra"
)

var (
	rmAll bool
)

var rmCmd = &cobra.Command{
	Use:   "rm [flags] SANDBOX_ID [SANDBOX_ID...]",
	Short: "Remove one or more sandboxes",
	Long:  `Deletes the specified Sandbox resources from your configured namespace.`,
	Example: `  # Remove a single sandbox
  kampfire rm my-sandbox

  # Remove multiple sandboxes
  kampfire rm sb-1 sb-2 sb-3

  # Remove all sandboxes in current namespace
  kampfire rm -a`,
	Args: func(cmd *cobra.Command, args []string) error {
		if !rmAll && len(args) == 0 {
			return fmt.Errorf("requires at least 1 arg(s), or use -a/--all to remove all sandboxes")
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
		if rmAll {
			sandboxes, err := sandbox.List(ctx, client, true)
			if err != nil {
				return err
			}
			if len(sandboxes) == 0 {
				return nil
			}
			for _, sb := range sandboxes {
				targets = append(targets, sb.Name)
			}
		} else {
			targets = args
		}

		var hasError bool
		for _, target := range targets {
			name := target
			if !rmAll {
				if sb, err := sandbox.Find(ctx, client, target); err == nil {
					name = sb.Name
				}
			}
			if err := sandbox.Delete(ctx, client, name); err != nil {
				ui.Error("Failed to remove sandbox %s: %s", target, err)
				hasError = true
			} else {
				fmt.Println(target)
			}
		}

		if hasError {
			return fmt.Errorf("some sandboxes could not be removed")
		}
		return nil
	},
}

func init() {
	rmCmd.Flags().BoolVarP(&rmAll, "all", "a", false, "Remove all sandboxes in current namespace")
	RootCmd.AddCommand(rmCmd)
}
