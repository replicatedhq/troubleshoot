package preflight

import (
	"context"

	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/internal/specs"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/k8sutil"
	"github.com/spf13/viper"
	"k8s.io/client-go/kubernetes"
)

type PreflightSpecs struct {
	PreflightSpec     *troubleshootv1beta2.Preflight
	HostPreflightSpec *troubleshootv1beta2.HostPreflight
	UploadResultSpecs []*troubleshootv1beta2.Preflight
}

func (p *PreflightSpecs) Read(args []string) error {
	config, err := k8sutil.GetRESTConfig()
	if err != nil {
		return errors.Wrap(err, "failed to convert kube flags to rest config")
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return errors.Wrap(err, "failed to convert create k8s client")
	}

	ctx := context.Background()
	kinds, err := specs.LoadFromCLIArgs(ctx, client, args, viper.GetViper())
	if err != nil {
		return err
	}

	for _, v := range kinds.PreflightsV1Beta2 {
		v := v // https://golang.org/doc/faq#closures_and_goroutines
		if v.Spec.UploadResultsTo == "" {
			p.PreflightSpec = ConcatPreflightSpec(p.PreflightSpec, &v)
		} else {
			p.UploadResultSpecs = append(p.UploadResultSpecs, &v)
		}
	}

	for _, v := range kinds.HostPreflightsV1Beta2 {
		v := v // https://golang.org/doc/faq#closures_and_goroutines
		p.HostPreflightSpec = ConcatHostPreflightSpec(p.HostPreflightSpec, &v)
	}

	return nil
}
