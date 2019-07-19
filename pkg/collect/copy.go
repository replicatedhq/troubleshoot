package collect

import (
	"bytes"
	"encoding/json"
	"fmt"

	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/remotecommand"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

type CopyOutput struct {
	Files map[string][]byte `json:"copy/,omitempty"`
}

func Copy(copyCollector *troubleshootv1beta1.Copy, redact bool) error {
	cfg, err := config.GetConfig()
	if err != nil {
		return err
	}

	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return err
	}

	pods, err := listPodsInSelectors(client, copyCollector.Namespace, copyCollector.Selector)
	if err != nil {
		return err
	}

	copyOutput := &CopyOutput{
		Files: make(map[string][]byte),
	}

	for _, pod := range pods {
		files, err := copyFiles(client, pod, copyCollector)
		if err != nil {
			return err
		}

		for k, v := range files {
			copyOutput.Files[k] = v
		}
	}

	if redact {
		// TODO
	}

	b, err := json.MarshalIndent(copyOutput, "", "  ")
	if err != nil {
		return err
	}

	fmt.Printf("%s\n", b)

	return nil
}

func copyFiles(client *kubernetes.Clientset, pod corev1.Pod, copyCollector *troubleshootv1beta1.Copy) (map[string][]byte, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, err
	}

	container := pod.Spec.Containers[0].Name
	if copyCollector.ContainerName != "" {
		container = copyCollector.ContainerName
	}

	command := []string{"cat", copyCollector.ContainerPath}

	output := new(bytes.Buffer)

	req := client.CoreV1().RESTClient().Post().Resource("pods").Name(pod.Name).Namespace(pod.Namespace).SubResource("exec")
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		return nil, err
	}

	parameterCodec := runtime.NewParameterCodec(scheme)
	req.VersionedParams(&corev1.PodExecOptions{
		Command:   command,
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

	var stderr bytes.Buffer
	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  nil,
		Stdout: output,
		Stderr: &stderr,
		Tty:    false,
	})
	if err != nil {
		return nil, err
	}

	return map[string][]byte{
		fmt.Sprintf("%s/%s/%s", pod.Namespace, pod.Name, copyCollector.ContainerPath): output.Bytes(),
	}, nil
}
