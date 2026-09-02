package cmd

import (
	"fmt"
	"os"
	"strings"

	"campfire/pkg/config"
	"campfire/pkg/ui"

	"github.com/spf13/cobra"
)

var (
	setServer    string
	setToken     string
	setNamespace string
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "View or update Campfire configuration (~/.config/campfire/config.json)",
	Long:  `Inspects or sets active API token, server endpoint, and scoped namespace.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, path, err := config.Load(cfgPath)
		if err != nil {
			return err
		}

		token := os.Getenv("CAMPFIRE_API_TOKEN")
		tokenSource := ""
		if token != "" {
			tokenSource = " " + ui.MutedStyle.Render("(from $CAMPFIRE_API_TOKEN)")
		} else if cfg.Token != "" {
			token = cfg.Token
			tokenSource = " " + ui.MutedStyle.Render("(from config)")
		}

		tokenDisplay := "(not configured)"
		if token != "" {
			if len(token) > 12 {
				tokenDisplay = token[:6] + "..." + token[len(token)-4:] + tokenSource
			} else {
				tokenDisplay = "********" + tokenSource
			}
		}

		serverDisplay := cfg.Server
		if serverDisplay == "" {
			serverDisplay = "(from kubeconfig)"
		}

		activeNS := config.ResolveNamespace(namespaceFlag, cfg)
		source := "(from context)"
		if namespaceFlag != "" {
			source = "(from -n flag)"
		} else if cfg.Namespace != "" {
			source = "(from config)"
		}

		ui.Info("Campfire Configuration (%s):", path)
		fmt.Printf("  • %-15s: %s %s\n", "Namespace", ui.TitleStyle.Render(activeNS), ui.MutedStyle.Render(source))
		fmt.Printf("  • %-15s: %s\n", "API Server", serverDisplay)
		fmt.Printf("  • %-15s: %s\n", "API Token", tokenDisplay)
		if cfg.KubeconfigPath != "" {
			source := ""
			if os.Getenv("KUBECONFIG") != "" {
				source = " " + ui.MutedStyle.Render("(from $KUBECONFIG)")
			}
			fmt.Printf("  • %-15s: %s%s\n", "Kubeconfig", cfg.KubeconfigPath, source)
		}
		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set configuration parameters",
	Example: `  # Set user API token
  campfire config set --token eyJhbGciOi...

  # Set scoped namespace
  campfire config set --namespace user-alice`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, path, err := config.Load(cfgPath)
		if err != nil {
			return err
		}

		updated := false
		if cmd.Flags().Changed("namespace") {
			cfg.Namespace = setNamespace
			updated = true
		}
		if cmd.Flags().Changed("token") {
			cfg.Token = strings.TrimSpace(setToken)
			updated = true
		}
		if cmd.Flags().Changed("server") {
			cfg.Server = strings.TrimSpace(setServer)
			updated = true
		}

		if !updated {
			return fmt.Errorf("no settings provided; specify at least one of --token, --namespace, --server")
		}

		if err := config.Save(path, cfg); err != nil {
			return err
		}

		ui.Success("Updated configuration at %s", path)
		return nil
	},
}

func init() {
	configSetCmd.Flags().StringVar(&setNamespace, "namespace", "", "Set the scoped user namespace")
	configSetCmd.Flags().StringVar(&setToken, "token", "", "Set the Kubernetes API bearer token")
	configSetCmd.Flags().StringVar(&setServer, "server", "", "Set the Kubernetes API server URL")

	configCmd.AddCommand(configSetCmd)
	RootCmd.AddCommand(configCmd)
}
