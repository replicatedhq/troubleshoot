package collect

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

func DeterministicIDForCollector(collector *troubleshootv1beta2.Collect) string {
	unsafeID := ""

	if collector.ClusterInfo != nil {
		unsafeID = "cluster-info"
	}

	if collector.ClusterResources != nil {
		unsafeID = "cluster-resources"
	}

	if collector.Secret != nil {
		unsafeID = fmt.Sprintf("secret-%s-%s", collector.Secret.Namespace, collector.Secret.SecretName)
	}

	if collector.Logs != nil {
		unsafeID = fmt.Sprintf("logs-%s-%s", collector.Logs.Namespace, selectorToString(collector.Logs.Selector))
	}

	if collector.Run != nil {
		unsafeID = "run"
		if collector.Run.CollectorName != "" {
			unsafeID = fmt.Sprintf("%s-%s", unsafeID, strings.ToLower(collector.Run.CollectorName))
		}
	}

	if collector.Exec != nil {
		unsafeID = "exec"
		if collector.Exec.CollectorName != "" {
			unsafeID = fmt.Sprintf("%s-%s", unsafeID, strings.ToLower(collector.Exec.CollectorName))
		}
	}

	if collector.Copy != nil {
		unsafeID = fmt.Sprintf("copy-%s-%s", selectorToString(collector.Copy.Selector), pathToString(collector.Copy.ContainerPath))
	}

	if collector.HTTP != nil {
		unsafeID = "http"
		if collector.HTTP.CollectorName != "" {
			unsafeID = fmt.Sprintf("%s-%s", unsafeID, strings.ToLower(collector.HTTP.CollectorName))
		}
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

func marshalNonNil(obj interface{}) ([]byte, error) {
	if obj == nil {
		return nil, nil
	}

	val := reflect.ValueOf(obj)
	switch val.Kind() {
	case reflect.Array, reflect.Slice, reflect.Map:
		if val.Len() == 0 {
			return nil, nil
		}
	}

	return json.MarshalIndent(obj, "", "  ")
}
