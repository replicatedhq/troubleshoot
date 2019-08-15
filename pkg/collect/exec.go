package collect

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/remotecommand"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

type ExecOutput struct {
	Results map[string][]byte `json:"exec/,omitempty"`
	Errors  map[string][]byte `json:"exec-errors/,omitempty"`
}

func Exec(execCollector *troubleshootv1beta1.Exec, redact bool) ([]byte, error) {
	if execCollector.Timeout == "" {
		return execWithoutTimeout(execCollector, redact)
	}

	timeout, err := time.ParseDuration(execCollector.Timeout)
	if err != nil {
		return nil, err
	}

	errCh := make(chan error, 1)
	resultCh := make(chan []byte, 1)

	go func() {
		b, err := execWithoutTimeout(execCollector, redact)
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

func execWithoutTimeout(execCollector *troubleshootv1beta1.Exec, redact bool) ([]byte, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, err
	}

	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	execOutput := &ExecOutput{
		Results: make(map[string][]byte),
		Errors:  make(map[string][]byte),
	}

	pods, podsErrors := listPodsInSelectors(client, execCollector.Namespace, execCollector.Selector)
	if len(podsErrors) > 0 {
		errorBytes, err := marshalNonNil(podsErrors)
		if err != nil {
			return nil, err
		}
		execOutput.Errors[getExecErrosFileName(execCollector)] = errorBytes
	}

	if len(pods) > 0 {
		for _, pod := range pods {
			stdout, stderr, execErrors := getExecOutputs(client, pod, execCollector, redact)
			execOutput.Results[fmt.Sprintf("%s/%s/%s-stdout.txt", pod.Namespace, pod.Name, execCollector.CollectorName)] = stdout
			execOutput.Results[fmt.Sprintf("%s/%s/%s-stderr.txt", pod.Namespace, pod.Name, execCollector.CollectorName)] = stderr
			if len(execErrors) > 0 {
				errorBytes, err := marshalNonNil(execErrors)
				if err != nil {
					return nil, err
				}
				execOutput.Results[fmt.Sprintf("%s/%s/%s-errors.json", pod.Namespace, pod.Name, execCollector.CollectorName)] = errorBytes
				continue
			}
		}

		if redact {
			execOutput, err = execOutput.Redact()
			if err != nil {
				return nil, err
			}
		}
	}

	b, err := json.MarshalIndent(execOutput, "", "  ")
	if err != nil {
		return nil, err
	}

	return b, nil
}

func getExecOutputs(client *kubernetes.Clientset, pod corev1.Pod, execCollector *troubleshootv1beta1.Exec, doRedact bool) ([]byte, []byte, []string) {
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, nil, []string{err.Error()}
	}

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

	exec, err := remotecommand.NewSPDYExecutor(cfg, "POST", req.URL())
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

func (r *ExecOutput) Redact() (*ExecOutput, error) {
	results, err := redactMap(r.Results)
	if err != nil {
		return nil, err
	}

	return &ExecOutput{
		Results: results,
		Errors:  r.Errors,
	}, nil
}

func getExecErrosFileName(execCollector *troubleshootv1beta1.Exec) string {
	if len(execCollector.CollectorName) > 0 {
		return fmt.Sprintf("%s.json", execCollector.CollectorName)
	}
	// TODO: random part
	return "errors.json"
}
