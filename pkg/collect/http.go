package collect

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type HTTPResponse struct {
	Status  int               `json:"status"`
	Body    string            `json:"body"`
	Headers map[string]string `json:"headers"`
}

type HTTPError struct {
	Message string `json:"message"`
}

type CollectHTTP struct {
	Collector    *troubleshootv1beta2.HTTP
	BundlePath   string
	Namespace    string
	ClientConfig *rest.Config
	Client       kubernetes.Interface
	RBACErrors
}

func (c *CollectHTTP) Title() string {
	return getCollectorName(c)
}

func (c *CollectHTTP) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
}

func (c *CollectHTTP) Collect(progressChan chan<- interface{}) (CollectorResult, error) {
	var response *http.Response
	var err error

	switch {
	case c.Collector.Get != nil:
		response, err = doRequest(
			"GET", c.Collector.Get.URL, c.Collector.Get.Headers,
			"", c.Collector.Get.InsecureSkipVerify, c.Collector.Get.Timeout)
	case c.Collector.Post != nil:
		response, err = doRequest(
			"POST", c.Collector.Post.URL, c.Collector.Post.Headers,
			c.Collector.Post.Body, c.Collector.Post.InsecureSkipVerify, c.Collector.Post.Timeout)
	case c.Collector.Put != nil:
		response, err = doRequest(
			"PUT", c.Collector.Put.URL, c.Collector.Put.Headers,
			c.Collector.Put.Body, c.Collector.Put.InsecureSkipVerify, c.Collector.Put.Timeout)
	default:
		return nil, errors.New("no supported http request type")
	}

	o, err := responseToOutput(response, err)
	if err != nil {
		return nil, err
	}

	fileName := "result.json"
	if c.Collector.CollectorName != "" {
		fileName = c.Collector.CollectorName + ".json"
	}

	output := NewResult()
	output.SaveResult(c.BundlePath, filepath.Join(c.Collector.Name, fileName), bytes.NewBuffer(o))

	return output, nil
}

func doRequest(method, url string, headers map[string]string, body string, insecureSkipVerify bool, timeout time.Duration) (*http.Response, error) {
	httpClient := &http.Client{
		Timeout: timeout,
	}

	if insecureSkipVerify {
		httpClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
	}

	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		return nil, err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	return httpClient.Do(req)
}

func responseToOutput(response *http.Response, err error) ([]byte, error) {
	output := make(map[string]interface{})
	if err != nil {
		output["error"] = HTTPError{
			Message: err.Error(),
		}
	} else {
		body, err := ioutil.ReadAll(response.Body)
		if err != nil {
			return nil, err
		}

		headers := make(map[string]string)
		for k, v := range response.Header {
			headers[k] = strings.Join(v, ",")
		}

		output["response"] = HTTPResponse{
			Status:  response.StatusCode,
			Body:    string(body),
			Headers: headers,
		}
	}

	b, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return nil, err
	}

	return b, nil
}
