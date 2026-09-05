package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/basarsubasi/kampfire/pkg/config"
	"github.com/basarsubasi/kampfire/pkg/ui"

	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	setToken      string
	setKubeconfig string
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "View or update Campfire configuration (~/.config/kampfire/config.json)",
	Long:  `Inspects or sets active API token and kubeconfig path.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, path, err := config.Load(cfgPath)
		if err != nil {
			return err
		}

		token := os.Getenv("KAMPFIRE_API_TOKEN")
		tokenSource := ""
		if token != "" {
			tokenSource = " " + ui.MutedStyle.Render("(from $KAMPFIRE_API_TOKEN)")
		} else if legacyToken := os.Getenv("CAMPFIRE_API_TOKEN"); legacyToken != "" {
			token = legacyToken
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

		loadingRules := config.NewClientConfigLoadingRules(cfg)
		kubePath := os.Getenv(config.KubeconfigEnvVar)
		kubeSource := ""
		if kubePath != "" {
			kubeSource = " " + ui.MutedStyle.Render(fmt.Sprintf("(from $%s)", config.KubeconfigEnvVar))
		} else if cfg.KubeconfigPath != "" {
			kubePath = cfg.KubeconfigPath
			kubeSource = " " + ui.MutedStyle.Render("(from config)")
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

		ui.Info("Kampfire Configuration (%s):", path)
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
  kampfire config set --token eyJhbGciOi...

  # Set default kubeconfig path
  kampfire config set --kubeconfig ~/.kube/custom-config`,
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
			val := strings.TrimSpace(setKubeconfig)
			if val != "" {
				if strings.HasPrefix(val, "~/") || val == "~" {
					if home, err := os.UserHomeDir(); err == nil {
						if val == "~" {
							val = home
						} else {
							val = filepath.Join(home, val[2:])
						}
					}
				}
				absPath, err := filepath.Abs(val)
				if err != nil {
					return fmt.Errorf("failed to resolve absolute path for %s: %w", val, err)
				}
				cfg.KubeconfigPath = absPath
			} else {
				cfg.KubeconfigPath = ""
			}
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
