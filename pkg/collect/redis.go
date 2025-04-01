package collect

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-redis/redis/v7"
	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
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
		klog.V(2).Infof("Connecting to redis in mutual TLSCertificate")
		return c.createMTLSClient(opt)
	}

	klog.V(2).Infof("Connecting to redis in plain text")
	return redis.NewClient(opt), nil
}

func (c *CollectRedis) createMTLSClient(opt *redis.Options) (*redis.Client, error) {
	tlsCfg, err := createTLSConfig(c.Context, c.Client, c.Collector.TLS)
	if err != nil {
		return nil, err
	}

	opt.TLSConfig = tlsCfg

	return redis.NewClient(opt), nil
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
