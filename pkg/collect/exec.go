package collect

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/remotecommand"
)

type ExecOutput map[string][]byte

func Exec(ctx *Context, execCollector *troubleshootv1beta1.Exec) (map[string][]byte, error) {
	if execCollector.Timeout == "" {
		return execWithoutTimeout(ctx, execCollector)
	}

	timeout, err := time.ParseDuration(execCollector.Timeout)
	if err != nil {
		return nil, err
	}

	errCh := make(chan error, 1)
	resultCh := make(chan map[string][]byte, 1)

	go func() {
		b, err := execWithoutTimeout(ctx, execCollector)
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

func execWithoutTimeout(ctx *Context, execCollector *troubleshootv1beta1.Exec) (map[string][]byte, error) {
	client, err := kubernetes.NewForConfig(ctx.ClientConfig)
	if err != nil {
		return nil, err
	}

	execOutput := ExecOutput{}

	pods, podsErrors := listPodsInSelectors(client, execCollector.Namespace, execCollector.Selector)
	if len(podsErrors) > 0 {
		errorBytes, err := marshalNonNil(podsErrors)
		if err != nil {
			return nil, err
		}
		execOutput[getExecErrosFileName(execCollector)] = errorBytes
	}

	if len(pods) > 0 {
		for _, pod := range pods {
			stdout, stderr, execErrors := getExecOutputs(ctx, client, pod, execCollector)

			bundlePath := filepath.Join(execCollector.Name, pod.Namespace, pod.Name)
			if len(stdout) > 0 {
				execOutput[filepath.Join(bundlePath, execCollector.CollectorName+"-stdout.txt")] = stdout
			}
			if len(stderr) > 0 {
				execOutput[filepath.Join(bundlePath, execCollector.CollectorName+"-stderr.txt")] = stderr
			}

			if len(execErrors) > 0 {
				errorBytes, err := marshalNonNil(execErrors)
				if err != nil {
					return nil, err
				}
				execOutput[filepath.Join(bundlePath, execCollector.CollectorName+"-errors.json")] = errorBytes
				continue
			}
		}

		if ctx.Redact {
			execOutput, err = execOutput.Redact()
			if err != nil {
				return nil, err
			}
		}
	}

	return execOutput, nil
}

func getExecOutputs(ctx *Context, client *kubernetes.Clientset, pod corev1.Pod, execCollector *troubleshootv1beta1.Exec) ([]byte, []byte, []string) {
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

	exec, err := remotecommand.NewSPDYExecutor(ctx.ClientConfig, "POST", req.URL())
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

func (r ExecOutput) Redact() (ExecOutput, error) {
	results, err := redactMap(r)
	if err != nil {
		return nil, err
	}

	return results, nil
}

func getExecErrosFileName(execCollector *troubleshootv1beta1.Exec) string {
	if len(execCollector.Name) > 0 {
		return fmt.Sprintf("%s-errors.json", execCollector.Name)
	}
	if len(execCollector.CollectorName) > 0 {
		return fmt.Sprintf("%s-errors.json", execCollector.CollectorName)
	}
	// TODO: random part
	return "errors.json"
}
