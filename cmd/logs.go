package cmd

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/basarsubasi/kampfire/pkg/sandbox"

	"github.com/spf13/cobra"
)

var (
	logsFollow     bool
	logsTail       int64
	logsHead       int64
	logsTimestamps bool
)

var logsCmd = &cobra.Command{
	Use:   "logs [flags] SANDBOX_ID",
	Short: "Fetch or stream the logs of a sandbox container",
	Long: `Prints standard output and standard error logs from the sandbox container.
Supports tailing the most recent lines (--tail), viewing the initial lines (--head),
and following log output in real time (--follow / -f).`,
	Example: `  # Fetch all logs for a sandbox
  kampfire logs my-sandbox

  # Follow logs in real time
  kampfire logs -f my-sandbox

  # Output the last 50 lines of logs
  kampfire logs --tail 50 my-sandbox

  # Output the first 20 lines of logs
  kampfire logs --head 20 my-sandbox

  # Follow new logs starting from the last 10 lines
  kampfire logs -f --tail 10 my-sandbox`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := sandbox.LogOptions{
			Follow:     logsFollow,
			Tail:       logsTail,
			Head:       logsHead,
			Timestamps: logsTimestamps,
		}

		if err := opts.Validate(); err != nil {
			return err
		}

		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		client, _, err := GetClient()
		if err != nil {
			return err
		}

		target := args[0]
		sandboxName := target
		if sb, err := sandbox.Find(ctx, client, target); err == nil {
			sandboxName = sb.Name
		}

		err = sandbox.StreamLogs(ctx, client, sandboxName, opts, cmd.OutOrStdout())
		if err != nil && !errors.Is(err, context.Canceled) {
			return err
		}

		return nil
	},
}

func init() {
	logsCmd.Flags().BoolVarP(&logsFollow, "follow", "f", false, "Follow log output")
	logsCmd.Flags().Int64Var(&logsTail, "tail", -1, "Number of lines to show from the end of the logs (default -1, shows all)")
	logsCmd.Flags().Int64Var(&logsHead, "head", -1, "Number of lines to show from the beginning of the logs (default -1, shows all)")
	logsCmd.Flags().BoolVarP(&logsTimestamps, "timestamps", "t", false, "Show timestamps")

	RootCmd.AddCommand(logsCmd)
}
