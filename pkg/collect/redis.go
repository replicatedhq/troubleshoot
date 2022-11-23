package collect

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-redis/redis/v7"
	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

type CollectRedis struct {
	Collector    *troubleshootv1beta2.Database
	BundlePath   string
	Namespace    string
	ClientConfig *rest.Config
	Client       kubernetes.Interface
	Context      context.Context
	RBACErrors
}

func (c *CollectRedis) Title() string {
	return getCollectorName(c)
}

func (c *CollectRedis) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
}

func (c *CollectRedis) createClient() (*redis.Client, error) {
	opt, err := redis.ParseURL(c.Collector.URI)
	if err != nil {
		return nil, err
	}

	if c.Collector.TLS != nil {
		klog.V(2).Infof("Connecting to redis in mutual TLS")
		return c.createMTLSClient(opt)
	}

	klog.V(2).Infof("Connecting to redis in plain text")
	return redis.NewClient(opt), nil
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

func (c *CollectRedis) createMTLSClient(opt *redis.Options) (*redis.Client, error) {
	tlsCfg, err := createTLSConfig(c.Context, c.Client, c.Collector.TLS)
	if err != nil {
		return nil, err
	}

	opt.TLSConfig = tlsCfg

	return redis.NewClient(opt), nil
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

func extractServerVersion(info string) string {
	lines := strings.Split(info, "\n")
	for _, line := range lines {
		lineParts := strings.Split(strings.TrimSpace(line), ":")
		if len(lineParts) == 2 {
			if lineParts[0] == "redis_version" {
				return strings.TrimSpace(lineParts[1])
			}
		}
	}

	return ""
}

func (c *CollectRedis) Collect(progressChan chan<- interface{}) (CollectorResult, error) {
	databaseConnection := DatabaseConnection{}

	client, err := c.createClient()
	if err != nil {
		databaseConnection.Error = err.Error()
	} else {
		defer client.Close()

		stringResult := client.Info("server")

		if stringResult.Err() != nil {
			databaseConnection.Error = stringResult.Err().Error()
		}

		databaseConnection.IsConnected = stringResult.Err() == nil

		if databaseConnection.Error == "" {
			databaseConnection.Version = extractServerVersion(stringResult.Val())
		}
	}

	b, err := json.Marshal(databaseConnection)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal database connection")
	}

	collectorName := c.Collector.CollectorName
	if collectorName == "" {
		collectorName = "redis"
	}

	output := NewResult()
	output.SaveResult(c.BundlePath, fmt.Sprintf("redis/%s.json", collectorName), bytes.NewBuffer(b))

	return output, nil
}
