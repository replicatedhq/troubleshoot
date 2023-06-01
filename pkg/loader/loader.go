package loader

import (
	"strings"

	"github.com/pkg/errors"
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

type TroubleshootV1beta2Kinds struct {
	Analyzers        []troubleshootv1beta2.Analyzer
	Collectors       []troubleshootv1beta2.Collector
	HostCollectors   []troubleshootv1beta2.HostCollector
	HostPreflights   []troubleshootv1beta2.HostPreflight
	Preflights       []troubleshootv1beta2.Preflight
	Redactors        []troubleshootv1beta2.Redactor
	RemoteCollectors []troubleshootv1beta2.RemoteCollector
	SupportBundles   []troubleshootv1beta2.SupportBundle
}

func newTroubleshootV1beta2Kinds() *TroubleshootV1beta2Kinds {
	return &TroubleshootV1beta2Kinds{}
}

// LoadFromBytes takes a list of bytes and returns a TroubleshootV1beta2Kinds object
// Under the hood, this function will convert the bytes to strings and call LoadFromStrings.
func LoadFromBytes(rawSpecs ...[]byte) (*TroubleshootV1beta2Kinds, error) {
	asStrings := []string{}
	for _, rawSpec := range rawSpecs {
		asStrings = append(asStrings, string(rawSpec))
	}

	return LoadFromStrings(asStrings...)
}

// LoadFromStrings takes a list of strings and returns a TroubleshootV1beta2Kinds object
// that contains all the parsed troubleshooting specs. This function accepts a list of strings (exploded)
// which need to be valid yaml documents. A string can be a multidoc yaml separated by "---" as well.
// This function will return an error if any of the documents are not valid yaml.
// If Secrets or ConfigMaps are found, they will be parsed and the support bundle, redactor or preflight
// spec will be extracted from them, else they will be ignored. Any other yaml documents will be ignored.
func LoadFromStrings(rawSpecs ...string) (*TroubleshootV1beta2Kinds, error) {
	splitdocs := []string{}
	multiRawDocs := []string{}

	// 1. First split documents by "---".
	// NOTE: If secrets have "---" in them i.e a multidoc, this logic will break
	for _, rawSpec := range rawSpecs {
		multiRawDocs = append(multiRawDocs, strings.Split(rawSpec, "\n---\n")...)
	}

	// 2. Go through each document to see if it is a configmap, secret or troubleshoot kind
	// For secrets and configmaps, extract support bundle, redactor or preflight specs
	// For troubleshoot kinds, pass them through
	for _, rawDoc := range multiRawDocs {
		var parsed parsedDoc

		err := yaml.Unmarshal([]byte(rawDoc), &parsed)
		if err != nil {
			return nil, types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES, errors.Wrap(err, "failed to parse yaml"))
		}

		if isConfigMap(parsed) || isSecret(parsed) {
			// Extract specs from configmap or secret
			obj, _, err := decoder.Decode([]byte(rawDoc), nil, nil)
			if err != nil {
				return nil, types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES,
					errors.Wrapf(err, "failed to decode raw spec: '%s'", string(rawDoc)),
				)
			}

			// 3. Extract the raw troubleshoot specs
			switch v := obj.(type) {
			case *v1.ConfigMap:
				spec, ok := v.Data[constants.SupportBundleKey]
				if ok {
					splitdocs = append(splitdocs, spec)
				}
				spec, ok = v.Data[constants.RedactorKey]
				if ok {
					splitdocs = append(splitdocs, spec)
				}
				spec, ok = v.Data[constants.PreflightKey]
				if ok {
					splitdocs = append(splitdocs, spec)
				}
			case *v1.Secret:
				specBytes, ok := v.Data[constants.SupportBundleKey]
				if ok {
					splitdocs = append(splitdocs, string(specBytes))
				}
				specBytes, ok = v.Data[constants.RedactorKey]
				if ok {
					splitdocs = append(splitdocs, string(specBytes))
				}
				specBytes, ok = v.Data[constants.PreflightKey]
				if ok {
					splitdocs = append(splitdocs, string(specBytes))
				}
				str, ok := v.StringData[constants.SupportBundleKey]
				if ok {
					splitdocs = append(splitdocs, str)
				}
				str, ok = v.StringData[constants.RedactorKey]
				if ok {
					splitdocs = append(splitdocs, str)
				}
				str, ok = v.StringData[constants.PreflightKey]
				if ok {
					splitdocs = append(splitdocs, str)
				}
			default:
				return nil, types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES, errors.Errorf("%T type is not a Secret or ConfigMap", v))
			}
		} else if parsed.APIVersion == "troubleshoot.sh/v1beta2" {
			// If it's not a configmap or secret, just append it to the splitdocs
			splitdocs = append(splitdocs, rawDoc)
		} else {
			klog.V(1).Infof("skip loading %q kind", parsed.Kind)
		}
	}

	// 4. Then load the specs into the kinds struct
	return loadFromSplitDocs(splitdocs)
}

func loadFromSplitDocs(splitdocs []string) (*TroubleshootV1beta2Kinds, error) {
	kinds := newTroubleshootV1beta2Kinds()

	for _, doc := range splitdocs {
		doc, err := docrewrite.ConvertToV1Beta2([]byte(doc))
		if err != nil {
			return nil, types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES, errors.Wrap(err, "failed to convert to v1beta2"))
		}

		obj, _, err := decoder.Decode([]byte(doc), nil, nil)
		if err != nil {
			return nil, types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES, errors.Wrapf(err, "failed to decode '%s'", doc))
		}

		switch spec := obj.(type) {
		case *troubleshootv1beta2.Analyzer:
			kinds.Analyzers = append(kinds.Analyzers, *spec)
		case *troubleshootv1beta2.Collector:
			kinds.Collectors = append(kinds.Collectors, *spec)
		case *troubleshootv1beta2.HostCollector:
			kinds.HostCollectors = append(kinds.HostCollectors, *spec)
		case *troubleshootv1beta2.HostPreflight:
			kinds.HostPreflights = append(kinds.HostPreflights, *spec)
		case *troubleshootv1beta2.Preflight:
			kinds.Preflights = append(kinds.Preflights, *spec)
		case *troubleshootv1beta2.Redactor:
			kinds.Redactors = append(kinds.Redactors, *spec)
		case *troubleshootv1beta2.RemoteCollector:
			kinds.RemoteCollectors = append(kinds.RemoteCollectors, *spec)
		case *troubleshootv1beta2.SupportBundle:
			kinds.SupportBundles = append(kinds.SupportBundles, *spec)
		default:
			return kinds, types.NewExitCodeError(constants.EXIT_CODE_SPEC_ISSUES, errors.Errorf("unknown troubleshoot kind %T", obj))
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
