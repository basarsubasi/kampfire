package cmd

import (
	"testing"
)

func TestPsCommandRegistration(t *testing.T) {
	cmd, _, err := RootCmd.Find([]string{"ps"})
	if err != nil || cmd != psCmd {
		t.Errorf("expected 'ps' command to be registered under RootCmd")
	}

	// Verify alias 'ls'
	aliasCmd, _, err := RootCmd.Find([]string{"ls"})
	if err != nil || aliasCmd != psCmd {
		t.Errorf("expected 'ls' alias to resolve to psCmd")
	}

	if psCmd.Flags().Lookup("all") == nil {
		t.Errorf("expected --all flag on psCmd")
	}
	if psCmd.Flags().ShorthandLookup("a") == nil {
		t.Errorf("expected -a shorthand on psCmd")
	}
	if psCmd.Flags().Lookup("quiet") == nil {
		t.Errorf("expected --quiet flag on psCmd")
	}
	if psCmd.Flags().ShorthandLookup("q") == nil {
		t.Errorf("expected -q shorthand on psCmd")
	}
}

func TestPsFlagsBinding(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantAll   bool
		wantQuiet bool
	}{
		{
			name:      "no flags",
			args:      []string{"ps"},
			wantAll:   false,
			wantQuiet: false,
		},
		{
			name:      "all shorthand -a",
			args:      []string{"ps", "-a"},
			wantAll:   true,
			wantQuiet: false,
		},
		{
			name:      "quiet shorthand -q",
			args:      []string{"ps", "-q"},
			wantAll:   false,
			wantQuiet: true,
		},
		{
			name:      "combined shorthand -aq",
			args:      []string{"ps", "-aq"},
			wantAll:   true,
			wantQuiet: true,
		},
		{
			name:      "long flags --all --quiet",
			args:      []string{"ps", "--all", "--quiet"},
			wantAll:   true,
			wantQuiet: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetFlags(RootCmd)
			psAll = false
			psQuiet = false

			err := psCmd.ParseFlags(tt.args[1:])
			if err != nil {
				t.Fatalf("unexpected parse error: %v", err)
			}

			if psAll != tt.wantAll {
				t.Errorf("psAll = %v, want %v", psAll, tt.wantAll)
			}
			if psQuiet != tt.wantQuiet {
				t.Errorf("psQuiet = %v, want %v", psQuiet, tt.wantQuiet)
			}
		})
	}
}
