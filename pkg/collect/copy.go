package collect

import (
	"bytes"
	"context"
	"fmt"
	"path"

	"path/filepath"

	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/remotecommand"
)

func Copy(c *Collector, copyCollector *troubleshootv1beta1.Copy) (map[string][]byte, error) {

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

			files, copyErrors := copyFiles(c, client, pod, copyCollector)
			if len(copyErrors) > 0 {
				key := filepath.Join(bundlePath, copyCollector.ContainerPath+"-errors.json")
				copyOutput[key], err = marshalNonNil(copyErrors)
				if err != nil {
					return nil, err
				}
				continue
			}

			for k, v := range files {
				copyOutput[filepath.Join(bundlePath, path.Dir(copyCollector.ContainerPath), k)] = v
			}
		}
	}

	return copyOutput, nil
}

func copyFiles(c *Collector, client *kubernetes.Clientset, pod corev1.Pod, copyCollector *troubleshootv1beta1.Copy) (map[string][]byte, map[string]string) {
	container := pod.Spec.Containers[0].Name
	if copyCollector.ContainerName != "" {
		container = copyCollector.ContainerName
	}
	//Command cd into the path directory, tars the target file or folder into a temp file, cat-copy it and removes the temp file.
	//Not using a temp file may end up in error "Refusing to write archive contents to terminal"
	command := []string{"sh", "-c", fmt.Sprintf("tar -C %v  -cf tmp.tar %v; cat tmp.tar; rm tmp.tar", path.Dir(copyCollector.ContainerPath), path.Base(copyCollector.ContainerPath))}

	req := client.CoreV1().RESTClient().Post().Resource("pods").Name(pod.Name).Namespace(pod.Namespace).SubResource("exec")
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		return nil, map[string]string{
			filepath.Join(copyCollector.ContainerPath, "error"): err.Error(),
		}
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

	exec, err := remotecommand.NewSPDYExecutor(c.ClientConfig, "POST", req.URL())
	if err != nil {
		return nil, map[string]string{
			filepath.Join(copyCollector.ContainerPath, "error"): err.Error(),
		}
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
		errors := map[string]string{
			filepath.Join(copyCollector.ContainerPath, "error"): err.Error(),
		}
		if s := output.String(); len(s) > 0 {
			errors[filepath.Join(copyCollector.ContainerPath, "stdout")] = s
		}
		if s := stderr.String(); len(s) > 0 {
			errors[filepath.Join(copyCollector.ContainerPath, "stderr")] = s
		}
		return nil, errors
	}

	return map[string][]byte{
		path.Base(copyCollector.ContainerPath) + ".tar": output.Bytes(),
	}, nil
}

func getCopyErrosFileName(copyCollector *troubleshootv1beta1.Copy) string {
	if len(copyCollector.Name) > 0 {
		return fmt.Sprintf("%s-errors.json", copyCollector.Name)
	}
	if len(copyCollector.CollectorName) > 0 {
		return fmt.Sprintf("%s-errors.json", copyCollector.CollectorName)
	}
	// TODO: random part
	return "errors.json"
}
