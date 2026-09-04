package cmd

import (
	"strings"
	"testing"
)

func TestIDECommandsRegistration(t *testing.T) {
	subCmds := ideCmd.Commands()
	names := make(map[string]bool)
	for _, c := range subCmds {
		names[c.Name()] = true
		for _, alias := range c.Aliases {
			names[alias] = true
		}
	}

	if !names["vscode"] {
		t.Errorf("expected 'vscode' command under ideCmd")
	}
	if !names["code"] {
		t.Errorf("expected 'code' alias under ideCmd")
	}
	if !names["antigravity"] {
		t.Errorf("expected 'antigravity' command under ideCmd")
	}
	if !names["agy"] {
		t.Errorf("expected 'agy' alias for antigravity under ideCmd")
	}
}

func TestIDEFlagsValidation(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		errContains string
	}{
		{
			name:        "vscode no args returns error",
			args:        []string{"ide", "vscode"},
			errContains: "accepts 1 arg(s), received 0",
		},
		{
			name:        "code alias no args returns error",
			args:        []string{"ide", "code"},
			errContains: "accepts 1 arg(s), received 0",
		},
		{
			name:        "antigravity no args returns error",
			args:        []string{"ide", "antigravity"},
			errContains: "accepts 1 arg(s), received 0",
		},
		{
			name:        "agy alias no args returns error",
			args:        []string{"ide", "agy"},
			errContains: "accepts 1 arg(s), received 0",
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

func TestIDEFlagsBinding(t *testing.T) {
	tests := []struct {
		name        string
		cmd         string
		args        []string
		wantBrowser bool
	}{
		{
			name:        "vscode default desktop",
			cmd:         "vscode",
			args:        []string{"ide", "vscode", "my-box"},
			wantBrowser: false,
		},
		{
			name:        "vscode browser flag",
			cmd:         "vscode",
			args:        []string{"ide", "vscode", "my-box", "--browser"},
			wantBrowser: true,
		},
		{
			name:        "antigravity default desktop",
			cmd:         "antigravity",
			args:        []string{"ide", "antigravity", "my-box"},
			wantBrowser: false,
		},
		{
			name:        "antigravity browser flag",
			cmd:         "antigravity",
			args:        []string{"ide", "antigravity", "my-box", "--browser"},
			wantBrowser: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetFlags(RootCmd)
			var targetCmd = vscodeCmd
			if tt.cmd == "antigravity" {
				targetCmd = agyCmd
			}
			err := targetCmd.ParseFlags(tt.args[2:])
			if err != nil {
				t.Fatalf("unexpected flag parse error: %v", err)
			}
			if ideBrowser != tt.wantBrowser {
				t.Errorf("ideBrowser = %v, want %v", ideBrowser, tt.wantBrowser)
			}
		})
	}
}
