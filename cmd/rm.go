package cmd

import (
	"context"
	"fmt"

	"campfire/pkg/sandbox"
	"campfire/pkg/ui"

	"github.com/spf13/cobra"
)

var rmCmd = &cobra.Command{
	Use:   "rm SANDBOX_ID [SANDBOX_ID...]",
	Short: "Remove one or more sandboxes",
	Long:  `Deletes the specified Sandbox resources from your configured namespace.`,
	Example: `  # Remove a single sandbox
  campfire rm my-sandbox

  # Remove multiple sandboxes
  campfire rm sb-1 sb-2 sb-3

  # Remove all sandboxes in namespace
  campfire rm $(campfire ps -q)`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		client, _, err := GetClient()
		if err != nil {
			return err
		}

		var hasError bool
		for _, name := range args {
			if err := sandbox.Delete(ctx, client, name); err != nil {
				ui.Error("Failed to remove sandbox %s: %s", name, err)
				hasError = true
			} else {
				fmt.Println(name)
			}
		}

		if hasError {
			return fmt.Errorf("some sandboxes could not be removed")
		}
		return nil
	},
}

func init() {
	RootCmd.AddCommand(rmCmd)
}
