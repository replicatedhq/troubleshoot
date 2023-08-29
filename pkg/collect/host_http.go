package collect

import (
	"bytes"
	"net/http"
	"path/filepath"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

type CollectHostHTTP struct {
	hostCollector *troubleshootv1beta2.HostHTTP
	BundlePath    string
}

func (c *CollectHostHTTP) Title() string {
	return hostCollectorTitleOrDefault(c.hostCollector.HostCollectorMeta, "HTTP Request")
}

func (c *CollectHostHTTP) IsExcluded() (bool, error) {
	return isExcluded(c.hostCollector.Exclude)
}

func (c *CollectHostHTTP) Collect(progressChan chan<- interface{}) (map[string][]byte, error) {
	httpCollector := c.hostCollector

	var response *http.Response
	var err error

	switch {
	case httpCollector.Get != nil:
		response, err = doRequest(
			"GET", httpCollector.Get.URL, httpCollector.Get.Headers,
			"", httpCollector.Get.InsecureSkipVerify, httpCollector.Get.Timeout)
	case httpCollector.Post != nil:
		response, err = doRequest(
			"POST", httpCollector.Post.URL, httpCollector.Post.Headers,
			httpCollector.Post.Body, httpCollector.Post.InsecureSkipVerify, httpCollector.Post.Timeout)
	case httpCollector.Put != nil:
		response, err = doRequest(
			"PUT", httpCollector.Put.URL, httpCollector.Put.Headers,
			httpCollector.Put.Body, httpCollector.Put.InsecureSkipVerify, httpCollector.Put.Timeout)
	default:
		return nil, errors.New("no supported http request type")
	}

	responseOutput, err := responseToOutput(response, err)
	if err != nil {
		return nil, err
	}

	collectorName := c.hostCollector.CollectorName
	if collectorName == "" {
		collectorName = "result"
	}
	name := filepath.Join("host-collectors/http", collectorName+".json")

	output := NewResult()
	output.SaveResult(c.BundlePath, name, bytes.NewBuffer(responseOutput))

	httpOutput := map[string][]byte{
		name: responseOutput,
	}

	return httpOutput, nil
}
