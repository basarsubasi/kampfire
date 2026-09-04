package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/client-go/tools/clientcmd"
)

// Config represents the Kampfire user configuration.
type Config struct {
	Token          string `json:"token,omitempty"`
	KubeconfigPath string `json:"kubeconfig_path,omitempty"`
}

// DefaultConfigPath returns ~/.config/kampfire/config.json (or ~/.config/campfire/config.json if it exists).
func DefaultConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	kampfirePath := filepath.Join(home, ".config", "kampfire", "config.json")
	if _, err := os.Stat(kampfirePath); err == nil {
		return kampfirePath, nil
	}
	campfirePath := filepath.Join(home, ".config", "campfire", "config.json")
	if _, err := os.Stat(campfirePath); err == nil {
		return campfirePath, nil
	}
	return kampfirePath, nil
}

// Load loads the configuration from the given path, or DefaultConfigPath() if path is empty.
// If the file does not exist, it attempts to import default settings from ~/.kube/config.
func Load(customPath string) (*Config, string, error) {
	path := customPath
	if path == "" {
		var err error
		path, err = DefaultConfigPath()
		if err != nil {
			return nil, "", err
		}
	}

	var cfg *Config
	if data, err := os.ReadFile(path); err == nil {
		var c Config
		if err := json.Unmarshal(data, &c); err != nil {
			return nil, path, fmt.Errorf("failed to parse config at %s: %w", path, err)
		}
		cfg = &c
	} else {
		// Config file does not exist yet. Seed from kubeconfig.
		var seedErr error
		cfg, seedErr = SeedFromKubeconfig()
		if seedErr != nil {
			cfg = &Config{}
		}

		// Try saving seeded config to disk
		_ = Save(path, cfg)
	}

	return cfg, path, nil
}

const KubeconfigEnvVar = "KAMPFIRE_KUBECONFIG"

// NewClientConfigLoadingRules creates clientcmd loading rules that prioritize KAMPFIRE_KUBECONFIG
// and ignore standard KUBECONFIG to prevent dev/prod cluster confusion.
func NewClientConfigLoadingRules(cfg *Config) *clientcmd.ClientConfigLoadingRules {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	// Clear clientcmd's default precedence chain (which embeds $KUBECONFIG)
	loadingRules.Precedence = []string{clientcmd.RecommendedHomeFile}

	kubeconfig := os.Getenv(KubeconfigEnvVar)
	if kubeconfig == "" && cfg != nil {
		kubeconfig = cfg.KubeconfigPath
	}
	if kubeconfig != "" {
		loadingRules.ExplicitPath = kubeconfig
	}
	return loadingRules
}

// ResolveNamespace resolves the active namespace following kubectl precedence:
// 1. Explicit CLI flag override (-n / --namespace)
// 2. Dynamic active context namespace from kubeconfig (how kubectl does it)
// 3. Fallback: "default"
func ResolveNamespace(cliFlag string, cfg *Config) string {
	if cliFlag != "" {
		return cliFlag
	}

	loadingRules := NewClientConfigLoadingRules(cfg)
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})
	if activeNS, _, err := clientConfig.Namespace(); err == nil && activeNS != "" {
		return activeNS
	}

	return "default"
}

// Save writes the configuration to disk.
func Save(path string, cfg *Config) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory %s: %w", dir, err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	return os.WriteFile(path, data, 0600)
}

// SeedFromKubeconfig reads the active context from ~/.kube/config (or KAMPFIRE_KUBECONFIG) and constructs an initial Config.
func SeedFromKubeconfig() (*Config, error) {
	loadingRules := NewClientConfigLoadingRules(nil)
	kubeConfig, err := loadingRules.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	currentContextName := kubeConfig.CurrentContext
	currentContext, exists := kubeConfig.Contexts[currentContextName]

	var token string
	if exists {
		if authInfo, ok := kubeConfig.AuthInfos[currentContext.AuthInfo]; ok {
			token = authInfo.Token
		}
	}

	// Also record default kubeconfig path if found
	kubeconfigPath := loadingRules.GetDefaultFilename()

	return &Config{
		Token:          token,
		KubeconfigPath: kubeconfigPath,
	}, nil
}
