package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"campfire/pkg/config"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func resetFlags(cmd *cobra.Command) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		f.Changed = false
		_ = f.Value.Set(f.DefValue)
	})
	for _, sub := range cmd.Commands() {
		resetFlags(sub)
	}
}

func executeCommand(args ...string) (string, error) {
	resetFlags(RootCmd)
	buf := new(bytes.Buffer)
	RootCmd.SetOut(buf)
	RootCmd.SetErr(buf)
	RootCmd.SetArgs(args)

	err := RootCmd.Execute()
	return buf.String(), err
}

func TestConfigSetKubeconfigFlag(t *testing.T) {
	tempDir := t.TempDir()
	tempCfg := filepath.Join(tempDir, "config.json")

	targetPath := "/custom/flag/kubeconfig"
	_, err := executeCommand("config", "set", "--kubeconfig", targetPath, "--config", tempCfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	loaded, _, err := config.Load(tempCfg)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	if loaded.KubeconfigPath != targetPath {
		t.Fatalf("expected KubeconfigPath %s, got %s", targetPath, loaded.KubeconfigPath)
	}
}

func TestConfigSetTokenFlag(t *testing.T) {
	tempDir := t.TempDir()
	tempCfg := filepath.Join(tempDir, "config.json")

	targetToken := "my-secret-api-token-12345"
	_, err := executeCommand("config", "set", "--token", targetToken, "--config", tempCfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	loaded, _, err := config.Load(tempCfg)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	if loaded.Token != targetToken {
		t.Fatalf("expected Token %s, got %s", targetToken, loaded.Token)
	}
}

func TestConfigViewPrecedence(t *testing.T) {
	tempDir := t.TempDir()
	tempCfg := filepath.Join(tempDir, "config.json")

	initial := &config.Config{
		KubeconfigPath: "/stored/kubeconfig/path",
		Token:          "stored-token-123456",
	}
	if err := config.Save(tempCfg, initial); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	origKubeconfig := os.Getenv("KUBECONFIG")
	origToken := os.Getenv("CAMPFIRE_API_TOKEN")
	defer func() {
		os.Setenv("KUBECONFIG", origKubeconfig)
		os.Setenv("CAMPFIRE_API_TOKEN", origToken)
	}()

	// 1. Without env vars: shows (from config)
	os.Unsetenv("KUBECONFIG")
	os.Unsetenv("CAMPFIRE_API_TOKEN")

	rescueStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	_, err := executeCommand("config", "--config", tempCfg)
	w.Close()
	os.Stdout = rescueStdout
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	out := buf.String()

	if !strings.Contains(out, "/stored/kubeconfig/path") || !strings.Contains(out, "(from config)") {
		t.Fatalf("expected /stored/kubeconfig/path (from config), got:\n%s", out)
	}
	if !strings.Contains(out, "(from config)") {
		t.Fatalf("expected token (from config), got:\n%s", out)
	}

	// 2. With KUBECONFIG and CAMPFIRE_API_TOKEN exported: precedence overrides
	os.Setenv("KUBECONFIG", "/exported/env/kubeconfig")
	os.Setenv("CAMPFIRE_API_TOKEN", "exported-secret-token-abcdef")

	r2, w2, _ := os.Pipe()
	os.Stdout = w2

	_, err = executeCommand("config", "--config", tempCfg)
	w2.Close()
	os.Stdout = rescueStdout
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var buf2 bytes.Buffer
	buf2.ReadFrom(r2)
	out2 := buf2.String()

	if !strings.Contains(out2, "/exported/env/kubeconfig") || !strings.Contains(out2, "(from $KUBECONFIG)") {
		t.Fatalf("expected /exported/env/kubeconfig (from $KUBECONFIG), got:\n%s", out2)
	}
	if !strings.Contains(out2, "(from $CAMPFIRE_API_TOKEN)") {
		t.Fatalf("expected token (from $CAMPFIRE_API_TOKEN), got:\n%s", out2)
	}
}
