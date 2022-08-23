package v1beta2

func (s *SupportBundle) ConcatSpec(bundle *SupportBundle) {
	for _, v := range bundle.Spec.Collectors {
		s.Spec.Collectors = append(s.Spec.Collectors, v)
	}
	for _, v := range bundle.Spec.AfterCollection {
		s.Spec.AfterCollection = append(s.Spec.AfterCollection, v)
	}
	for _, v := range bundle.Spec.HostCollectors {
		s.Spec.HostCollectors = append(s.Spec.HostCollectors, v)
	}
	for _, v := range bundle.Spec.HostAnalyzers {
		s.Spec.HostAnalyzers = append(s.Spec.HostAnalyzers, v)
	}
	for _, v := range bundle.Spec.Analyzers {
		s.Spec.Analyzers = append(s.Spec.Analyzers, v)
	}
}
