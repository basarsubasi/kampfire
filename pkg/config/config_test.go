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

	// Export KUBECONFIG and CAMPFIRE_API_TOKEN
	origKube := os.Getenv("KUBECONFIG")
	origToken := os.Getenv("CAMPFIRE_API_TOKEN")
	defer func() {
		os.Setenv("KUBECONFIG", origKube)
		os.Setenv("CAMPFIRE_API_TOKEN", origToken)
	}()
	os.Setenv("KUBECONFIG", "/temporary/env/kubeconfig")
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
