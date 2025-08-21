package collect

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/client/troubleshootclientset/scheme"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/klog/v2"
)

const (
	DefaultSonobuoyNamespace               = "sonobuoy"
	DefaultSonobuoyAggregatorPodName       = "sonobuoy"
	DefaultSonobuoyAggregatorContainerName = "kube-sonobuoy"
	DefaultSonobuoyAggregatorResultsPath   = "/tmp/sonobuoy"
)

type CollectSonobuoyResults struct {
	Collector    *troubleshootv1beta2.Sonobuoy
	BundlePath   string
	Namespace    string // this is not used
	ClientConfig *rest.Config
	Client       kubernetes.Interface
	Context      context.Context
	RBACErrors
}

func (c *CollectSonobuoyResults) Title() string {
	return getCollectorName(c)
}

func (c *CollectSonobuoyResults) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
}

func (c *CollectSonobuoyResults) SkipRedaction() bool {
	return c.Collector.SkipRedaction
}

func (c *CollectSonobuoyResults) Collect(progressChan chan<- interface{}) (CollectorResult, error) {
	namespace := DefaultSonobuoyNamespace
	if c.Collector.Namespace != "" {
		namespace = c.Collector.Namespace
	}

	podName := DefaultSonobuoyAggregatorPodName
	resultsPath := DefaultSonobuoyAggregatorResultsPath
	containerName := DefaultSonobuoyAggregatorContainerName

	_, err := c.Client.CoreV1().Pods(namespace).Get(c.Context, podName, metav1.GetOptions{})
	if kerrors.IsNotFound(err) {
		return nil, fmt.Errorf("sonobuoy pod %s in namespace %s not found", podName, namespace)
	} else if err != nil {
		return nil, fmt.Errorf("failed to get sonobuoy pod %s in namespace %s: %v", podName, namespace, err)
	}

	reader, ec, err := sonobuoyRetrieveResults(c.Context, c.Client, c.ClientConfig, namespace, podName, containerName, resultsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve sonobuoy results: %v", err)
	}

	output := NewResult()

	ec2 := make(chan error, 1)

	go func() {
		defer close(ec2)

		gz, err := gzip.NewReader(reader)
		if err != nil {
			ec2 <- errors.Wrap(err, "failed to create gzip reader")
			return
		}
		defer gz.Close()

		tr := tar.NewReader(gz)

		for {
			header, err := tr.Next()
			if err == io.EOF {
				return
			} else if err != nil {
				ec2 <- errors.Wrap(err, "failed to read tar header")
				return
			}
			if header.Typeflag != tar.TypeReg {
				continue
			}
			filename := filepath.Clean(header.Name) // sanitize the filename
			klog.V(2).Infof("Sonobuoy collector found file: %s", filename)
			err = output.SaveResult(c.BundlePath, filepath.Join("sonobuoy", filename), tr)
			if err != nil {
				ec2 <- errors.Wrapf(err, "failed to save result for %s", filename)
				return
			}
		}
	}()

	err = <-ec2
	if err != nil {
		_, _ = io.Copy(io.Discard, reader) // ensure the stream is closed
		return nil, fmt.Errorf("failed to write sonobuoy results: %v", err)
	}

	err = <-ec
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve sonobuoy results: %v", err)
	}

	return output, nil
}

// sonobuoyRetrieveResults copies results from a sonobuoy run into a Reader in tar format.
// It also returns a channel of errors, where any errors encountered when writing results
// will be sent, and an error in the case where the config validation fails.
func sonobuoyRetrieveResults(
	ctx context.Context, client kubernetes.Interface, restConfig *rest.Config, namespace, podName, containerName, path string,
) (io.Reader, <-chan error, error) {
	ec := make(chan error, 1)

	cmd := sonobuoyTarCmd(path)

	klog.V(2).Infof(
		"Sonobuoy collector runing command: kubectl exec -n %s %s -c %s -- %s",
		namespace, podName, containerName, strings.Join(cmd, " "),
	)
	restClient := client.CoreV1().RESTClient()
	req := restClient.Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec").
		Param("container", containerName)
	req.VersionedParams(&corev1.PodExecOptions{
		Container: containerName,
		Command:   cmd,
		Stdin:     false,
		Stdout:    true,
		Stderr:    false,
	}, scheme.ParameterCodec)
	executor, err := remotecommand.NewSPDYExecutor(restConfig, "POST", req.URL())
	if err != nil {
		return nil, ec, err
	}
	reader, writer := io.Pipe()
	go func(writer *io.PipeWriter, ec chan error) {
		defer writer.Close()
		defer close(ec)
		err = executor.StreamWithContext(ctx, remotecommand.StreamOptions{
			Stdout: writer,
			Tty:    false,
		})
		if err != nil {
			ec <- err
		}
	}(writer, ec)

	return reader, ec, nil
}

func sonobuoyTarCmd(path string) []string {
	return []string{
		"/sonobuoy",
		"splat",
		path,
	}
}
