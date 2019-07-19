package collect

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/websocket"
	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
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

	u := client.CoreV1().RESTClient().Get().Namespace(pod.Namespace).Name(pod.Name).
		Resource("pods").SubResource("exec").
		Param("command", "/bin/cat").Param("command", copyCollector.ContainerPath).
		Param("container", container).Param("stderr", "true").Param("stdout", "true").URL()

	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	case "http":
		u.Scheme = "ws"
	default:
		return nil, err
	}

	req := &http.Request{
		Method: http.MethodGet,
		URL:    u,
	}

	tlsConfig, err := rest.TLSConfigFor(cfg)
	if err != nil {
		return nil, err
	}

	dialer := &websocket.Dialer{
		Proxy:           http.ProxyFromEnvironment,
		TLSClientConfig: tlsConfig,
	}

	c, _, err := dialer.Dial(u.String(), req.Header)
	if err != nil {
		return nil, err
	}
	defer c.Close()

	var res []byte
	for {
		msgT, p, err := c.ReadMessage()
		if err != nil {
			if _, ok := err.(*websocket.CloseError); ok {
				break
			}
			fmt.Printf("err %T %v\n", err, err)
			break
		}
		if msgT != 2 {
			return nil, fmt.Errorf("unknown message type %d", msgT)
		}
		res = append(res, p...)
	}

	return map[string][]byte{
		copyCollector.ContainerPath: res,
	}, nil
}
