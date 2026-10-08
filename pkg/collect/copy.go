package collect

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/k8sutil"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

type CollectCopy struct {
	Collector    *troubleshootv1beta2.Copy
	BundlePath   string
	Namespace    string
	ClientConfig *rest.Config
	Client       kubernetes.Interface
	Context      context.Context
	RBACErrors
}

func (c *CollectCopy) Title() string {
	return getCollectorName(c)
}

func (c *CollectCopy) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
}

// Copy function gets a file or folder from a container specified in the specs.
func (c *CollectCopy) Collect(progressChan chan<- interface{}) (CollectorResult, error) {
	client, err := kubernetes.NewForConfig(c.ClientConfig)
	if err != nil {
		return nil, err
	}

	output := NewResult()

	ctx := context.Background()

	pods, podsErrors := listPodsInSelectors(ctx, client, c.Collector.Namespace, c.Collector.Selector)
	if len(podsErrors) > 0 {
		output.SaveResult(c.BundlePath, getCopyErrosFileName(c.Collector), marshalErrors(podsErrors))
	}

	// Strip any trailing slash, otherwise the tar command below looks for e.g. /tmp/dir/dir
	containerPath := filepath.Clean(c.Collector.ContainerPath)

	if len(pods) > 0 {
		for _, pod := range pods {

			containerName := pod.Spec.Containers[0].Name
			if c.Collector.ContainerName != "" {
				containerName = c.Collector.ContainerName
			}

			subPath := filepath.Join(c.Collector.Name, pod.Namespace, pod.Name, c.Collector.ContainerName)

			c.Collector.ExtractArchive = true // TODO: existing regression. this flag is always ignored and this matches current behaviour

			copyErrors := map[string]string{}

			dstPath := filepath.Join(c.BundlePath, subPath, filepath.Dir(containerPath))
			files, stderr, err := copyFilesFromPod(ctx, dstPath, c.ClientConfig, client, pod.Name, containerName, pod.Namespace, containerPath, c.Collector.ExtractArchive)
			if err != nil {
				copyErrors[filepath.Join(containerPath, "error")] = err.Error()
				if len(stderr) > 0 {
					copyErrors[filepath.Join(containerPath, "stderr")] = string(stderr)
				}

				key := filepath.Join(subPath, containerPath+"-errors.json")
				output.SaveResult(c.BundlePath, key, marshalErrors(copyErrors))
				continue
			}

			for k, v := range files {
				output[filepath.Join(subPath, filepath.Dir(containerPath), k)] = v
			}
		}
	}

	return output, nil
}

func copyFilesFromPod(ctx context.Context, dstPath string, clientConfig *restclient.Config, client kubernetes.Interface, podName string, containerName string, namespace string, containerPath string, extract bool) (CollectorResult, []byte, error) {
	command := []string{"tar", "-C", filepath.Dir(containerPath), "-cf", "-", filepath.Base(containerPath)}
	req := client.CoreV1().RESTClient().Post().Resource("pods").Name(podName).Namespace(namespace).SubResource("exec")
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		return nil, nil, errors.Wrap(err, "failed to add runtime scheme")
	}

	// Stdin must be false because StreamOptions.Stdin is nil below.
	// A mismatch causes the SPDY fallback (after WebSocket fails on RBAC)
	// to hang: the API server opens a stdin stream but never receives EOF.
	parameterCodec := runtime.NewParameterCodec(scheme)
	req.VersionedParams(&corev1.PodExecOptions{
		Command:   command,
		Container: containerName,
		Stdin:     false,
		Stdout:    true,
		Stderr:    true,
		TTY:       false,
	}, parameterCodec)

	exec, err := k8sutil.NewFallbackExecutor(clientConfig, req.URL())
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to create executor")
	}

	result := NewResult()

	var stdoutWriter io.Writer
	var waitForExtract func(streamErr error) error
	if extract {
		stdoutWriter, waitForExtract = newTarExtractor(dstPath, result)
	} else {
		w, err := result.GetWriter(dstPath, filepath.Base(containerPath)+".tar")
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed to craete dest file")
		}
		defer result.CloseWriter(dstPath, filepath.Base(containerPath)+".tar", w)

		stdoutWriter = w
	}

	var stderr bytes.Buffer
	copyError := exec.Stream(remotecommand.StreamOptions{
		Stdin:  nil,
		Stdout: stdoutWriter,
		Stderr: &stderr,
		Tty:    false,
	})
	if waitForExtract != nil {
		if err := waitForExtract(copyError); err != nil && copyError == nil {
			return result, stderr.Bytes(), errors.Wrap(err, "failed to extract files")
		}
	}
	if copyError != nil {
		return result, stderr.Bytes(), errors.Wrap(copyError, "failed to stream command output")
	}

	return result, stderr.Bytes(), nil
}

// newTarExtractor returns a writer that extracts the tar stream written to it into dstPath,
// recording every extracted file in result. The returned wait function must be called once
// the stream has ended, passing the stream's error (if any). It waits for extraction to
// finish and returns the extraction error.
//
// Errors from the writer are not enough on their own: client-go only logs stdout write
// errors and reports the exec as successful, so extraction failures would otherwise be lost.
func newTarExtractor(dstPath string, result CollectorResult) (io.Writer, func(streamErr error) error) {
	pipeReader, pipeWriter := io.Pipe()
	done := make(chan error, 1)

	go func() {
		err := extractTar(pipeReader, dstPath, result)
		if err == nil {
			// Consume the end-of-archive padding so the writer doesn't fail on a closed pipe
			_, err = io.Copy(io.Discard, pipeReader)
		}
		// Unblocks the writer if extraction stopped early
		pipeReader.CloseWithError(err)
		done <- err
	}()

	wait := func(streamErr error) error {
		pipeWriter.CloseWithError(streamErr)
		return <-done
	}
	return pipeWriter, wait
}

// extractTar extracts the regular files and directories in a tar stream into dstPath.
// Directories are created writable regardless of their mode in the archive, so that
// read-only directories in the container don't prevent their contents from being saved.
// Symlinks are saved as "<name>.symlink" text files containing the link target.
func extractTar(reader io.Reader, dstPath string, result CollectorResult) error {
	tarReader := tar.NewReader(reader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return errors.Wrap(err, "failed to read header from tar")
		}

		if !filepath.IsLocal(header.Name) {
			return errors.Errorf("tar entry %q points outside the destination directory", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(filepath.Join(dstPath, header.Name), 0777); err != nil {
				return errors.Wrapf(err, "failed to create directory %s", header.Name)
			}
		case tar.TypeReg:
			if err := result.SaveResult(dstPath, header.Name, tarReader); err != nil {
				return errors.Wrapf(err, "failed to save result for file %s", header.Name)
			}
		case tar.TypeSymlink:
			// Symlinks are not recreated, since their target could point anywhere on the machine
			// the bundle is collected or extracted on. Record where the link pointed instead.
			name := filepath.Clean(header.Name) + ".symlink"
			if err := result.SaveResult(dstPath, name, strings.NewReader(header.Linkname+"\n")); err != nil {
				return errors.Wrapf(err, "failed to save result for symlink %s", header.Name)
			}
		}
	}
}

func getCopyErrosFileName(copyCollector *troubleshootv1beta2.Copy) string {
	if len(copyCollector.Name) > 0 {
		return fmt.Sprintf("%s-errors.json", copyCollector.Name)
	}
	if len(copyCollector.CollectorName) > 0 {
		return fmt.Sprintf("%s-errors.json", copyCollector.CollectorName)
	}
	// TODO: random part
	return "errors.json"
}
