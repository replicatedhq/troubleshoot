package collect

import (
	"net/http"
	"path/filepath"

	"github.com/pkg/errors"
)

func HostHTTP(c *HostCollector) (map[string][]byte, error) {
	httpCollector := c.Collect.HTTP
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

	output, err := responseToOutput(response, err, false)
	if err != nil {
		return nil, err
	}

	fileName := "result.json"
	if httpCollector.CollectorName != "" {
		fileName = httpCollector.CollectorName + ".json"
	}
	httpOutput := map[string][]byte{
		filepath.Join("http", fileName): output,
	}

	return httpOutput, nil
}
