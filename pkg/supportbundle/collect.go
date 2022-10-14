package supportbundle

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/pkg/errors"
	analyze "github.com/replicatedhq/troubleshoot/pkg/analyze"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/replicatedhq/troubleshoot/pkg/convert"
	"github.com/replicatedhq/troubleshoot/pkg/version"
	"gopkg.in/yaml.v2"
	"k8s.io/client-go/kubernetes"
)

func runHostCollectors(hostCollectors []*troubleshootv1beta2.HostCollect, additionalRedactors *troubleshootv1beta2.Redactor, bundlePath string, opts SupportBundleCreateOpts) (collect.CollectorResult, error) {
	collectSpecs := make([]*troubleshootv1beta2.HostCollect, 0, 0)
	collectSpecs = append(collectSpecs, hostCollectors...)

	allCollectedData := make(map[string][]byte)

	var collectors []collect.HostCollector
	for _, desiredCollector := range collectSpecs {
		collector, ok := collect.GetHostCollector(desiredCollector, bundlePath)
		if ok {
			collectors = append(collectors, collector)
		}
	}

	collectResult := collect.NewResult()

	for _, collector := range collectors {
		isExcluded, _ := collector.IsExcluded()
		if isExcluded {
			continue
		}

		opts.ProgressChan <- fmt.Sprintf("[%s] Running host collector...", collector.Title())
		result, err := collector.Collect(opts.ProgressChan)
		if err != nil {
			opts.ProgressChan <- errors.Errorf("failed to run host collector: %s: %v", collector.Title(), err)
		}
		for k, v := range result {
			allCollectedData[k] = v
		}
	}

	collectResult = allCollectedData

	globalRedactors := []*troubleshootv1beta2.Redact{}
	if additionalRedactors != nil {
		globalRedactors = additionalRedactors.Spec.Redactors
	}

	if opts.Redact {
		err := collect.RedactResult(bundlePath, collectResult, globalRedactors)
		if err != nil {
			err = errors.Wrap(err, "failed to redact")
			return collectResult, err
		}
	}

	return collectResult, nil
}

func runCollectors(collectors []*troubleshootv1beta2.Collect, additionalRedactors *troubleshootv1beta2.Redactor, bundlePath string, opts SupportBundleCreateOpts) (collect.CollectorResult, error) {
	collectSpecs := make([]*troubleshootv1beta2.Collect, 0)
	collectSpecs = append(collectSpecs, collectors...)
	collectSpecs = ensureCollectorInList(collectSpecs, troubleshootv1beta2.Collect{ClusterInfo: &troubleshootv1beta2.ClusterInfo{}})
	collectSpecs = ensureCollectorInList(collectSpecs, troubleshootv1beta2.Collect{ClusterResources: &troubleshootv1beta2.ClusterResources{}})

	var allCollectors []collect.Collector

	allCollectedData := make(map[string][]byte)

	k8sClient, err := kubernetes.NewForConfig(opts.KubernetesRestConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to instantiate Kubernetes client")
	}

	for _, desiredCollector := range collectSpecs {
		if collectorInterface, ok := collect.GetCollector(desiredCollector, bundlePath, opts.Namespace, opts.KubernetesRestConfig, k8sClient, opts.SinceTime); ok {
			if collector, ok := collectorInterface.(collect.Collector); ok {
				err := collector.CheckRBAC(context.Background(), collector, desiredCollector, opts.KubernetesRestConfig, opts.Namespace)
				if err != nil {
					return nil, errors.Wrap(err, "failed to check RBAC for collectors")
				}

				if mergeCollector, ok := collectorInterface.(collect.MergeableCollector); ok {
					allCollectors, err = mergeCollector.Merge(allCollectors)
					if err != nil {
						msg := fmt.Sprintf("failed to merge collector: %s: %s", collector.Title(), err)
						opts.CollectorProgressCallback(opts.ProgressChan, msg)
					}
				} else {
					allCollectors = append(allCollectors, collector)
				}
			}
		}
	}

	foundForbidden := false
	for _, c := range allCollectors {
		for _, e := range c.GetRBACErrors() {
			foundForbidden = true
			opts.ProgressChan <- e
		}
	}

	if foundForbidden && !opts.CollectWithoutPermissions {
		return nil, errors.New("insufficient permissions to run all collectors")
	}

	for _, collector := range allCollectors {
		isExcluded, _ := collector.IsExcluded()
		if isExcluded {
			continue
		}

		// skip collectors with RBAC errors unless its the ClusterResources collector
		if collector.HasRBACErrors() {
			if _, ok := collector.(*collect.CollectClusterResources); !ok {
				msg := fmt.Sprintf("skipping collector %s with insufficient RBAC permissions", collector.Title())
				opts.CollectorProgressCallback(opts.ProgressChan, msg)
				continue
			}
		}

		opts.CollectorProgressCallback(opts.ProgressChan, collector.Title())
		result, err := collector.Collect(opts.ProgressChan)
		if err != nil {
			opts.ProgressChan <- errors.Errorf("failed to run collector: %s: %v", collector.Title(), err)
		}
		for k, v := range result {
			allCollectedData[k] = v
		}
	}

	collectResult := allCollectedData

	globalRedactors := []*troubleshootv1beta2.Redact{}
	if additionalRedactors != nil {
		globalRedactors = additionalRedactors.Spec.Redactors
	}

	if opts.Redact {
		err := collect.RedactResult(bundlePath, collectResult, globalRedactors)
		if err != nil {
			err = errors.Wrap(err, "failed to redact")
			return collectResult, err
		}
	}

	return collectResult, nil
}

func findFileName(basename, extension string) (string, error) {
	n := 1
	name := basename
	for {
		filename := name + "." + extension
		if _, err := os.Stat(filename); os.IsNotExist(err) {
			return filename, nil
		} else if err != nil {
			return "", errors.Wrap(err, "check file exists")
		}

		name = fmt.Sprintf("%s (%d)", basename, n)
		n = n + 1
	}
}

func ensureCollectorInList(list []*troubleshootv1beta2.Collect, collector troubleshootv1beta2.Collect) []*troubleshootv1beta2.Collect {
	for _, inList := range list {
		if collector.ClusterResources != nil && inList.ClusterResources != nil {
			return list
		}
		if collector.ClusterInfo != nil && inList.ClusterInfo != nil {
			return list
		}
	}

	return append(list, &collector)
}

const VersionFilename = "version.yaml"

func getVersionFile() (io.Reader, error) {
	version := troubleshootv1beta2.SupportBundleVersion{
		ApiVersion: "troubleshoot.sh/v1beta2",
		Kind:       "SupportBundle",
		Spec: troubleshootv1beta2.SupportBundleVersionSpec{
			VersionNumber: version.Version(),
		},
	}
	b, err := yaml.Marshal(version)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal version data")
	}

	return bytes.NewBuffer(b), nil
}

const AnalysisFilename = "analysis.json"

func getAnalysisFile(analyzeResults []*analyze.AnalyzeResult) (io.Reader, error) {
	data := convert.FromAnalyzerResult(analyzeResults)
	analysis, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal analysis")
	}

	return bytes.NewBuffer(analysis), nil
}
