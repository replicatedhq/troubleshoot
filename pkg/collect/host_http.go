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

	if httpCollector.Get != nil {
		response, err = doGet(httpCollector.Get)
	} else if httpCollector.Post != nil {
		response, err = doPost(httpCollector.Post)
	} else if httpCollector.Put != nil {
		response, err = doPut(httpCollector.Put)
	} else {
		return nil, errors.New("no supported http request type")
	}

	responseOutput, err := responseToOutput(response, err, false)
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
