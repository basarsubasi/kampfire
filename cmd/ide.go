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
	Use:     "vscode [flags] SANDBOX_ID",
	Aliases: []string{"code"},
	Short:   "Connect desktop VS Code to sandbox via kubectl exec",
	Long: `Connects desktop VS Code to the sandbox container using Kubernetes exec.
By default, launches your desktop VS Code directly into the container's home directory.
Use --browser to launch code-server in a web browser instead.`,
	Example: `  # Open sandbox in desktop VS Code (via kubectl exec)
  kampfire ide vscode my-sandbox
  kampfire ide code my-sandbox

  # Open sandbox in web browser instead (via code-server)
  kampfire ide vscode my-sandbox --browser
  kampfire ide code my-sandbox --browser`,
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

var agyCmd = &cobra.Command{
	Use:     "agy [flags] SANDBOX_ID",
	Aliases: []string{"antigravity"},
	Short:   "Connect Antigravity IDE to sandbox via kubectl exec",
	Long: `Connects Antigravity IDE to the sandbox container using Kubernetes exec.
By default, launches your desktop Antigravity IDE directly into the container's home directory.
Use --browser to launch code-server in a web browser instead.`,
	Example: `  # Open sandbox in desktop Antigravity IDE
  kampfire ide agy my-sandbox
  kampfire ide antigravity my-sandbox

  # Open sandbox in web browser instead
  kampfire ide agy my-sandbox --browser
  kampfire ide antigravity my-sandbox --browser`,
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
		return ide.OpenAntigravity(ctx, client, sandboxName, ideBrowser)
	},
}

func init() {
	vscodeCmd.Flags().BoolVar(&ideBrowser, "browser", false, "Open in web browser instead of desktop IDE")
	agyCmd.Flags().BoolVar(&ideBrowser, "browser", false, "Open in web browser instead of desktop IDE")

	ideCmd.AddCommand(vscodeCmd)
	ideCmd.AddCommand(agyCmd)
	RootCmd.AddCommand(ideCmd)
}
