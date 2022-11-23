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

func (c *CollectRedis) createMTLSClient(opt *redis.Options) (*redis.Client, error) {
	rootCA, err := x509.SystemCertPool()
	if err != nil {
		rootCA = x509.NewCertPool()
	}
	tParams := c.Collector.TLS

	if tParams.SkipVerify {
		opt.TLSConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
		return redis.NewClient(opt), nil
	}

	var caCert, clientCert, clientKey []byte
	if tParams.Secret != nil {
		caCert, clientCert, clientKey, err = c.getTLSParamsFromSecret(tParams.Secret)
		if err != nil {
			return nil, err
		}
	} else {
		caCert = []byte(tParams.CACert)
		clientCert = []byte(tParams.ClientCert)
		clientKey = []byte(tParams.ClientKey)
	}

	if ok := rootCA.AppendCertsFromPEM(caCert); !ok {
		return nil, fmt.Errorf("failed to append CA cert to root CA bundle")
	}

	certPair, err := tls.X509KeyPair(clientCert, clientKey)
	if err != nil {
		return nil, err
	}

	opt.TLSConfig = &tls.Config{
		Certificates: []tls.Certificate{certPair},
		RootCAs:      rootCA,
	}

	return redis.NewClient(opt), nil
}

func (c *CollectRedis) getTLSParamsFromSecret(secretParams *troubleshootv1beta2.TLSSecret) ([]byte, []byte, []byte, error) {
	var caCert, clientCert, clientKey []byte
	secret, err := c.Client.CoreV1().Secrets(secretParams.Namespace).Get(c.Context, secretParams.Name, metav1.GetOptions{})
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "failed to get secret")
	}

	if val, ok := secret.StringData["cacert"]; ok {
		caCert = []byte(val)
	} else {
		return nil, nil, nil, fmt.Errorf("failed to find 'cacert' key for CA cert data in secret")
	}

	if val, ok := secret.StringData["clientCert"]; ok {
		clientCert = []byte(val)
	} else {
		return nil, nil, nil, fmt.Errorf("failed to find 'clientCert' for client cert data in secret")
	}

	if val, ok := secret.StringData["clientKey"]; ok {
		clientKey = []byte(val)
	} else {
		return nil, nil, nil, fmt.Errorf("failed to find 'clientKey' for client key data in secret")
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
