package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestCompletionCommandRegistration(t *testing.T) {
	cmd, _, err := RootCmd.Find([]string{"completion"})
	if err != nil || cmd != completionCmd {
		t.Errorf("expected 'completion' command under RootCmd")
	}
}

func TestCompletionShellGeneration(t *testing.T) {
	shells := []string{"bash", "zsh", "fish", "powershell"}
	for _, sh := range shells {
		t.Run(sh, func(t *testing.T) {
			buf := new(bytes.Buffer)
			completionCmd.SetOut(buf)
			err := completionCmd.RunE(completionCmd, []string{sh})
			if err != nil {
				t.Fatalf("completion %s failed: %v", sh, err)
			}
		})
	}
}

func TestCompletionInvalidShell(t *testing.T) {
	resetFlags(RootCmd)
	_, err := executeCommand("completion", "unknown-shell")
	if err == nil {
		t.Fatalf("expected error for unsupported shell, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported shell type") && !strings.Contains(err.Error(), "invalid argument") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCompleteSandboxNames_LimitArgs(t *testing.T) {
	// If a single-target command like exec already has an arg, it should return nil
	results, directive := completeSandboxNames(execCmd, []string{"existing-target"}, "")
	if len(results) != 0 {
		t.Errorf("expected no suggestions when target is already provided, got %v", results)
	}
	_ = directive
}
