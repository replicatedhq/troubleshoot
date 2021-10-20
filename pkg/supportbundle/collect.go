package supportbundle

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/pkg/errors"
	analyze "github.com/replicatedhq/troubleshoot/pkg/analyze"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/replicatedhq/troubleshoot/pkg/convert"
	"github.com/replicatedhq/troubleshoot/pkg/version"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// TODO (dan): This is VERY similar to the Preflight collect package and should be refactored.
func runCollectors(collectors []*troubleshootv1beta2.Collect, additionalRedactors *troubleshootv1beta2.Redactor, bundlePath string, opts SupportBundleCreateOpts) (collect.CollectorResult, error) {

	collectSpecs := make([]*troubleshootv1beta2.Collect, 0)
	collectSpecs = append(collectSpecs, collectors...)
	collectSpecs = ensureCollectorInList(collectSpecs, troubleshootv1beta2.Collect{ClusterInfo: &troubleshootv1beta2.ClusterInfo{}})
	collectSpecs = ensureCollectorInList(collectSpecs, troubleshootv1beta2.Collect{ClusterResources: &troubleshootv1beta2.ClusterResources{}})

	var cleanedCollectors collect.Collectors
	for _, desiredCollector := range collectSpecs {
		collector := collect.Collector{
			Redact:       opts.Redact,
			Collect:      desiredCollector,
			ClientConfig: opts.KubernetesRestConfig,
			Namespace:    opts.Namespace,
			BundlePath:   bundlePath,
		}
		cleanedCollectors = append(cleanedCollectors, &collector)
	}

	k8sClient, err := kubernetes.NewForConfig(opts.KubernetesRestConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to instantiate kuberentes client")
	}

	if err := cleanedCollectors.CheckRBAC(context.Background()); err != nil {
		return nil, errors.Wrap(err, "failed to check RBAC for collectors")
	}

	foundForbidden := false
	for _, c := range cleanedCollectors {
		for _, e := range c.RBACErrors {
			foundForbidden = true
			opts.ProgressChan <- e
		}
	}

	if foundForbidden && !opts.CollectWithoutPermissions {
		return nil, errors.New("insufficient permissions to run all collectors")
	}

	globalRedactors := []*troubleshootv1beta2.Redact{}
	if additionalRedactors != nil {
		globalRedactors = additionalRedactors.Spec.Redactors
	}

	if opts.SinceTime != nil {
		applyLogSinceTime(*opts.SinceTime, &cleanedCollectors)
	}

	result := collect.NewResult()

	// Run preflights collectors synchronously
	for _, collector := range cleanedCollectors {
		if len(collector.RBACErrors) > 0 {
			// don't skip clusterResources collector due to RBAC issues
			if collector.Collect.ClusterResources == nil {
				msg := fmt.Sprintf("skipping collector %s with insufficient RBAC permissions", collector.GetDisplayName())
				opts.CollectorProgressCallback(opts.ProgressChan, msg)
				continue
			}
		}

		opts.CollectorProgressCallback(opts.ProgressChan, collector.GetDisplayName())

		files, err := collector.RunCollectorSync(opts.KubernetesRestConfig, k8sClient, globalRedactors)
		if err != nil {
			opts.ProgressChan <- fmt.Errorf("failed to run collector %q: %v", collector.GetDisplayName(), err)
			continue
		}

		for k, v := range files {
			result[k] = v
		}
	}

	return result, nil
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

func applyLogSinceTime(sinceTime time.Time, collectors *collect.Collectors) {

	for _, collector := range *collectors {
		if collector.Collect.Logs != nil {
			if collector.Collect.Logs.Limits == nil {
				collector.Collect.Logs.Limits = new(troubleshootv1beta2.LogLimits)
			}
			collector.Collect.Logs.Limits.SinceTime = metav1.NewTime(sinceTime)
		}
	}
}
