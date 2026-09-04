package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/basarsubasi/kampfire/pkg/sandbox"
	"github.com/basarsubasi/kampfire/pkg/ui"

	"github.com/spf13/cobra"
)

var (
	topWatch bool
)

var topCmd = &cobra.Command{
	Use:   "top [flags] [SANDBOX_ID]",
	Short: "Display resource (CPU/Memory) usage of sandboxes",
	Long: `Shows real-time CPU and Memory usage for running sandboxes in the current namespace.
Requires metrics-server to be installed and active in the Kubernetes cluster.`,
	Example: `  # View resource usage for all sandboxes
  kampfire top

  # Stream/watch resource usage with live updates
  kampfire top -w

  # View resource usage for a specific sandbox
  kampfire top my-sandbox`,
	Args: cobra.MaximumNArgs(1),
	ValidArgsFunction: completeSandboxNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}

		client, _, err := GetClient()
		if err != nil {
			return err
		}

		target := ""
		if len(args) > 0 {
			target = args[0]
		}

		renderMetrics := func() error {
			metrics, err := sandbox.GetMetrics(ctx, client, target)
			if err != nil {
				return err
			}

			if len(metrics) == 0 {
				if target != "" {
					ui.Info("Sandbox %s not found in namespace %s.", ui.TitleStyle.Render(target), ui.TitleStyle.Render(client.Namespace))
				} else {
					ui.Info("No sandboxes found in namespace %s.", ui.TitleStyle.Render(client.Namespace))
				}
				return nil
			}

			headers := []string{"SANDBOX ID", "NAME", "CPU", "MEMORY", "STATUS"}
			var rows [][]string
			hasUnknown := false

			for _, m := range metrics {
				if m.CPU == "<unknown>" {
					hasUnknown = true
				}
				rows = append(rows, []string{
					m.ID,
					ui.TitleStyle.Render(m.Name),
					m.CPU,
					m.Memory,
					ui.StatusBadge(m.Status),
				})
			}

			ui.PrintTable(headers, rows)
			if hasUnknown {
				ui.Info("Note: Metrics showing '<unknown>' indicates metrics-server is not reporting for those pods yet.")
			}
			return nil
		}

		if !topWatch {
			return renderMetrics()
		}

		// Watch mode: loop every 2 seconds until interrupted
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(sigCh)

		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		// Initial display
		fmt.Print("\033[H\033[2J") // Clear terminal
		if err := renderMetrics(); err != nil {
			return err
		}

		for {
			select {
			case <-sigCh:
				return nil
			case <-ctx.Done():
				return nil
			case <-ticker.C:
				fmt.Print("\033[H\033[2J") // Clear terminal
				if err := renderMetrics(); err != nil {
					return err
				}
			}
		}
	},
}

func init() {
	topCmd.Flags().BoolVarP(&topWatch, "watch", "w", false, "Watch resource usage in real time")
	RootCmd.AddCommand(topCmd)
}
