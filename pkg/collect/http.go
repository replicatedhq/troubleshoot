package collect

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"

	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	"github.com/replicatedhq/troubleshoot/pkg/redact"
)

type HTTPOutput map[string][]byte

type httpResponse struct {
	Status  int               `json:"status"`
	Body    string            `json:"body"`
	Headers map[string]string `json:"headers"`
}

type httpError struct {
	Message string `json:"message"`
}

func HTTP(ctx *Context, httpCollector *troubleshootv1beta1.HTTP) ([]byte, error) {
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

	output, err := responseToOutput(response, err, ctx.Redact)
	if err != nil {
		return nil, err
	}

	fileName := "result.json"
	if httpCollector.CollectorName != "" {
		fileName = httpCollector.CollectorName + ".json"
	}
	httpOutput := HTTPOutput{
		filepath.Join(httpCollector.Name, fileName): output,
	}

	b, err := json.MarshalIndent(httpOutput, "", "  ")
	if err != nil {
		return nil, err
	}

	return b, nil
}

func doGet(get *troubleshootv1beta1.Get) (*http.Response, error) {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{
		InsecureSkipVerify: get.InsecureSkipVerify,
	}

	req, err := http.NewRequest("GET", get.URL, nil)
	if err != nil {
		return nil, err
	}

	for k, v := range get.Headers {
		req.Header.Set(k, v)
	}

	return http.DefaultClient.Do(req)
}

func doPost(post *troubleshootv1beta1.Post) (*http.Response, error) {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{
		InsecureSkipVerify: post.InsecureSkipVerify,
	}

	req, err := http.NewRequest("POST", post.URL, strings.NewReader(post.Body))
	if err != nil {
		return nil, err
	}

	for k, v := range post.Headers {
		req.Header.Set(k, v)
	}

	return http.DefaultClient.Do(req)
}

func doPut(put *troubleshootv1beta1.Put) (*http.Response, error) {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{
		InsecureSkipVerify: put.InsecureSkipVerify,
	}

	req, err := http.NewRequest("PUT", put.URL, strings.NewReader(put.Body))
	if err != nil {
		return nil, err
	}

	for k, v := range put.Headers {
		req.Header.Set(k, v)
	}

	return http.DefaultClient.Do(req)
}

func responseToOutput(response *http.Response, err error, doRedact bool) ([]byte, error) {
	output := make(map[string]interface{})
	if err != nil {
		output["error"] = httpError{
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

		output["response"] = httpResponse{
			Status:  response.StatusCode,
			Body:    string(body),
			Headers: headers,
		}
	}

	b, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return nil, err
	}

	if doRedact {
		b, err = redact.Redact(b)
		if err != nil {
			return nil, err
		}
	}

	return b, nil
}
