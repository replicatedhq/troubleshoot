package preflight

import (
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

func ConcatPreflightSpec(target *troubleshootv1beta2.Preflight, source *troubleshootv1beta2.Preflight) *troubleshootv1beta2.Preflight {
	newSpec := target.DeepCopy()
	newSpec.Spec.Collectors = append(target.Spec.Collectors, source.Spec.Collectors...)
	newSpec.Spec.RemoteCollectors = append(target.Spec.RemoteCollectors, source.Spec.RemoteCollectors...)
	newSpec.Spec.Analyzers = append(target.Spec.Analyzers, source.Spec.Analyzers...)
	return newSpec
}

func ConcatHostPreflightSpec(target *troubleshootv1beta2.HostPreflight, source *troubleshootv1beta2.HostPreflight) *troubleshootv1beta2.HostPreflight {
	newSpec := target.DeepCopy()
	newSpec.Spec.Collectors = append(target.Spec.Collectors, source.Spec.Collectors...)
	newSpec.Spec.RemoteCollectors = append(target.Spec.RemoteCollectors, source.Spec.RemoteCollectors...)
	newSpec.Spec.Analyzers = append(target.Spec.Analyzers, source.Spec.Analyzers...)
	return newSpec
}
