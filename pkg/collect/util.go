package collect

import (
	"fmt"
	"strings"

	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
)

func DeterministicIDForCollector(collector *troubleshootv1beta1.Collect) string {
	if collector.ClusterInfo != nil {
		return "cluster-info"
	}
	if collector.ClusterResources != nil {
		return "cluster-resources"
	}
	if collector.Secret != nil {
		return fmt.Sprintf("secret-%s%s", collector.Secret.Namespace, collector.Secret.Name)
	}
	if collector.Logs != nil {
		randomString := "abcdef" // TODO
		return fmt.Sprintf("logs-%s%s", collector.Logs.Namespace, randomString)
	}
	if collector.Run != nil {
		return fmt.Sprintf("run-%s", strings.ToLower(collector.Run.Name))
	}
	return ""
}
