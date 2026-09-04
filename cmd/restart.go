package cmd

import (
	"context"
	"fmt"

	"github.com/basarsubasi/kampfire/pkg/sandbox"
	"github.com/basarsubasi/kampfire/pkg/ui"

	"github.com/spf13/cobra"
)

var restartDetach bool

var restartCmd = &cobra.Command{
	Use:   "restart [flags] SANDBOX_ID [SANDBOX_ID...]",
	Short: "Restart one or more sandboxes",
	Long:  `Stops and immediately starts the specified sandboxes.`,
	Example: `  # Restart a sandbox
  kampfire restart my-sandbox

  # Restart in background without waiting for readiness
  kampfire restart -d my-sandbox`,
	Args:              cobra.MinimumNArgs(1),
	ValidArgsFunction: completeSandboxNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		client, _, err := GetClient()
		if err != nil {
			return err
		}

		var hasError bool
		for _, target := range args {
			sb, err := sandbox.Stop(ctx, client, target)
			if err != nil {
				ui.Error("Failed to stop sandbox %s: %s", target, err)
				hasError = true
				continue
			}

			_, err = sandbox.Start(ctx, client, sb.Name)
			if err != nil {
				ui.Error("Failed to start sandbox %s: %s", target, err)
				hasError = true
				continue
			}

			if restartDetach {
				fmt.Println(target)
				continue
			}

			ui.Info("Restarting sandbox %s...", ui.TitleStyle.Render(sb.Name))
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
				ui.Error("Failed waiting for sandbox %s to restart: %s", target, waitErr)
				hasError = true
			} else {
				ui.Success("Sandbox %s restarted!", ui.TitleStyle.Render(sb.Name))
				fmt.Println(target)
			}
		}

		if hasError {
			return fmt.Errorf("some sandboxes could not be restarted")
		}
		return nil
	},
}

func init() {
	restartCmd.Flags().BoolVarP(&restartDetach, "detach", "d", false, "Restart sandbox in background without waiting for readiness")
	RootCmd.AddCommand(restartCmd)
}
