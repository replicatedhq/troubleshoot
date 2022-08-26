package v1beta2

// the intention with these appends is to swap them out at a later date with more specific handlers for merging the spec fields
func (s *SupportBundle) ConcatSpec(bundle *SupportBundle) {

    s.Spec.MergeCollectors(&bundle.Spec)

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

func (s *SupportBundleSpec) MergeCollectors(spec *SupportBundleSpec) {
    for _,c := range spec.Collectors {

        if c.ClusterInfo != nil {
            // we only actually want one of these so skip if there's already one
            y := 0
            // we want to move away from checking for specific collectors in favor of allowing collectors to expose their own merge method
            for _,v := range s.Collectors{
                if v.ClusterInfo != nil {
                    y = 1
                }
            }
            if y != 1 {
                s.Collectors = append(s.Collectors, c)
            }
            continue
        }

        s.Collectors = append(s.Collectors, c)

    }
}

