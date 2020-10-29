package collect

import (
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// from dd-agent
func getCertificates(certFilePath, keyFilePath string) ([]tls.Certificate, error) {
	var certs []tls.Certificate
	cert, err := tls.LoadX509KeyPair(certFilePath, keyFilePath)
	if err != nil {
		return certs, err
	}
	return append(certs, cert), nil
}

func KubeletMetrics(c *Collector) (map[string][]byte, error) {
	// get all nodes, query the kubelet on each
	// hopefully we don't need to deplyo a daemonset and query locally

	fmt.Printf("%#v\n", c.ClientConfig)
	client, err := kubernetes.NewForConfig(c.ClientConfig)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()

	nodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to list nodes")
	}

	kubeletMetricsOutput := map[string][]byte{}

	for _, node := range nodes.Items {
		nodeAddress := node.Status.Addresses[0].Address

		// TODO read the CA and cert to make this trusted
		// it's all in c.ClientConfig

		customTransport := http.DefaultTransport.(*http.Transport).Clone()

		tlsConfig := &tls.Config{}
		tlsConfig.InsecureSkipVerify = true

		customTransport.TLSClientConfig = tlsConfig

		tlsConfig.Certificates, err = getCertificates("/tmp/client-kube-apiserver.crt", "/tmp/client-kube-apiserver.key")
		if err != nil {
			return nil, err
		}

		// headers := http.Header{}
		// headers.Set("Authorization", fmt.Sprintf("%s", "i don't know what this is or how to get it"))

		fmt.Printf("\n---> %s\n", c.ClientConfig.BearerToken)

		req := http.Request{}
		// req.Header = headers
		u, err := url.Parse(fmt.Sprintf("https://%s:10250/metrics", nodeAddress))
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse url")
		}
		req.URL = u

		httpClient := http.Client{
			Transport: customTransport,
		}

		resp, err := httpClient.Do(&req)
		if err != nil {
			return nil, errors.Wrap(err, "failed to execute kubelet request")
		}
		defer resp.Body.Close()

		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read body")
		}

		fmt.Printf("%s\n", b)
	}

	return kubeletMetricsOutput, nil
}
