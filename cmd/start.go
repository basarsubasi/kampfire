package cmd

import (
	"context"
	"fmt"

	"github.com/basarsubasi/kampfire/pkg/sandbox"
	"github.com/basarsubasi/kampfire/pkg/ui"

	"github.com/spf13/cobra"
)

var (
	startAll    bool
	startDetach bool
)

var startCmd = &cobra.Command{
	Use:   "start [flags] SANDBOX_ID [SANDBOX_ID...]",
	Short: "Start one or more stopped sandboxes",
	Long: `Resumes stopped sandboxes by provisioning their pods again.
Any persistent volumes mounted to the sandbox will be reattached with data intact.`,
	Example: `  # Start a stopped sandbox and wait for readiness
  kampfire start my-sandbox

  # Start multiple sandboxes in background
  kampfire start -d sb-1 sb-2

  # Start all stopped sandboxes in current namespace
  kampfire start -a`,
	Args: func(cmd *cobra.Command, args []string) error {
		if !startAll && len(args) == 0 {
			return fmt.Errorf("requires at least 1 arg(s), or use -a/--all to start all sandboxes")
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
		if startAll {
			sandboxes, err := sandbox.List(ctx, client, true)
			if err != nil {
				return err
			}
			for _, sb := range sandboxes {
				if sb.Status == "Stopped" {
					targets = append(targets, sb.Name)
				}
			}
			if len(targets) == 0 {
				ui.Info("No stopped sandboxes to start in namespace %s.", client.Namespace)
				return nil
			}
		} else {
			targets = args
		}

		var hasError bool
		for _, target := range targets {
			sb, err := sandbox.Start(ctx, client, target)
			if err != nil {
				ui.Error("Failed to start sandbox %s: %s", target, err)
				hasError = true
				continue
			}

			if startDetach {
				fmt.Println(target)
				continue
			}

			ui.Info("Starting sandbox %s...", ui.TitleStyle.Render(sb.Name))
			var lastStatusLen int
			_, waitErr := sandbox.WaitReady(ctx, client, sb.Name, func(update sandbox.StatusUpdate) {
				text := fmt.Sprintf("\r  %s %s [%.1fs]", ui.ColorAmber, update.Status, update.Elapsed.Seconds())
				pad := 0
				if lastStatusLen > len(text) {
					pad = lastStatusLen - len(text)
				}
				lastStatusLen = len(text)
				fmt.Printf("%s%s", text, stringsRepeat(" ", pad))
			})
			fmt.Printf("\r%s\r", stringsRepeat(" ", lastStatusLen+5))

			if waitErr != nil {
				ui.Error("Failed waiting for sandbox %s to become ready: %s", target, waitErr)
				hasError = true
			} else {
				ui.Success("Sandbox %s is running!", ui.TitleStyle.Render(sb.Name))
				fmt.Println(target)
			}
		}

		if hasError {
			return fmt.Errorf("some sandboxes could not be started")
		}
		return nil
	},
}

func init() {
	startCmd.Flags().BoolVarP(&startAll, "all", "a", false, "Start all stopped sandboxes in current namespace")
	startCmd.Flags().BoolVarP(&startDetach, "detach", "d", false, "Start sandbox in background without waiting for readiness")
	RootCmd.AddCommand(startCmd)
}
