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
}

type execResult struct {
	Stdout []byte `json:"-"`
	Stderr []byte `json:"-"`
	Error  error  `json:"error,omitempty"`
}

func Exec(execCollector *troubleshootv1beta1.Exec, redact bool) error {
	if execCollector.Timeout == "" {
		return execWithoutTimeout(execCollector, redact)
	}

	timeout, err := time.ParseDuration(execCollector.Timeout)
	if err != nil {
		return err
	}

	execChan := make(chan error, 1)
	go func() {
		execChan <- execWithoutTimeout(execCollector, redact)
	}()

	select {
	case <-time.After(timeout):
		return errors.New("timeout")
	case err := <-execChan:
		return err
	}
}

func execWithoutTimeout(execCollector *troubleshootv1beta1.Exec, redact bool) error {
	cfg, err := config.GetConfig()
	if err != nil {
		return err
	}

	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return err
	}

	pods, err := listPodsInSelectors(client, execCollector.Namespace, execCollector.Selector)
	if err != nil {
		return err
	}

	execOutput := &ExecOutput{
		Results: make(map[string][]byte),
	}

	for _, pod := range pods {
		output, err := getExecOutputs(client, pod, execCollector, redact)
		if err != nil {
			return err
		}

		execOutput.Results[fmt.Sprintf("%s/%s/%s-stdout.txt", pod.Namespace, pod.Name, execCollector.Name)] = output.Stdout
		execOutput.Results[fmt.Sprintf("%s/%s/%s-stderr.txt", pod.Namespace, pod.Name, execCollector.Name)] = output.Stderr
		if output.Error == nil {
			continue
		}

		errOutput := map[string]string{
			"error": output.Error.Error(),
		}
		b, err := json.MarshalIndent(errOutput, "", "  ")
		if err != nil {
			return err
		}

		execOutput.Results[fmt.Sprintf("%s/%s/%s.json", pod.Namespace, pod.Name, execCollector.Name)] = b
	}

	if redact {
		execOutput, err = execOutput.Redact()
		if err != nil {
			return err
		}
	}

	b, err := json.MarshalIndent(execOutput, "", "  ")
	if err != nil {
		return err
	}

	fmt.Printf("%s\n", b)

	return nil
}

func getExecOutputs(client *kubernetes.Clientset, pod corev1.Pod, execCollector *troubleshootv1beta1.Exec, doRedact bool) (*execResult, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, err
	}

	container := pod.Spec.Containers[0].Name
	if execCollector.ContainerName != "" {
		container = execCollector.ContainerName
	}

	req := client.CoreV1().RESTClient().Post().Resource("pods").Name(pod.Name).Namespace(pod.Namespace).SubResource("exec")
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		return nil, err
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
		return nil, err
	}

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)

	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  nil,
		Stdout: stdout,
		Stderr: stderr,
		Tty:    false,
	})

	return &execResult{
		Stdout: stdout.Bytes(),
		Stderr: stderr.Bytes(),
		Error:  err,
	}, nil
}

func (r *ExecOutput) Redact() (*ExecOutput, error) {
	results, err := redactMap(r.Results)
	if err != nil {
		return nil, err
	}

	return &ExecOutput{
		Results: results,
	}, nil
}
