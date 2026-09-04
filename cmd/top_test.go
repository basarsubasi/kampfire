package cmd

import (
	"strings"
	"testing"
)

func TestTopCommandRegistration(t *testing.T) {
	cmd, _, err := RootCmd.Find([]string{"top"})
	if err != nil || cmd != topCmd {
		t.Errorf("expected 'top' command to be registered under RootCmd")
	}

	if topCmd.Flags().Lookup("watch") == nil {
		t.Errorf("expected --watch flag on topCmd")
	}
	if topCmd.Flags().ShorthandLookup("w") == nil {
		t.Errorf("expected -w shorthand on topCmd")
	}
}

func TestTopCommandArgsValidation(t *testing.T) {
	resetFlags(RootCmd)
	_, err := executeCommand("top", "arg1", "arg2")
	if err == nil {
		t.Fatalf("expected error when passing more than 1 arg to top, got nil")
	}
	if !strings.Contains(err.Error(), "accepts at most 1 arg(s), received 2") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestTopFlagsParsing(t *testing.T) {
	resetFlags(RootCmd)
	topWatch = false

	err := topCmd.ParseFlags([]string{"-w"})
	if err != nil {
		t.Fatalf("unexpected flag parse error: %v", err)
	}
	if !topWatch {
		t.Errorf("expected topWatch to be true with -w")
	}
}
