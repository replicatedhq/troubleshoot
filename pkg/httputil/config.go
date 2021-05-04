package httputil

import (
	"net/http"
)

var (
	httpTransport *http.Transport
	httpClient    = &http.Client{}
)

func AddTransport(transport *http.Transport) {
	httpTransport = transport
}

func GetHttpClient() *http.Client {

	if httpTransport != nil {
		httpClient.Transport = httpTransport
		return httpClient
	}

	return http.DefaultClient
}
