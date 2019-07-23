package collect

import (
	"encoding/json"
	"fmt"
	"reflect"
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
		unsafeID = fmt.Sprintf("run-%s", strings.ToLower(collector.Run.CollectorName))
	}

	if collector.Exec != nil {
		unsafeID = fmt.Sprintf("exec-%s", strings.ToLower(collector.Exec.CollectorName))
	}

	if collector.Copy != nil {
		unsafeID = fmt.Sprintf("copy-%s-%s", selectorToString(collector.Copy.Selector), pathToString(collector.Copy.ContainerPath))
	}

	if collector.HTTP != nil {
		unsafeID = fmt.Sprintf("http-%s", strings.ToLower(collector.HTTP.CollectorName))
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
