package cli

import (
	"fmt"
	"strings"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/spf13/cobra"
)

// HTTPCmd runs the http collector against a single endpoint and prints the
// native result JSON ({"response":{"status":...}} or {"error":{...}}).
func HTTPCmd() *cobra.Command {
	var (
		method   string
		url      string
		headers  map[string]string
		body     string
		timeout  string
		proxy    string
		insecure bool
		caCert   string
	)

	cmd := &cobra.Command{
		Use:   "http",
		Short: "Run the http collector against an endpoint",
		Long: `Run the http collector: issue an HTTP request and print the collector result as JSON.

The result contains the response status, body and headers, or an error object.

Examples:
  collect http --url http://myapp.default.svc:8080/healthz
  collect http --method POST --url https://api.internal/ping \
      --header 'Content-Type=application/json' --body '{"ping":true}'`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			var tls *troubleshootv1beta2.TLSParams
			if caCert != "" {
				tls = &troubleshootv1beta2.TLSParams{CACert: caCert}
			}

			spec := &troubleshootv1beta2.HTTP{
				CollectorMeta: troubleshootv1beta2.CollectorMeta{CollectorName: "http"},
			}
			switch strings.ToUpper(method) {
			case "GET":
				spec.Get = &troubleshootv1beta2.Get{URL: url, Headers: headers, Timeout: timeout, Proxy: proxy, InsecureSkipVerify: insecure, TLS: tls}
			case "POST":
				spec.Post = &troubleshootv1beta2.Post{URL: url, Headers: headers, Body: body, Timeout: timeout, Proxy: proxy, InsecureSkipVerify: insecure, TLS: tls}
			case "PUT":
				spec.Put = &troubleshootv1beta2.Put{URL: url, Headers: headers, Body: body, Timeout: timeout, Proxy: proxy, InsecureSkipVerify: insecure, TLS: tls}
			default:
				return fmt.Errorf("unsupported --method %q (use GET, POST, or PUT)", method)
			}

			c := &collect.CollectHTTP{Collector: spec}
			res, err := c.Collect(nil)
			if err != nil {
				return err
			}
			return printCollectorResult(res)
		},
	}

	f := cmd.Flags()
	f.StringVar(&url, "url", "", "request URL (required)")
	cmd.MarkFlagRequired("url")
	f.StringVar(&method, "method", "GET", "HTTP method: GET, POST, or PUT")
	f.StringToStringVar(&headers, "header", nil, "request header as key=value (repeatable)")
	f.StringVar(&body, "body", "", "request body (POST/PUT only)")
	f.StringVar(&timeout, "timeout", "", "request timeout, e.g. 15s (empty = no timeout)")
	f.StringVar(&proxy, "proxy", "", "proxy URL to use for the request")
	f.BoolVar(&insecure, "insecure-skip-verify", false, "do not verify the server's TLS certificate")
	f.StringVar(&caCert, "tls-cacert", "", "CA certificate to trust (PEM contents or a file path)")

	return cmd
}
