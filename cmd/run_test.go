package cmd

import (
	"testing"
)

func TestRunFlagsRegistration(t *testing.T) {
	flags := []string{"cpu", "memory", "publish", "persist", "persist-size", "with-pull-secret"}
	for _, f := range flags {
		if runCmd.Flags().Lookup(f) == nil {
			t.Errorf("expected flag --%s to be registered on runCmd", f)
		}
	}
	if runCmd.Flags().ShorthandLookup("p") == nil {
		t.Errorf("expected shorthand -p to be registered on runCmd")
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

	args := []string{"--cpu", "500m", "--memory", "1Gi", "-p", "8080:80", "-p", "3000", "--persist", "/data", "--persist-size", "10Gi", "--with-pull-secret", "ghcr-creds"}
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


