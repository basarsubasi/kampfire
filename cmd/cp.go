package cmd

import (
	"context"

	"github.com/basarsubasi/kampfire/pkg/transfer"
	"github.com/basarsubasi/kampfire/pkg/ui"

	"github.com/spf13/cobra"
)

var cpCmd = &cobra.Command{
	Use:   "cp SRC DEST",
	Short: "Copy files/folders between local machine and a sandbox container",
	Long: `Transfers files or directories between the local filesystem and a sandbox container.
Follows Docker cp syntax: specify remote paths as SANDBOX_ID:PATH.`,
	Example: `  # Copy local file into sandbox
  kampfire cp ./main.py my-sandbox:/workspace/main.py

  # Copy file from sandbox to local directory
  kampfire cp my-sandbox:/tmp/result.json ./result.json

  # Copy directory recursively into sandbox
  kampfire cp ./src my-sandbox:/app/src`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		client, _, err := GetClient()
		if err != nil {
			return err
		}

		src := args[0]
		dest := args[1]

		ui.Info("Copying %s -> %s...", src, dest)
		if err := transfer.Copy(ctx, client, src, dest); err != nil {
			return err
		}

		ui.Success("Successfully copied %s to %s", src, dest)
		return nil
	},
}

func init() {
	RootCmd.AddCommand(cpCmd)
}
