package loader

import (
	"context"

	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/internal/util"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/client/troubleshootclientset/scheme"
	"github.com/replicatedhq/troubleshoot/pkg/constants"
	"github.com/replicatedhq/troubleshoot/pkg/docrewrite"
	"github.com/replicatedhq/troubleshoot/pkg/types"
	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
)

var decoder runtime.Decoder

func init() {
	// Allow serializing Secrets and ConfigMaps
	_ = v1.AddToScheme(scheme.Scheme)
	decoder = scheme.Codecs.UniversalDeserializer()
}

type parsedDoc struct {
	Kind       string            `json:"kind" yaml:"kind"`
	APIVersion string            `json:"apiVersion" yaml:"apiVersion"`
	Data       map[string]any    `json:"data" yaml:"data"`
	StringData map[string]string `json:"stringData" yaml:"stringData"`
}

type LoadOptions struct {
	RawSpecs []string
	RawSpec  string

	// If true, the loader will return an error if any of the specs are not valid
	// else the invalid specs will be ignored
	Strict bool
}

// LoadSpecs takes sources to load specs from and returns a TroubleshootKinds object
// that contains all the parsed troubleshoot specs.
//
// The fetched specs should be yaml documents. The documents can be multidoc yamls
// separated by "---" which get split and parsed one at a time. All troubleshoot
// specs are extracted from the documents and returned in a TroubleshootKinds object.
//
// If Secrets or ConfigMaps are found, they are parsed and the support bundle, redactor
// or preflight spec extracted from them. All other yaml documents will be ignored.
//
// If the `Strict` flag is set to true, this function will return an error if any of
// the documents are not valid, else the invalid documents will be ignored.
func LoadSpecs(ctx context.Context, opt LoadOptions) (*TroubleshootKinds, error) {
	opt.RawSpecs = append(opt.RawSpecs, opt.RawSpec)
	l := specLoader{
		strict: opt.Strict,
	}
	return l.loadFromStrings(opt.RawSpecs...)
}

type specLoader struct {
	strict bool
}

type TroubleshootKinds struct {
	AnalyzersV1Beta2        []troubleshootv1beta2.Analyzer
	CollectorsV1Beta2       []troubleshootv1beta2.Collector
	HostCollectorsV1Beta2   []troubleshootv1beta2.HostCollector
	HostPreflightsV1Beta2   []troubleshootv1beta2.HostPreflight
	PreflightsV1Beta2       []troubleshootv1beta2.Preflight
	RedactorsV1Beta2        []troubleshootv1beta2.Redactor
	RemoteCollectorsV1Beta2 []troubleshootv1beta2.RemoteCollector
	SupportBundlesV1Beta2   []troubleshootv1beta2.SupportBundle
}

func (kinds *TroubleshootKinds) IsEmpty() bool {
	return len(kinds.AnalyzersV1Beta2) == 0 &&
		len(kinds.CollectorsV1Beta2) == 0 &&
		len(kinds.HostCollectorsV1Beta2) == 0 &&
		len(kinds.HostPreflightsV1Beta2) == 0 &&
		len(kinds.PreflightsV1Beta2) == 0 &&
		len(kinds.RedactorsV1Beta2) == 0 &&
		len(kinds.RemoteCollectorsV1Beta2) == 0 &&
		len(kinds.SupportBundlesV1Beta2) == 0
}

func NewTroubleshootKinds() *TroubleshootKinds {
	return &TroubleshootKinds{}
}

// loadFromStrings accepts a list of strings (exploded) which should be yaml documents
func (l *specLoader) loadFromStrings(rawSpecs ...string) (*TroubleshootKinds, error) {
	splitdocs := []string{}
	multiRawDocs := []string{}

	// 1. First split multidoc yaml documents.
	for _, rawSpec := range rawSpecs {
		multiRawDocs = append(multiRawDocs, util.SplitYAML(rawSpec)...)
	}

	// 2. Go through each document to see if it is a configmap, secret or troubleshoot kind
	// For secrets and configmaps, extract support bundle, redactor or preflight specs
	// For troubleshoot kinds, pass them through
	for _, rawDoc := range multiRawDocs {
		var parsed parsedDoc

		err := yaml.Unmarshal([]byte(rawDoc), &parsed)
		if err != nil {
			if !l.strict {
				continue
			}
			return nil, types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES, errors.Wrapf(err, "failed to parse yaml: '%s'", string(rawDoc)))
		}

		if isConfigMap(parsed) || isSecret(parsed) {
			// Extract specs from configmap or secret
			obj, _, err := decoder.Decode([]byte(rawDoc), nil, nil)
			if err != nil {
				if !l.strict {
					continue
				}
				return nil, types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES,
					errors.Wrapf(err, "failed to decode raw spec: '%s'", string(rawDoc)),
				)
			}

			// 3. Extract the raw troubleshoot specs
			switch v := obj.(type) {
			case *v1.ConfigMap:
				specs, err := l.getSpecFromConfigMap(v)
				if err != nil {
					return nil, types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES, err)
				}
				splitdocs = append(splitdocs, specs...)
			case *v1.Secret:
				specs, err := l.getSpecFromSecret(v)
				if err != nil {
					return nil, types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES, err)
				}
				splitdocs = append(splitdocs, specs...)
			default:
				return nil, types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES, errors.Errorf("%T type is not a Secret or ConfigMap", v))
			}
		} else if parsed.APIVersion == constants.Troubleshootv1beta2Kind {
			// If it's not a configmap or secret, just append it to the splitdocs
			splitdocs = append(splitdocs, rawDoc)
		} else {
			klog.V(1).Infof("skip loading %q kind", parsed.Kind)
		}
	}

	// 4. Then load the specs into the kinds struct
	return l.loadFromSplitDocs(splitdocs)
}

