package collect

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"crypto/tls"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"net/http"
)

type S3StatusResult struct {
	BucketName  string `json:"bucketName"`
	Endpoint    string `json:"endpoint,omitempty"`
	Region      string `json:"region,omitempty"`
	IsConnected bool   `json:"isConnected"`
	Error       string `json:"error,omitempty"`
}

type CollectS3Status struct {
	Collector  *troubleshootv1beta2.S3Status
	BundlePath string
	RBACErrors
}

func (c *CollectS3Status) Title() string {
	return getCollectorName(c)
}

func (c *CollectS3Status) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
}

func (c *CollectS3Status) Collect(progressChan chan<- interface{}) (CollectorResult, error) {
	result := S3StatusResult{
		BucketName: c.Collector.BucketName,
		Endpoint:   c.Collector.Endpoint,
		Region:     c.Collector.Region,
	}

	region := c.Collector.Region
	if region == "" {
		region = "us-east-1"
	}

	opts := s3.Options{
		Region: region,
		Credentials: credentials.NewStaticCredentialsProvider(
			c.Collector.AccessKeyID,
			c.Collector.SecretAccessKey,
			"",
		),
		UsePathStyle: c.Collector.UsePathStyle,
	}

	if c.Collector.Endpoint != "" {
		opts.BaseEndpoint = aws.String(c.Collector.Endpoint)
	}

	if c.Collector.Insecure {
		opts.HTTPClient = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}
	}

	client := s3.New(opts)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(c.Collector.BucketName),
	})
	if err != nil {
		result.Error = err.Error()
	} else {
		result.IsConnected = true
	}

	b, err := json.Marshal(result)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal s3 status result")
	}

	collectorName := c.Collector.CollectorName
	if collectorName == "" {
		collectorName = "s3Status"
	}

	output := NewResult()
	output.SaveResult(c.BundlePath, fmt.Sprintf("s3Status/%s.json", collectorName), bytes.NewBuffer(b))

	return output, nil
}
