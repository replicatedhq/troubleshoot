package collect

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

//Copy function gets a file or folder from a container specified in the specs.
func Copy(c *Collector, copyCollector *troubleshootv1beta2.Copy) (map[string][]byte, error) {
	client, err := kubernetes.NewForConfig(c.ClientConfig)
	if err != nil {
		return nil, err
	}

	copyOutput := map[string][]byte{}

	ctx := context.Background()

	pods, podsErrors := listPodsInSelectors(ctx, client, copyCollector.Namespace, copyCollector.Selector)
	if len(podsErrors) > 0 {
		errorBytes, err := marshalNonNil(podsErrors)
		if err != nil {
			return nil, err
		}
		copyOutput[getCopyErrosFileName(copyCollector)] = errorBytes
	}

	if len(pods) > 0 {
		for _, pod := range pods {
			bundlePath := filepath.Join(copyCollector.Name, pod.Namespace, pod.Name, copyCollector.ContainerName)

			files, copyErrors := copyFiles(ctx, client, c, pod, copyCollector)
			if len(copyErrors) > 0 {
				key := filepath.Join(bundlePath, copyCollector.ContainerPath+"-errors.json")
				copyOutput[key], err = marshalNonNil(copyErrors)
				if err != nil {
					return nil, err
				}
				continue
			}

			for k, v := range files {
				copyOutput[filepath.Join(bundlePath, filepath.Dir(copyCollector.ContainerPath), k)] = v
			}
		}
	}

	return copyOutput, nil
}

func copyFiles(ctx context.Context, client *kubernetes.Clientset, c *Collector, pod corev1.Pod, copyCollector *troubleshootv1beta2.Copy) (map[string][]byte, map[string]string) {
	containerName := pod.Spec.Containers[0].Name
	if copyCollector.ContainerName != "" {
		containerName = copyCollector.ContainerName
	}

	stdout, stderr, err := getFilesFromPod(ctx, c.ClientConfig, client, pod.Name, containerName, pod.Namespace, copyCollector.ContainerPath)
	if err != nil {
		errors := map[string]string{
			filepath.Join(copyCollector.ContainerPath, "error"): err.Error(),
		}
		if len(stdout) > 0 {
			errors[filepath.Join(copyCollector.ContainerPath, "stdout")] = string(stdout)
		}
		if len(stderr) > 0 {
			errors[filepath.Join(copyCollector.ContainerPath, "stderr")] = string(stderr)
		}
		return nil, errors
	}

	return map[string][]byte{
		filepath.Base(copyCollector.ContainerPath) + ".tar": stdout,
	}, nil
}

func getFilesFromPod(ctx context.Context, clientConfig *restclient.Config, client kubernetes.Interface, podName string, containerName string, namespace string, containerPath string) ([]byte, []byte, error) {
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
		Stdin:     true,
		Stdout:    false,
		Stderr:    true,
		TTY:       false,
	}, parameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(clientConfig, "POST", req.URL())
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to create SPDY executor")
	}

	output := new(bytes.Buffer)
	var stderr bytes.Buffer
	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  nil,
		Stdout: output,
		Stderr: &stderr,
		Tty:    false,
	})
	if err != nil {
		return output.Bytes(), stderr.Bytes(), errors.Wrap(err, "failed to stream command output")
	}

	return output.Bytes(), stderr.Bytes(), nil
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
