package cmd

import (
	"strings"
	"testing"
)

func TestRmFlagsValidation(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		errContains string
	}{
		{
			name:        "no args without -a returns error",
			args:        []string{"rm"},
			errContains: "requires at least 1 arg(s), or use -a/--all to remove all sandboxes",
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

func TestRmFlagsBinding(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantAll bool
	}{
		{
			name:    "single target without flag",
			args:    []string{"rm", "my-sandbox"},
			wantAll: false,
		},
		{
			name:    "shorthand -a flag",
			args:    []string{"rm", "-a"},
			wantAll: true,
		},
		{
			name:    "long flag --all",
			args:    []string{"rm", "--all"},
			wantAll: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetFlags(RootCmd)
			err := rmCmd.ParseFlags(tt.args[1:])
			if err != nil {
				t.Fatalf("unexpected flag parse error on rmCmd: %v", err)
			}

			if rmAll != tt.wantAll {
				t.Errorf("rmAll = %v, want %v", rmAll, tt.wantAll)
			}
		})
	}
}
