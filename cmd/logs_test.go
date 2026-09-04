package cmd

import (
	"strings"
	"testing"
)

func TestLogsFlagsValidation(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		errContains string
	}{
		{
			name:        "no args returns error",
			args:        []string{"logs"},
			errContains: "accepts 1 arg(s), received 0",
		},
		{
			name:        "both head and tail specified returns error",
			args:        []string{"logs", "my-box", "--head", "10", "--tail", "10"},
			errContains: "cannot specify both --tail and --head",
		},
		{
			name:        "negative tail returns error",
			args:        []string{"logs", "my-box", "--tail", "-5"},
			errContains: "--tail cannot be negative",
		},
		{
			name:        "negative head returns error",
			args:        []string{"logs", "my-box", "--head", "-3"},
			errContains: "--head cannot be negative",
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

func TestLogsFlagsBinding(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		wantFollow     bool
		wantTimestamps bool
		wantTail       int64
		wantHead       int64
	}{
		{
			name:           "default flags",
			args:           []string{"logs", "my-box"},
			wantFollow:     false,
			wantTimestamps: false,
			wantTail:       -1,
			wantHead:       -1,
		},
		{
			name:           "shorthand -f and -t",
			args:           []string{"logs", "my-box", "-f", "-t"},
			wantFollow:     true,
			wantTimestamps: true,
			wantTail:       -1,
			wantHead:       -1,
		},
		{
			name:           "long flags --follow and --timestamps",
			args:           []string{"logs", "my-box", "--follow", "--timestamps"},
			wantFollow:     true,
			wantTimestamps: true,
			wantTail:       -1,
			wantHead:       -1,
		},
		{
			name:           "follow with tail",
			args:           []string{"logs", "my-box", "-f", "--tail", "42"},
			wantFollow:     true,
			wantTimestamps: false,
			wantTail:       42,
			wantHead:       -1,
		},
		{
			name:           "timestamps with head",
			args:           []string{"logs", "my-box", "-t", "--head", "15"},
			wantFollow:     false,
			wantTimestamps: true,
			wantTail:       -1,
			wantHead:       15,
		},
		{
			name:           "all supported non-conflicting flags combined",
			args:           []string{"logs", "my-box", "-f", "-t", "--tail", "100"},
			wantFollow:     true,
			wantTimestamps: true,
			wantTail:       100,
			wantHead:       -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetFlags(RootCmd)
			err := logsCmd.ParseFlags(tt.args[1:])
			if err != nil {
				t.Fatalf("unexpected flag parse error on logsCmd: %v", err)
			}

			if logsFollow != tt.wantFollow {
				t.Errorf("logsFollow = %v, want %v", logsFollow, tt.wantFollow)
			}
			if logsTimestamps != tt.wantTimestamps {
				t.Errorf("logsTimestamps = %v, want %v", logsTimestamps, tt.wantTimestamps)
			}
			if logsTail != tt.wantTail {
				t.Errorf("logsTail = %v, want %v", logsTail, tt.wantTail)
			}
			if logsHead != tt.wantHead {
				t.Errorf("logsHead = %v, want %v", logsHead, tt.wantHead)
			}
		})
	}
}
