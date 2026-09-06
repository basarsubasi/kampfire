package cmd

import (
	"testing"
)

func TestRunFlagsRegistration(t *testing.T) {
	flags := []string{"cpu", "memory", "publish", "persist", "persist-size", "with-pull-secret", "no-keepalive", "timeout", "env"}
	for _, f := range flags {
		if runCmd.Flags().Lookup(f) == nil {
			t.Errorf("expected flag --%s to be registered on runCmd", f)
		}
	}
	if runCmd.Flags().ShorthandLookup("p") == nil {
		t.Errorf("expected shorthand -p to be registered on runCmd")
	}
	if runCmd.Flags().ShorthandLookup("e") == nil {
		t.Errorf("expected shorthand -e to be registered on runCmd")
	}
}

func TestRunFlagsParsing(t *testing.T) {
	resetFlags(RootCmd)
	runCPU = ""
	runMemory = ""
	runPublish = nil
	runPersist = ""
	runPersistSize = ""
	runWithPullSecret = ""
	runNoKeepAlive = false
	runTimeout = "5m"
	runEnv = nil

	args := []string{"--cpu", "500m", "--memory", "1Gi", "-p", "8080:80", "-p", "3000", "--persist", "/data", "--persist-size", "10Gi", "--with-pull-secret", "ghcr-creds", "--no-keepalive", "--timeout", "10m", "-e", "FOO=BAR", "--env", "BAZ=QUX"}
	err := runCmd.ParseFlags(args)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	if runCPU != "500m" {
		t.Errorf("expected runCPU = 500m, got %s", runCPU)
	}
	if runMemory != "1Gi" {
		t.Errorf("expected runMemory = 1Gi, got %s", runMemory)
	}
	if len(runPublish) != 2 {
		t.Fatalf("expected 2 published ports, got %d", len(runPublish))
	}
	if runPublish[0] != "8080:80" || runPublish[1] != "3000" {
		t.Errorf("expected ports [8080:80, 3000], got %v", runPublish)
	}
	if runPersist != "/data" {
		t.Errorf("expected runPersist = /data, got %s", runPersist)
	}
	if runPersistSize != "10Gi" {
		t.Errorf("expected runPersistSize = 10Gi, got %s", runPersistSize)
	}
	if runWithPullSecret != "ghcr-creds" {
		t.Errorf("expected runWithPullSecret = ghcr-creds, got %s", runWithPullSecret)
	}
	if !runNoKeepAlive {
		t.Errorf("expected runNoKeepAlive = true, got %v", runNoKeepAlive)
	}
	if runTimeout != "10m" {
		t.Errorf("expected runTimeout = 10m, got %s", runTimeout)
	}
	if len(runEnv) != 2 || runEnv[0] != "FOO=BAR" || runEnv[1] != "BAZ=QUX" {
		t.Errorf("expected runEnv to be [FOO=BAR, BAZ=QUX], got %v", runEnv)
	}
}

func TestRunFlagsPersistDefault(t *testing.T) {
	resetFlags(RootCmd)
	runPersist = ""
	runPersistSize = "5Gi"

	args := []string{"--persist", "/workspace"}
	err := runCmd.ParseFlags(args)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	if runPersist != "/workspace" {
		t.Errorf("expected runPersist to be /workspace, got %q", runPersist)
	}
	if runPersistSize != "5Gi" {
		t.Errorf("expected default runPersistSize to be 5Gi, got %q", runPersistSize)
	}
}

func TestRunFlagsWithPullSecret(t *testing.T) {
	resetFlags(RootCmd)
	runWithPullSecret = ""

	args := []string{"--with-pull-secret", "my-registry-secret"}
	err := runCmd.ParseFlags(args)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	if runWithPullSecret != "my-registry-secret" {
		t.Errorf("expected runWithPullSecret = my-registry-secret, got %q", runWithPullSecret)
	}
}

func TestRunFlagsNoKeepAlive(t *testing.T) {
	resetFlags(RootCmd)
	runNoKeepAlive = false

	args := []string{"--no-keepalive"}
	err := runCmd.ParseFlags(args)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	if !runNoKeepAlive {
		t.Errorf("expected runNoKeepAlive = true, got %v", runNoKeepAlive)
	}
}

func TestRunFlagsTimeout(t *testing.T) {
	resetFlags(RootCmd)
	runTimeout = "5m"

	args := []string{"--timeout", "15m"}
	err := runCmd.ParseFlags(args)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	if runTimeout != "15m" {
		t.Errorf("expected runTimeout = 15m, got %s", runTimeout)
	}
}

func TestRunFlagsEnv(t *testing.T) {
	resetFlags(RootCmd)
	runEnv = nil

	args := []string{"-e", "VAR1=hello", "--env", "VAR2=world", "-e", "FLAG_ONLY"}
	err := runCmd.ParseFlags(args)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	if len(runEnv) != 3 {
		t.Fatalf("expected 3 env vars, got %d: %v", len(runEnv), runEnv)
	}
	if runEnv[0] != "VAR1=hello" || runEnv[1] != "VAR2=world" || runEnv[2] != "FLAG_ONLY" {
		t.Errorf("expected [VAR1=hello, VAR2=world, FLAG_ONLY], got %v", runEnv)
	}
}


