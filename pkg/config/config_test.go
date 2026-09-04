package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDoesNotMutateStoredKubeconfigWithEnv(t *testing.T) {
	tempDir := t.TempDir()
	cfgFile := filepath.Join(tempDir, "config.json")

	initialCfg := &Config{
		KubeconfigPath: "/initial/stored/kubeconfig",
		Token:          "initial-token",
	}
	if err := Save(cfgFile, initialCfg); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	// Export KAMPFIRE_KUBECONFIG and CAMPFIRE_API_TOKEN
	origKube := os.Getenv(KubeconfigEnvVar)
	origToken := os.Getenv("CAMPFIRE_API_TOKEN")
	defer func() {
		os.Setenv(KubeconfigEnvVar, origKube)
		os.Setenv("CAMPFIRE_API_TOKEN", origToken)
	}()
	os.Setenv(KubeconfigEnvVar, "/temporary/env/kubeconfig")
	os.Setenv("CAMPFIRE_API_TOKEN", "temporary-env-token")

	// Load should load stored config without mutating KubeconfigPath inside cfg struct
	loadedCfg, _, err := Load(cfgFile)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	if loadedCfg.KubeconfigPath != "/initial/stored/kubeconfig" {
		t.Fatalf("expected loaded KubeconfigPath to be /initial/stored/kubeconfig, got %s", loadedCfg.KubeconfigPath)
	}
	if loadedCfg.Token != "initial-token" {
		t.Fatalf("expected loaded Token to be initial-token, got %s", loadedCfg.Token)
	}

	// Save updated token and ensure KubeconfigPath on disk is not corrupted by env var
	loadedCfg.Token = "updated-token"
	if err := Save(cfgFile, loadedCfg); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	reloaded, _, err := Load(cfgFile)
	if err != nil {
		t.Fatalf("failed to reload config: %v", err)
	}
	if reloaded.KubeconfigPath != "/initial/stored/kubeconfig" {
		t.Fatalf("expected persisted KubeconfigPath to remain /initial/stored/kubeconfig, got %s", reloaded.KubeconfigPath)
	}
}

func TestNewClientConfigLoadingRules_IgnoresClassicKubeconfig(t *testing.T) {
	origClassic := os.Getenv("KUBECONFIG")
	origKampfire := os.Getenv(KubeconfigEnvVar)
	defer func() {
		os.Setenv("KUBECONFIG", origClassic)
		os.Setenv(KubeconfigEnvVar, origKampfire)
	}()

	// 1. Classic KUBECONFIG is set, but KAMPFIRE_KUBECONFIG is not
	os.Setenv("KUBECONFIG", "/classic/cluster/kubeconfig")
	os.Unsetenv(KubeconfigEnvVar)

	rules := NewClientConfigLoadingRules(nil)
	if rules.ExplicitPath != "" {
		t.Errorf("expected empty ExplicitPath when KAMPFIRE_KUBECONFIG is unset, got %s", rules.ExplicitPath)
	}
	for _, p := range rules.Precedence {
		if p == "/classic/cluster/kubeconfig" {
			t.Errorf("expected classic KUBECONFIG to NOT be present in precedence chain, but found it")
		}
	}

	// 2. KAMPFIRE_KUBECONFIG is set
	os.Setenv(KubeconfigEnvVar, "/kampfire/dev/kubeconfig")
	rulesWithKampfire := NewClientConfigLoadingRules(nil)
	if rulesWithKampfire.ExplicitPath != "/kampfire/dev/kubeconfig" {
		t.Errorf("expected ExplicitPath to be /kampfire/dev/kubeconfig, got %s", rulesWithKampfire.ExplicitPath)
	}

	// 3. Fallback to cfg.KubeconfigPath when KAMPFIRE_KUBECONFIG is unset
	os.Unsetenv(KubeconfigEnvVar)
	cfg := &Config{KubeconfigPath: "/stored/path/kubeconfig"}
	rulesWithCfg := NewClientConfigLoadingRules(cfg)
	if rulesWithCfg.ExplicitPath != "/stored/path/kubeconfig" {
		t.Errorf("expected ExplicitPath to be /stored/path/kubeconfig, got %s", rulesWithCfg.ExplicitPath)
	}
}
