package transfer

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/basarsubasi/kampfire/pkg/k8s"
	"github.com/basarsubasi/kampfire/pkg/sandbox"
	"github.com/basarsubasi/kampfire/pkg/terminal"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

// Copy copies a file or directory between the local machine and a sandbox container.
func Copy(ctx context.Context, client *k8s.Client, src, dest string) error {
	srcIsRemote := strings.Contains(src, ":")
	destIsRemote := strings.Contains(dest, ":")

	if srcIsRemote && destIsRemote {
		return fmt.Errorf("copying directly between two remote sandboxes is not supported")
	}
	if !srcIsRemote && !destIsRemote {
		return fmt.Errorf("one path must be remote (e.g. sandbox-id:/path)")
	}

	if srcIsRemote {
		parts := strings.SplitN(src, ":", 2)
		target, remotePath := parts[0], parts[1]
		podName := target
		if sb, err := sandbox.Find(ctx, client, target); err == nil {
			podName = sb.Name
		}
		return copyFromSandbox(ctx, client, podName, remotePath, dest)
	}

	parts := strings.SplitN(dest, ":", 2)
	target, remotePath := parts[0], parts[1]
	podName := target
	if sb, err := sandbox.Find(ctx, client, target); err == nil {
		podName = sb.Name
	}
	return copyToSandbox(ctx, client, src, podName, remotePath)
}

func copyToSandbox(ctx context.Context, client *k8s.Client, localPath, podName, remotePath string) error {
	info, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("local path %s not found: %w", localPath, err)
	}

	destDir, destFileName := resolveDestDirAndName(ctx, client, podName, remotePath, info.IsDir(), info.Name())

	// Make destination directory inside container
	mkdirCmd := []string{"mkdir", "-p", destDir}
	req := client.Clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(client.Namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Command: mkdirCmd,
			Stdout:  true,
			Stderr:  true,
		}, scheme.ParameterCodec)

	exec, err := client.NewExecutor("POST", req.URL())
	if err != nil {
		return err
	}
	if err := exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: io.Discard,
		Stderr: os.Stderr,
	}); err != nil {
		return fmt.Errorf("failed to create remote directory %s: %w", destDir, err)
	}

	// Pipe local tar into remote tar -xf - -C destDir
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		tw := tar.NewWriter(pw)
		defer tw.Close()

		if !info.IsDir() {
			file, err := os.Open(localPath)
			if err != nil {
				_ = pw.CloseWithError(err)
				return
			}
			defer file.Close()

			tarName := info.Name()
			if destFileName != "" && destFileName != "/" && destFileName != "." {
				tarName = destFileName
			}

			hdr, err := tar.FileInfoHeader(info, "")
			if err != nil {
				_ = pw.CloseWithError(err)
				return
			}
			hdr.Name = strings.TrimSuffix(tarName, "/")
			if hdr.Name == "" || hdr.Name == "." {
				hdr.Name = info.Name()
			}

			if err := tw.WriteHeader(hdr); err != nil {
				_ = pw.CloseWithError(err)
				return
			}
			if _, err := io.Copy(tw, file); err != nil {
				_ = pw.CloseWithError(err)
				return
			}
		} else {
			_ = filepath.Walk(localPath, func(path string, fi os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				relPath, err := filepath.Rel(localPath, path)
				if err != nil {
					return err
				}
				if relPath == "." {
					return nil
				}

				hdr, err := tar.FileInfoHeader(fi, "")
				if err != nil {
					return err
				}
				hdr.Name = relPath

				if err := tw.WriteHeader(hdr); err != nil {
					return err
				}
				if fi.IsDir() {
					return nil
				}

				f, err := os.Open(path)
				if err != nil {
					return err
				}
				defer f.Close()
				_, err = io.Copy(tw, f)
				return err
			})
		}
	}()

	tarCmd := []string{"tar", "-xf", "-", "-C", destDir}
	tarReq := client.Clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(client.Namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Command: tarCmd,
			Stdin:   true,
			Stdout:  true,
			Stderr:  true,
		}, scheme.ParameterCodec)

	tarExec, err := client.NewExecutor("POST", tarReq.URL())
	if err != nil {
		return err
	}

	return tarExec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  pr,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	})
}

func copyFromSandbox(ctx context.Context, client *k8s.Client, podName, remotePath, localPath string) error {
	remoteDir := filepath.Dir(remotePath)
	remoteBase := filepath.Base(remotePath)
	if remotePath == "/" || remotePath == "." || strings.HasSuffix(remotePath, "/") {
		remoteDir = filepath.Clean(remotePath)
		remoteBase = "."
	}

	tarCmd := []string{"tar", "-cf", "-", "-C", remoteDir, remoteBase}
	req := client.Clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(client.Namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Command: tarCmd,
			Stdout:  true,
			Stderr:  true,
		}, scheme.ParameterCodec)

	exec, err := client.NewExecutor("POST", req.URL())
	if err != nil {
		return err
	}

	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		_ = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
			Stdout: pw,
			Stderr: os.Stderr,
		})
	}()

	isLocalDir := false
	if fi, err := os.Stat(localPath); err == nil && fi.IsDir() {
		isLocalDir = true
	}

	tr := tar.NewReader(pr)
	first := true
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := localPath
		if isLocalDir {
			target = filepath.Join(localPath, filepath.Base(header.Name))
		} else if !first {
			target = filepath.Join(localPath, header.Name)
		}
		first = false
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, header.FileInfo().Mode())
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		}
	}

	return nil
}

