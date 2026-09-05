package transfer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/basarsubasi/kampfire/pkg/k8s"
)

func TestCopyPathValidation(t *testing.T) {
	ctx := context.Background()
	client := &k8s.Client{}

	tests := []struct {
		name    string
		src     string
		dest    string
		wantErr string
	}{
		{
			name:    "both remote paths error",
			src:     "box1:/app",
			dest:    "box2:/app",
			wantErr: "copying directly between two remote sandboxes is not supported",
		},
		{
			name:    "neither remote path error",
			src:     "./src",
			dest:    "/tmp/dest",
			wantErr: "one path must be remote",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Copy(ctx, client, tt.src, tt.dest)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestCopyToSandbox_LocalPathNotFound(t *testing.T) {
	ctx := context.Background()
	client := &k8s.Client{}

	nonExistent := filepath.Join(t.TempDir(), "does-not-exist.txt")
	err := copyToSandbox(ctx, client, nonExistent, "test-pod", "/workspace")
	if err == nil {
		t.Fatalf("expected error for non-existent local file, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got %v", err)
	}
}

func TestLocalDirectoryDetection(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "sample.txt")
	if err := os.WriteFile(filePath, []byte("hello world"), 0644); err != nil {
		t.Fatal(err)
	}

	infoFile, err := os.Stat(filePath)
	if err != nil || infoFile.IsDir() {
		t.Fatalf("expected sample.txt to be a file")
	}

	infoDir, err := os.Stat(tempDir)
	if err != nil || !infoDir.IsDir() {
		t.Fatalf("expected tempDir to be a directory")
	}
}
