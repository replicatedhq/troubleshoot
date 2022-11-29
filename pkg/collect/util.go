package collect

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"strings"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func DeterministicIDForCollector(collector *troubleshootv1beta2.Collect) string {
	unsafeID := ""

	if collector.ClusterInfo != nil {
		unsafeID = "cluster-info"
	}

	if collector.ClusterResources != nil {
		unsafeID = "cluster-resources"
	}

	if collector.Secret != nil {
		if collector.Secret.Name != "" {
			unsafeID = fmt.Sprintf("secret-%s-%s", collector.Secret.Namespace, collector.Secret.Name)
		} else {
			unsafeID = fmt.Sprintf("secret-%s-%s", collector.Secret.Namespace, selectorToString(collector.Secret.Selector))
		}
	}

	if collector.ConfigMap != nil {
		if collector.ConfigMap.Name != "" {
			unsafeID = fmt.Sprintf("configmap-%s-%s", collector.ConfigMap.Namespace, collector.ConfigMap.Name)
		} else {
			unsafeID = fmt.Sprintf("configmap-%s-%s", collector.ConfigMap.Namespace, selectorToString(collector.ConfigMap.Selector))
		}
	}

	if collector.Logs != nil {
		unsafeID = fmt.Sprintf("logs-%s-%s", collector.Logs.Namespace, selectorToString(collector.Logs.Selector))
	}

	if collector.Run != nil {
		unsafeID = "run"
		if collector.Run.CollectorName != "" {
			unsafeID = fmt.Sprintf("%s-%s", unsafeID, strings.ToLower(collector.Run.CollectorName))
		}
	}

	if collector.Exec != nil {
		unsafeID = "exec"
		if collector.Exec.CollectorName != "" {
			unsafeID = fmt.Sprintf("%s-%s", unsafeID, strings.ToLower(collector.Exec.CollectorName))
		}
	}

	if collector.Copy != nil {
		unsafeID = fmt.Sprintf("copy-%s-%s", selectorToString(collector.Copy.Selector), pathToString(collector.Copy.ContainerPath))
	}

	if collector.HTTP != nil {
		unsafeID = "http"
		if collector.HTTP.CollectorName != "" {
			unsafeID = fmt.Sprintf("%s-%s", unsafeID, strings.ToLower(collector.HTTP.CollectorName))
		}
	}

	return rfc1035(unsafeID)
}

func selectorToString(selector []string) string {
	return strings.Replace(strings.Join(selector, "-"), "=", "-", -1)
}

func pathToString(path string) string {
	return strings.Replace(path, "/", "-", -1)
}

func rfc1035(in string) string {
	reg := regexp.MustCompile("[^a-z0-9\\-]+")
	out := reg.ReplaceAllString(in, "-")

	if len(out) > 63 {
		out = out[:63]
	}

	return out
}

// Use for error maps and arrays. These are guaraneteed to not result in a error when marshaling.
func marshalErrors(errors interface{}) io.Reader {
	if errors == nil {
		return nil
	}

	val := reflect.ValueOf(errors)
	switch val.Kind() {
	case reflect.Array, reflect.Slice, reflect.Map:
		if val.Len() == 0 {
			return nil
		}
	}

	m, _ := json.MarshalIndent(errors, "", "  ")
	return bytes.NewBuffer(m)
}

// listNodesNamesInSelector returns a list of node names matching the label
// selector,
func listNodesNamesInSelector(ctx context.Context, client *kubernetes.Clientset, selector string) ([]string, error) {
	var names []string
	nodes, err := listNodesInSelector(ctx, client, selector)
	if err != nil {
		return nil, err
	}
	for _, node := range nodes {
		names = append(names, node.GetName())
	}
	return names, nil
}

// listNodesInSelector returns a list of node names matching the label
// selector,
func listNodesInSelector(ctx context.Context, client *kubernetes.Clientset, selector string) ([]corev1.Node, error) {
	listOptions := metav1.ListOptions{
		LabelSelector: selector,
	}

	nodes, err := client.CoreV1().Nodes().List(ctx, listOptions)
	if err != nil {
		return nil, fmt.Errorf("Can't get the list of nodes, got: %w", err)
	}

	return nodes.Items, nil
}

func createTLSConfig(ctx context.Context, client kubernetes.Interface, params *troubleshootv1beta2.TLSParams) (*tls.Config, error) {
	rootCA, err := x509.SystemCertPool()
	if err != nil {
		rootCA = x509.NewCertPool()
	}

	tlsCfg := &tls.Config{}

	if params.SkipVerify {
		tlsCfg.InsecureSkipVerify = true
		return tlsCfg, nil
	}

	var caCert, clientCert, clientKey string
	if params.Secret != nil {
		caCert, clientCert, clientKey, err = getTLSParamsFromSecret(ctx, client, params.Secret)
		if err != nil {
			return nil, err
		}
	} else {
		caCert = params.CACert
		clientCert = params.ClientCert
		clientKey = params.ClientKey
	}

	if ok := rootCA.AppendCertsFromPEM([]byte(caCert)); !ok {
		return nil, fmt.Errorf("failed to append CA cert to root CA bundle")
	}
	tlsCfg.RootCAs = rootCA

	if clientCert == "" && clientKey == "" {
		return tlsCfg, nil
	}

	certPair, err := tls.X509KeyPair([]byte(clientCert), []byte(clientKey))
	if err != nil {
		return nil, err
	}

	tlsCfg.Certificates = []tls.Certificate{certPair}

	return tlsCfg, nil
}

func getTLSParamsFromSecret(ctx context.Context, client kubernetes.Interface, secretParams *troubleshootv1beta2.TLSSecret) (string, string, string, error) {
	var caCert, clientCert, clientKey string
	secret, err := client.CoreV1().Secrets(secretParams.Namespace).Get(ctx, secretParams.Name, metav1.GetOptions{})
	if err != nil {
		return "", "", "", errors.Wrap(err, "failed to get secret")
	}

	if val, ok := secret.StringData["cacert"]; ok {
		caCert = val
	} else {
		return "", "", "", fmt.Errorf("failed to find 'cacert' key for CA cert data in secret")
	}

	var foundClientCert, foundClientKey bool
	if val, ok := secret.StringData["clientCert"]; ok {
		clientCert = val
		foundClientCert = true
	}

	if val, ok := secret.StringData["clientKey"]; ok {
		clientKey = val
		foundClientKey = true
	}

	if !foundClientCert && !foundClientKey {
		// Cert only configuration
		return caCert, "", "", nil
	}

	if !foundClientKey {
		return "", "", "", fmt.Errorf("failed to find 'clientKey' for client key data in secret")
	}

	if !foundClientCert {
		return "", "", "", fmt.Errorf("failed to find 'clientCert' for client cert data in secret")
	}

	return caCert, clientCert, clientKey, nil
}
