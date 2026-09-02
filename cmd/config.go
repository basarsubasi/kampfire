package cmd

import (
	"fmt"
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

		tokenDisplay := "(not configured)"
		if cfg.Token != "" {
			if len(cfg.Token) > 12 {
				tokenDisplay = cfg.Token[:6] + "..." + cfg.Token[len(cfg.Token)-4:]
			} else {
				tokenDisplay = "********"
			}
		}

		serverDisplay := cfg.Server
		if serverDisplay == "" {
			serverDisplay = "(from kubeconfig)"
		}

		ui.Info("Campfire Configuration (%s):", path)
		fmt.Printf("  • %-15s: %s\n", "Namespace", ui.TitleStyle.Render(cfg.Namespace))
		fmt.Printf("  • %-15s: %s\n", "API Server", serverDisplay)
		fmt.Printf("  • %-15s: %s\n", "API Token", tokenDisplay)
		if cfg.KubeconfigPath != "" {
			fmt.Printf("  • %-15s: %s\n", "Kubeconfig", cfg.KubeconfigPath)
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
		if setNamespace != "" {
			cfg.Namespace = setNamespace
			updated = true
		}
		if setToken != "" {
			cfg.Token = strings.TrimSpace(setToken)
			updated = true
		}
		if setServer != "" {
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
