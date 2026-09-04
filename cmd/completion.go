package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/basarsubasi/kampfire/pkg/sandbox"

	"github.com/spf13/cobra"
)

// completeSandboxNames provides dynamic tab completion for sandbox names and IDs across commands.
func completeSandboxNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// If the command already received its target argument, do not offer more sandbox completions
	if len(args) > 0 && cmd.Name() != "rm" && cmd.Name() != "port-forward" {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	client, _, err := GetClient()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	sandboxes, err := sandbox.List(context.Background(), client, true)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var suggestions []string
	for _, sb := range sandboxes {
		desc := fmt.Sprintf("%s - %s", sb.ID, sb.Status)
		if strings.HasPrefix(sb.Name, toComplete) {
			suggestions = append(suggestions, fmt.Sprintf("%s\t(%s)", sb.Name, desc))
		}
		if strings.HasPrefix(sb.ID, toComplete) {
			suggestions = append(suggestions, fmt.Sprintf("%s\t(%s - %s)", sb.ID, sb.Name, sb.Status))
		}
	}

	return suggestions, cobra.ShellCompDirectiveNoFileComp
}

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion scripts",
	Long: `Generate shell completion scripts for kampfire commands and flags.
Supports dynamic autocompletion of sandbox names and IDs for all commands.

Installation instructions:

  Bash:
    $ source <(kampfire completion bash)
    # To load completions automatically in future sessions:
    $ kampfire completion bash > /etc/bash_completion.d/kampfire

  Zsh:
    # If shell completion is not already enabled in your environment:
    $ echo "autoload -U compinit; compinit" >> ~/.zshrc
    $ source <(kampfire completion zsh)
    # To load completions automatically in future sessions:
    $ kampfire completion zsh > "${fpath[1]}/_kampfire"

  Fish:
    $ kampfire completion fish | source
    # To load completions automatically in future sessions:
    $ kampfire completion fish > ~/.config/fish/completions/kampfire.fish

  PowerShell:
    PS> kampfire completion powershell | Out-String | Invoke-Expression
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		out := cmd.OutOrStdout()
		switch args[0] {
		case "bash":
			return RootCmd.GenBashCompletion(out)
		case "zsh":
			return RootCmd.GenZshCompletion(out)
		case "fish":
			return RootCmd.GenFishCompletion(out, true)
		case "powershell":
			return RootCmd.GenPowerShellCompletionWithDesc(out)
		default:
			return fmt.Errorf("unsupported shell type %q: supported shells are bash, zsh, fish, powershell", args[0])
		}
	},
}

func init() {
	RootCmd.AddCommand(completionCmd)
}
