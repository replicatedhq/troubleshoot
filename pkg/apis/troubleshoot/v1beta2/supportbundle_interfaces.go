package v1beta2

func (s *SupportBundle) ConcatSpec(bundle *SupportBundle) {
	for _, v := range bundle.Spec.Collectors {
		s.Spec.Collectors = append(bundle.Spec.Collectors, v)
	}
	for _, v := range bundle.Spec.AfterCollection {
		s.Spec.AfterCollection = append(bundle.Spec.AfterCollection, v)
	}
	for _, v := range bundle.Spec.HostCollectors {
		s.Spec.HostCollectors = append(bundle.Spec.HostCollectors, v)
	}
	for _, v := range bundle.Spec.HostAnalyzers {
		s.Spec.HostAnalyzers = append(bundle.Spec.HostAnalyzers, v)
	}
	for _, v := range bundle.Spec.Analyzers {
		s.Spec.Analyzers = append(bundle.Spec.Analyzers, v)
	}
}
