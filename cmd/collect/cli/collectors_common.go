// Package cli implements the `collect` command and its subcommands.
//
// The per-collector subcommands (http, postgres, mysql, mssql, redis) run a
// single collector and print its native result JSON to stdout. This lets a
// collector run inside a Pod using the troubleshoot image — e.g. as a runPod
// collector — so the check executes from within the cluster instead of from
// wherever the CLI happens to be invoked.
package cli

import (
	"fmt"

	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/replicatedhq/troubleshoot/pkg/constants"
	"github.com/replicatedhq/troubleshoot/pkg/k8sutil"
	"github.com/replicatedhq/troubleshoot/pkg/version"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// printCollectorResult writes the native collector result JSON to stdout.
// The single-collector subcommands run with an empty BundlePath, so the result
// is held in memory (map value = the JSON bytes) rather than written to disk.
func printCollectorResult(res collect.CollectorResult) error {
	for _, b := range res {
		fmt.Println(string(b))
	}
	return nil
}

// k8sClientForCollectors builds a Kubernetes client from the ambient kubeconfig
// or in-cluster config. Only the collectors that resolve TLS material from a
// Secret need this; plain connections and inline/file TLS do not.
func k8sClientForCollectors() (kubernetes.Interface, *rest.Config, error) {
	cfg, err := k8sutil.GetRESTConfig()
	if err != nil {
		return nil, nil, err
	}
	cfg.QPS = constants.DEFAULT_CLIENT_QPS
	cfg.Burst = constants.DEFAULT_CLIENT_BURST
	cfg.UserAgent = fmt.Sprintf("%s/%s", constants.DEFAULT_CLIENT_USER_AGENT, version.Version())
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, nil, err
	}
	return client, cfg, nil
}
