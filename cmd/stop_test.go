package cmd

import (
	"strings"
	"testing"
)

func TestStopCommandRegistration(t *testing.T) {
	cmd, _, err := RootCmd.Find([]string{"stop"})
	if err != nil || cmd != stopCmd {
		t.Errorf("expected 'stop' command to be registered under RootCmd")
	}

	if stopCmd.Flags().Lookup("all") == nil {
		t.Errorf("expected --all flag on stopCmd")
	}
	if stopCmd.Flags().ShorthandLookup("a") == nil {
		t.Errorf("expected -a shorthand on stopCmd")
	}
}

func TestStopFlagsValidation(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		errContains string
	}{
		{
			name:        "no args without -a returns error",
			args:        []string{"stop"},
			errContains: "requires at least 1 arg(s), or use -a/--all to stop all sandboxes",
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

func TestStopFlagsBinding(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantAll bool
	}{
		{
			name:    "single target without flag",
			args:    []string{"stop", "my-sandbox"},
			wantAll: false,
		},
		{
			name:    "shorthand -a flag",
			args:    []string{"stop", "-a"},
			wantAll: true,
		},
		{
			name:    "long flag --all",
			args:    []string{"stop", "--all"},
			wantAll: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetFlags(RootCmd)
			err := stopCmd.ParseFlags(tt.args[1:])
			if err != nil {
				t.Fatalf("unexpected flag parse error: %v", err)
			}
			if stopAll != tt.wantAll {
				t.Errorf("stopAll = %v, want %v", stopAll, tt.wantAll)
			}
		})
	}
}
