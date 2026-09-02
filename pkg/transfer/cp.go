package transfer

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"campfire/pkg/k8s"

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
		sandboxID, remotePath := parts[0], parts[1]
		return copyFromSandbox(ctx, client, sandboxID, remotePath, dest)
	}

	parts := strings.SplitN(dest, ":", 2)
	sandboxID, remotePath := parts[0], parts[1]
	return copyToSandbox(ctx, client, src, sandboxID, remotePath)
}

func copyToSandbox(ctx context.Context, client *k8s.Client, localPath, podName, remotePath string) error {
	info, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("local path %s not found: %w", localPath, err)
	}

	destDir := remotePath
	destFileName := ""
	if !info.IsDir() {
		destDir = filepath.Dir(remotePath)
		destFileName = filepath.Base(remotePath)
	}

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

	exec, err := remotecommand.NewSPDYExecutor(client.RestConfig, "POST", req.URL())
	if err != nil {
		return err
	}
	if err := exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: os.Stdout,
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
			if destFileName != "" {
				tarName = destFileName
			}

			hdr, err := tar.FileInfoHeader(info, "")
			if err != nil {
				_ = pw.CloseWithError(err)
				return
			}
			hdr.Name = tarName

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

	tarExec, err := remotecommand.NewSPDYExecutor(client.RestConfig, "POST", tarReq.URL())
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

	exec, err := remotecommand.NewSPDYExecutor(client.RestConfig, "POST", req.URL())
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

	tr := tar.NewReader(pr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(localPath, header.Name)
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
