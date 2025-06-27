package collect

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

type CollectExec struct {
	Collector    *troubleshootv1beta2.Exec
	BundlePath   string
	Namespace    string
	ClientConfig *rest.Config
	Client       kubernetes.Interface
	Context      context.Context
	RBACErrors
}

func (c *CollectExec) Title() string {
	return getCollectorName(c)
}

func (c *CollectExec) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
}

func (c *CollectExec) SkipRedaction() bool {
	return c.Collector.SkipRedaction
}

func (c *CollectExec) Collect(progressChan chan<- interface{}) (CollectorResult, error) {
	if c.Collector.Timeout == "" {
		return execWithoutTimeout(c.ClientConfig, c.BundlePath, c.Collector)
	}

	timeout, err := time.ParseDuration(c.Collector.Timeout)
	if err != nil {
		return nil, err
	}

	errCh := make(chan error, 1)
	resultCh := make(chan CollectorResult, 1)

	// TODO: Use a context with timeout instead of a goroutine
	go func() {
		b, err := execWithoutTimeout(c.ClientConfig, c.BundlePath, c.Collector)
		if err != nil {
			errCh <- err
		} else {
			resultCh <- b
		}
	}()

	select {
	case <-time.After(timeout):
		return nil, errors.New("timeout")
	case result := <-resultCh:
		return result, nil
	case err := <-errCh:
		return nil, err
	}
}

func execWithoutTimeout(clientConfig *rest.Config, bundlePath string, execCollector *troubleshootv1beta2.Exec) (CollectorResult, error) {
	client, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}

	output := NewResult()

	ctx := context.Background()

	pods, podsErrors := listPodsInSelectors(ctx, client, execCollector.Namespace, execCollector.Selector)
	if len(podsErrors) > 0 {
		output.SaveResult(bundlePath, getExecErrorsFileName(execCollector), marshalErrors(podsErrors))
	}

	if len(pods) > 0 {
		// When the selector refers to more than one replica of a pod, the exec collector will execute in only one of the pods
		pod := pods[0]
		stdout, stderr, execErrors := getExecOutputs(ctx, clientConfig, client, pod, execCollector)

		path := filepath.Join(execCollector.Name, pod.Namespace, pod.Name)
		if len(stdout) > 0 {
			output.SaveResult(bundlePath, filepath.Join(path, execCollector.CollectorName+"-stdout.txt"), bytes.NewBuffer(stdout))
		}
		if len(stderr) > 0 {
			output.SaveResult(bundlePath, filepath.Join(path, execCollector.CollectorName+"-stderr.txt"), bytes.NewBuffer(stderr))
		}

		if len(execErrors) > 0 {
			output.SaveResult(bundlePath, filepath.Join(path, execCollector.CollectorName+"-errors.json"), marshalErrors(execErrors))
		}
	}

	return output, nil
}

func getExecOutputs(
	ctx context.Context, clientConfig *rest.Config, client *kubernetes.Clientset, pod corev1.Pod, execCollector *troubleshootv1beta2.Exec,
) ([]byte, []byte, []string) {
	container := pod.Spec.Containers[0].Name
	if execCollector.ContainerName != "" {
		container = execCollector.ContainerName
	}

	req := client.CoreV1().RESTClient().Post().Resource("pods").Name(pod.Name).Namespace(pod.Namespace).SubResource("exec")
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		return nil, nil, []string{err.Error()}
	}

	parameterCodec := runtime.NewParameterCodec(scheme)
	req.VersionedParams(&corev1.PodExecOptions{
		Command:   append(execCollector.Command, execCollector.Args...),
		Container: container,
		Stdin:     true,
		Stdout:    false,
		Stderr:    true,
		TTY:       false,
	}, parameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(clientConfig, "POST", req.URL())
	if err != nil {
		return nil, nil, []string{err.Error()}
	}

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)

	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  nil,
		Stdout: stdout,
		Stderr: stderr,
		Tty:    false,
	})

	if err != nil {
		return stdout.Bytes(), stderr.Bytes(), []string{err.Error()}
	}

	return stdout.Bytes(), stderr.Bytes(), nil
}

func getExecErrorsFileName(execCollector *troubleshootv1beta2.Exec) string {
	if len(execCollector.Name) > 0 {
		return fmt.Sprintf("%s-errors.json", execCollector.Name)
	}
	if len(execCollector.CollectorName) > 0 {
		return fmt.Sprintf("%s-errors.json", execCollector.CollectorName)
	}
	// TODO: random part
	return "errors.json"
}
