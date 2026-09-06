package cmd

import (
	"strings"
	"testing"
)

func TestExecCommandRegistration(t *testing.T) {
	cmd, _, err := RootCmd.Find([]string{"exec"})
	if err != nil || cmd != execCmd {
		t.Errorf("expected 'exec' command to be registered under RootCmd")
	}

	if execCmd.Flags().Lookup("interactive") == nil {
		t.Errorf("expected --interactive flag on execCmd")
	}
	if execCmd.Flags().ShorthandLookup("i") == nil {
		t.Errorf("expected -i shorthand on execCmd")
	}
	if execCmd.Flags().Lookup("tty") == nil {
		t.Errorf("expected --tty flag on execCmd")
	}
	if execCmd.Flags().ShorthandLookup("t") == nil {
		t.Errorf("expected -t shorthand on execCmd")
	}
}

func TestExecArgsValidation(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		errContains string
	}{
		{
			name:        "no args returns error",
			args:        []string{"exec"},
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

func TestExecFlagsBinding(t *testing.T) {
	tests := []struct {
		name            string
		args            []string
		wantInteractive bool
		wantTTY         bool
	}{
		{
			name:            "no flags",
			args:            []string{"exec", "my-box", "echo", "hi"},
			wantInteractive: false,
			wantTTY:         false,
		},
		{
			name:            "interactive shorthand",
			args:            []string{"exec", "-i", "my-box"},
			wantInteractive: true,
			wantTTY:         false,
		},
		{
			name:            "tty shorthand",
			args:            []string{"exec", "-t", "my-box"},
			wantInteractive: false,
			wantTTY:         true,
		},
		{
			name:            "combined shorthand -it",
			args:            []string{"exec", "-it", "my-box", "/bin/sh"},
			wantInteractive: true,
			wantTTY:         true,
		},
		{
			name:            "long flags",
			args:            []string{"exec", "--interactive", "--tty", "my-box"},
			wantInteractive: true,
			wantTTY:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetFlags(RootCmd)
			execInteractive = false
			execTTY = false

			err := execCmd.ParseFlags(tt.args[1:])
			if err != nil {
				t.Fatalf("unexpected parse error: %v", err)
			}

			if execInteractive != tt.wantInteractive {
				t.Errorf("execInteractive = %v, want %v", execInteractive, tt.wantInteractive)
			}
			if execTTY != tt.wantTTY {
				t.Errorf("execTTY = %v, want %v", execTTY, tt.wantTTY)
			}
		})
	}
}
