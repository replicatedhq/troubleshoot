package oci

import (
	"github.com/replicatedhq/troubleshoot/pkg/oci/types"
)

const (
	preflightSpecMediaType     = "replicated.preflight.spec"
	supportBundleSpecMediaType = "replicated.supportbundle.spec"
	helmValuesMediaType        = "helm.values.yaml"
)

type PreflightLayers struct {
	spec   []byte
	values []byte
}

var _ types.Layers = &PreflightLayers{}

func (l *PreflightLayers) GetAllowedMediaTypes() []string {
	return []string{
		preflightSpecMediaType,
		helmValuesMediaType,
	}
}

func (l *PreflightLayers) GetSpec() []byte {
	return l.spec
}

func (l *PreflightLayers) GetValues() []byte {
	return l.values
}

func (l *PreflightLayers) SetLayer(medaiType string, data []byte) {
	switch medaiType {
	case preflightSpecMediaType:
		l.spec = data
	case helmValuesMediaType:
		l.values = data
	}
}

func (l *PreflightLayers) IsEmpty() bool {
	return len(l.spec) == 0 && len(l.values) == 0
}

type SupportBundleLayers struct {
	spec   []byte
	values []byte
}

var _ types.Layers = &SupportBundleLayers{}

func (l *SupportBundleLayers) GetAllowedMediaTypes() []string {
	return []string{
		supportBundleSpecMediaType,
		helmValuesMediaType,
	}
}

func (l *SupportBundleLayers) GetSpec() []byte {
	return l.spec
}

func (l *SupportBundleLayers) GetValues() []byte {
	return l.values
}

func (l *SupportBundleLayers) SetLayer(medaiType string, data []byte) {
	switch medaiType {
	case supportBundleSpecMediaType:
		l.spec = data
	case helmValuesMediaType:
		l.values = data
	}
}

func (l *SupportBundleLayers) IsEmpty() bool {
	return len(l.spec) == 0 && len(l.values) == 0
}
