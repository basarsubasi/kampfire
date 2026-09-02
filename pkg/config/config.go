package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/client-go/tools/clientcmd"
)

// Config represents the Campfire user configuration.
type Config struct {
	Server                string `json:"server,omitempty"`
	Token                 string `json:"token,omitempty"`
	Namespace             string `json:"namespace,omitempty"`
	CAData                string `json:"ca_data,omitempty"`
	InsecureSkipTLSVerify bool   `json:"insecure_skip_tls_verify,omitempty"`
	KubeconfigPath        string `json:"kubeconfig_path,omitempty"`
}

// DefaultConfigPath returns ~/.config/campfire/config.json.
func DefaultConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	return filepath.Join(home, ".config", "campfire", "config.json"), nil
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

	if data, err := os.ReadFile(path); err == nil {
		var cfg Config
		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, path, fmt.Errorf("failed to parse config at %s: %w", path, err)
		}
		if cfg.Namespace == "" {
			cfg.Namespace = "default"
		}
		return &cfg, path, nil
	}

	// Config file does not exist yet. Seed from kubeconfig.
	cfg, err := SeedFromKubeconfig()
	if err != nil {
		// Fallback minimal config
		cfg = &Config{
			Namespace: "default",
		}
	}

	// Try saving seeded config to disk
	_ = Save(path, cfg)
	return cfg, path, nil
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

// SeedFromKubeconfig reads the active context from ~/.kube/config and constructs an initial Config.
func SeedFromKubeconfig() (*Config, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	kubeConfig, err := loadingRules.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	currentContextName := kubeConfig.CurrentContext
	currentContext, exists := kubeConfig.Contexts[currentContextName]

	ns := "default"
	if exists && currentContext.Namespace != "" {
		ns = currentContext.Namespace
	}

	var serverURL string
	var caData string
	var insecure bool

	if exists {
		if cluster, ok := kubeConfig.Clusters[currentContext.Cluster]; ok {
			serverURL = cluster.Server
			if len(cluster.CertificateAuthorityData) > 0 {
				caData = string(cluster.CertificateAuthorityData)
			}
			insecure = cluster.InsecureSkipTLSVerify
		}
	}

	var token string
	if exists {
		if authInfo, ok := kubeConfig.AuthInfos[currentContext.AuthInfo]; ok {
			token = authInfo.Token
		}
	}

	// Also record default kubeconfig path if found
	kubeconfigPath := loadingRules.GetDefaultFilename()

	return &Config{
		Server:                serverURL,
		Token:                 token,
		Namespace:             ns,
		CAData:                caData,
		InsecureSkipTLSVerify: insecure,
		KubeconfigPath:        kubeconfigPath,
	}, nil
}
