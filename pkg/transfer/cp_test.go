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

func TestResolveDestDirAndName(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name         string
		remotePath   string
		isSrcDir     bool
		srcFileName  string
		wantDestDir  string
		wantFileName string
	}{
		{
			name:         "copying to root /",
			remotePath:   "/",
			isSrcDir:     false,
			srcFileName:  "marker.txt",
			wantDestDir:  "/",
			wantFileName: "marker.txt",
		},
		{
			name:         "copying to directory with trailing slash",
			remotePath:   "/tmp/",
			isSrcDir:     false,
			srcFileName:  "marker.txt",
			wantDestDir:  "/tmp",
			wantFileName: "marker.txt",
		},
		{
			name:         "copying to current dir .",
			remotePath:   ".",
			isSrcDir:     false,
			srcFileName:  "marker.txt",
			wantDestDir:  ".",
			wantFileName: "marker.txt",
		},
		{
			name:         "copying to specific file path",
			remotePath:   "/etc/custom.conf",
			isSrcDir:     false,
			srcFileName:  "marker.txt",
			wantDestDir:  "/etc",
			wantFileName: "custom.conf",
		},
		{
			name:         "copying directory",
			remotePath:   "/workspace",
			isSrcDir:     true,
			srcFileName:  "my-dir",
			wantDestDir:  "/workspace",
			wantFileName: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotDir, gotFile := resolveDestDirAndName(ctx, nil, "pod", tt.remotePath, tt.isSrcDir, tt.srcFileName)
			if gotDir != tt.wantDestDir {
				t.Errorf("got destDir %q, want %q", gotDir, tt.wantDestDir)
			}
			if gotFile != tt.wantFileName {
				t.Errorf("got destFileName %q, want %q", gotFile, tt.wantFileName)
			}
		})
	}
}

