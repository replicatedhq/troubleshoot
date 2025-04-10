package collect

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/debug"
	"github.com/segmentio/ksuid"
)

type CollectHostHTTPLoadBalancer struct {
	hostCollector *troubleshootv1beta2.HTTPLoadBalancer
	BundlePath    string
}

func (c *CollectHostHTTPLoadBalancer) Title() string {
	return hostCollectorTitleOrDefault(c.hostCollector.HostCollectorMeta, "HTTP Load Balancer")
}

func (c *CollectHostHTTPLoadBalancer) IsExcluded() (bool, error) {
	return isExcluded(c.hostCollector.Exclude)
}

func (c *CollectHostHTTPLoadBalancer) SkipRedaction() bool {
	return c.hostCollector.SkipRedaction
}

func (c *CollectHostHTTPLoadBalancer) Collect(progressChan chan<- interface{}) (map[string][]byte, error) {
	listenAddress := fmt.Sprintf("0.0.0.0:%d", c.hostCollector.Port)

	timeout := 60 * time.Minute
	if c.hostCollector.Timeout != "" {
		var err error
		timeout, err = time.ParseDuration(c.hostCollector.Timeout)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse timeout %q", c.hostCollector.Timeout)
		}
	}

	requestToken := ksuid.New().Bytes()
	responseToken := ksuid.New().Bytes()

	listenErr := make(chan error, 1)

	go func() {
		mux := http.NewServeMux()
		server := http.Server{
			Addr:    listenAddress,
			Handler: mux,
		}

		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "POST" {
				return
			}
			body, err := ioutil.ReadAll(r.Body)
			if err != nil {
				return
			}
			if !bytes.Equal(body, requestToken) {
				return
			}
			_, err = w.Write(responseToken)
			if err != nil {
				return
			}
			server.Shutdown(context.Background())
		})

		err := http.ListenAndServe(listenAddress, mux)
		if err != http.ErrServerClosed {
			listenErr <- err
		}
	}()

	var networkStatus NetworkStatus

	stopAfter := time.Now().Add(timeout)
	for {
		if len(listenErr) > 0 {
			err := <-listenErr
			if strings.Contains(err.Error(), "address already in use") {
				networkStatus = NetworkStatusAddressInUse
				break
			}
			if strings.Contains(err.Error(), "permission denied") {
				networkStatus = NetworkStatusBindPermissionDenied
				break
			}
			debug.Println(err.Error())
			networkStatus = NetworkStatusErrorOther
			break
		}
		if time.Now().After(stopAfter) {
			break
		}

		networkStatus = attemptPOST(c.hostCollector.Address, requestToken, responseToken)

		if networkStatus == NetworkStatusErrorOther || networkStatus == NetworkStatusConnectionTimeout {
			progressChan <- errors.Errorf("http post %s: network status %q", c.hostCollector.Address, networkStatus)
			time.Sleep(time.Second)
			continue
		}

		break
	}

	result := NetworkStatusResult{
		Status: networkStatus,
	}

	b, err := json.Marshal(result)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal result")
	}

	collectorName := c.hostCollector.CollectorName
	if collectorName == "" {
		collectorName = "httpLoadBalancer"
	}
	name := filepath.Join("host-collectors/httpLoadBalancer", collectorName+".json")

	output := NewResult()
	output.SaveResult(c.BundlePath, name, bytes.NewBuffer(b))

	return map[string][]byte{
		name: b,
	}, nil
}

func attemptPOST(address string, request []byte, response []byte) NetworkStatus {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Create a new transport every time to ensure a new TCP connection so the load balancer does
	// not forward every request to the same backend
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   50 * time.Millisecond,
			DualStack: true,
		}).DialContext,
		ForceAttemptHTTP2: true,
	}
	client := http.Client{
		Transport: transport,
	}

	buf := bytes.NewBuffer(request)
	req, err := http.NewRequestWithContext(ctx, "POST", address, buf)
	if err != nil {
		debug.Println(err.Error())
		return NetworkStatusErrorOther
	}

	resp, err := client.Do(req)
	if err != nil {
		if strings.Contains(err.Error(), "connection refused") {
			return NetworkStatusConnectionRefused
		}
		if strings.Contains(err.Error(), "i/o timeout") {
			return NetworkStatusConnectionTimeout
		}

		return NetworkStatusErrorOther
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return NetworkStatusErrorOther
	}
	if !bytes.Equal(body, response) {
		return NetworkStatusErrorOther
	}

	return NetworkStatusConnected
}

func (c *CollectHostHTTPLoadBalancer) RemoteCollect(progressChan chan<- interface{}) (map[string][]byte, error) {
	return nil, ErrRemoteCollectorNotImplemented
}
