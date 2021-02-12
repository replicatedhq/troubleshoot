package collect

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/segmentio/ksuid"
)

func HostHTTPLoadBalancer(c *HostCollector) (map[string][]byte, error) {
	listenAddress := fmt.Sprintf("0.0.0.0:%d", c.Collect.HTTPLoadBalancer.Port)

	timeout := 60 * time.Minute
	if c.Collect.HTTPLoadBalancer.Timeout != "" {
		var err error
		timeout, err = time.ParseDuration(c.Collect.HTTPLoadBalancer.Timeout)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse timeout %q", c.Collect.HTTPLoadBalancer.Timeout)
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
			log.Println(err.Error())
			networkStatus = NetworkStatusErrorOther
			break
		}
		if time.Now().After(stopAfter) {
			break
		}

		networkStatus = attemptPOST(c.Collect.HTTPLoadBalancer.Address, requestToken, responseToken)

		if networkStatus == NetworkStatusErrorOther || networkStatus == NetworkStatusConnectionTimeout {
			time.Sleep(50 * time.Millisecond)
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

	name := path.Join("httpLoadBalancer", "httpLoadBalancer.json")
	if c.Collect.HTTPLoadBalancer.CollectorName != "" {
		name = path.Join("httpLoadBalancer", fmt.Sprintf("%s.json", c.Collect.HTTPLoadBalancer.CollectorName))
	}

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
		fmt.Println(err.Error())
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
