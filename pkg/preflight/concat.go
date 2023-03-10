package preflight

import (
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

func ConcatPreflightSpec(target *troubleshootv1beta2.Preflight, source *troubleshootv1beta2.Preflight) *troubleshootv1beta2.Preflight {
	if source == nil {
		return target
	}
	var newSpec *troubleshootv1beta2.Preflight
	if target == nil {
		newSpec = source
	} else {
		newSpec = target.DeepCopy()
		newSpec.Spec.Collectors = append(newSpec.Spec.Collectors, source.Spec.Collectors...)
		newSpec.Spec.RemoteCollectors = append(newSpec.Spec.RemoteCollectors, source.Spec.RemoteCollectors...)
		newSpec.Spec.Analyzers = append(newSpec.Spec.Analyzers, source.Spec.Analyzers...)
	}
	return newSpec
}

func ConcatHostPreflightSpec(target *troubleshootv1beta2.HostPreflight, source *troubleshootv1beta2.HostPreflight) *troubleshootv1beta2.HostPreflight {
	if source == nil {
		return target
	}
	var newSpec *troubleshootv1beta2.HostPreflight
	if target == nil {
		newSpec = source
	} else {
		newSpec = target.DeepCopy()
		newSpec.Spec.Collectors = append(newSpec.Spec.Collectors, source.Spec.Collectors...)
		newSpec.Spec.RemoteCollectors = append(newSpec.Spec.RemoteCollectors, source.Spec.RemoteCollectors...)
		newSpec.Spec.Analyzers = append(newSpec.Spec.Analyzers, source.Spec.Analyzers...)
	}
	return newSpec
}
