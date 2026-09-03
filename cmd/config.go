package cmd

import (
	"fmt"
	"os"
	"strings"

	"campfire/pkg/config"
	"campfire/pkg/ui"

	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	setToken      string
	setKubeconfig string
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "View or update Campfire configuration (~/.config/campfire/config.json)",
	Long:  `Inspects or sets active API token and kubeconfig path.`,
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

		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		kubePath := os.Getenv("KUBECONFIG")
		kubeSource := ""
		if kubePath != "" {
			kubeSource = " " + ui.MutedStyle.Render("(from $KUBECONFIG)")
			loadingRules.ExplicitPath = kubePath
		} else if cfg.KubeconfigPath != "" {
			kubePath = cfg.KubeconfigPath
			kubeSource = " " + ui.MutedStyle.Render("(from config)")
			loadingRules.ExplicitPath = kubePath
		}
		clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})

		serverDisplay := "(from kubeconfig)"
		if rc, err := clientConfig.ClientConfig(); err == nil && rc.Host != "" {
			serverDisplay = rc.Host
		}

		activeNS := config.ResolveNamespace(namespaceFlag, cfg)
		source := "(from context)"
		if namespaceFlag != "" {
			source = "(from -n flag)"
		}

		ui.Info("Campfire Configuration (%s):", path)
		fmt.Printf("  • %-15s: %s %s\n", "Namespace", ui.TitleStyle.Render(activeNS), ui.MutedStyle.Render(source))
		fmt.Printf("  • %-15s: %s\n", "API Server", serverDisplay)
		fmt.Printf("  • %-15s: %s\n", "API Token", tokenDisplay)
		if kubePath != "" {
			fmt.Printf("  • %-15s: %s%s\n", "Kubeconfig", kubePath, kubeSource)
		}
		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set configuration parameters",
	Example: `  # Set user API token
  campfire config set --token eyJhbGciOi...

  # Set default kubeconfig path
  campfire config set --kubeconfig ~/.kube/custom-config`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, path, err := config.Load(cfgPath)
		if err != nil {
			return err
		}

		updated := false
		if cmd.Flags().Changed("token") {
			cfg.Token = strings.TrimSpace(setToken)
			updated = true
		}
		if cmd.Flags().Changed("kubeconfig") {
			cfg.KubeconfigPath = strings.TrimSpace(setKubeconfig)
			updated = true
		}

		if !updated {
			return fmt.Errorf("no settings provided; specify at least one of --token, --kubeconfig")
		}

		if err := config.Save(path, cfg); err != nil {
			return err
		}

		ui.Success("Updated configuration at %s", path)
		return nil
	},
}

func init() {
	configSetCmd.Flags().StringVar(&setToken, "token", "", "Set the Kubernetes API bearer token")
	configSetCmd.Flags().StringVar(&setKubeconfig, "kubeconfig", "", "Set the default kubeconfig path")

	configCmd.AddCommand(configSetCmd)
	RootCmd.AddCommand(configCmd)
}
