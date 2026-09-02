package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"campfire/pkg/sandbox"
	"campfire/pkg/ui"

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
  campfire ps

  # List all sandboxes (including terminating)
  campfire ps -a

  # Output only IDs (useful for scripting: campfire rm $(campfire ps -q))
  campfire ps -q`,
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
				fmt.Println(sb.Name)
			}
			return nil
		}

		if len(sandboxes) == 0 {
			ui.Info("No sandboxes found in namespace %s.", ui.TitleStyle.Render(client.Namespace))
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 8, 3, ' ', 0)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			ui.HeaderStyle.Render("NAME / ID"),
			ui.HeaderStyle.Render("IMAGE"),
			ui.HeaderStyle.Render("STATUS"),
			ui.HeaderStyle.Render("AGE"),
			ui.HeaderStyle.Render("IP"))

		for _, sb := range sandboxes {
			ip := sb.PodIP
			if ip == "" {
				ip = "-"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				ui.TitleStyle.Render(sb.Name),
				sb.Image,
				ui.StatusBadge(sb.Status),
				sb.Age,
				ui.MutedStyle.Render(ip))
		}

		return w.Flush()
	},
}

func init() {
	psCmd.Flags().BoolVarP(&psAll, "all", "a", false, "Show all sandboxes (including terminating)")
	psCmd.Flags().BoolVarP(&psQuiet, "quiet", "q", false, "Only display sandbox names/IDs")

	RootCmd.AddCommand(psCmd)
}
