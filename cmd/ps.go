package cmd

import (
	"context"
	"fmt"

	"github.com/basarsubasi/kampfire/pkg/sandbox"
	"github.com/basarsubasi/kampfire/pkg/ui"

	"github.com/spf13/cobra"
)

var (
	psAll   bool
	psQuiet bool
)

var psCmd = &cobra.Command{
	Use:     "ps [flags]",
	Aliases: []string{"ls"},
	Short:   "List sandboxes in your configured namespace",
	Long:    `Displays active sandboxes, their container images, running status, age, and assigned pod IP.`,
	Example: `  # List running sandboxes
  kampfire ps

  # List all sandboxes (including terminating)
  kampfire ps -a

  # Output only IDs (useful for scripting: kampfire rm $(kampfire ps -q))
  kampfire ps -q`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		client, _, err := GetClient()
		if err != nil {
			return err
		}

		sandboxes, err := sandbox.List(ctx, client, psAll)
		if err != nil {
			return err
		}

		if psQuiet {
			for _, sb := range sandboxes {
				fmt.Println(sb.ID)
			}
			return nil
		}

		if len(sandboxes) == 0 {
			ui.Info("No sandboxes found in namespace %s.", ui.TitleStyle.Render(client.Namespace))
			return nil
		}

		headers := []string{"SANDBOX ID", "NAME", "IMAGE", "STATUS", "AGE", "IP", "PORTS"}
		var rows [][]string

		for _, sb := range sandboxes {
			ip := sb.PodIP
			if ip == "" {
				ip = "-"
			}
			ports := sb.Ports
			if ports == "" {
				ports = "-"
			}
			rows = append(rows, []string{
				sb.ID,
				ui.TitleStyle.Render(sb.Name),
				sb.Image,
				ui.StatusBadge(sb.Status),
				sb.Age,
				ui.MutedStyle.Render(ip),
				ui.MutedStyle.Render(ports),
			})
		}

		ui.PrintTable(headers, rows)
		return nil
	},
}

func init() {
	psCmd.Flags().BoolVarP(&psAll, "all", "a", false, "Show all sandboxes (including terminating)")
	psCmd.Flags().BoolVarP(&psQuiet, "quiet", "q", false, "Only display sandbox names/IDs")

	RootCmd.AddCommand(psCmd)
}
