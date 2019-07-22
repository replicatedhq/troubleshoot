package collect

import (
	"fmt"
	"regexp"
	"strings"

	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
)

func DeterministicIDForCollector(collector *troubleshootv1beta1.Collect) string {
	unsafeID := ""

	if collector.ClusterInfo != nil {
		unsafeID = "cluster-info"
	}

	if collector.ClusterResources != nil {
		unsafeID = "cluster-resources"
	}

	if collector.Secret != nil {
		unsafeID = fmt.Sprintf("secret-%s-%s", collector.Secret.Namespace, collector.Secret.Name)
	}

	if collector.Logs != nil {
		unsafeID = fmt.Sprintf("logs-%s-%s", collector.Logs.Namespace, selectorToString(collector.Logs.Selector))
	}

	if collector.Run != nil {
		unsafeID = fmt.Sprintf("run-%s", strings.ToLower(collector.Run.Name))
	}

	if collector.Exec != nil {
		unsafeID = fmt.Sprintf("exec-%s", strings.ToLower(collector.Exec.Name))
	}

	if collector.Copy != nil {
		unsafeID = fmt.Sprintf("copy-%s-%s", selectorToString(collector.Copy.Selector), pathToString(collector.Copy.ContainerPath))
	}

	if collector.HTTP != nil {
		unsafeID = fmt.Sprintf("http-%s", strings.ToLower(collector.HTTP.Name))
	}

	return rfc1035(unsafeID)
}

func selectorToString(selector []string) string {
	return strings.Replace(strings.Join(selector, "-"), "=", "-", -1)
}

func pathToString(path string) string {
	return strings.Replace(path, "/", "-", -1)
}

func rfc1035(in string) string {
	reg := regexp.MustCompile("[^a-z0-9\\-]+")
	out := reg.ReplaceAllString(in, "-")

	if len(out) > 63 {
		out = out[:63]
	}

	return out
}
