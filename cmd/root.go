package cmd

import (
	"fmt"
	"os"

	"campfire/pkg/config"
	"campfire/pkg/k8s"
	"campfire/pkg/ui"

	"github.com/spf13/cobra"
)

var (
	cfgPath       string
	namespaceFlag string
	verbose       bool

	loadedConfig *config.Config
	k8sClient    *k8s.Client
)

// RootCmd is the base command for campfire.
var RootCmd = &cobra.Command{
	Use:   "campfire",
	Short: "🔥 campfire - Docker-style CLI for Kubernetes Agent Sandboxes",
	Long: `campfire is a developer-first CLI adhering to Docker conventions on top of
Kubernetes SIG Agent Sandbox (agents.x-k8s.io).

It manages sandboxes directly within your configured namespace, offering fast provisioning,
realistic interactive PTY terminal sessions, file transfer, and one-command VS Code IDE launch.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	defaultCfg, _ := config.DefaultConfigPath()
	RootCmd.PersistentFlags().StringVar(&cfgPath, "config", defaultCfg, "path to campfire config file")
	RootCmd.PersistentFlags().StringVarP(&namespaceFlag, "namespace", "n", "", "Kubernetes namespace (defaults to config or active context)")
	RootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose debug logging")
}

// GetClient initializes and returns the Kubernetes client and config.
func GetClient() (*k8s.Client, *config.Config, error) {
	if k8sClient != nil {
		if namespaceFlag != "" {
			k8sClient.Namespace = namespaceFlag
		}
		return k8sClient, loadedConfig, nil
	}

	cfg, path, err := config.Load(cfgPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load campfire config: %w", err)
	}
	loadedConfig = cfg

	client, err := k8s.NewClient(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize kubernetes client (config at %s): %w", path, err)
	}
	if namespaceFlag != "" {
		client.Namespace = namespaceFlag
	}
	k8sClient = client

	return k8sClient, loadedConfig, nil
}

// Execute runs the root command.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		ui.Error("%s", err)
		os.Exit(1)
	}
}
