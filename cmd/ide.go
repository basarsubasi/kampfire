package cmd

import (
	"context"

	"campfire/pkg/ide"
	"campfire/pkg/sandbox"

	"github.com/spf13/cobra"
)

var ideCmd = &cobra.Command{
	Use:   "ide",
	Short: "Launch development environments connected to your sandboxes",
}

var vscodeCmd = &cobra.Command{
	Use:   "vscode SANDBOX_ID",
	Short: "Auto-install VS Code server inside sandbox and open in browser",
	Long: `Checks if code-server is installed inside the sandbox container.
If missing, automatically installs standalone code-server, launches the daemon,
tunnels the port securely via Kubernetes SPDY port-forwarding, and launches your browser.`,
	Example: `  # Launch VS Code in browser connected to my-sandbox
  campfire ide vscode my-sandbox`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		client, _, err := GetClient()
		if err != nil {
			return err
		}

		target := args[0]
		sandboxName := target
		if sb, err := sandbox.Find(ctx, client, target); err == nil {
			sandboxName = sb.Name
		}
		return ide.OpenVSCode(ctx, client, sandboxName)
	},
}

func init() {
	ideCmd.AddCommand(vscodeCmd)
	RootCmd.AddCommand(ideCmd)
}
