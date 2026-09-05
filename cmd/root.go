package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/basarsubasi/kampfire/pkg/config"
	"github.com/basarsubasi/kampfire/pkg/k8s"
	"github.com/basarsubasi/kampfire/pkg/ui"

	"github.com/spf13/cobra"
	executil "k8s.io/client-go/util/exec"
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
	Use:   "kampfire",
	Short: "kampfire - Docker-style CLI for Kubernetes Agent Sandboxes",
	Long: `kampfire is a developer-first CLI adhering to Docker conventions on top of
Kubernetes SIG Agent Sandbox (agents.x-k8s.io).

It manages sandboxes directly within your configured namespace, offering fast provisioning,
realistic interactive PTY terminal sessions, file transfer, and one-command VS Code IDE launch.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	defaultCfg, _ := config.DefaultConfigPath()
	RootCmd.PersistentFlags().StringVar(&cfgPath, "config", defaultCfg, "path to kampfire config file")
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
		return nil, nil, fmt.Errorf("failed to load kampfire config: %w", err)
	}
	loadedConfig = cfg

	client, err := k8s.NewClient(cfg, namespaceFlag)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize kubernetes client (config at %s): %w", path, err)
	}
	k8sClient = client

	return k8sClient, loadedConfig, nil
}

// Execute runs the root command.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		ui.Error("%s", err)
		var exitErr executil.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.ExitStatus())
		}
		os.Exit(1)
	}
}