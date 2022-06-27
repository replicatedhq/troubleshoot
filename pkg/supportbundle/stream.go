package supportbundle

import (
	"context"
	"fmt"
	"sync"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"k8s.io/client-go/kubernetes"
)

func StreamSupportBundleFromSpec(spec *troubleshootv1beta2.SupportBundleSpec, additionalRedactors *troubleshootv1beta2.Redactor, opts SupportBundleCreateOpts) error {
	if opts.KubernetesRestConfig == nil {
		return errors.New("did not receive kube rest config")
	}

	collectSpecs := make([]*troubleshootv1beta2.Collect, 0)
	collectSpecs = append(collectSpecs, spec.Collectors...)
	collectSpecs = ensureCollectorInList(collectSpecs, troubleshootv1beta2.Collect{ClusterInfo: &troubleshootv1beta2.ClusterInfo{}})
	collectSpecs = ensureCollectorInList(collectSpecs, troubleshootv1beta2.Collect{ClusterResources: &troubleshootv1beta2.ClusterResources{}})

	var cleanedCollectors collect.Collectors
	for _, desiredCollector := range collectSpecs {
		collector := collect.Collector{
			Redact:       opts.Redact,
			Collect:      desiredCollector,
			ClientConfig: opts.KubernetesRestConfig,
			Namespace:    opts.Namespace,
		}
		cleanedCollectors = append(cleanedCollectors, &collector)
	}

	k8sClient, err := kubernetes.NewForConfig(opts.KubernetesRestConfig)
	if err != nil {
		return errors.Wrap(err, "failed to instantiate kuberentes client")
	}

	if err := cleanedCollectors.CheckRBAC(context.Background()); err != nil {
		return errors.Wrap(err, "failed to check RBAC for collectors")
	}

	foundForbidden := false
	for _, c := range cleanedCollectors {
		for _, e := range c.RBACErrors {
			foundForbidden = true
			if opts.ProgressChan != nil {
				opts.ProgressChan <- e
			}
		}
	}

	if foundForbidden && !opts.CollectWithoutPermissions {
		return errors.New("insufficient permissions to run all collectors")
	}

	globalRedactors := []*troubleshootv1beta2.Redact{}
	if additionalRedactors != nil {
		globalRedactors = additionalRedactors.Spec.Redactors
	}

	if opts.SinceTime != nil {
		applyLogSinceTime(*opts.SinceTime, &cleanedCollectors)
	}

	var wg sync.WaitGroup

	for _, cleanedCollector := range cleanedCollectors {
		if len(cleanedCollector.RBACErrors) > 0 {
			opts.CollectorProgressCallback(opts.ProgressChan, fmt.Sprintf("skipping collector %s with insufficient RBAC permissions", cleanedCollector.GetDisplayName()))
			continue
		}

		wg.Add(1)
		go func(c *collect.Collector, wg *sync.WaitGroup) {
			defer wg.Done()

			// start collecting and streaming
			if err := c.CollectAndStream(opts.KubernetesRestConfig, k8sClient, globalRedactors); err != nil {
				opts.CollectorProgressCallback(opts.ProgressChan, fmt.Sprintf("failed to collect and stream %s: %s", c.GetDisplayName(), err.Error()))
			}
		}(cleanedCollector, &wg)
	}

	wg.Wait()
	return nil
}