// InjectSSHKeys copies host SSH keys (~/.ssh) into the container's home directory.
func InjectSSHKeys(ctx context.Context, client *k8s.Client, podName string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to determine user home directory: %w", err)
	}

	sshDir := filepath.Join(home, ".ssh")
	info, err := os.Stat(sshDir)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("no ~/.ssh directory found on host (%s)", sshDir)
	}

	entries, err := os.ReadDir(sshDir)
	if err != nil {
		return fmt.Errorf("failed to read ~/.ssh directory: %w", err)
	}

	if len(entries) == 0 {
		return fmt.Errorf("~/.ssh directory is empty on host")
	}

	// 1. Determine container's $HOME
	remoteHome, _, err := terminal.ExecSimple(ctx, client, podName, []string{"sh", "-c", "echo -n $HOME"})
	remoteHome = strings.TrimSpace(remoteHome)
	if err != nil || remoteHome == "" {
		remoteHome = "/root"
	}
	destDir := fmt.Sprintf("%s/.ssh", remoteHome)

	// 2. Stage files into a clean temporary directory with proper permissions
	tempStaging, err := os.MkdirTemp("", "kampfire-ssh-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary staging directory: %w", err)
	}
	defer os.RemoveAll(tempStaging)

	copiedCount := 0
	for _, entry := range entries {
		// Only copy regular files (skip sockets like agent.*, fifos, subdirectories)
		if entry.IsDir() || !entry.Type().IsRegular() {
			continue
		}

		srcFile := filepath.Join(sshDir, entry.Name())
		data, err := os.ReadFile(srcFile)
		if err != nil {
			continue
		}

		destFile := filepath.Join(tempStaging, entry.Name())
		mode := os.FileMode(0600)
		if strings.HasSuffix(entry.Name(), ".pub") || entry.Name() == "known_hosts" || entry.Name() == "config" {
			mode = 0644
		}

		if err := os.WriteFile(destFile, data, mode); err != nil {
			return fmt.Errorf("failed to stage SSH file %s: %w", entry.Name(), err)
		}
		copiedCount++
	}

	if copiedCount == 0 {
		return fmt.Errorf("no valid SSH key files found in %s", sshDir)
	}

	// 3. Copy staged files to sandbox
	if err := Copy(ctx, client, tempStaging, fmt.Sprintf("%s:%s", podName, destDir)); err != nil {
		return fmt.Errorf("failed to copy SSH keys into sandbox: %w", err)
	}

	// 4. Ensure strict permissions inside container (required by OpenSSH)
	chmodScript := fmt.Sprintf("chmod 700 %s && chmod 600 %s/* 2>/dev/null; chmod 644 %s/*.pub %s/known_hosts %s/config 2>/dev/null || true",
		destDir, destDir, destDir, destDir, destDir)
	_, _, _ = terminal.ExecSimple(ctx, client, podName, []string{"sh", "-c", chmodScript})

	return nil
}

// resolveDestDirAndName determines the target directory and tar entry filename
// when copying a file or directory into a sandbox.
func resolveDestDirAndName(ctx context.Context, client *k8s.Client, podName, remotePath string, isSrcDir bool, srcFileName string) (string, string) {
	clean := filepath.Clean(remotePath)
	if isSrcDir {
		return clean, ""
	}

	// If remotePath is "/" or "." or ends with "/", it explicitly represents a directory destination
	if clean == "/" || clean == "." || strings.HasSuffix(remotePath, "/") {
		return clean, srcFileName
	}

	// If the remote path is an existing directory inside the container
	if remotePathIsDir(ctx, client, podName, remotePath) {
		return clean, srcFileName
	}

	// Otherwise it represents a specific target file path (destination directory + destination filename)
	dir := filepath.Dir(remotePath)
	base := filepath.Base(remotePath)
	if base == "/" || base == "." || base == "" {
		base = srcFileName
	}
	return dir, base
}

// remotePathIsDir checks if the path inside the container is an existing directory.
func remotePathIsDir(ctx context.Context, client *k8s.Client, podName, remotePath string) bool {
	if client == nil || client.Clientset == nil {
		return false
	}

	req := client.Clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(client.Namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Command: []string{"test", "-d", remotePath},
			Stdout:  true,
			Stderr:  true,
		}, scheme.ParameterCodec)

	exec, err := client.NewExecutor("POST", req.URL())
	if err != nil {
		return false
	}

	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: io.Discard,
		Stderr: io.Discard,
	})
	return err == nil
}

