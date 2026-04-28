package k8sutil

import (
	"net/url"

	"k8s.io/apimachinery/pkg/util/httpstream"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

// NewFallbackExecutor creates an executor that tries WebSocket first and falls
// back to SPDY if the server does not support it. Use this in place of
// remotecommand.NewSPDYExecutor everywhere.
func NewFallbackExecutor(config *restclient.Config, method string, url *url.URL) (remotecommand.Executor, error) {
	wsExec, err := remotecommand.NewWebSocketExecutor(config, method, url.String())
	if err != nil {
		return nil, err
	}
	spdyExec, err := remotecommand.NewSPDYExecutor(config, method, url)
	if err != nil {
		return nil, err
	}
	return remotecommand.NewFallbackExecutor(wsExec, spdyExec, httpstream.IsUpgradeFailure)
}
