package collect

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	neturl "net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

type HTTPResponse struct {
	Status  int               `json:"status"`
	Body    string            `json:"body"`
	Headers map[string]string `json:"headers"`
	RawJSON json.RawMessage   `json:"raw_json,omitempty"`
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
			"", c.Collector.Get.InsecureSkipVerify, c.Collector.Get.Timeout, c.Collector.TLS.CACert, c.Collector.Proxy)
	case c.Collector.Post != nil:
		response, err = doRequest(
			"POST", c.Collector.Post.URL, c.Collector.Post.Headers,
			c.Collector.Post.Body, c.Collector.Post.InsecureSkipVerify, c.Collector.Post.Timeout, c.Collector.TLS.CACert, c.Collector.Proxy)
	case c.Collector.Put != nil:
		response, err = doRequest(
			"PUT", c.Collector.Put.URL, c.Collector.Put.Headers,
			c.Collector.Put.Body, c.Collector.Put.InsecureSkipVerify, c.Collector.Put.Timeout, c.Collector.TLS.CACert, c.Collector.Proxy)
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

func doRequest(method, url string, headers map[string]string, body string, insecureSkipVerify bool, timeout string, cacert string, proxy string) (*http.Response, error) {
	t, err := parseTimeout(timeout)
	if err != nil {
		return nil, err
	}

	var httpClient *http.Client
	var tlsConfig *tls.Config

	if cacert != "" {
		certPool := x509.NewCertPool()

		if !certPool.AppendCertsFromPEM([]byte(cacert)) {
			return nil, errors.New("failed to append certificate to cert pool")
		}

		tlsConfig = &tls.Config{
			RootCAs:            certPool,
			InsecureSkipVerify: true,
		}

		fmt.Printf("Using tlsConfig cert: %+v", tlsConfig)

		fn := func(req *http.Request) (*neturl.URL, error) {
			return neturl.Parse(proxy)
		}

		httpClient = &http.Client{
			Timeout: t,
			Transport: &LoggingTransport{
				Transport: &http.Transport{
					Proxy:           fn,
					TLSClientConfig: tlsConfig,
				},
			},
		}

	} else if insecureSkipVerify {

		tlsConfig = &tls.Config{
			InsecureSkipVerify: true,
		}

		httpClient = &http.Client{
			Timeout: t,
			Transport: &LoggingTransport{
				Transport: &http.Transport{
					TLSClientConfig: tlsConfig,
				},
			},
		}

	} else {
		httpClient = &http.Client{
			Timeout: t,
			Transport: &LoggingTransport{
				Transport: &http.Transport{},
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

type LoggingTransport struct {
	Transport http.RoundTripper
}

func (t *LoggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Log the request
	dumpReq, err := httputil.DumpRequestOut(req, true)
	if err != nil {
		klog.V(2).Infof("Failed to dump request: %v", err)
	} else {
		klog.V(2).Infof("Request: %s", dumpReq)
	}

	resp, err := t.Transport.RoundTrip(req)

	fmt.Printf("Response: %v", resp)

	// Log the response
	if err != nil {
		klog.V(2).Infof("Request failed: %v", err)
	} else {
		dumpResp, err := httputil.DumpResponse(resp, true)
		if err != nil {
			klog.V(2).Infof("Failed to dump response: %v", err)
		} else {
			klog.V(2).Infof("Response: %s", dumpResp)
		}
	}

	return resp, err
}

func responseToOutput(response *http.Response, err error) ([]byte, error) {
	output := make(map[string]interface{})
	if err != nil {
		output["error"] = HTTPError{
			Message: err.Error(),
		}
	} else {
		body, err := io.ReadAll(response.Body)
		if err != nil {
			return nil, err
		}

		headers := make(map[string]string)
		for k, v := range response.Header {
			headers[k] = strings.Join(v, ",")
		}

		var rawJSON json.RawMessage
		if err := json.Unmarshal(body, &rawJSON); err != nil {
			klog.Infof("failed to unmarshal response body as JSON: %v", err)
			rawJSON = json.RawMessage{}
		}

		output["response"] = HTTPResponse{
			Status:  response.StatusCode,
			Body:    string(body),
			Headers: headers,
			RawJSON: rawJSON,
		}
	}

	b, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return nil, err
	}

	return b, nil
}

// parseTimeout parses a string into a time.Duration.
// If the string is empty, it returns 0.
func parseTimeout(s string) (time.Duration, error) {
	var timeout time.Duration
	var err error
	if s == "" {
		timeout = 0
	} else {
		timeout, err = time.ParseDuration(s)
		if err != nil {
			return 0, err
		}
	}

	if timeout < 0 {
		return 0, errors.New("timeout must be a positive duration")
	}

	return timeout, nil
}
