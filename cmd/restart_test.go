package cmd

import (
	"strings"
	"testing"
)

func TestRestartCommandRegistration(t *testing.T) {
	cmd, _, err := RootCmd.Find([]string{"restart"})
	if err != nil || cmd != restartCmd {
		t.Errorf("expected 'restart' command to be registered under RootCmd")
	}

	if restartCmd.Flags().Lookup("detach") == nil {
		t.Errorf("expected --detach flag on restartCmd")
	}
	if restartCmd.Flags().ShorthandLookup("d") == nil {
		t.Errorf("expected -d shorthand on restartCmd")
	}
}

func TestRestartFlagsValidation(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		errContains string
	}{
		{
			name:        "no args returns error",
			args:        []string{"restart"},
			errContains: "requires at least 1 arg(s)",
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

func TestRestartFlagsBinding(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantDetach bool
	}{
		{
			name:       "single target without flags",
			args:       []string{"restart", "my-sandbox"},
			wantDetach: false,
		},
		{
			name:       "detach flag -d",
			args:       []string{"restart", "-d", "my-sandbox"},
			wantDetach: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetFlags(RootCmd)
			err := restartCmd.ParseFlags(tt.args[1:])
			if err != nil {
				t.Fatalf("unexpected flag parse error: %v", err)
			}
			if restartDetach != tt.wantDetach {
				t.Errorf("restartDetach = %v, want %v", restartDetach, tt.wantDetach)
			}
		})
	}
}
