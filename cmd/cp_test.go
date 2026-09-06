package cmd

import (
	"strings"
	"testing"
)

func TestCpCommandRegistration(t *testing.T) {
	cmd, _, err := RootCmd.Find([]string{"cp"})
	if err != nil || cmd != cpCmd {
		t.Errorf("expected 'cp' command to be registered under RootCmd")
	}
}

func TestCpArgsValidation(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		errContains string
	}{
		{
			name:        "no args returns error",
			args:        []string{"cp"},
			errContains: "accepts 2 arg(s), received 0",
		},
		{
			name:        "1 arg returns error",
			args:        []string{"cp", "source.txt"},
			errContains: "accepts 2 arg(s), received 1",
		},
		{
			name:        "3 args returns error",
			args:        []string{"cp", "src1", "src2", "dest"},
			errContains: "accepts 2 arg(s), received 3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := executeCommand(tt.args...)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.errContains)
			}
			if !strings.Contains(err.Error(), tt.errContains) {
				t.Fatalf("expected error containing %q, got %q", tt.errContains, err.Error())
			}
		})
	}
}
