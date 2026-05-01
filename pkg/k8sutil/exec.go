package k8sutil

import (
	"net/url"

	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/streaming/pkg/httpstream"
)

// NewFallbackExecutor creates an executor that tries WebSocket first and falls
// back to SPDY if the server does not support it. Use this in place of
// remotecommand.NewSPDYExecutor everywhere.
func NewFallbackExecutor(config *restclient.Config, u *url.URL) (remotecommand.Executor, error) {
	// WebSocket upgrade requires GET per RFC 6455; SPDY uses POST.
	wsExec, err := remotecommand.NewWebSocketExecutor(config, "GET", u.String())
	if err != nil {
		return nil, err
	}
	spdyExec, err := remotecommand.NewSPDYExecutor(config, "POST", u)
	if err != nil {
		return nil, err
	}
	return remotecommand.NewFallbackExecutor(wsExec, spdyExec, func(err error) bool {
		return httpstream.IsUpgradeFailure(err) || httpstream.IsHTTPSProxyError(err)
	})
}
