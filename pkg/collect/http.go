package collect

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"

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

var (
	httpInsecureClient = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
)

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

	if c.Collector.Get != nil {
		response, err = doGet(c.Collector.Get)
	} else if c.Collector.Post != nil {
		response, err = doPost(c.Collector.Post)
	} else if c.Collector.Put != nil {
		response, err = doPut(c.Collector.Put)
	} else {
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

func doGet(get *troubleshootv1beta2.Get) (*http.Response, error) {

	httpClient := http.DefaultClient
	if get.InsecureSkipVerify {
		httpClient = httpInsecureClient
	}
	if get.Timeout != 0 {
		httpClient.Timeout = get.Timeout
	}
	req, err := http.NewRequest("GET", get.URL, nil)
	if err != nil {
		return nil, err
	}

	for k, v := range get.Headers {
		req.Header.Set(k, v)
	}

	return httpClient.Do(req)
}

func doPost(post *troubleshootv1beta2.Post) (*http.Response, error) {
	httpClient := http.DefaultClient
	if post.InsecureSkipVerify {
		httpClient = httpInsecureClient
	}
	if post.Timeout != 0 {
		httpClient.Timeout = post.Timeout
	}
	req, err := http.NewRequest("POST", post.URL, strings.NewReader(post.Body))
	if err != nil {
		return nil, err
	}

	for k, v := range post.Headers {
		req.Header.Set(k, v)
	}

	return httpClient.Do(req)
}

func doPut(put *troubleshootv1beta2.Put) (*http.Response, error) {
	httpClient := http.DefaultClient
	if put.InsecureSkipVerify {
		httpClient = httpInsecureClient
	}
	if put.Timeout != 0 {
		httpClient.Timeout = put.Timeout
	}
	req, err := http.NewRequest("PUT", put.URL, strings.NewReader(put.Body))
	if err != nil {
		return nil, err
	}

	for k, v := range put.Headers {
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
