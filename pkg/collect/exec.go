package collect

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/remotecommand"
)

func Exec(c *Collector, execCollector *troubleshootv1beta2.Exec) (CollectorResult, error) {
	if execCollector.Timeout == "" {
		return execWithoutTimeout(c, execCollector)
	}

	timeout, err := time.ParseDuration(execCollector.Timeout)
	if err != nil {
		return nil, err
	}

	errCh := make(chan error, 1)
	resultCh := make(chan CollectorResult, 1)

	go func() {
		b, err := execWithoutTimeout(c, execCollector)
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

func execWithoutTimeout(c *Collector, execCollector *troubleshootv1beta2.Exec) (CollectorResult, error) {
	client, err := kubernetes.NewForConfig(c.ClientConfig)
	if err != nil {
		return nil, err
	}

	output := NewResult()

	ctx := context.Background()

	pods, podsErrors := listPodsInSelectors(ctx, client, execCollector.Namespace, execCollector.Selector)
	if len(podsErrors) > 0 {
		output.SaveResult(c.BundlePath, getExecErrosFileName(execCollector), marshalErrors(podsErrors))
	}

	if len(pods) > 0 {
		for _, pod := range pods {
			stdout, stderr, execErrors := getExecOutputs(c, client, pod, execCollector)

			bundlePath := filepath.Join(execCollector.Name, pod.Namespace, pod.Name)
			if len(stdout) > 0 {
				output.SaveResult(c.BundlePath, filepath.Join(bundlePath, execCollector.CollectorName+"-stdout.txt"), bytes.NewBuffer(stdout))
			}
			if len(stderr) > 0 {
				output.SaveResult(c.BundlePath, filepath.Join(bundlePath, execCollector.CollectorName+"-stderr.txt"), bytes.NewBuffer(stderr))
			}

			if len(execErrors) > 0 {
				output.SaveResult(c.BundlePath, filepath.Join(bundlePath, execCollector.CollectorName+"-errors.json"), marshalErrors(execErrors))
				continue
			}
		}
	}

	return output, nil
}

func getExecOutputs(c *Collector, client *kubernetes.Clientset, pod corev1.Pod, execCollector *troubleshootv1beta2.Exec) ([]byte, []byte, []string) {
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

	exec, err := remotecommand.NewSPDYExecutor(c.ClientConfig, "POST", req.URL())
	if err != nil {
		return nil, nil, []string{err.Error()}
	}

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)

	err = exec.Stream(remotecommand.StreamOptions{
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

func getExecErrosFileName(execCollector *troubleshootv1beta2.Exec) string {
	if len(execCollector.Name) > 0 {
		return fmt.Sprintf("%s-errors.json", execCollector.Name)
	}
	if len(execCollector.CollectorName) > 0 {
		return fmt.Sprintf("%s-errors.json", execCollector.CollectorName)
	}
	// TODO: random part
	return "errors.json"
}
