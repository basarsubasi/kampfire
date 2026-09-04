package cmd

import (
	"strings"
	"testing"
)

func TestStartCommandRegistration(t *testing.T) {
	cmd, _, err := RootCmd.Find([]string{"start"})
	if err != nil || cmd != startCmd {
		t.Errorf("expected 'start' command to be registered under RootCmd")
	}

	if startCmd.Flags().Lookup("all") == nil {
		t.Errorf("expected --all flag on startCmd")
	}
	if startCmd.Flags().ShorthandLookup("a") == nil {
		t.Errorf("expected -a shorthand on startCmd")
	}
	if startCmd.Flags().Lookup("detach") == nil {
		t.Errorf("expected --detach flag on startCmd")
	}
	if startCmd.Flags().ShorthandLookup("d") == nil {
		t.Errorf("expected -d shorthand on startCmd")
	}
}

func TestStartFlagsValidation(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		errContains string
	}{
		{
			name:        "no args without -a returns error",
			args:        []string{"start"},
			errContains: "requires at least 1 arg(s), or use -a/--all to start all sandboxes",
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

func TestStartFlagsBinding(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantAll    bool
		wantDetach bool
	}{
		{
			name:       "single target without flags",
			args:       []string{"start", "my-sandbox"},
			wantAll:    false,
			wantDetach: false,
		},
		{
			name:       "shorthand -a flag",
			args:       []string{"start", "-a"},
			wantAll:    true,
			wantDetach: false,
		},
		{
			name:       "detach flag -d",
			args:       []string{"start", "-d", "my-sandbox"},
			wantAll:    false,
			wantDetach: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetFlags(RootCmd)
			err := startCmd.ParseFlags(tt.args[1:])
			if err != nil {
				t.Fatalf("unexpected flag parse error: %v", err)
			}
			if startAll != tt.wantAll {
				t.Errorf("startAll = %v, want %v", startAll, tt.wantAll)
			}
			if startDetach != tt.wantDetach {
				t.Errorf("startDetach = %v, want %v", startDetach, tt.wantDetach)
			}
		})
	}
}