func (l *specLoader) loadFromSplitDocs(splitdocs []string) (*TroubleshootKinds, error) {
	kinds := NewTroubleshootKinds()

	for _, doc := range splitdocs {
		converted, err := docrewrite.ConvertToV1Beta2([]byte(doc))
		if err != nil {
			if !l.strict {
				continue
			}
			return nil, types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES,
				errors.Wrapf(err, "failed to convert doc to troubleshoot.sh/v1beta2 kind: '\n%s'", doc),
			)
		}

		obj, _, err := decoder.Decode([]byte(converted), nil, nil)
		if err != nil {
			if !l.strict {
				continue
			}
			return nil, types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES,
				errors.Wrapf(err, "failed to decode '%s'", converted),
			)
		}

		switch spec := obj.(type) {
		case *troubleshootv1beta2.Analyzer:
			kinds.AnalyzersV1Beta2 = append(kinds.AnalyzersV1Beta2, *spec)
		case *troubleshootv1beta2.Collector:
			kinds.CollectorsV1Beta2 = append(kinds.CollectorsV1Beta2, *spec)
		case *troubleshootv1beta2.HostCollector:
			kinds.HostCollectorsV1Beta2 = append(kinds.HostCollectorsV1Beta2, *spec)
		case *troubleshootv1beta2.HostPreflight:
			kinds.HostPreflightsV1Beta2 = append(kinds.HostPreflightsV1Beta2, *spec)
		case *troubleshootv1beta2.Preflight:
			kinds.PreflightsV1Beta2 = append(kinds.PreflightsV1Beta2, *spec)
		case *troubleshootv1beta2.Redactor:
			kinds.RedactorsV1Beta2 = append(kinds.RedactorsV1Beta2, *spec)
		case *troubleshootv1beta2.RemoteCollector:
			kinds.RemoteCollectorsV1Beta2 = append(kinds.RemoteCollectorsV1Beta2, *spec)
		case *troubleshootv1beta2.SupportBundle:
			kinds.SupportBundlesV1Beta2 = append(kinds.SupportBundlesV1Beta2, *spec)
		default:
			return nil, types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES, errors.Errorf("unknown troubleshoot kind %T", obj))
		}
	}

	klog.V(1).Info("loaded troubleshoot specs successfully")
	return kinds, nil
}

func isSecret(parsedDocHead parsedDoc) bool {
	if parsedDocHead.Kind == "Secret" && parsedDocHead.APIVersion == "v1" {
		return true
	}

	return false
}

func isConfigMap(parsedDocHead parsedDoc) bool {
	if parsedDocHead.Kind == "ConfigMap" && parsedDocHead.APIVersion == "v1" {
		return true
	}

	return false
}

// getSpecFromConfigMap extracts multiple troubleshoot specs from a secret
func (l *specLoader) getSpecFromConfigMap(cm *v1.ConfigMap) ([]string, error) {
	specs := []string{}

	str, ok := cm.Data[constants.SupportBundleKey]
	if ok {
		specs = append(specs, util.SplitYAML(str)...)
	}
	str, ok = cm.Data[constants.RedactorKey]
	if ok {
		specs = append(specs, util.SplitYAML(str)...)
	}
	str, ok = cm.Data[constants.PreflightKey]
	if ok {
		specs = append(specs, util.SplitYAML(str)...)
	}

	return specs, nil
}

// getSpecFromSecret extracts multiple troubleshoot specs from a secret
func (l *specLoader) getSpecFromSecret(secret *v1.Secret) ([]string, error) {
	specs := []string{}

	specBytes, ok := secret.Data[constants.SupportBundleKey]
	if ok {
		specs = append(specs, util.SplitYAML(string(specBytes))...)
	}
	specBytes, ok = secret.Data[constants.RedactorKey]
	if ok {
		specs = append(specs, util.SplitYAML(string(specBytes))...)
	}
	specBytes, ok = secret.Data[constants.PreflightKey]
	if ok {
		specs = append(specs, util.SplitYAML(string(specBytes))...)
	}
	str, ok := secret.StringData[constants.SupportBundleKey]
	if ok {
		specs = append(specs, util.SplitYAML(str)...)
	}
	str, ok = secret.StringData[constants.RedactorKey]
	if ok {
		specs = append(specs, util.SplitYAML(str)...)
	}
	str, ok = secret.StringData[constants.PreflightKey]
	if ok {
		specs = append(specs, util.SplitYAML(str)...)
	}
	return specs, nil
}
