package collect

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
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

func (c *CollectCopy) SkipRedaction() bool {
	return c.Collector.SkipRedaction
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

	if len(pods) > 0 {
		for _, pod := range pods {

			containerName := pod.Spec.Containers[0].Name
			if c.Collector.ContainerName != "" {
				containerName = c.Collector.ContainerName
			}

			subPath := filepath.Join(c.Collector.Name, pod.Namespace, pod.Name, c.Collector.ContainerName)

			c.Collector.ExtractArchive = true // TODO: existing regression. this flag is always ignored and this matches current behaviour

			copyErrors := map[string]string{}

			dstPath := filepath.Join(c.BundlePath, subPath, filepath.Dir(c.Collector.ContainerPath))
			files, stderr, err := copyFilesFromPod(ctx, dstPath, c.ClientConfig, client, pod.Name, containerName, pod.Namespace, c.Collector.ContainerPath, c.Collector.ExtractArchive)
			if err != nil {
				copyErrors[filepath.Join(c.Collector.ContainerPath, "error")] = err.Error()
				if len(stderr) > 0 {
					copyErrors[filepath.Join(c.Collector.ContainerPath, "stderr")] = string(stderr)
				}

				key := filepath.Join(subPath, c.Collector.ContainerPath+"-errors.json")
				output.SaveResult(c.BundlePath, key, marshalErrors(copyErrors))
				continue
			}

			for k, v := range files {
				output[filepath.Join(subPath, filepath.Dir(c.Collector.ContainerPath), k)] = v
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

	parameterCodec := runtime.NewParameterCodec(scheme)
	req.VersionedParams(&corev1.PodExecOptions{
		Command:   command,
		Container: containerName,
		Stdout:    true,
		Stderr:    true,
	}, parameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(clientConfig, "POST", req.URL())
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to create SPDY executor")
	}

	result := NewResult()

	var stdoutWriter io.Writer
	var copyError error
	if extract {
		pipeReader, pipeWriter := io.Pipe()
		tarReader := tar.NewReader(pipeReader)
		stdoutWriter = pipeWriter

		go func() {
			// this can cause "read/write on closed pipe" error, but without this exec.Stream blocks
			defer pipeWriter.Close()

			for {
				header, err := tarReader.Next()
				if err == io.EOF {
					return
				}
				if err != nil {
					pipeWriter.CloseWithError(errors.Wrap(err, "failed to read header from tar"))
					return
				}

				switch header.Typeflag {
				case tar.TypeDir:
					name := filepath.Join(dstPath, header.Name)
					if err := os.MkdirAll(name, os.FileMode(header.Mode)); err != nil {
						pipeWriter.CloseWithError(errors.Wrap(err, "failed to mkdir"))
						return
					}
				case tar.TypeReg:
					err := result.SaveResult(dstPath, header.Name, tarReader)
					if err != nil {
						pipeWriter.CloseWithError(errors.Wrapf(err, "failed to save result for file %s", header.Name))
						return
					}
				}
			}
		}()
	} else {
		w, err := result.GetWriter(dstPath, filepath.Base(containerPath)+".tar")
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed to craete dest file")
		}
		defer result.CloseWriter(dstPath, filepath.Base(containerPath)+".tar", w)

		stdoutWriter = w
	}

	var stderr bytes.Buffer
	copyError = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: stdoutWriter,
		Stderr: &stderr,
	})
	if copyError != nil {
		return result, stderr.Bytes(), errors.Wrap(copyError, "failed to stream command output")
	}

	return result, stderr.Bytes(), nil
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
