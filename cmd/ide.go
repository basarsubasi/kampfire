package cmd

import (
	"context"

	"github.com/basarsubasi/kampfire/pkg/ide"
	"github.com/basarsubasi/kampfire/pkg/sandbox"

	"github.com/spf13/cobra"
)

var ideCmd = &cobra.Command{
	Use:   "ide",
	Short: "Launch development environments connected to your sandboxes",
}

var (
	ideBrowser bool
)

var vscodeCmd = &cobra.Command{
	Use:   "vscode [flags] SANDBOX_ID",
	Short: "Auto-install VS Code server inside sandbox and open in desktop VS Code",
	Long: `Checks if code-server is installed inside the sandbox container.
If missing, automatically installs standalone code-server, launches the daemon,
tunnels the port securely via Kubernetes SPDY port-forwarding, and opens desktop VS Code.`,
	Example: `  # Open sandbox in desktop VS Code
  kampfire ide vscode my-sandbox

  # Open sandbox in web browser instead
  kampfire ide vscode my-sandbox --browser`,
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
		return ide.OpenVSCode(ctx, client, sandboxName, ideBrowser)
	},
}

func init() {
	vscodeCmd.Flags().BoolVar(&ideBrowser, "browser", false, "Open in web browser instead of desktop VS Code")

	ideCmd.AddCommand(vscodeCmd)
	RootCmd.AddCommand(ideCmd)
}
